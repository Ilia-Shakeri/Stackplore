// Package demo provides deterministic synthetic infrastructure for evaluation and tests.
package demo

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const (
	SourceID       domain.SourceID      = "source_demo"
	EnvironmentID  domain.EnvironmentID = "env_demo_shop"
	FixtureVersion                      = "demo-v1"
)

var observedAt = time.Date(2026, time.August, 16, 8, 0, 0, 0, time.UTC)

type Provider struct {
	source        domain.InfrastructureSource
	environment   domain.Environment
	resources     []domain.ResourceSnapshot
	health        map[domain.ResourceID]domain.HealthSnapshot
	relationships []domain.Relationship
	logs          []domain.LogRecord
	events        []domain.InfrastructureEvent
	alerts        []domain.Alert
}

func New() (*Provider, error) {
	status := availableStatus()
	provider := &Provider{
		source: domain.InfrastructureSource{ID: SourceID, Provider: domain.ProviderDemo, Name: "Demo shop", Synthetic: true},
		environment: domain.Environment{
			ID: EnvironmentID, SourceID: SourceID, Provider: domain.ProviderDemo, Name: "Demo production", Status: status, Synthetic: true,
		},
		health: make(map[domain.ResourceID]domain.HealthSnapshot),
	}
	definitions := []struct {
		name, kind, nativeID string
		health               domain.HealthState
	}{
		{"API", "demo.service", "api", domain.HealthHealthy},
		{"Checkout", "demo.service", "checkout", domain.HealthUnhealthy},
		{"Gateway", "demo.gateway", "gateway", domain.HealthDegraded},
		{"PostgreSQL", "demo.database", "postgres", domain.HealthHealthy},
		{"Redis", "demo.cache", "redis", domain.HealthHealthy},
		{"Worker", "demo.worker", "worker", domain.HealthHealthy},
	}
	for _, definition := range definitions {
		identity, err := domain.NewResourceIdentity(
			domain.ProviderDemo, SourceID, EnvironmentID, domain.ResourceKind(definition.kind),
			domain.Scope{Kind: domain.ScopeProject, Name: "demo-shop"}, domain.NativeResourceID(definition.nativeID),
		)
		if err != nil {
			return nil, err
		}
		ref, err := domain.NewResourceRef(identity)
		if err != nil {
			return nil, err
		}
		provider.resources = append(provider.resources, domain.ResourceSnapshot{
			Resource: domain.Resource{Ref: ref, DisplayName: definition.name, Synthetic: true}, Status: status,
		})
		healthStatus := status
		evidenceStatus := domain.EvidencePositive
		code := domain.ReasonCode("demo.health.ready")
		if definition.health == domain.HealthDegraded {
			evidenceStatus = domain.EvidenceUnknown
			code = "demo.health.latency"
		}
		if definition.health == domain.HealthUnhealthy {
			evidenceStatus = domain.EvidenceNegative
			code = "demo.health.failure"
		}
		provider.health[ref.ID] = domain.HealthSnapshot{
			ResourceID: ref.ID, State: definition.health, Status: healthStatus,
			Evidence: []domain.HealthEvidence{{
				ID: domain.EvidenceID("evidence_" + definition.nativeID), Code: code,
				SourceCode: code, Status: evidenceStatus, ObservedAt: observedAt, Synthetic: true,
			}},
		}
	}
	if err := seedObservability(provider); err != nil {
		return nil, err
	}
	if err := provider.validate(); err != nil {
		return nil, err
	}
	return provider, nil
}

func MustNew() *Provider {
	provider, err := New()
	if err != nil {
		panic("invalid demo fixture: " + err.Error())
	}
	return provider
}

func (provider *Provider) Source() domain.InfrastructureSource {
	return provider.source
}

func (provider *Provider) FixtureVersion() string {
	return FixtureVersion
}

func (provider *Provider) ObservationTime() time.Time {
	return observedAt
}

func ObservationTime() time.Time {
	return observedAt
}

func (provider *Provider) CaptureInventory(ctx context.Context) (domain.InventoryCapture, error) {
	ref := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: SourceID}
	if err := provider.validateContext(ctx, ref); err != nil {
		return domain.InventoryCapture{}, invalid("demo.inventory.capture", err)
	}
	capture := domain.InventoryCapture{
		Source: ref, Status: availableStatus(), Environments: []domain.Environment{provider.environment},
		Resources: slices.Clone(provider.resources), Relationships: slices.Clone(provider.relationships), Complete: true,
		Health: make([]domain.HealthSnapshot, 0, len(provider.health)),
	}
	for _, resource := range provider.resources {
		capture.Health = append(capture.Health, provider.health[resource.Resource.Ref.ID])
	}
	if err := capture.ValidateFor(ref); err != nil {
		return domain.InventoryCapture{}, invalid("demo.inventory.capture", err)
	}
	return capture, nil
}

func (provider *Provider) ListEnvironments(ctx context.Context, query domain.EnvironmentQuery) (domain.EnvironmentPage, error) {
	if err := provider.validateContext(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.EnvironmentPage{}, invalid("demo.inventory.environments", err)
	}
	offset, err := decodeCursor(query.Page.Cursor, environmentFingerprint(query))
	if err != nil || offset > 1 {
		return domain.EnvironmentPage{}, invalid("demo.inventory.environments", err)
	}
	items := []domain.Environment{provider.environment}
	pageItems, page := paginate(items, offset, query.Page.Limit, environmentFingerprint(query))
	result := domain.EnvironmentPage{Status: availableStatus(), Environments: pageItems, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.EnvironmentPage{}, invalid("demo.inventory.environments", err)
	}
	return result, nil
}

func (provider *Provider) ListResources(ctx context.Context, query domain.InventoryQuery) (domain.ResourcePage, error) {
	if err := provider.validateContext(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.ResourcePage{}, invalid("demo.inventory.resources", err)
	}
	items := make([]domain.ResourceSnapshot, 0, len(provider.resources))
	for _, item := range provider.resources {
		item.Health = provider.health[item.Resource.Ref.ID].InventoryState()
		if !query.Filter.MatchesSnapshot(item) || !containsKind(query.Filter.Kinds, item.Resource.Ref.Identity.Kind) ||
			!containsScope(query.Filter.Scopes, item.Resource.Ref.Identity.Scope) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(left, right int) bool { return compare(items[left], items[right], query) < 0 })
	fingerprint := resourceFingerprint(query)
	offset, err := decodeCursor(query.Page.Cursor, fingerprint)
	if err != nil || offset > len(items) {
		return domain.ResourcePage{}, invalid("demo.inventory.resources", err)
	}
	pageItems, page := paginate(items, offset, query.Page.Limit, fingerprint)
	result := domain.ResourcePage{Status: availableStatus(), Resources: pageItems, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.ResourcePage{}, invalid("demo.inventory.resources", err)
	}
	return result, nil
}

func (provider *Provider) GetResource(ctx context.Context, query domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	if err := provider.validateContext(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.ResourceSnapshot{}, invalid("demo.inventory.detail", err)
	}
	for _, item := range provider.resources {
		if item.Resource.Ref.ID == query.ResourceID {
			return item, nil
		}
	}
	return domain.ResourceSnapshot{}, notFound("demo.inventory.detail")
}

func (provider *Provider) GetHealth(ctx context.Context, query domain.HealthQuery) (domain.HealthResult, error) {
	if err := provider.validateContext(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.HealthResult{}, invalid("demo.health.read", err)
	}
	result := domain.HealthResult{Status: availableStatus(), Snapshots: make([]domain.HealthSnapshot, 0, len(query.Resources))}
	for _, resource := range query.Resources {
		snapshot, exists := provider.health[resource.ID]
		if !exists {
			return domain.HealthResult{}, notFound("demo.health.read")
		}
		result.Snapshots = append(result.Snapshots, snapshot)
	}
	if err := result.ValidateFor(query); err != nil {
		return domain.HealthResult{}, invalid("demo.health.read", err)
	}
	return result, nil
}

func (provider *Provider) GetCapabilities(ctx context.Context, source domain.ProviderRef) (domain.CapabilitySet, error) {
	if err := provider.validateContext(ctx, source); err != nil {
		return domain.CapabilitySet{}, invalid("demo.capability.read", err)
	}
	capabilities := []domain.CapabilityName{
		domain.CapabilityInventoryList, domain.CapabilityInventoryDetail, domain.CapabilityHealth,
		domain.CapabilityTopology, domain.CapabilityLogRead, domain.CapabilityLogTail,
		domain.CapabilityMetricCurrent, domain.CapabilityMetricHistory,
		domain.CapabilityEventHistory, domain.CapabilityEventWatch,
		domain.CapabilityDockerRestart, domain.CapabilityWorkloadRestart, domain.CapabilityWorkloadScale,
	}
	result := domain.CapabilitySet{Source: source, Capabilities: make([]domain.CapabilityStatus, 0, len(capabilities))}
	for _, name := range capabilities {
		status := unsupportedStatus()
		switch name {
		case domain.CapabilityInventoryList, domain.CapabilityInventoryDetail, domain.CapabilityHealth,
			domain.CapabilityTopology, domain.CapabilityLogRead, domain.CapabilityMetricCurrent,
			domain.CapabilityMetricHistory, domain.CapabilityEventHistory:
			status = availableStatus()
		}
		result.Capabilities = append(result.Capabilities, domain.CapabilityStatus{Name: name, Status: status})
	}
	if err := result.ValidateFor(source); err != nil {
		return domain.CapabilitySet{}, invalid("demo.capability.read", err)
	}
	return result, nil
}

func (provider *Provider) validate() error {
	if err := provider.source.Validate(); err != nil {
		return err
	}
	if err := provider.environment.Validate(); err != nil {
		return err
	}
	for _, resource := range provider.resources {
		if err := resource.Validate(); err != nil {
			return err
		}
		if err := provider.health[resource.Resource.Ref.ID].Validate(); err != nil {
			return err
		}
	}
	for _, relationship := range provider.relationships {
		if err := relationship.Validate(); err != nil {
			return err
		}
	}
	for _, record := range provider.logs {
		if err := record.Validate(); err != nil {
			return err
		}
	}
	for _, event := range provider.events {
		if err := event.Validate(); err != nil {
			return err
		}
	}
	for _, alert := range provider.alerts {
		if err := alert.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (provider *Provider) Alerts() []domain.Alert {
	return slices.Clone(provider.alerts)
}

func (provider *Provider) validateContext(ctx context.Context, source domain.ProviderRef) error {
	if provider == nil || ctx == nil {
		return fmt.Errorf("provider boundary is invalid")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if source != (domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: SourceID}) {
		return fmt.Errorf("source is not available")
	}
	return nil
}

func availableStatus() domain.ObservationStatus {
	return domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: observedAt}
}

func unsupportedStatus() domain.ObservationStatus {
	return domain.ObservationStatus{
		State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete,
		ObservedAt: observedAt, ReasonCode: "demo.capability.not_implemented",
	}
}

func containsKind(filter []domain.ResourceKind, value domain.ResourceKind) bool {
	return len(filter) == 0 || slices.Contains(filter, value)
}

func containsScope(filter []domain.Scope, value domain.Scope) bool {
	return len(filter) == 0 || slices.Contains(filter, value)
}

func compare(left, right domain.ResourceSnapshot, query domain.InventoryQuery) int {
	comparison := 0
	switch query.SortField {
	case domain.ResourceSortName:
		comparison = strings.Compare(left.Resource.DisplayName, right.Resource.DisplayName)
	case domain.ResourceSortKind:
		comparison = strings.Compare(string(left.Resource.Ref.Identity.Kind), string(right.Resource.Ref.Identity.Kind))
	case domain.ResourceSortObservedAt:
		comparison = left.Status.ObservedAt.Compare(right.Status.ObservedAt)
	}
	if comparison != 0 && query.SortDirection == domain.SortDescending {
		comparison = -comparison
	}
	if comparison == 0 {
		comparison = strings.Compare(string(left.Resource.Ref.ID), string(right.Resource.Ref.ID))
	}
	return comparison
}

func environmentFingerprint(query domain.EnvironmentQuery) string {
	return fingerprint(string(query.Source.Provider), string(query.Source.SourceID), "environment")
}

func resourceFingerprint(query domain.InventoryQuery) string {
	kinds := make([]string, len(query.Filter.Kinds))
	for index, kind := range query.Filter.Kinds {
		kinds[index] = string(kind)
	}
	scopes := make([]string, len(query.Filter.Scopes))
	for index, scope := range query.Filter.Scopes {
		scopes[index] = string(scope.Kind) + ":" + scope.Name
	}
	return fingerprint(
		string(query.Source.Provider), string(query.Source.SourceID), query.Filter.Search,
		fmt.Sprint(query.Filter),
		strings.Join(kinds, ","), strings.Join(scopes, ","), string(query.SortField), string(query.SortDirection),
	)
}

func fingerprint(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:8])
}

func encodeCursor(fingerprint string, offset int) domain.PageCursor {
	return domain.PageCursor(base64.RawURLEncoding.EncodeToString([]byte(fingerprint + ":" + strconv.Itoa(offset))))
}

func decodeCursor(cursor domain.PageCursor, fingerprint string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(string(cursor))
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != string(cursor) {
		return 0, fmt.Errorf("cursor is invalid")
	}
	prefix, offsetText, exists := strings.Cut(string(decoded), ":")
	offset, err := strconv.Atoi(offsetText)
	if !exists || err != nil || prefix != fingerprint || offset < 0 {
		return 0, fmt.Errorf("cursor does not match query")
	}
	return offset, nil
}

func paginate[T any](items []T, offset, limit int, fingerprint string) ([]T, domain.PageInfo) {
	end := min(offset+limit, len(items))
	pageItems := slices.Clone(items[offset:end])
	page := domain.PageInfo{Limit: limit, HasMore: end < len(items)}
	if page.HasMore {
		page.NextCursor = encodeCursor(fingerprint, end)
	}
	return pageItems, page
}

func invalid(operation domain.ProviderOperation, cause error) error {
	if cause == nil {
		cause = fmt.Errorf("invalid request")
	}
	result, _ := domain.NewProviderError(operation, domain.ProviderErrorInvalidRequest, false, cause)
	return result
}

func notFound(operation domain.ProviderOperation) error {
	result, _ := domain.NewProviderError(operation, domain.ProviderErrorInvalidRequest, false, fmt.Errorf("resource not found"))
	return result
}

var (
	_ domain.InventoryProvider  = (*Provider)(nil)
	_ domain.HealthProvider     = (*Provider)(nil)
	_ domain.CapabilityProvider = (*Provider)(nil)
)

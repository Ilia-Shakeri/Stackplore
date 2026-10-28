package assistantevidence

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/logpolicy"
)

// CaptureAuthority uses the existing provider-neutral inventory capture port.
// Docker, Kubernetes and Demo implement it with their operator-bound providers.
// A successful capability enum alone is deliberately insufficient.
type CaptureAuthority struct {
	Source   domain.ProviderRef
	Provider domain.InventoryCaptureProvider
	Now      func() time.Time
	Logs     *logpolicy.Reader
}

func (CaptureAuthority) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected evidence authority]")
}
func (CaptureAuthority) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (authority CaptureAuthority) Check(ctx context.Context, scope contextbuilder.Scope, kind assistant.StepKind, logConsent bool) error {
	if ctx == nil || authority.Provider == nil || authority.Now == nil || authority.Source.Validate() != nil || scope.Source != authority.Source || len(scope.Resources) < 1 || len(scope.Resources) > MaximumResources {
		return ErrInvalid
	}
	if !assistant.Timestamp(authority.Now().UTC().Truncate(time.Microsecond)) {
		return ErrInvalid
	}
	if kind == assistant.Logs && !logConsent {
		return ErrDenied
	}
	seen := make(map[domain.ResourceID]bool, len(scope.Resources))
	for _, ref := range scope.Resources {
		if ref.Validate() != nil || !scope.Source.Matches(ref) || seen[ref.ID] {
			return ErrInvalid
		}
		seen[ref.ID] = true
	}
	if _, valid := readCapability(kind); !valid {
		return ErrInvalid
	}
	work, cancel := context.WithTimeout(ctx, MaximumDuration)
	defer cancel()
	capture, err := authority.Provider.CaptureInventory(work)
	if err != nil {
		return safeError(work, err)
	}
	if work.Err() != nil {
		return ErrCancelled
	}
	fresh := func(at time.Time) bool {
		now := authority.Now().UTC().Truncate(time.Microsecond)
		return !at.IsZero() && !at.After(now) && (scope.Source.Provider == domain.ProviderDemo || now.Sub(at) <= MaximumAuthorityAge)
	}
	if capture.ValidateFor(scope.Source) != nil || capture.Status.State != domain.AvailabilityAvailable || !fresh(capture.Status.ObservedAt) {
		return ErrDenied
	}
	actual := make(map[domain.ResourceID]domain.ResourceSnapshot, len(capture.Resources))
	for _, resource := range capture.Resources {
		actual[resource.Resource.Ref.ID] = resource
	}
	for _, ref := range scope.Resources {
		resource, found := actual[ref.ID]
		if ref.Validate() != nil || !scope.Source.Matches(ref) || !found || resource.Resource.Ref != ref || resource.Status.State != domain.AvailabilityAvailable || !fresh(resource.Status.ObservedAt) {
			return ErrDenied
		}
		covered := len(capture.Coverage) == 0 && capture.Complete
		for _, coverage := range capture.Coverage {
			if coverage.Matches(ref) && coverage.Status.State == domain.AvailabilityAvailable && fresh(coverage.Status.ObservedAt) {
				covered = true
			}
		}
		if !covered {
			return ErrDenied
		}
	}
	return authority.family(work, scope, kind, fresh)
}

func (authority CaptureAuthority) family(ctx context.Context, scope contextbuilder.Scope, kind assistant.StepKind, fresh func(time.Time) bool) error {
	var status domain.ObservationStatus
	switch kind {
	case assistant.Inventory:
		return nil
	case assistant.Health:
		provider, ok := authority.Provider.(domain.HealthProvider)
		if !ok {
			return ErrUnavailable
		}
		query := domain.HealthQuery{Source: scope.Source, Resources: cloneScope(scope).Resources}
		result, err := provider.GetHealth(ctx, query)
		if err != nil {
			return safeError(ctx, err)
		}
		if result.ValidateFor(query) != nil {
			return ErrUnavailable
		}
		if len(result.Snapshots) != len(query.Resources) {
			return ErrDenied
		}
		for _, snapshot := range result.Snapshots {
			if snapshot.Status.State != domain.AvailabilityAvailable || snapshot.Status.Completeness != domain.CompletenessComplete || !fresh(snapshot.Status.ObservedAt) {
				return ErrDenied
			}
		}
		status = result.Status
	case assistant.Topology:
		provider, ok := authority.Provider.(domain.TopologyProvider)
		if !ok {
			return ErrUnavailable
		}
		query := domain.TopologyQuery{Source: scope.Source, Resources: cloneScope(scope).Resources, Page: domain.PageRequest{Limit: 1}}
		page, err := provider.GetRelationships(ctx, query)
		if err != nil {
			return safeError(ctx, err)
		}
		if page.ValidateFor(query) != nil {
			return ErrUnavailable
		}
		// A derived edge can depend on a third provider partition. Endpoint
		// inventory alone cannot prove that partition is still readable.
		if page.Status.Completeness != domain.CompletenessComplete {
			return ErrDenied
		}
		status = page.Status
	case assistant.Metrics:
		provider, ok := authority.Provider.(domain.CurrentMetricProvider)
		if !ok {
			return ErrUnavailable
		}
		query := domain.CurrentMetricQuery{Source: scope.Source, Resources: cloneScope(scope).Resources, Names: []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes, domain.MetricRestartCount}}
		result, err := provider.GetCurrentMetrics(ctx, query)
		if err != nil {
			return safeError(ctx, err)
		}
		if result.ValidateFor(query) != nil {
			return ErrUnavailable
		}
		if len(result.Sets) != len(query.Resources) {
			return ErrDenied
		}
		for _, set := range result.Sets {
			if set.Status.State != domain.AvailabilityAvailable || !fresh(set.Status.ObservedAt) {
				return ErrDenied
			}
		}
		status = result.Status
	case assistant.Events:
		provider, ok := authority.Provider.(domain.EventHistoryProvider)
		if !ok {
			return ErrUnavailable
		}
		query := domain.EventQuery{Source: scope.Source, Resources: cloneScope(scope).Resources, Range: scope.Range, Page: domain.PageRequest{Limit: 1}}
		page, err := provider.ListEvents(ctx, query)
		if err != nil {
			return safeError(ctx, err)
		}
		if page.ValidateFor(query) != nil {
			return ErrUnavailable
		}
		status = page.Status
	case assistant.Logs:
		if authority.Logs == nil || authority.Logs.Provider == nil || authority.Logs.Settings == nil || authority.Logs.Redactor == nil || scope.Range.End.Sub(scope.Range.Start) > contextbuilder.MaximumLogWindow {
			return ErrDenied
		}
		query := domain.LogReadQuery{Source: scope.Source, Range: scope.Range, LineLimit: 1, ByteLimit: contextbuilder.MaximumLogBytes}
		for _, ref := range scope.Resources {
			query.Targets = append(query.Targets, domain.LogTarget{Resource: ref})
		}
		capture, err := authority.Logs.Capture(ctx, query)
		if err != nil {
			return safeError(ctx, err)
		}
		_, page, valid := capture.Snapshot()
		if !valid {
			return ErrUnavailable
		}
		status = page.Status
	default:
		return ErrInvalid
	}
	if ctx.Err() != nil {
		return ErrCancelled
	}
	if status.State != domain.AvailabilityAvailable || !fresh(status.ObservedAt) {
		return ErrDenied
	}
	return nil
}

var _ LiveScopeAuthority = CaptureAuthority{}

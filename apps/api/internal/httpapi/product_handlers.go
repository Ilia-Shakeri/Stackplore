package httpapi

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/alerting"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/realtime"
	"github.com/go-chi/chi/v5"
)

// ProductProvider is the safe read boundary shared by all infrastructure sources.
type ProductProvider interface {
	domain.InventoryProvider
	domain.HealthProvider
	domain.TopologyProvider
	domain.CapabilityProvider
	Source() domain.InfrastructureSource
}

type productHTTP struct {
	auth                       authHTTP
	providers                  map[domain.SourceID]ProductProvider
	ordered                    []ProductProvider
	defaultID                  domain.SourceID
	streams                    *realtime.Manager
	streamGate                 *streamGate
	auditReader                AuditReader
	alertReader                AlertReader
	notificationReader         NotificationReader
	notificationSettingsReader NotificationSettingsReader
	notificationTester         NotificationTester
	alertManager               AlertManager
	sourceSettings             SourceSettings
}

type AlertManager interface {
	Acknowledge(context.Context, domain.AlertID, domain.UserID, domain.RequestID) (domain.Alert, error)
	ListRules(context.Context, domain.PageRequest) (domain.AlertRulePage, error)
	CreateRule(context.Context, alerting.RuleInput, domain.UserID, domain.RequestID) (domain.AlertRuleRecord, error)
	ReplaceRule(context.Context, domain.AlertRuleID, int64, alerting.RuleInput, domain.UserID, domain.RequestID) (domain.AlertRuleRecord, error)
	DeleteRule(context.Context, domain.AlertRuleID, int64, domain.UserID, domain.RequestID) error
}

var _ AlertManager = (*alerting.ManagementService)(nil)

func newProductHTTP(auth authHTTP, providers []ProductProvider, audit AuditReader, alerts AlertReader, notifications NotificationReader, notificationSettings NotificationSettingsReader, notificationTester NotificationTester, managers ...AlertManager) productHTTP {
	byID := make(map[domain.SourceID]ProductProvider, len(providers))
	for _, provider := range providers {
		byID[provider.Source().ID] = provider
	}
	streams, err := realtime.NewManager(realtime.ManagerOptions{
		MaximumFeeds: 64, ReplaySize: 128, QueueSize: 16, MaximumFrame: 256 * 1024,
	})
	if err != nil {
		panic("invalid built-in realtime limits")
	}
	var manager AlertManager
	if len(managers) == 1 {
		manager = managers[0]
	}
	return productHTTP{
		auth: auth, providers: byID, ordered: slices.Clone(providers), defaultID: providers[0].Source().ID,
		streams: streams, streamGate: newStreamGate(64, 4),
		auditReader: audit, alertReader: alerts, notificationReader: notifications,
		notificationSettingsReader: notificationSettings, notificationTester: notificationTester,
		alertManager: manager,
	}
}

func validateProductProviders(providers []ProductProvider) error {
	seen := make(map[domain.SourceID]struct{}, len(providers))
	for _, provider := range providers {
		if provider == nil || provider.Source().Validate() != nil {
			return errors.New("invalid product provider")
		}
		if _, exists := seen[provider.Source().ID]; exists {
			return errors.New("duplicate product provider")
		}
		seen[provider.Source().ID] = struct{}{}
	}
	return nil
}

type sourceResponse struct {
	ID             domain.SourceID `json:"id"`
	Provider       string          `json:"provider"`
	Name           string          `json:"name"`
	Synthetic      bool            `json:"synthetic"`
	FixtureVersion string          `json:"fixture_version,omitempty"`
	ObservedAt     string          `json:"observed_at"`
	Availability   string          `json:"availability"`
	Completeness   string          `json:"completeness"`
	Resources      int             `json:"resource_count"`
	Health         healthSummary   `json:"health"`
}

type healthSummary struct {
	Healthy   int `json:"healthy"`
	Degraded  int `json:"degraded"`
	Unhealthy int `json:"unhealthy"`
	Unknown   int `json:"unknown"`
}

type environmentResponse struct {
	ID           domain.EnvironmentID `json:"id"`
	SourceID     domain.SourceID      `json:"source_id"`
	Provider     string               `json:"provider"`
	Name         string               `json:"name"`
	Availability string               `json:"availability"`
	Completeness string               `json:"completeness"`
	ObservedAt   string               `json:"observed_at"`
	Synthetic    bool                 `json:"synthetic"`
}

type environmentPageResponse struct {
	observationResponse
	Items []environmentResponse `json:"items"`
	Page  pageResponse          `json:"page"`
}

type resourceResponse struct {
	ID            domain.ResourceID `json:"id"`
	Provider      string            `json:"provider"`
	SourceID      domain.SourceID   `json:"source_id"`
	EnvironmentID string            `json:"environment_id"`
	Kind          string            `json:"kind"`
	ScopeKind     string            `json:"scope_kind"`
	ScopeName     string            `json:"scope_name,omitempty"`
	Name          string            `json:"name"`
	Synthetic     bool              `json:"synthetic"`
	ObservedAt    string            `json:"observed_at"`
	Health        string            `json:"health,omitempty"`
}

type resourcePageResponse struct {
	observationResponse
	Items []resourceResponse `json:"items"`
	Page  pageResponse       `json:"page"`
}

type observationResponse struct {
	Availability string `json:"availability"`
	Completeness string `json:"completeness"`
	ObservedAt   string `json:"observed_at"`
	ReasonCode   string `json:"reason_code,omitempty"`
}

func observationJSON(status domain.ObservationStatus) observationResponse {
	return observationResponse{
		Availability: string(status.State), Completeness: string(status.Completeness),
		ObservedAt: authTimestamp(status.ObservedAt), ReasonCode: string(status.ReasonCode),
	}
}

type healthResponse struct {
	ResourceID domain.ResourceID `json:"resource_id"`
	State      string            `json:"state"`
	ObservedAt string            `json:"observed_at"`
	Synthetic  bool              `json:"synthetic"`
	Reasons    []string          `json:"reason_codes"`
}

func (handler productHTTP) sources(w http.ResponseWriter, r *http.Request) {
	if !requireNoBodyAndQuery(w, r) || !handler.authenticate(w, r) {
		return
	}
	items := make([]sourceResponse, 0, len(handler.ordered))
	for _, provider := range handler.ordered {
		items = append(items, handler.sourceSummary(r, provider))
	}
	writeJSON(w, http.StatusOK, items)
}

func (handler productHTTP) sourceSummary(r *http.Request, provider ProductProvider) sourceResponse {
	ref := providerRef(provider)
	query := domain.InventoryQuery{
		Source: ref, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending,
		Page: domain.PageRequest{Limit: domain.MaxPageSize},
	}
	page, err := provider.ListResources(r.Context(), query)
	if err != nil {
		now := time.Now().UTC().Truncate(time.Microsecond)
		source := provider.Source()
		return sourceResponse{
			ID: source.ID, Provider: string(source.Provider), Name: source.Name, Synthetic: source.Synthetic,
			ObservedAt: authTimestamp(now), Availability: string(domain.AvailabilityUnavailable),
			Completeness: string(domain.CompletenessComplete), Health: healthSummary{},
		}
	}
	refs := make([]domain.ResourceRef, min(len(page.Resources), domain.MaxHealthTargets))
	for index := range refs {
		refs[index] = page.Resources[index].Resource.Ref
	}
	completeness := page.Status.Completeness
	if page.Page.HasMore || len(refs) < len(page.Resources) {
		completeness = domain.CompletenessPartial
	}
	summary := healthSummary{Unknown: len(page.Resources)}
	if len(refs) > 0 {
		health, healthErr := provider.GetHealth(r.Context(), domain.HealthQuery{Source: ref, Resources: refs})
		if healthErr != nil || !health.Status.AllowsData() {
			completeness = domain.CompletenessPartial
		} else {
			if health.Status.State != domain.AvailabilityAvailable || health.Status.Completeness != domain.CompletenessComplete {
				completeness = domain.CompletenessPartial
			}
			wanted := make(map[domain.ResourceID]struct{}, len(refs))
			for _, resource := range refs {
				wanted[resource.ID] = struct{}{}
			}
			seen := make(map[domain.ResourceID]struct{}, len(refs))
			for _, snapshot := range health.Snapshots {
				if _, expected := wanted[snapshot.ResourceID]; !expected {
					completeness = domain.CompletenessPartial
					continue
				}
				if _, duplicate := seen[snapshot.ResourceID]; duplicate {
					completeness = domain.CompletenessPartial
					continue
				}
				seen[snapshot.ResourceID] = struct{}{}
				if !snapshot.Status.AllowsData() || snapshot.Status.State != domain.AvailabilityAvailable || snapshot.Status.Completeness != domain.CompletenessComplete {
					completeness = domain.CompletenessPartial
				}
				if !snapshot.Status.AllowsData() {
					continue
				}
				switch snapshot.State {
				case domain.HealthHealthy:
					summary.Healthy++
					summary.Unknown--
				case domain.HealthDegraded:
					summary.Degraded++
					summary.Unknown--
				case domain.HealthUnhealthy:
					summary.Unhealthy++
					summary.Unknown--
				}
			}
			if len(seen) < len(refs) {
				completeness = domain.CompletenessPartial
			}
		}
	}
	source := provider.Source()
	fixtureVersion := ""
	if fixture, ok := provider.(interface{ FixtureVersion() string }); ok {
		fixtureVersion = fixture.FixtureVersion()
	}
	return sourceResponse{
		ID: source.ID, Provider: string(source.Provider), Name: source.Name, Synthetic: source.Synthetic,
		FixtureVersion: fixtureVersion, ObservedAt: authTimestamp(page.Status.ObservedAt),
		Availability: string(page.Status.State), Completeness: string(completeness),
		Resources: len(page.Resources), Health: summary,
	}
}

func (handler productHTTP) environments(w http.ResponseWriter, r *http.Request) {
	if !requireNoBody(w, r) || !handler.authenticate(w, r) {
		return
	}
	pageRequest, err := parsePageQuery(r, 50)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	provider, err := handler.providerFor(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	page, err := provider.ListEnvironments(r.Context(), domain.EnvironmentQuery{
		Source: providerRef(provider), Page: pageRequest,
	})
	if err != nil {
		writeProviderProblem(w, r, err)
		return
	}
	items := make([]environmentResponse, len(page.Environments))
	for index, environment := range page.Environments {
		items[index] = environmentResponse{
			ID: environment.ID, SourceID: environment.SourceID, Provider: string(environment.Provider),
			Name: environment.Name, Availability: string(environment.Status.State),
			Completeness: string(environment.Status.Completeness),
			ObservedAt:   authTimestamp(environment.Status.ObservedAt), Synthetic: environment.Synthetic,
		}
	}
	writeJSON(w, http.StatusOK, environmentPageResponse{
		observationResponse: observationJSON(page.Status), Items: items, Page: pageJSON(page.Page),
	})
}

func (handler productHTTP) resources(w http.ResponseWriter, r *http.Request) {
	if !requireNoBody(w, r) || !handler.authenticate(w, r) {
		return
	}
	query, err := handler.parseResourceQuery(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	provider, err := handler.providerFor(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	query.Source = providerRef(provider)
	page, err := provider.ListResources(r.Context(), query)
	if err != nil {
		writeProviderProblem(w, r, err)
		return
	}
	items := make([]resourceResponse, len(page.Resources))
	for index, snapshot := range page.Resources {
		items[index] = resourceJSON(snapshot, "")
	}
	writeJSON(w, http.StatusOK, resourcePageResponse{
		observationResponse: observationJSON(page.Status), Items: items,
		Page: pageResponse{Limit: page.Page.Limit, HasMore: page.Page.HasMore, NextCursor: string(page.Page.NextCursor)},
	})
}

func (handler productHTTP) resource(w http.ResponseWriter, r *http.Request) {
	if !requireNoBody(w, r) || !requireOnlySourceQuery(w, r) || !handler.authenticate(w, r) {
		return
	}
	provider, err := handler.providerFor(r)
	if err != nil {
		writeProblem(w, r, notFoundProblem)
		return
	}
	id := domain.ResourceID(chi.URLParam(r, "resource_id"))
	if id.Validate() != nil {
		writeProblem(w, r, notFoundProblem)
		return
	}
	snapshot, err := provider.GetResource(r.Context(), domain.ResourceDetailQuery{Source: providerRef(provider), ResourceID: id})
	if err != nil {
		writeResourceProblem(w, r, err)
		return
	}
	health, err := provider.GetHealth(r.Context(), domain.HealthQuery{Source: providerRef(provider), Resources: []domain.ResourceRef{snapshot.Resource.Ref}})
	if err != nil {
		writeProviderProblem(w, r, err)
		return
	}
	if len(health.Snapshots) != 1 {
		writeProblem(w, r, unavailableProblem)
		return
	}
	writeJSON(w, http.StatusOK, resourceJSON(snapshot, string(health.Snapshots[0].State)))
}

func (handler productHTTP) health(w http.ResponseWriter, r *http.Request) {
	if !requireNoBody(w, r) || !requireOnlySourceQuery(w, r) || !handler.authenticate(w, r) {
		return
	}
	provider, err := handler.providerFor(r)
	if err != nil {
		writeProblem(w, r, notFoundProblem)
		return
	}
	id := domain.ResourceID(chi.URLParam(r, "resource_id"))
	if id.Validate() != nil {
		writeProblem(w, r, notFoundProblem)
		return
	}
	snapshot, err := provider.GetResource(r.Context(), domain.ResourceDetailQuery{Source: providerRef(provider), ResourceID: id})
	if err != nil {
		writeResourceProblem(w, r, err)
		return
	}
	result, err := provider.GetHealth(r.Context(), domain.HealthQuery{Source: providerRef(provider), Resources: []domain.ResourceRef{snapshot.Resource.Ref}})
	if err != nil {
		writeProviderProblem(w, r, err)
		return
	}
	if len(result.Snapshots) != 1 {
		writeProblem(w, r, unavailableProblem)
		return
	}
	health := result.Snapshots[0]
	reasons := make([]string, len(health.Evidence))
	for index := range health.Evidence {
		reasons[index] = string(health.Evidence[index].Code)
	}
	writeJSON(w, http.StatusOK, healthResponse{
		ResourceID: health.ResourceID, State: string(health.State),
		ObservedAt: authTimestamp(health.Status.ObservedAt), Synthetic: snapshot.Resource.Synthetic, Reasons: reasons,
	})
}

func (handler productHTTP) authenticate(w http.ResponseWriter, r *http.Request) bool {
	_, ok := handler.authenticateIdentity(w, r)
	return ok
}

func writeProviderProblem(w http.ResponseWriter, r *http.Request, err error) {
	var providerError *domain.ProviderError
	if !errors.As(err, &providerError) {
		writeProblem(w, r, internalProblem)
		return
	}
	switch providerError.Code {
	case domain.ProviderErrorInvalidRequest:
		writeProblem(w, r, invalidRequestProblem)
	case domain.ProviderErrorPermissionDenied:
		writeProblem(w, r, forbiddenProblem)
	case domain.ProviderErrorRateLimited:
		writeProblem(w, r, rateLimitProblem)
	case domain.ProviderErrorCanceled, domain.ProviderErrorDeadlineExceeded, domain.ProviderErrorUnavailable, domain.ProviderErrorUnsupported:
		writeProblem(w, r, unavailableProblem)
	default:
		writeProblem(w, r, internalProblem)
	}
}

func writeResourceProblem(w http.ResponseWriter, r *http.Request, err error) {
	var providerError *domain.ProviderError
	if errors.As(err, &providerError) && providerError.Code == domain.ProviderErrorInvalidRequest {
		writeProblem(w, r, notFoundProblem)
		return
	}
	writeProviderProblem(w, r, err)
}

func (handler productHTTP) authenticateIdentity(w http.ResponseWriter, r *http.Request) (auth.Authentication, bool) {
	sessionToken, ok := readCookieValue(r, sessionCookieName)
	if !ok {
		writeProblem(w, r, unauthenticatedProblem)
		return auth.Authentication{}, false
	}
	identity, err := handler.auth.service.Authenticate(r.Context(), sessionToken)
	if err != nil {
		if errors.Is(err, auth.ErrUnauthenticated) {
			clearAuthCookie(w, sessionCookieName, handler.auth.secureCookies, true)
		}
		handler.auth.writeError(w, r, err)
		return auth.Authentication{}, false
	}
	handler.auth.renewSessionCookie(w, r, sessionToken)
	return identity, true
}

func (handler productHTTP) parseResourceQuery(r *http.Request) (domain.InventoryQuery, error) {
	values := r.URL.Query()
	for key, items := range values {
		if len(items) != 1 || items[0] == "" && key != "search" {
			return domain.InventoryQuery{}, errors.New("invalid resource query")
		}
		switch key {
		case "search", "kind", "limit", "cursor", "sort", "direction", "source_id":
		default:
			return domain.InventoryQuery{}, errors.New("unknown resource query")
		}
	}
	limit := 50
	if value := values.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return domain.InventoryQuery{}, err
		}
		limit = parsed
	}
	sortField := domain.ResourceSortName
	if value := values.Get("sort"); value != "" {
		sortField = domain.ResourceSortField(value)
	}
	direction := domain.SortAscending
	if value := values.Get("direction"); value != "" {
		direction = domain.SortDirection(value)
	}
	query := domain.InventoryQuery{
		Source: domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "source_pending"}, Filter: domain.ResourceFilter{Search: values.Get("search")},
		SortField: sortField, SortDirection: direction,
		Page: domain.PageRequest{Limit: limit, Cursor: domain.PageCursor(values.Get("cursor"))},
	}
	if kind := values.Get("kind"); kind != "" {
		query.Filter.Kinds = []domain.ResourceKind{domain.ResourceKind(strings.TrimSpace(kind))}
	}
	if err := query.Filter.Validate(); err != nil {
		return domain.InventoryQuery{}, err
	}
	if err := query.SortField.Validate(); err != nil {
		return domain.InventoryQuery{}, err
	}
	if err := query.SortDirection.Validate(); err != nil {
		return domain.InventoryQuery{}, err
	}
	return query, query.Page.Validate()
}

func resourceJSON(snapshot domain.ResourceSnapshot, health string) resourceResponse {
	identity := snapshot.Resource.Ref.Identity
	return resourceResponse{
		ID: snapshot.Resource.Ref.ID, Provider: string(identity.Provider), SourceID: identity.SourceID,
		EnvironmentID: string(identity.EnvironmentID), Kind: string(identity.Kind), ScopeKind: string(identity.Scope.Kind),
		ScopeName: identity.Scope.Name, Name: snapshot.Resource.DisplayName, Synthetic: snapshot.Resource.Synthetic,
		ObservedAt: authTimestamp(snapshot.Status.ObservedAt), Health: health,
	}
}

func parsePageQuery(r *http.Request, defaultLimit int) (domain.PageRequest, error) {
	values := r.URL.Query()
	for key, items := range values {
		if len(items) != 1 || items[0] == "" || key != "limit" && key != "cursor" && key != "source_id" {
			return domain.PageRequest{}, errors.New("invalid page query")
		}
	}
	limit := defaultLimit
	if value := values.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return domain.PageRequest{}, err
		}
		limit = parsed
	}
	page := domain.PageRequest{Limit: limit, Cursor: domain.PageCursor(values.Get("cursor"))}
	return page, page.Validate()
}

func providerRef(provider ProductProvider) domain.ProviderRef {
	source := provider.Source()
	return domain.ProviderRef{Provider: source.Provider, SourceID: source.ID}
}

func (handler productHTTP) providerFor(r *http.Request) (ProductProvider, error) {
	values := r.URL.Query()["source_id"]
	id := handler.defaultID
	if len(values) > 1 || len(values) == 1 && values[0] == "" {
		return nil, errors.New("invalid source")
	}
	if len(values) == 1 {
		id = domain.SourceID(values[0])
		if id.Validate() != nil {
			return nil, errors.New("invalid source")
		}
	}
	provider, exists := handler.providers[id]
	if !exists {
		return nil, errors.New("source not found")
	}
	return provider, nil
}

func requestHasOnlySourceQuery(r *http.Request) bool {
	values := r.URL.Query()
	for key, items := range values {
		if key != "source_id" || len(items) != 1 || items[0] == "" {
			return false
		}
	}
	return true
}

func requireOnlySourceQuery(w http.ResponseWriter, r *http.Request) bool {
	if !requestHasOnlySourceQuery(r) {
		writeProblem(w, r, invalidRequestProblem)
		return false
	}
	return true
}

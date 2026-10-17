package httpapi

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type explorationQueryProvider struct {
	ProductProvider
	inventories     []domain.InventoryQuery
	logs            []domain.LogReadQuery
	events          []domain.EventQuery
	histories       []domain.MetricHistoryQuery
	relationships   []domain.TopologyQuery
	current         []domain.CurrentMetricQuery
	inventoryStatus *domain.ObservationStatus
}

func (provider *explorationQueryProvider) ObservationTime() time.Time { return demo.ObservationTime() }

func explorationQueryStatus() domain.ObservationStatus {
	return domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: demo.ObservationTime()}
}

func (provider *explorationQueryProvider) ListResources(ctx context.Context, query domain.InventoryQuery) (domain.ResourcePage, error) {
	provider.inventories = append(provider.inventories, query)
	if err := query.Validate(); err != nil {
		return domain.ResourcePage{}, err
	}
	status := explorationQueryStatus()
	if provider.inventoryStatus != nil {
		status = *provider.inventoryStatus
	}
	return domain.ResourcePage{Status: status, Page: domain.PageInfo{Limit: query.Page.Limit}}, nil
}

func (provider *explorationQueryProvider) GetRelationships(ctx context.Context, query domain.TopologyQuery) (domain.RelationshipPage, error) {
	provider.relationships = append(provider.relationships, query)
	if err := query.Validate(); err != nil {
		return domain.RelationshipPage{}, err
	}
	return domain.RelationshipPage{Status: explorationQueryStatus(), Page: domain.PageInfo{Limit: query.Page.Limit}}, nil
}

func (provider *explorationQueryProvider) GetCurrentMetrics(ctx context.Context, query domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
	provider.current = append(provider.current, query)
	if err := query.Validate(); err != nil {
		return domain.CurrentMetricResult{}, err
	}
	return domain.CurrentMetricResult{Status: explorationQueryStatus()}, nil
}

func (provider *explorationQueryProvider) ReadLogs(ctx context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	provider.logs = append(provider.logs, query)
	if err := query.Validate(); err != nil {
		return domain.LogPage{}, err
	}
	return domain.LogPage{Status: explorationQueryStatus(), Page: domain.PageInfo{Limit: query.LineLimit}}, nil
}

func (provider *explorationQueryProvider) ListEvents(ctx context.Context, query domain.EventQuery) (domain.EventPage, error) {
	provider.events = append(provider.events, query)
	if err := query.Validate(); err != nil {
		return domain.EventPage{}, err
	}
	return domain.EventPage{Status: explorationQueryStatus(), Page: domain.PageInfo{Limit: query.Page.Limit}}, nil
}

func (provider *explorationQueryProvider) GetMetricHistory(ctx context.Context, query domain.MetricHistoryQuery) (domain.MetricHistoryResult, error) {
	provider.histories = append(provider.histories, query)
	if err := query.Validate(); err != nil {
		return domain.MetricHistoryResult{}, err
	}
	return domain.MetricHistoryResult{Status: explorationQueryStatus()}, nil
}

func explorationResourceID(t *testing.T, provider ProductProvider) domain.ResourceID {
	t.Helper()
	page, err := provider.ListResources(context.Background(), domain.InventoryQuery{Source: providerRef(provider), SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 1}})
	if err != nil || len(page.Resources) != 1 {
		t.Fatalf("resource fixture failed: %v", err)
	}
	return page.Resources[0].Resource.Ref.ID
}

func TestProductResourceQueryForwardsEveryFixedFilter(t *testing.T) {
	provider := &explorationQueryProvider{ProductProvider: demo.MustNew()}
	values := url.Values{
		"source_id": {string(provider.Source().ID)}, "provider": {"docker"}, "environment_id": {"env_shop"},
		"kind": {"docker.container"}, "health": {"degraded"}, "availability": {"stale"},
		"scope_kind": {"project"}, "scope_name": {"shop"}, "search": {"worker"},
		"sort": {"observed_at"}, "direction": {"desc"}, "limit": {"25"}, "cursor": {"bmV4dA"},
	}
	response := authenticatedProductRequest(boundaryHandler(t, provider), "/api/v1/resources?"+values.Encode())
	if response.Code != http.StatusOK || len(provider.inventories) != 1 {
		t.Fatalf("filtered inventory status %d; calls %d", response.Code, len(provider.inventories))
	}
	query := provider.inventories[0]
	expected := domain.ResourceFilter{Provider: domain.ProviderDocker, EnvironmentID: "env_shop", Kinds: []domain.ResourceKind{"docker.container"}, Health: domain.HealthDegraded, Availability: domain.AvailabilityStale, ScopeKind: domain.ScopeProject, ScopeName: "shop", Search: "worker"}
	if !reflect.DeepEqual(query.Filter, expected) || query.Source != providerRef(provider) || query.SortField != domain.ResourceSortObservedAt || query.SortDirection != domain.SortDescending || query.Page.Limit != 25 || query.Page.Cursor != "bmV4dA" {
		t.Fatalf("wrong fixed filter mapping: %#v", query)
	}
}

func TestProductResourceQueryRejectsMalformedFixedFilters(t *testing.T) {
	cases := []string{
		"provider=unknown", "provider=demo&provider=docker", "provider=", "environment_id=bad+identifier",
		"health=green", "availability=live", "scope_kind=host", "scope_name=+shop", "scope_name=shop%0Aname",
		"scope_name=" + strings.Repeat("a", 257), "kind=+docker.container+", "sort=random", "direction=down", "limit=201", "cursor=bad+cursor",
	}
	for _, query := range cases {
		t.Run(query, func(t *testing.T) {
			provider := &explorationQueryProvider{ProductProvider: demo.MustNew()}
			response := authenticatedProductRequest(boundaryHandler(t, provider), "/api/v1/resources?"+query)
			if response.Code != http.StatusBadRequest || len(provider.inventories) != 0 {
				t.Fatalf("invalid filter status %d; provider calls %d", response.Code, len(provider.inventories))
			}
		})
	}
}

func TestProductObservationQueryForwardsExactRangeAndLogTarget(t *testing.T) {
	provider := &explorationQueryProvider{ProductProvider: demo.MustNew()}
	id := explorationResourceID(t, provider.ProductProvider)
	from, to := "2026-08-16T07:00:00.000001Z", "2026-08-16T08:00:00.000002Z"
	start, _ := time.Parse(time.RFC3339Nano, from)
	end, _ := time.Parse(time.RFC3339Nano, to)
	values := url.Values{"resource_id": {string(id)}, "source_id": {string(provider.Source().ID)}, "from": {from}, "to": {to}, "limit": {"25"}, "cursor": {"bmV4dA"}}
	handler := boundaryHandler(t, provider)
	logs := maps.Clone(values)
	logs.Set("container", "api-v2")
	logs.Set("previous", "true")
	response := authenticatedProductRequest(handler, "/api/v1/logs?"+logs.Encode())
	if response.Code != http.StatusOK || len(provider.logs) != 1 {
		t.Fatalf("logs status %d; calls %d", response.Code, len(provider.logs))
	}
	logQuery := provider.logs[0]
	if len(logQuery.Targets) != 1 || logQuery.Targets[0].Resource.ID != id || logQuery.Targets[0].Container != "api-v2" || !logQuery.Targets[0].Previous || !logQuery.Range.Start.Equal(start) || !logQuery.Range.End.Equal(end) || logQuery.LineLimit != 25 || logQuery.ByteLimit != domain.MaxLogBytes || logQuery.Cursor != "bmV4dA" {
		t.Fatalf("log query mapping wrong: %#v", logQuery)
	}
	response = authenticatedProductRequest(handler, "/api/v1/events?"+values.Encode())
	if response.Code != http.StatusOK || len(provider.events) != 1 {
		t.Fatalf("events status %d; calls %d", response.Code, len(provider.events))
	}
	eventQuery := provider.events[0]
	if len(eventQuery.Resources) != 1 || eventQuery.Resources[0].ID != id || !eventQuery.Range.Start.Equal(start) || !eventQuery.Range.End.Equal(end) || eventQuery.Page.Limit != 25 || eventQuery.Page.Cursor != "bmV4dA" {
		t.Fatalf("event query mapping wrong: %#v", eventQuery)
	}
	values.Del("cursor")
	response = authenticatedProductRequest(handler, "/api/v1/metrics/history?"+values.Encode())
	if response.Code != http.StatusOK || len(provider.histories) != 1 {
		t.Fatalf("history status %d; calls %d", response.Code, len(provider.histories))
	}
	history := provider.histories[0]
	if len(history.Resources) != 1 || history.Resources[0].ID != id || !history.Range.Start.Equal(start) || !history.Range.End.Equal(end) || history.PointLimit != 25 {
		t.Fatalf("history query mapping wrong: %#v", history)
	}
}

func TestProductObservationRangeRejectsAmbiguousAndUnboundedRequests(t *testing.T) {
	future := time.Now().UTC().Add(5 * time.Minute).Truncate(time.Microsecond)
	invalidRanges := []string{
		"from=2026-08-16T07:00:00Z", "to=2026-08-16T08:00:00Z",
		"from=2026-08-16T07:00:00Z&from=2026-08-16T06:00:00Z&to=2026-08-16T08:00:00Z",
		"from=&to=2026-08-16T08:00:00Z", "from=2026-02-30T07:00:00Z&to=2026-03-01T08:00:00Z",
		"from=2026-08-16T08:00:00Z&to=2026-08-16T07:00:00Z", "from=2026-08-16T08:00:00Z&to=2026-08-16T08:00:00Z",
		"from=2026-08-16T07:00:00%2B00:00&to=2026-08-16T08:00:00Z",
		"from=2026-08-16T07:00:00.0000001Z&to=2026-08-16T08:00:00Z",
		"from=2026-08-16T07:00:00.000001000Z&to=2026-08-16T08:00:00Z",
		"from=2026-08-16T07:00:00,000001Z&to=2026-08-16T08:00:00Z",
		"from=0000-08-16T07:00:00Z&to=0000-08-16T08:00:00Z",
		url.Values{"from": {future.Add(-time.Hour).Format(time.RFC3339Nano)}, "to": {future.Format(time.RFC3339Nano)}}.Encode(),
	}
	for _, path := range []string{"/api/v1/logs", "/api/v1/events", "/api/v1/metrics/history"} {
		for _, query := range invalidRanges {
			t.Run(path+"?"+query, func(t *testing.T) {
				provider := &explorationQueryProvider{ProductProvider: demo.MustNew()}
				response := authenticatedProductRequest(boundaryHandler(t, provider), path+"?"+query)
				if response.Code != http.StatusBadRequest || len(provider.logs)+len(provider.events)+len(provider.histories) != 0 {
					t.Fatalf("invalid range status %d; observation calls %d", response.Code, len(provider.logs)+len(provider.events)+len(provider.histories))
				}
			})
		}
	}
	for _, path := range []string{"/api/v1/topology", "/api/v1/metrics/current"} {
		response := authenticatedProductRequest(boundaryHandler(t, demo.MustNew()), path+"?from=2026-08-16T07:00:00Z&to=2026-08-16T08:00:00Z")
		if response.Code != http.StatusBadRequest {
			t.Errorf("unsupported range %s status %d", path, response.Code)
		}
	}
}

func TestProductObservationRangeHonorsExactSpanCaps(t *testing.T) {
	for _, spec := range []struct {
		path string
		span time.Duration
	}{{"/api/v1/logs", 7 * 24 * time.Hour}, {"/api/v1/events", 30 * 24 * time.Hour}, {"/api/v1/metrics/history", 30 * 24 * time.Hour}} {
		for _, excess := range []time.Duration{0, time.Microsecond} {
			t.Run(spec.path+excess.String(), func(t *testing.T) {
				provider := &explorationQueryProvider{ProductProvider: demo.MustNew()}
				resourceID := explorationResourceID(t, provider.ProductProvider)
				end := demo.ObservationTime()
				values := url.Values{"resource_id": {string(resourceID)}, "from": {end.Add(-spec.span - excess).Format(time.RFC3339Nano)}, "to": {end.Format(time.RFC3339Nano)}}
				response := authenticatedProductRequest(boundaryHandler(t, provider), spec.path+"?"+values.Encode())
				expected := http.StatusOK
				if excess != 0 {
					expected = http.StatusBadRequest
				}
				if response.Code != expected {
					t.Fatalf("range cap status %d; want %d", response.Code, expected)
				}
			})
		}
	}
	response := authenticatedProductRequest(boundaryHandler(t, demo.MustNew()), "/api/v1/metrics/history?minutes=60&from=2026-08-16T07:00:00Z&to=2026-08-16T08:00:00Z")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("conflicting history range status %d", response.Code)
	}
}

func TestProductLogSelectionRejectsUntargetedAndUnsafeInputs(t *testing.T) {
	base := demo.MustNew()
	id := string(explorationResourceID(t, base))
	for _, query := range []string{"container=api", "previous=true", "previous=false", "resource_id=" + id + "&previous=1", "resource_id=" + id + "&previous=true&previous=false", "resource_id=" + id + "&container=../api", "resource_id=" + id + "&container=api%2Fchild", "resource_id=" + id + "&container=" + strings.Repeat("a", 257)} {
		t.Run(query, func(t *testing.T) {
			provider := &explorationQueryProvider{ProductProvider: base}
			response := authenticatedProductRequest(boundaryHandler(t, provider), "/api/v1/logs?"+query)
			if response.Code != http.StatusBadRequest || len(provider.logs) != 0 {
				t.Fatalf("unsafe selection status %d; log calls %d", response.Code, len(provider.logs))
			}
		})
	}
	provider := &explorationQueryProvider{ProductProvider: base}
	response := authenticatedProductRequest(boundaryHandler(t, provider), "/api/v1/logs?resource_id="+id+"&previous=false&container="+strings.Repeat("a", 256))
	if response.Code != http.StatusOK || len(provider.logs) != 1 || provider.logs[0].Targets[0].Previous {
		t.Fatalf("bounded current container status %d", response.Code)
	}
}

func TestProductInventoryCursorsBindEveryNewFilter(t *testing.T) {
	handler := boundaryHandler(t, demo.MustNew())
	first := authenticatedProductRequest(handler, "/api/v1/resources?limit=1")
	var page resourcePageResponse
	if first.Code != http.StatusOK || json.Unmarshal(first.Body.Bytes(), &page) != nil || !page.Page.HasMore {
		t.Fatal("missing resource cursor fixture")
	}
	for _, filter := range []string{"provider=demo", "environment_id=env_other", "health=healthy", "availability=available", "scope_kind=project", "scope_name=shop"} {
		response := authenticatedProductRequest(handler, "/api/v1/resources?limit=1&cursor="+url.QueryEscape(page.Page.NextCursor)+"&"+filter)
		if response.Code != http.StatusBadRequest {
			t.Errorf("rebound filter %s status %d", filter, response.Code)
		}
	}
}

func TestProductTopologyCarriesBoundedRelationshipEvidence(t *testing.T) {
	response := authenticatedProductRequest(boundaryHandler(t, demo.MustNew()), "/api/v1/topology")
	var result relationshipPageResponse
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil || len(result.Items) == 0 {
		t.Fatal("missing topology fixture")
	}
	for _, edge := range result.Items {
		if len(edge.Evidence) < 1 || len(edge.Evidence) > domain.MaxRelationshipEvidence {
			t.Fatalf("evidence count %d", len(edge.Evidence))
		}
		for _, evidence := range edge.Evidence {
			if evidence.ID == "" || evidence.Provider != domain.ProviderDemo || evidence.SourceID != demo.SourceID || evidence.Provenance == "" || evidence.Certainty == "" || evidence.ObservedAt == "" || !evidence.Synthetic || evidence.Confidence < 0 || evidence.Confidence > 1 {
				t.Fatal("unsafe or incomplete evidence mapping")
			}
		}
	}
}

func TestProductUnfilteredEventsAndTopologyUseWholeSourcePages(t *testing.T) {
	for _, path := range []string{"/api/v1/events", "/api/v1/topology"} {
		t.Run(path, func(t *testing.T) {
			provider := &explorationQueryProvider{ProductProvider: demo.MustNew()}
			response := authenticatedProductRequest(boundaryHandler(t, provider), path+"?limit=25")
			if response.Code != http.StatusOK || len(provider.inventories) != 0 {
				t.Fatalf("source page status %d; unnecessary inventory reads %d", response.Code, len(provider.inventories))
			}
			if path == "/api/v1/events" {
				if len(provider.events) != 1 || provider.events[0].Resources != nil || provider.events[0].Page.Limit != 25 {
					t.Fatalf("events silently scoped to a resource prefix: %#v", provider.events)
				}
			} else if len(provider.relationships) != 1 || provider.relationships[0].Resources != nil || provider.relationships[0].Page.Limit != 25 {
				t.Fatalf("topology silently scoped to a resource prefix: %#v", provider.relationships)
			}
		})
	}
}

func TestProductSelectedEventsAndTopologyKeepExactResource(t *testing.T) {
	for _, path := range []string{"/api/v1/events", "/api/v1/topology"} {
		t.Run(path, func(t *testing.T) {
			provider := &explorationQueryProvider{ProductProvider: demo.MustNew()}
			resourceID := explorationResourceID(t, provider.ProductProvider)
			response := authenticatedProductRequest(boundaryHandler(t, provider), path+"?resource_id="+string(resourceID))
			if response.Code != http.StatusOK || len(provider.inventories) != 0 {
				t.Fatalf("selected page status %d; inventory calls %d", response.Code, len(provider.inventories))
			}
			var resources []domain.ResourceRef
			if path == "/api/v1/events" && len(provider.events) == 1 {
				resources = provider.events[0].Resources
			}
			if path == "/api/v1/topology" && len(provider.relationships) == 1 {
				resources = provider.relationships[0].Resources
			}
			if len(resources) != 1 || resources[0].ID != resourceID {
				t.Fatalf("selected resource lost: %#v", resources)
			}
		})
	}
}

func TestProductEmptyObservationReadsPreserveInventoryAvailability(t *testing.T) {
	for _, state := range []domain.AvailabilityState{domain.AvailabilityAvailable, domain.AvailabilityStale, domain.AvailabilityUnavailable, domain.AvailabilityPermissionDenied, domain.AvailabilityUnsupported} {
		for _, path := range []string{"/api/v1/logs", "/api/v1/metrics/current", "/api/v1/metrics/history"} {
			t.Run(path+"/"+string(state), func(t *testing.T) {
				status := explorationQueryStatus()
				status.State = state
				if state != domain.AvailabilityAvailable {
					status.ReasonCode = "fixture.inventory.unavailable"
				}
				if state == domain.AvailabilityStale {
					status.Completeness = domain.CompletenessPartial
				}
				provider := &explorationQueryProvider{ProductProvider: demo.MustNew(), inventoryStatus: &status}
				response := authenticatedProductRequest(boundaryHandler(t, provider), path)
				var result struct {
					Availability string            `json:"availability"`
					Completeness string            `json:"completeness"`
					Reason       string            `json:"reason_code"`
					Items        []json.RawMessage `json:"items"`
				}
				if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.Availability != string(status.State) || result.Completeness != string(status.Completeness) || result.Reason != string(status.ReasonCode) || result.Items == nil || len(result.Items) != 0 {
					t.Fatalf("empty source result wrong: status %d, response %#v", response.Code, result)
				}
				if len(provider.logs)+len(provider.current)+len(provider.histories) != 0 {
					t.Fatal("empty resource selection reached target-only adapter")
				}
			})
		}
	}
}

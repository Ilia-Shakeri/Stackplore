package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type readBoundaryProvider struct {
	ProductProvider
	lookupErr      error
	readErr        error
	resources      []domain.ResourceSnapshot
	detailCalls    int
	inventoryCalls int
	healthTargets  int
	healthResult   *domain.HealthResult
	metricStates   []domain.AvailabilityState
}

func (provider *readBoundaryProvider) ListResources(ctx context.Context, query domain.InventoryQuery) (domain.ResourcePage, error) {
	provider.inventoryCalls++
	if provider.lookupErr != nil {
		return domain.ResourcePage{}, provider.lookupErr
	}
	if provider.resources == nil {
		return provider.ProductProvider.ListResources(ctx, query)
	}
	items := provider.resources[:min(len(provider.resources), query.Page.Limit)]
	page := domain.PageInfo{Limit: query.Page.Limit}
	if len(provider.resources) > query.Page.Limit {
		page.HasMore, page.NextCursor = true, "fixture_next"
	}
	return domain.ResourcePage{Status: provider.resources[0].Status, Resources: items, Page: page}, nil
}

func (provider *readBoundaryProvider) GetResource(ctx context.Context, query domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	provider.detailCalls++
	if provider.lookupErr != nil {
		return domain.ResourceSnapshot{}, provider.lookupErr
	}
	for _, snapshot := range provider.resources {
		if snapshot.Resource.Ref.ID == query.ResourceID {
			return snapshot, nil
		}
	}
	return provider.ProductProvider.GetResource(ctx, query)
}

func (provider *readBoundaryProvider) GetHealth(ctx context.Context, query domain.HealthQuery) (domain.HealthResult, error) {
	provider.healthTargets = len(query.Resources)
	if provider.readErr != nil {
		return domain.HealthResult{}, provider.readErr
	}
	if provider.healthResult != nil {
		return *provider.healthResult, nil
	}
	if provider.resources != nil {
		status := provider.resources[0].Status
		result := domain.HealthResult{Status: status}
		for index, ref := range query.Resources {
			result.Snapshots = append(result.Snapshots, domain.HealthSnapshot{
				ResourceID: ref.ID, State: domain.HealthHealthy, Status: status,
				Evidence: []domain.HealthEvidence{{ID: domain.EvidenceID(fmt.Sprintf("evidence_fixture_%d", index)), Code: "fixture.health.ready", SourceCode: "fixture.health.ready", Status: domain.EvidencePositive, ObservedAt: status.ObservedAt, Synthetic: true}},
			})
		}
		return result, result.ValidateFor(query)
	}
	return provider.ProductProvider.GetHealth(ctx, query)
}

func (provider *readBoundaryProvider) GetRelationships(ctx context.Context, query domain.TopologyQuery) (domain.RelationshipPage, error) {
	if provider.readErr != nil {
		return domain.RelationshipPage{}, provider.readErr
	}
	if provider.resources != nil {
		return domain.RelationshipPage{Status: provider.resources[0].Status, Page: domain.PageInfo{Limit: query.Page.Limit}}, nil
	}
	return provider.ProductProvider.GetRelationships(ctx, query)
}

func (provider *readBoundaryProvider) ReadLogs(ctx context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	if provider.readErr != nil {
		return domain.LogPage{}, provider.readErr
	}
	if provider.resources != nil {
		return domain.LogPage{Status: provider.resources[0].Status, Page: domain.PageInfo{Limit: query.LineLimit}}, nil
	}
	return provider.ProductProvider.(domain.LogReadProvider).ReadLogs(ctx, query)
}

func (provider *readBoundaryProvider) GetCurrentMetrics(ctx context.Context, query domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
	if provider.readErr != nil {
		return domain.CurrentMetricResult{}, provider.readErr
	}
	if provider.resources == nil {
		return provider.ProductProvider.(domain.CurrentMetricProvider).GetCurrentMetrics(ctx, query)
	}
	status := provider.resources[0].Status
	result := domain.CurrentMetricResult{Status: status}
	for resourceIndex, ref := range query.Resources {
		set := domain.CurrentMetricSet{ResourceID: ref.ID, Status: status}
		if resourceIndex < len(provider.metricStates) {
			set.Status.State = provider.metricStates[resourceIndex]
			if set.Status.State != domain.AvailabilityAvailable {
				set.Status.ReasonCode = "fixture.metric.unavailable"
			}
		}
		if !set.Status.AllowsData() {
			result.Sets = append(result.Sets, set)
			continue
		}
		for nameIndex, name := range query.Names {
			unit := domain.MetricUnitCount
			if name == domain.MetricCPUUsageCores {
				unit = domain.MetricUnitCores
			} else if name == domain.MetricMemoryUsageBytes {
				unit = domain.MetricUnitBytes
			}
			set.Samples = append(set.Samples, domain.MetricSample{
				ID: domain.MetricSampleID(fmt.Sprintf("metric_fixture_%d_%d", resourceIndex, nameIndex)), ResourceID: ref.ID,
				Name: name, Unit: unit, Value: 1, SampledAt: status.ObservedAt, Synthetic: true,
			})
		}
		result.Sets = append(result.Sets, set)
	}
	return result, result.ValidateFor(query)
}

func (provider *readBoundaryProvider) GetMetricHistory(ctx context.Context, query domain.MetricHistoryQuery) (domain.MetricHistoryResult, error) {
	if provider.readErr != nil {
		return domain.MetricHistoryResult{}, provider.readErr
	}
	if provider.resources != nil {
		return domain.MetricHistoryResult{Status: provider.resources[0].Status}, nil
	}
	return provider.ProductProvider.(domain.MetricHistoryProvider).GetMetricHistory(ctx, query)
}

func (provider *readBoundaryProvider) ListEvents(ctx context.Context, query domain.EventQuery) (domain.EventPage, error) {
	if provider.readErr != nil {
		return domain.EventPage{}, provider.readErr
	}
	if provider.resources != nil {
		return domain.EventPage{Status: provider.resources[0].Status, Page: domain.PageInfo{Limit: query.Page.Limit}}, nil
	}
	return provider.ProductProvider.(domain.EventHistoryProvider).ListEvents(ctx, query)
}

func boundaryHandler(t *testing.T, provider ProductProvider) http.Handler {
	t.Helper()
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.ProductProviders = []ProductProvider{provider}
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{}, nil
		}}
	})
	return handler
}

func TestProductReadsRejectBodiesAndMalformedQueries(t *testing.T) {
	provider := demo.MustNew()
	page, err := provider.ListResources(context.Background(), domain.InventoryQuery{
		Source: providerRef(provider), SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	id := string(page.Resources[0].Resource.Ref.ID)
	for _, path := range []string{
		"/api/v1/sources", "/api/v1/environments", "/api/v1/resources", "/api/v1/resources/" + id,
		"/api/v1/resources/" + id + "/health", "/api/v1/topology", "/api/v1/logs",
		"/api/v1/metrics/current", "/api/v1/metrics/history", "/api/v1/events", "/api/v1/alerts",
	} {
		t.Run(path, func(t *testing.T) {
			for _, chunked := range []bool{false, true} {
				request := makeRequest(http.MethodGet, path, "198.51.100.10:4000")
				request.Body = http.NoBody
				request.ContentLength = 2
				if chunked {
					request.ContentLength = -1
					request.TransferEncoding = []string{"chunked"}
				}
				response := serve(boundaryHandler(t, provider), request)
				if response.Code != http.StatusBadRequest || response.Body.Len() == 0 {
					t.Fatalf("body rejection = %d, empty %v", response.Code, response.Body.Len() == 0)
				}
			}
		})
	}
	for _, path := range []string{
		"/api/v1/sources?unknown=value", "/api/v1/resources/" + id + "?unknown=value",
		"/api/v1/resources/" + id + "/health?unknown=value", "/api/v1/metrics/history?minutes=bad",
		"/api/v1/metrics/history?minutes=0", "/api/v1/metrics/history?source_id=source_missing",
		"/api/v1/metrics/history?resource_id=bad", "/api/v1/metrics/history?unknown=value",
		"/api/v1/resources?search=%GG", "/api/v1/resources?search=ok;bad", "/api/v1/resources?limit=1&limit=2",
	} {
		response := authenticatedProductRequest(boundaryHandler(t, provider), path)
		if response.Code != http.StatusBadRequest {
			t.Errorf("bad query %s = %d; want 400", path, response.Code)
		}
	}
}

func TestProductReadsPreserveProviderDenialAndOutage(t *testing.T) {
	base := demo.MustNew()
	page, _ := base.ListResources(context.Background(), domain.InventoryQuery{
		Source: providerRef(base), SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 1},
	})
	id := string(page.Resources[0].Resource.Ref.ID)
	for _, code := range []domain.ProviderErrorCode{domain.ProviderErrorPermissionDenied, domain.ProviderErrorUnavailable} {
		want := http.StatusForbidden
		if code == domain.ProviderErrorUnavailable {
			want = http.StatusServiceUnavailable
		}
		failure, _ := domain.NewProviderError("fixture.resource.read", code, code == domain.ProviderErrorUnavailable, errors.New("synthetic provider detail"))
		for _, lookup := range []bool{false, true} {
			provider := &readBoundaryProvider{ProductProvider: base, readErr: failure}
			paths := []string{
				"/api/v1/resources/" + id, "/api/v1/resources/" + id + "/health", "/api/v1/topology?resource_id=" + id,
				"/api/v1/logs?resource_id=" + id, "/api/v1/metrics/current?resource_id=" + id,
				"/api/v1/metrics/history?resource_id=" + id, "/api/v1/events?resource_id=" + id,
			}
			if lookup {
				provider.lookupErr = failure
				paths = append(paths, "/api/v1/resources")
			}
			for _, path := range paths {
				response := authenticatedProductRequest(boundaryHandler(t, provider), path)
				if !lookup && path == "/api/v1/resources/"+id {
					var resource resourceResponse
					if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &resource) != nil || string(resource.ID) != id || resource.Health == "" || strings.Contains(response.Body.String(), "synthetic provider detail") {
						t.Errorf("optional health failure lost known identity: status=%d resource=%#v", response.Code, resource)
					}
					continue
				}
				if response.Code != want || strings.Contains(response.Body.String(), "synthetic provider detail") {
					t.Errorf("read %s (lookup %v) = %d; want %d", path, lookup, response.Code, want)
				}
			}
		}
	}
}

func TestMetricRepliesStayBoundedAndExactResourceReadsSkipInventory(t *testing.T) {
	provider := fixtureReadProvider(t, 130)
	response := authenticatedProductRequest(boundaryHandler(t, provider), "/api/v1/metrics/current")
	var metrics metricResponseSet
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &metrics) != nil || len(metrics.Items) != domain.MaxPageSize ||
		metrics.Completeness != "partial" || metrics.ReasonCode != "metric.response_bound" {
		t.Fatalf("large metric reply = %d, items %d, state %s, reason %s", response.Code, len(metrics.Items), metrics.Completeness, metrics.ReasonCode)
	}
	provider.detailCalls, provider.inventoryCalls = 0, 0
	id := string(provider.resources[129].Resource.Ref.ID)
	for _, path := range []string{"topology", "logs", "metrics/current", "metrics/history", "events"} {
		response := authenticatedProductRequest(boundaryHandler(t, provider), "/api/v1/"+path+"?resource_id="+id)
		if response.Code != http.StatusOK {
			t.Errorf("exact read %s beyond first page = %d; want 200", path, response.Code)
		}
	}
	if provider.detailCalls != 5 || provider.inventoryCalls != 0 {
		t.Fatalf("exact reads used detail %d times and inventory %d times", provider.detailCalls, provider.inventoryCalls)
	}
}

func fixtureReadProvider(t *testing.T, count int) *readBoundaryProvider {
	t.Helper()
	provider := &readBoundaryProvider{ProductProvider: demo.MustNew()}
	for index := range count {
		identity, err := domain.NewResourceIdentity(domain.ProviderDemo, demo.SourceID, demo.EnvironmentID,
			"demo.service", domain.Scope{Kind: domain.ScopeProject, Name: "fixture"}, domain.NativeResourceID(fmt.Sprintf("resource_%03d", index)))
		if err != nil {
			t.Fatal(err)
		}
		ref, err := domain.NewResourceRef(identity)
		if err != nil {
			t.Fatal(err)
		}
		provider.resources = append(provider.resources, domain.ResourceSnapshot{
			Resource: domain.Resource{Ref: ref, DisplayName: fmt.Sprintf("Resource %03d", index), Synthetic: true},
			Status:   domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: time.Now().UTC().Truncate(time.Microsecond)},
		})
	}
	return provider
}

func TestSourceSummaryReportsBoundedCountsAndMissingHealth(t *testing.T) {
	for _, count := range []int{130, 260} {
		provider := fixtureReadProvider(t, count)
		response := authenticatedProductRequest(boundaryHandler(t, provider), "/api/v1/sources")
		var sources []sourceResponse
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &sources) != nil || len(sources) != 1 {
			t.Fatal("source summary fixture failed")
		}
		bounded := min(count, domain.MaxPageSize)
		if sources[0].Resources != bounded || sources[0].Completeness != "partial" || sources[0].Health.Healthy != domain.MaxHealthTargets ||
			sources[0].Health.Unknown != bounded-domain.MaxHealthTargets || provider.healthTargets != domain.MaxHealthTargets {
			t.Fatalf("bounded source summary = %#v, health targets %d", sources[0], provider.healthTargets)
		}
	}
	for _, denied := range []bool{false, true} {
		provider := fixtureReadProvider(t, 3)
		provider.healthResult = &domain.HealthResult{Status: provider.resources[0].Status}
		if denied {
			provider.readErr, _ = domain.NewProviderError("fixture.health.read", domain.ProviderErrorPermissionDenied, false, errors.New("fixture denial"))
		}
		response := authenticatedProductRequest(boundaryHandler(t, provider), "/api/v1/sources")
		var sources []sourceResponse
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &sources) != nil || len(sources) != 1 ||
			sources[0].Completeness != "partial" || sources[0].Health.Unknown != 3 || sources[0].Health.Healthy != 0 {
			t.Fatalf("missing source health = %d, %#v", response.Code, sources)
		}
	}
}

func TestCurrentMetricsRetainPartialAndWhollyDeniedStates(t *testing.T) {
	for _, test := range []struct {
		states       []domain.AvailabilityState
		availability string
		completeness string
		items        int
		reason       string
	}{
		{[]domain.AvailabilityState{domain.AvailabilityPermissionDenied, domain.AvailabilityAvailable}, "available", "partial", 3, "metric.partial_resources"},
		{[]domain.AvailabilityState{domain.AvailabilityPermissionDenied, domain.AvailabilityPermissionDenied}, "permission_denied", "complete", 0, "metric.resources_permission_denied"},
		{[]domain.AvailabilityState{domain.AvailabilityUnsupported, domain.AvailabilityUnsupported}, "unsupported", "complete", 0, "metric.resources_unsupported"},
		{[]domain.AvailabilityState{domain.AvailabilityUnavailable, domain.AvailabilityUnavailable}, "unavailable", "complete", 0, "metric.resources_unavailable"},
	} {
		provider := fixtureReadProvider(t, 2)
		provider.metricStates = test.states
		response := authenticatedProductRequest(boundaryHandler(t, provider), "/api/v1/metrics/current")
		var metrics metricResponseSet
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &metrics) != nil || metrics.Availability != test.availability ||
			metrics.Completeness != test.completeness || len(metrics.Items) != test.items || metrics.ReasonCode != test.reason {
			t.Fatalf("metric state = %d, %#v; want %s/%s", response.Code, metrics, test.availability, test.completeness)
		}
	}
}

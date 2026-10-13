package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
)

type observationFixture struct {
	store      *Store
	options    Options
	ctx        context.Context
	source     domain.SourceConfiguration
	token      domain.CaptureToken
	batch      domain.CaptureBatch
	completion domain.CaptureCompletion
	at         time.Time
}

func newObservationFixture(t *testing.T) observationFixture {
	t.Helper()
	store, options, _ := openIntegrationStore(t)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Microsecond)
	configuration := domain.SourceConfiguration{Source: domain.InfrastructureSource{ID: "source_capture", Provider: domain.ProviderDemo, Name: "Capture fixture", Synthetic: true},
		ProfileID: domain.SourceProfileDemoFixture, Enabled: true, Version: 1, CreatedAt: at.Add(-time.Minute), UpdatedAt: at.Add(-time.Minute)}
	if err := store.EnsureSourceConfigurations(ctx, []domain.SourceConfiguration{configuration}); err != nil {
		t.Fatalf("ensure observation source: %v", err)
	}
	token, err := store.BeginCapture(ctx, configuration.Source.ID, configuration.Version, at)
	if err != nil {
		t.Fatalf("begin observation capture: %v", err)
	}
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: at.Add(-2 * time.Second)}
	environment := domain.Environment{ID: "environment_capture", SourceID: configuration.Source.ID, Provider: domain.ProviderDemo, Name: "Capture environment", Status: status, Synthetic: true}
	batch := domain.CaptureBatch{Token: token, Environments: []domain.Environment{environment}}
	for index, name := range []string{"alpha", "beta", "gamma"} {
		identity, err := domain.NewResourceIdentity(domain.ProviderDemo, configuration.Source.ID, environment.ID, "demo.service", domain.Scope{Kind: domain.ScopeGlobal}, domain.NativeResourceID(name))
		if err != nil {
			t.Fatal("make observation resource identity")
		}
		ref, err := domain.NewResourceRef(identity)
		if err != nil {
			t.Fatal("make observation resource reference")
		}
		batch.Resources = append(batch.Resources, domain.ResourceSnapshot{Resource: domain.Resource{Ref: ref, DisplayName: name, Version: "version_one", Synthetic: true}, Status: status})
		batch.Health = append(batch.Health, domain.HealthSnapshot{ResourceID: ref.ID, State: domain.HealthHealthy, Status: status, Evidence: []domain.HealthEvidence{{ID: "evidence_healthy", Code: "health.ready", SourceCode: "demo.health", Status: domain.EvidencePositive, ObservedAt: status.ObservedAt, Synthetic: true}}})
		batch.Metrics = append(batch.Metrics, domain.MetricSample{ID: domain.MetricSampleID(fmt.Sprintf("metric_capture_%d", index)), ResourceID: ref.ID, Name: domain.MetricCPUUsageCores, Unit: domain.MetricUnitCores, Value: float64(index+1) / 2, SampledAt: at.Add(-5 * time.Second), Interval: 5 * time.Second, Synthetic: true})
		resourceID := ref.ID
		environmentID := environment.ID
		batch.Events = append(batch.Events, domain.InfrastructureEvent{ID: domain.EventID(fmt.Sprintf("event_capture_%d", index)), Provider: domain.ProviderDemo, SourceID: configuration.Source.ID, EnvironmentID: &environmentID, ResourceID: &resourceID,
			Kind: "demo.health.changed", Severity: domain.SeverityWarning, ReasonCode: "demo.health.changed", Summary: "Health changed", OccurredAt: at.Add(-8*time.Second + time.Duration(index)*time.Second), ObservedAt: status.ObservedAt, Synthetic: true})
	}
	batch.WorkloadReplicas = []domain.WorkloadReplicaSnapshot{{Resource: batch.Resources[0].Resource.Ref, Desired: 3, Ready: 2, Status: status}}
	relationship, err := domain.NewRelationship(batch.Resources[0].Resource.Ref.ID, batch.Resources[1].Resource.Ref.ID, domain.RelationshipDependsOn, domain.RelationshipSourceToTarget,
		[]domain.RelationshipEvidence{{ID: "evidence_relation", Provider: domain.ProviderDemo, SourceID: configuration.Source.ID, Provenance: domain.ProvenanceDeclaredDependency, Certainty: domain.EvidenceExplicit, Confidence: 1, ObservedAt: status.ObservedAt, Synthetic: true}})
	if err != nil {
		t.Fatal("make observation relationship")
	}
	batch.Relationships = []domain.Relationship{relationship}
	capabilities := domain.CapabilitySet{Source: token.Source}
	for _, name := range domain.CapabilityNames() {
		capabilities.Capabilities = append(capabilities.Capabilities, domain.CapabilityStatus{Name: name, Status: status})
	}
	completion := domain.CaptureCompletion{Token: token, Status: status, Capabilities: capabilities, Complete: true}
	if err := store.SaveCapture(ctx, batch); err != nil {
		t.Fatalf("save observation capture: %v", err)
	}
	if err := store.FinishCapture(ctx, completion); err != nil {
		t.Fatalf("finish observation capture: %v", err)
	}
	return observationFixture{store: store, options: options, ctx: ctx, source: configuration, token: token, batch: batch, completion: completion, at: at}
}

func TestObservationCaptureRoundTripAndRestart(t *testing.T) {
	fixture := newObservationFixture(t)
	store, ctx, source := fixture.store, fixture.ctx, fixture.token.Source
	environments, err := store.ListEnvironments(ctx, domain.EnvironmentQuery{Source: source, Page: domain.PageRequest{Limit: 10}})
	if err != nil || len(environments.Environments) != 1 {
		t.Fatalf("persisted environments: %v", err)
	}
	query := domain.InventoryQuery{Source: source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 10}}
	resources, err := store.ListResources(ctx, query)
	if err != nil || len(resources.Resources) != 3 || resources.Resources[0].Resource.DisplayName != "alpha" {
		t.Fatalf("persisted inventory: %v", err)
	}
	refs := []domain.ResourceRef{fixture.batch.Resources[0].Resource.Ref, fixture.batch.Resources[1].Resource.Ref, fixture.batch.Resources[2].Resource.Ref}
	health, err := store.GetHealth(ctx, domain.HealthQuery{Source: source, Resources: refs})
	if err != nil || len(health.Snapshots) != 3 || len(health.Snapshots[0].Evidence) != 1 {
		t.Fatalf("persisted health: %v", err)
	}
	relationships, err := store.GetRelationships(ctx, domain.TopologyQuery{Source: source, Page: domain.PageRequest{Limit: 10}})
	if err != nil || len(relationships.Relationships) != 1 || relationships.Relationships[0].Evidence[0].Confidence != 1 {
		t.Fatalf("persisted topology: %v", err)
	}
	metricQuery := domain.CurrentMetricQuery{Source: source, Resources: refs, Names: []domain.MetricName{domain.MetricCPUUsageCores}}
	metrics, err := store.GetCurrentMetrics(ctx, metricQuery)
	if err != nil || len(metrics.Sets) != 3 || metrics.Sets[0].Samples[0].Value != 0.5 {
		t.Fatalf("persisted current metrics: %v", err)
	}
	history, err := store.GetMetricHistory(ctx, domain.MetricHistoryQuery{Source: source, Resources: refs, Names: metricQuery.Names, Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: fixture.at}, PointLimit: 10})
	if err != nil || len(history.Samples) != 3 {
		t.Fatalf("persisted metric history: %v", err)
	}
	events, err := store.ListEvents(ctx, domain.EventQuery{Source: source, Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: fixture.at}, Page: domain.PageRequest{Limit: 10}})
	if err != nil || len(events.Events) != 3 {
		t.Fatalf("persisted events: %v", err)
	}
	workload, err := store.GetWorkloadReplicas(ctx, domain.ResourceDetailQuery{Source: source, ResourceID: refs[0].ID})
	if err != nil || workload.Desired != 3 || workload.Ready != 2 {
		t.Fatalf("persisted workload replicas: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal("close capture fixture")
	}
	restarted, err := Open(ctx, fixture.options, postgresmigrations.FS)
	if err != nil {
		t.Fatalf("restart capture store: %v", err)
	}
	defer restarted.Close()
	resource, err := restarted.GetResource(ctx, domain.ResourceDetailQuery{Source: source, ResourceID: refs[0].ID})
	if err != nil || resource.Resource.Version != "version_one" || resource.Resource.DisplayName != "alpha" {
		t.Fatalf("capture restart lost detail: %v", err)
	}
}

func TestObservationKeysetKeepsImmutableSortValueAfterAnchorChanges(t *testing.T) {
	fixture := newObservationFixture(t)
	query := domain.InventoryQuery{Source: fixture.token.Source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 1}}
	first, err := fixture.store.ListResources(fixture.ctx, query)
	if err != nil || !first.Page.HasMore || first.Resources[0].Resource.DisplayName != "alpha" {
		t.Fatalf("first inventory page: %v", err)
	}
	anchor := first.Resources[0].Resource.Ref.ID
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE resource_snapshots SET display_name='zzzz' WHERE resource_id=$1`, string(anchor)); err != nil {
		t.Fatal("rename keyset fixture anchor")
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE resources SET retired_at_us=last_seen_at_us WHERE id=$1`, string(anchor)); err != nil {
		t.Fatal("retire keyset fixture anchor")
	}
	query.Page.Cursor = first.Page.NextCursor
	second, err := fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(second.Resources) != 1 || second.Resources[0].Resource.DisplayName != "beta" {
		t.Fatalf("immutable sort continuation: %v", err)
	}
	query.Filter.Search = "gamma"
	if _, err := fixture.store.ListResources(fixture.ctx, query); !errors.Is(err, domain.ErrRepositoryInvalidInput) {
		t.Fatal("cursor accepted changed query")
	}
}

func TestObservationCursorAcceptsMaximumUnicodeName(t *testing.T) {
	fixture := newObservationFixture(t)
	var name strings.Builder
	for index := range 256 {
		name.WriteRune(rune(0x10000 + index*3901))
	}
	value := name.String()
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE resource_snapshots SET display_name=$1`, value); err != nil {
		t.Fatal("seed maximum Unicode names")
	}
	query := domain.InventoryQuery{Source: fixture.token.Source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 1}}
	first, err := fixture.store.ListResources(fixture.ctx, query)
	if err != nil || !first.Page.HasMore || len(first.Page.NextCursor) > domain.MaxPageCursorBytes {
		t.Fatalf("maximum Unicode cursor: %v", err)
	}
	query.Page.Cursor = first.Page.NextCursor
	second, err := fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(second.Resources) != 1 || first.Resources[0].Resource.Ref.ID == second.Resources[0].Resource.Ref.ID {
		t.Fatalf("maximum Unicode continuation: %v", err)
	}
}

func TestPartialCapturePreservesUnseenAndFullCaptureHidesBeforePrune(t *testing.T) {
	fixture := newObservationFixture(t)
	query := domain.InventoryQuery{Source: fixture.token.Source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 10}}
	for cycle, complete := range []bool{false, true} {
		token, err := fixture.store.BeginCapture(fixture.ctx, fixture.source.Source.ID, 1, fixture.at.Add(time.Duration(cycle+1)*time.Second))
		if err != nil {
			t.Fatal("begin subsequent capture")
		}
		batch := domain.CaptureBatch{Token: token, Environments: fixture.batch.Environments, Resources: fixture.batch.Resources[:1]}
		if err := fixture.store.SaveCapture(fixture.ctx, batch); err != nil {
			t.Fatal("save subsequent capture")
		}
		completion := fixture.completion
		completion.Token = token
		completion.Complete = complete
		if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
			t.Fatal("finish subsequent capture")
		}
		page, err := fixture.store.ListResources(fixture.ctx, query)
		want := 3
		if complete {
			want = 1
		}
		if err != nil || len(page.Resources) != want {
			t.Fatalf("capture visibility complete=%t: %v", complete, err)
		}
		if !complete {
			continue
		}
		if retired, err := fixture.store.RetireUnseen(fixture.ctx, token, 1); err != nil || retired != 1 {
			t.Fatalf("bounded retirement: %v", err)
		}
		page, err = fixture.store.ListResources(fixture.ctx, query)
		if err != nil || len(page.Resources) != 1 {
			t.Fatal("pending retirement exposed unseen resources")
		}
	}
}

func TestCaptureRejectsChangedOrDisabledConfigurationAndConflictingHistory(t *testing.T) {
	fixture := newObservationFixture(t)
	token, err := fixture.store.BeginCapture(fixture.ctx, fixture.source.Source.ID, 1, fixture.at.Add(time.Second))
	if err != nil {
		t.Fatal("begin duplicate capture")
	}
	batch := domain.CaptureBatch{Token: token, Metrics: fixture.batch.Metrics, Events: fixture.batch.Events}
	for index := range batch.Events {
		batch.Events[index].ObservedAt = fixture.at
	}
	if err := fixture.store.SaveCapture(fixture.ctx, batch); err != nil {
		t.Fatalf("same event facts with later receipt: %v", err)
	}
	batch.Metrics = append([]domain.MetricSample(nil), batch.Metrics...)
	batch.Metrics[0].Value = 99
	if err := fixture.store.SaveCapture(fixture.ctx, batch); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("conflicting immutable metric payload accepted")
	}
	batch.Metrics = nil
	batch.Events = append([]domain.InfrastructureEvent(nil), batch.Events...)
	batch.Events[0].Summary = "Changed fact"
	if err := fixture.store.SaveCapture(fixture.ctx, batch); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("conflicting immutable event payload accepted")
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE infrastructure_sources SET row_version=row_version+1,enabled=0 WHERE id=$1`, string(fixture.source.Source.ID)); err != nil {
		t.Fatal("disable capture fixture source")
	}
	batch.Events = fixture.batch.Events
	if err := fixture.store.SaveCapture(fixture.ctx, batch); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("disabled stale capture committed")
	}
	completion := fixture.completion
	completion.Token = token
	if err := fixture.store.FinishCapture(fixture.ctx, completion); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("disabled stale capture finalized")
	}
}

func TestSourcePermissionAndUnsupportedStatesBlockPersistedData(t *testing.T) {
	fixture := newObservationFixture(t)
	query := domain.InventoryQuery{Source: fixture.token.Source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 10}}
	refs := []domain.ResourceRef{fixture.batch.Resources[0].Resource.Ref}
	for _, state := range []domain.AvailabilityState{domain.AvailabilityPermissionDenied, domain.AvailabilityUnsupported} {
		if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE source_status SET availability=$1,completeness='complete',reason_code='source.blocked' WHERE source_id=$2`, string(state), string(fixture.source.Source.ID)); err != nil {
			t.Fatal("block source status")
		}
		status, err := fixture.store.GetObservationStatus(fixture.ctx, fixture.token.Source)
		if err != nil || status.State != state || status.AllowsData() {
			t.Fatal("blocked source became stale readable")
		}
		page, err := fixture.store.ListResources(fixture.ctx, query)
		if err != nil || len(page.Resources) != 0 || page.Status.State != state {
			t.Fatal("blocked source returned inventory")
		}
		var providerErr *domain.ProviderError
		if _, err := fixture.store.GetResource(fixture.ctx, domain.ResourceDetailQuery{Source: fixture.token.Source, ResourceID: refs[0].ID}); !errors.As(err, &providerErr) {
			t.Fatal("blocked source returned detail")
		}
		health, err := fixture.store.GetHealth(fixture.ctx, domain.HealthQuery{Source: fixture.token.Source, Resources: refs})
		if err != nil || len(health.Snapshots) != 0 || health.Status.State != state {
			t.Fatal("blocked source returned health")
		}
		metrics, err := fixture.store.GetCurrentMetrics(fixture.ctx, domain.CurrentMetricQuery{Source: fixture.token.Source, Resources: refs, Names: []domain.MetricName{domain.MetricCPUUsageCores}})
		if err != nil || len(metrics.Sets) != 0 || metrics.Status.State != state {
			t.Fatal("blocked source returned metrics")
		}
		capabilities, err := fixture.store.GetCapabilities(fixture.ctx, fixture.token.Source)
		if err != nil {
			t.Fatal("blocked capabilities read failed")
		}
		for _, capability := range capabilities.Capabilities {
			if capability.Status.State != state {
				t.Fatal("blocked source kept live capabilities")
			}
		}
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE source_status SET availability='unavailable',reason_code='source.outage' WHERE source_id=$1`, string(fixture.source.Source.ID)); err != nil {
		t.Fatal("set outage source status")
	}
	page, err := fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(page.Resources) != 3 || page.Status.State != domain.AvailabilityStale {
		t.Fatal("outage did not retain stale inventory")
	}
	workload, err := fixture.store.GetWorkloadReplicas(fixture.ctx, domain.ResourceDetailQuery{Source: fixture.token.Source, ResourceID: refs[0].ID})
	if err != nil || workload.Status.State != domain.AvailabilityStale {
		t.Fatal("outage workload replicas remained live")
	}
}

func TestObservationInventoryAllSortsAndFilters(t *testing.T) {
	fixture := newObservationFixture(t)
	for _, field := range []domain.ResourceSortField{domain.ResourceSortName, domain.ResourceSortKind, domain.ResourceSortObservedAt} {
		for _, direction := range []domain.SortDirection{domain.SortAscending, domain.SortDescending} {
			query := domain.InventoryQuery{Source: fixture.token.Source, SortField: field, SortDirection: direction, Page: domain.PageRequest{Limit: 1}}
			ids := []string{}
			for range 4 {
				page, err := fixture.store.ListResources(fixture.ctx, query)
				if err != nil {
					t.Fatalf("inventory field=%s direction=%s: %v", field, direction, err)
				}
				for _, resource := range page.Resources {
					ids = append(ids, string(resource.Resource.Ref.ID))
				}
				if !page.Page.HasMore {
					break
				}
				query.Page.Cursor = page.Page.NextCursor
			}
			sort.Strings(ids)
			if len(ids) != 3 || ids[0] == ids[1] || ids[1] == ids[2] {
				t.Fatal("inventory pagination skipped or repeated rows")
			}
		}
	}
	query := domain.InventoryQuery{Source: fixture.token.Source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Filter: domain.ResourceFilter{Search: "BETA", Kinds: []domain.ResourceKind{"demo.service"}, Scopes: []domain.Scope{{Kind: domain.ScopeGlobal}}}, Page: domain.PageRequest{Limit: 10}}
	page, err := fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(page.Resources) != 1 || page.Resources[0].Resource.DisplayName != "beta" {
		t.Fatalf("inventory filters: %v", err)
	}
}

func TestObservationSearchUsesDomainUnicodeCaseRules(t *testing.T) {
	fixture := newObservationFixture(t)
	for _, scenario := range []struct{ name, search string }{{"ΒΕΤΑ", "βετα"}, {"ПРОВЕРКА", "проверка"}, {"İSTANBUL", "istanbul"}} {
		if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE resource_snapshots SET display_name=$1 WHERE resource_id=$2`, scenario.name, string(fixture.batch.Resources[0].Resource.Ref.ID)); err != nil {
			t.Fatal("seed Unicode case-search name")
		}
		query := domain.InventoryQuery{Source: fixture.token.Source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Filter: domain.ResourceFilter{Search: scenario.search}, Page: domain.PageRequest{Limit: 10}}
		page, err := fixture.store.ListResources(fixture.ctx, query)
		if err != nil || len(page.Resources) != 1 || page.Resources[0].Resource.DisplayName != scenario.name {
			t.Fatalf("Unicode domain search mismatch for %s: %v", scenario.name, err)
		}
	}
}

func TestObservationHealthHistoryStoresChangesAndRefreshesCurrent(t *testing.T) {
	fixture := newObservationFixture(t)
	token, err := fixture.store.BeginCapture(fixture.ctx, fixture.source.Source.ID, 1, fixture.at.Add(2*time.Second))
	if err != nil {
		t.Fatal("begin health refresh capture")
	}
	health := fixture.batch.Health[0]
	health.Evidence = append([]domain.HealthEvidence(nil), health.Evidence...)
	health.Status.ObservedAt = fixture.at.Add(time.Second)
	health.Evidence[0].ObservedAt = health.Status.ObservedAt
	if err := fixture.store.SaveCapture(fixture.ctx, domain.CaptureBatch{Token: token, Health: []domain.HealthSnapshot{health}}); err != nil {
		t.Fatal("refresh unchanged health")
	}
	var count int
	var currentTime int64
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT
(SELECT count(*) FROM health_history),(SELECT observed_at_us FROM resource_health_current WHERE resource_id=$1)`, string(health.ResourceID)).Scan(&count, &currentTime); err != nil {
		t.Fatal("inspect refreshed health history")
	}
	if count != 3 || currentTime != health.Status.ObservedAt.UnixMicro() {
		t.Fatal("unchanged health grew history or failed to refresh current observation")
	}
	health.State = domain.HealthDegraded
	health.Status.ReasonCode = "health.degraded"
	if err := fixture.store.SaveCapture(fixture.ctx, domain.CaptureBatch{Token: token, Health: []domain.HealthSnapshot{health}}); err != nil {
		t.Fatal("save changed health")
	}
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM health_history`).Scan(&count); err != nil || count != 4 {
		t.Fatal("changed health failed to append one history point")
	}
	if err := fixture.store.SaveCapture(fixture.ctx, domain.CaptureBatch{Token: token, Health: []domain.HealthSnapshot{health}}); err != nil {
		t.Fatal("repeat same changed health")
	}
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM health_history`).Scan(&count); err != nil || count != 4 {
		t.Fatal("repeated health change grew history")
	}
}

func TestObservationCapturePreservesPayloadAndPortClocks(t *testing.T) {
	fixture := newObservationFixture(t)
	status, err := fixture.store.GetObservationStatus(fixture.ctx, fixture.token.Source)
	if err != nil || !status.ObservedAt.Equal(fixture.completion.Status.ObservedAt) {
		t.Fatal("capture replaced inventory observation with write time")
	}
	token, err := fixture.store.BeginCapture(fixture.ctx, fixture.source.Source.ID, 1, fixture.at.Add(time.Second))
	if err != nil {
		t.Fatal("begin separate metric-clock capture")
	}
	sample := fixture.batch.Metrics[0]
	sample.ID = "metric_capture_later"
	sample.SampledAt = fixture.at.Add(-time.Second)
	if err := fixture.store.SaveCapture(fixture.ctx, domain.CaptureBatch{Token: token, Metrics: []domain.MetricSample{sample}}); err != nil {
		t.Fatal("save later metric observation")
	}
	completion := fixture.completion
	completion.Token = token
	completion.Complete = false
	completion.Capabilities.Capabilities = append([]domain.CapabilityStatus(nil), completion.Capabilities.Capabilities...)
	for index := range completion.Capabilities.Capabilities {
		if completion.Capabilities.Capabilities[index].Name == domain.CapabilityMetricCurrent || completion.Capabilities.Capabilities[index].Name == domain.CapabilityMetricHistory {
			completion.Capabilities.Capabilities[index].Status.ObservedAt = sample.SampledAt
		}
	}
	if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
		t.Fatal("finish separate metric-clock capture")
	}
	current, err := fixture.store.GetCurrentMetrics(fixture.ctx, domain.CurrentMetricQuery{Source: token.Source, Resources: []domain.ResourceRef{fixture.batch.Resources[0].Resource.Ref}, Names: []domain.MetricName{sample.Name}})
	if err != nil || len(current.Sets) != 1 || len(current.Sets[0].Samples) != 1 || current.Sets[0].Samples[0].ID != sample.ID || !current.Status.ObservedAt.Equal(sample.SampledAt) {
		t.Fatal("later metric port used earlier inventory clock or write time")
	}
}

func TestObservationCaptureInProgressAndAbortedWriterStayPartial(t *testing.T) {
	fixture := newObservationFixture(t)
	token, err := fixture.store.BeginCapture(fixture.ctx, fixture.source.Source.ID, 1, fixture.at.Add(time.Second))
	if err != nil {
		t.Fatal("begin visible capture attempt")
	}
	query := domain.InventoryQuery{Source: token.Source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 10}}
	page, err := fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(page.Resources) != 3 || page.Status.State != domain.AvailabilityStale || page.Status.Completeness != domain.CompletenessPartial || page.Status.ReasonCode != "source.collection_in_progress" {
		t.Fatal("active capture advertised old complete inventory")
	}
	updated := fixture.batch.Resources[0]
	updated.Resource.DisplayName = "alpha refreshed"
	updated.Status.ObservedAt = token.ObservedAt
	if err := fixture.store.SaveCapture(fixture.ctx, domain.CaptureBatch{Token: token, Resources: []domain.ResourceSnapshot{updated}}); err != nil {
		t.Fatal("save one current row before finish")
	}
	page, err = fixture.store.ListResources(fixture.ctx, query)
	if err != nil || len(page.Resources) != 3 || !page.Status.ObservedAt.Equal(token.ObservedAt) || page.Status.Completeness != domain.CompletenessPartial {
		t.Fatal("mixed current rows failed validation or claimed completeness")
	}
	if err := fixture.store.Close(); err != nil {
		t.Fatal("stop unfinished capture writer")
	}
	restarted, err := Open(fixture.ctx, fixture.options, postgresmigrations.FS)
	if err != nil {
		t.Fatal("restart unfinished capture store")
	}
	defer restarted.Close()
	page, err = restarted.ListResources(fixture.ctx, query)
	if err != nil || len(page.Resources) != 3 || page.Status.State != domain.AvailabilityStale || page.Status.Completeness != domain.CompletenessPartial {
		t.Fatal("aborted capture became complete after restart")
	}
	recovery, err := restarted.BeginCapture(fixture.ctx, fixture.source.Source.ID, 1, fixture.at.Add(2*time.Second))
	if err != nil {
		t.Fatal("begin capture recovery")
	}
	if err := restarted.SaveCapture(fixture.ctx, domain.CaptureBatch{Token: token, Resources: []domain.ResourceSnapshot{updated}}); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("superseded writer committed after restart")
	}
	recovered := fixture.batch
	recovered.Token = recovery
	recovered.Resources = append([]domain.ResourceSnapshot(nil), recovered.Resources...)
	for index := range recovered.Resources {
		recovered.Resources[index].Status.ObservedAt = recovery.ObservedAt
	}
	if err := restarted.SaveCapture(fixture.ctx, recovered); err != nil {
		t.Fatal("save complete recovery capture")
	}
	completion := fixture.completion
	completion.Token = recovery
	completion.Status.ObservedAt = recovery.ObservedAt
	if err := restarted.FinishCapture(fixture.ctx, completion); err != nil {
		t.Fatal("finish complete recovery capture")
	}
	page, err = restarted.ListResources(fixture.ctx, query)
	if err != nil || len(page.Resources) != 3 || page.Status.State != domain.AvailabilityAvailable || page.Status.Completeness != domain.CompletenessComplete {
		t.Fatal("finished recovery did not clear partial attempt state")
	}
}

func TestObservationInitialPartialRowsAndDeniedCaptureStates(t *testing.T) {
	fixture := newObservationFixture(t)
	configuration := fixture.source
	configuration.Source.ID = "source_initial_capture"
	if err := fixture.store.EnsureSourceConfigurations(fixture.ctx, []domain.SourceConfiguration{configuration}); err != nil {
		t.Fatal("seed initial capture source")
	}
	token, err := fixture.store.BeginCapture(fixture.ctx, configuration.Source.ID, 1, fixture.at)
	if err != nil {
		t.Fatal("begin initial capture")
	}
	status, err := fixture.store.GetObservationStatus(fixture.ctx, token.Source)
	if err != nil || status.State != domain.AvailabilityUnavailable || status.AllowsData() {
		t.Fatal("empty initial capture returned available data")
	}
	environment := fixture.batch.Environments[0]
	environment.ID = "environment_initial_capture"
	environment.SourceID = configuration.Source.ID
	if err := fixture.store.SaveCapture(fixture.ctx, domain.CaptureBatch{Token: token, Environments: []domain.Environment{environment}}); err != nil {
		t.Fatal("save initial environment chunk")
	}
	status, err = fixture.store.GetObservationStatus(fixture.ctx, token.Source)
	if err != nil || status.State != domain.AvailabilityStale || status.Completeness != domain.CompletenessPartial {
		t.Fatal("initial bounded rows advertised complete inventory")
	}
	page, err := fixture.store.ListEnvironments(fixture.ctx, domain.EnvironmentQuery{Source: token.Source, Page: domain.PageRequest{Limit: 10}})
	if err != nil || len(page.Environments) != 1 || page.Status.State != domain.AvailabilityStale || page.Status.Completeness != domain.CompletenessPartial {
		t.Fatal("initial persisted environment chunk could not be read as partial")
	}
	capabilities, err := fixture.store.GetCapabilities(fixture.ctx, token.Source)
	if err != nil {
		t.Fatal("read first partial capture capabilities")
	}
	for _, capability := range capabilities.Capabilities {
		if capability.Name == domain.CapabilityInventoryList && (capability.Status.State != domain.AvailabilityStale || capability.Status.Completeness != domain.CompletenessPartial) {
			t.Fatal("first partial chunk capability differs from reader")
		}
		if capability.Name == domain.CapabilityInventoryDetail && capability.Status.State != domain.AvailabilityUnsupported {
			t.Fatal("first environment chunk invented resource detail capability")
		}
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE source_status SET availability='permission_denied',completeness='complete',reason_code='source.denied' WHERE source_id=$1`, string(fixture.source.Source.ID)); err != nil {
		t.Fatal("deny cached fixture")
	}
	if _, err := fixture.store.BeginCapture(fixture.ctx, fixture.source.Source.ID, 1, fixture.at.Add(time.Second)); err != nil {
		t.Fatal("begin denied retry attempt")
	}
	status, err = fixture.store.GetObservationStatus(fixture.ctx, fixture.token.Source)
	if err != nil || status.State != domain.AvailabilityPermissionDenied || status.AllowsData() {
		t.Fatal("capture attempt bypassed denied source state")
	}
}

func TestObservationPortPermissionOverridesOldCache(t *testing.T) {
	fixture := newObservationFixture(t)
	refs := []domain.ResourceRef{fixture.batch.Resources[0].Resource.Ref}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE source_capabilities
SET availability='permission_denied',completeness='complete',reason_code='source.port_denied' WHERE source_id=$1`, string(fixture.source.Source.ID)); err != nil {
		t.Fatal("deny observation ports")
	}
	inventory, err := fixture.store.ListResources(fixture.ctx, domain.InventoryQuery{Source: fixture.token.Source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 10}})
	if err != nil || inventory.Status.State != domain.AvailabilityPermissionDenied || len(inventory.Resources) != 0 {
		t.Fatal("denied inventory port returned old cache")
	}
	environments, err := fixture.store.ListEnvironments(fixture.ctx, domain.EnvironmentQuery{Source: fixture.token.Source, Page: domain.PageRequest{Limit: 10}})
	if err != nil || environments.Status.State != domain.AvailabilityPermissionDenied || len(environments.Environments) != 0 {
		t.Fatal("denied inventory port returned environments")
	}
	var providerErr *domain.ProviderError
	_, err = fixture.store.GetWorkloadReplicas(fixture.ctx, domain.ResourceDetailQuery{Source: fixture.token.Source, ResourceID: refs[0].ID})
	if !errors.As(err, &providerErr) || providerErr.Code != domain.ProviderErrorPermissionDenied {
		t.Fatal("denied detail port returned workload replicas")
	}
	health, err := fixture.store.GetHealth(fixture.ctx, domain.HealthQuery{Source: fixture.token.Source, Resources: refs})
	if err != nil || health.Status.State != domain.AvailabilityPermissionDenied || len(health.Snapshots) != 0 {
		t.Fatal("denied health port returned old cache")
	}
	topology, err := fixture.store.GetRelationships(fixture.ctx, domain.TopologyQuery{Source: fixture.token.Source, Page: domain.PageRequest{Limit: 10}})
	if err != nil || topology.Status.State != domain.AvailabilityPermissionDenied || len(topology.Relationships) != 0 {
		t.Fatal("denied topology port returned old cache")
	}
	metrics, err := fixture.store.GetCurrentMetrics(fixture.ctx, domain.CurrentMetricQuery{Source: fixture.token.Source, Resources: refs, Names: []domain.MetricName{domain.MetricCPUUsageCores}})
	if err != nil || metrics.Status.State != domain.AvailabilityPermissionDenied || len(metrics.Sets) != 0 {
		t.Fatal("denied metric port returned old cache")
	}
	history, err := fixture.store.GetMetricHistory(fixture.ctx, domain.MetricHistoryQuery{Source: fixture.token.Source, Resources: refs, Names: []domain.MetricName{domain.MetricCPUUsageCores}, Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: fixture.at}, PointLimit: 10})
	if err != nil || history.Status.State != domain.AvailabilityPermissionDenied || len(history.Samples) != 0 {
		t.Fatal("denied metric history port returned old cache")
	}
	events, err := fixture.store.ListEvents(fixture.ctx, domain.EventQuery{Source: fixture.token.Source, Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: fixture.at}, Page: domain.PageRequest{Limit: 10}})
	if err != nil || events.Status.State != domain.AvailabilityPermissionDenied || len(events.Events) != 0 {
		t.Fatal("denied event history port returned old cache")
	}
}

func TestObservationOutageAndDisableMarkAllCachedPortsStale(t *testing.T) {
	fixture := newObservationFixture(t)
	refs := []domain.ResourceRef{fixture.batch.Resources[0].Resource.Ref}
	for _, disabled := range []bool{false, true} {
		if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE source_status SET availability='unavailable',reason_code='source.outage' WHERE source_id=$1`, string(fixture.source.Source.ID)); err != nil {
			t.Fatal("set observation outage")
		}
		if disabled {
			if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE infrastructure_sources SET enabled=0 WHERE id=$1`, string(fixture.source.Source.ID)); err != nil {
				t.Fatal("disable cached observation source")
			}
		}
		inventory, err := fixture.store.ListResources(fixture.ctx, domain.InventoryQuery{Source: fixture.token.Source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 10}})
		if err != nil || len(inventory.Resources) != 3 || inventory.Status.State != domain.AvailabilityStale || inventory.Resources[0].Status.State != domain.AvailabilityStale {
			t.Fatal("outage inventory or nested resource stayed live")
		}
		environments, err := fixture.store.ListEnvironments(fixture.ctx, domain.EnvironmentQuery{Source: fixture.token.Source, Page: domain.PageRequest{Limit: 10}})
		if err != nil || len(environments.Environments) != 1 || environments.Environments[0].Status.State != domain.AvailabilityStale {
			t.Fatal("outage environment stayed live")
		}
		health, err := fixture.store.GetHealth(fixture.ctx, domain.HealthQuery{Source: fixture.token.Source, Resources: refs})
		if err != nil || len(health.Snapshots) != 1 || health.Status.State != domain.AvailabilityStale || health.Snapshots[0].Status.State != domain.AvailabilityStale {
			t.Fatal("outage health stayed live")
		}
		metrics, err := fixture.store.GetCurrentMetrics(fixture.ctx, domain.CurrentMetricQuery{Source: fixture.token.Source, Resources: refs, Names: []domain.MetricName{domain.MetricCPUUsageCores}})
		if err != nil || len(metrics.Sets) != 1 || metrics.Status.State != domain.AvailabilityStale || metrics.Sets[0].Status.State != domain.AvailabilityStale {
			t.Fatal("outage metrics stayed live")
		}
		capabilities, err := fixture.store.GetCapabilities(fixture.ctx, fixture.token.Source)
		if err != nil {
			t.Fatal("read outage capabilities")
		}
		for _, capability := range capabilities.Capabilities {
			if capability.Status.State != domain.AvailabilityStale {
				t.Fatal("outage capability stayed live")
			}
		}
	}
}

func TestObservationCachedHistoryKeepsPartialReasonAndCapabilityTruth(t *testing.T) {
	fixture := newObservationFixture(t)
	metricQuery := domain.MetricHistoryQuery{Source: fixture.token.Source, Resources: []domain.ResourceRef{fixture.batch.Resources[0].Resource.Ref}, Names: []domain.MetricName{domain.MetricCPUUsageCores},
		Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: fixture.at}, PointLimit: 10}
	eventQuery := domain.EventQuery{Source: fixture.token.Source, Range: metricQuery.Range, Page: domain.PageRequest{Limit: 10}}
	for _, scenario := range []struct {
		state, want      domain.AvailabilityState
		completeness     domain.Completeness
		wantCompleteness domain.Completeness
		reason           string
	}{
		{domain.AvailabilityAvailable, domain.AvailabilityAvailable, domain.CompletenessPartial, domain.CompletenessPartial, "observation.history_bound"},
		{domain.AvailabilityUnavailable, domain.AvailabilityStale, domain.CompletenessComplete, domain.CompletenessPartial, "observation.provider_unavailable"},
		{domain.AvailabilityUnsupported, domain.AvailabilityAvailable, domain.CompletenessComplete, domain.CompletenessComplete, "observation.history_unsupported"},
	} {
		if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE source_capabilities SET availability=$1,completeness=$2,reason_code=$3
WHERE source_id=$4 AND capability_name IN ('metric.history','event.history')`, string(scenario.state), string(scenario.completeness), scenario.reason, string(fixture.source.Source.ID)); err != nil {
			t.Fatal("set capped or unavailable history capability")
		}
		metrics, err := fixture.store.GetMetricHistory(fixture.ctx, metricQuery)
		if err != nil || len(metrics.Samples) != 1 || metrics.Status.State != scenario.want || metrics.Status.Completeness != scenario.wantCompleteness {
			t.Fatalf("cached metric history lost partial reason or state: %v", err)
		}
		events, err := fixture.store.ListEvents(fixture.ctx, eventQuery)
		if err != nil || len(events.Events) != 3 || events.Status.State != scenario.want || events.Status.Completeness != scenario.wantCompleteness {
			t.Fatalf("cached event history lost partial reason or state: %v", err)
		}
		capabilities, err := fixture.store.GetCapabilities(fixture.ctx, fixture.token.Source)
		if err != nil {
			t.Fatal("read cached history capabilities")
		}
		for _, capability := range capabilities.Capabilities {
			if capability.Name != domain.CapabilityMetricHistory && capability.Name != domain.CapabilityEventHistory {
				continue
			}
			if capability.Status.State != scenario.want || capability.Status.Completeness != scenario.wantCompleteness {
				t.Fatal("history capability differs from persisted reader status")
			}
		}
	}
	emptyRange := domain.TimeRange{Start: fixture.at.Add(time.Second), End: fixture.at.Add(2 * time.Second)}
	metricQuery.Range, eventQuery.Range = emptyRange, emptyRange
	metrics, err := fixture.store.GetMetricHistory(fixture.ctx, metricQuery)
	if err != nil || len(metrics.Samples) != 0 || metrics.Status.State != domain.AvailabilityAvailable {
		t.Fatal("empty metric range lost local history capability")
	}
	events, err := fixture.store.ListEvents(fixture.ctx, eventQuery)
	if err != nil || len(events.Events) != 0 || events.Status.State != domain.AvailabilityAvailable {
		t.Fatal("empty event range lost local history capability")
	}
}

package alerting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type collectionEvidenceStub struct {
	current domain.SourceCollectionOutcome
	history domain.SourceCollectionHistory
	err     error
}

func (stub collectionEvidenceStub) GetSourceCollectionStatus(context.Context, domain.ProviderRef) (domain.SourceCollectionOutcome, error) {
	return stub.current, stub.err
}

func (stub collectionEvidenceStub) ListSourceCollectionHistory(context.Context, domain.ProviderRef, domain.TimeRange, int) (domain.SourceCollectionHistory, error) {
	return stub.history, stub.err
}

func collectionOutcome(at time.Time) domain.SourceCollectionOutcome {
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: at}
	return domain.SourceCollectionOutcome{CollectedAt: at, Status: status, MetricStatus: status, EventStatus: status}
}

func coveredCollections(at time.Time, duration time.Duration) collectionEvidenceStub {
	observations := []domain.SourceCollectionOutcome{}
	for offset := duration; offset >= 0; offset -= 30 * time.Second {
		observations = append(observations, collectionOutcome(at.Add(-offset)))
	}
	return collectionEvidenceStub{current: collectionOutcome(at), history: domain.SourceCollectionHistory{Observations: observations, Complete: true}}
}

type metricEventEvidenceProvider struct {
	resource domain.ResourceRef
	at       time.Time
	metric   domain.MetricHistoryResult
	health   domain.HealthResult
	pages    []domain.EventPage
	notFound bool
}

func (provider metricEventEvidenceProvider) Source() domain.InfrastructureSource {
	return domain.InfrastructureSource{ID: "source_demo", Provider: domain.ProviderDemo, Name: "Demo", Synthetic: true}
}

func (provider metricEventEvidenceProvider) ObservationTime() time.Time { return provider.at }

func (provider metricEventEvidenceProvider) ListEnvironments(context.Context, domain.EnvironmentQuery) (domain.EnvironmentPage, error) {
	return domain.EnvironmentPage{}, errors.New("cache inventory must not check source connection")
}

func (provider metricEventEvidenceProvider) ListResources(context.Context, domain.InventoryQuery) (domain.ResourcePage, error) {
	return domain.ResourcePage{}, nil
}

func (provider metricEventEvidenceProvider) GetResource(context.Context, domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	if provider.notFound {
		return domain.ResourceSnapshot{}, domain.ErrRepositoryNotFound
	}
	return domain.ResourceSnapshot{Resource: domain.Resource{Ref: provider.resource, DisplayName: "Service", Synthetic: true}, Status: collectionOutcome(provider.at).Status}, nil
}

func (provider metricEventEvidenceProvider) GetHealth(context.Context, domain.HealthQuery) (domain.HealthResult, error) {
	return provider.health, nil
}

func (provider metricEventEvidenceProvider) GetMetricHistory(context.Context, domain.MetricHistoryQuery) (domain.MetricHistoryResult, error) {
	return provider.metric, nil
}

func (provider metricEventEvidenceProvider) ListEvents(_ context.Context, query domain.EventQuery) (domain.EventPage, error) {
	index := 0
	if query.Page.Cursor != "" {
		index = 1
	}
	if index >= len(provider.pages) {
		return domain.EventPage{}, errors.New("unexpected event page")
	}
	return provider.pages[index], nil
}

func metricEvidenceFixture(t *testing.T, kind domain.AlertRuleKind) (EvaluationTask, metricEventEvidenceProvider, collectionEvidenceStub) {
	t.Helper()
	at := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	task := workerTask(t, at)
	task.Rule.Kind = kind
	task.Rule.Window = time.Minute
	comparison := domain.ComparisonGreaterThanOrEqual
	task.Rule.Comparison = &comparison
	if kind == domain.AlertRuleRestartThreshold {
		threshold := uint64(1)
		task.Rule.ThresholdCount = &threshold
	} else {
		threshold := .75
		task.Rule.ThresholdValue = &threshold
	}
	provider := metricEventEvidenceProvider{resource: *task.Resource, at: at,
		metric: domain.MetricHistoryResult{Status: collectionOutcome(at).Status, Samples: metricSamples(at.Add(-time.Minute), metricName(kind), 30*time.Second, 1, 0)},
	}
	for index := range provider.metric.Samples {
		provider.metric.Samples[index].ResourceID = task.Resource.ID
	}
	return task, provider, coveredCollections(at, time.Minute)
}

func newFixtureEvidence(t *testing.T, provider EvidenceProvider, collections collectionEvidenceStub) *ProviderEvidence {
	t.Helper()
	resolver, err := NewProviderEvidence([]EvidenceProvider{provider}, collections)
	if err != nil {
		t.Fatal(err)
	}
	return resolver
}

func TestProviderMetricThresholdUsesWindowRatherThanLastSample(t *testing.T) {
	for _, kind := range []domain.AlertRuleKind{domain.AlertRuleCPUThreshold, domain.AlertRuleMemoryThreshold} {
		t.Run(string(kind), func(t *testing.T) {
			task, provider, collections := metricEvidenceFixture(t, kind)
			provider.metric.Samples[0].Value, provider.metric.Samples[1].Value = 0, 1
			result, err := newFixtureEvidence(t, provider, collections).ResolveAlertEvidence(context.Background(), task, provider.at)
			if err != nil || result.Matched || result.EvidenceRange.End.Sub(result.EvidenceRange.Start) != time.Minute {
				t.Fatalf("sample mean = %#v, %v; latest1 must not fire .75 window", result, err)
			}
		})
	}
}

func TestProviderMetricEvidenceUnknownDoesNotClearOrFire(t *testing.T) {
	tests := []string{"stale", "denied", "partial", "future", "missing", "capture_gap", "capture_denied", "capture_capped"}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			task, provider, collections := metricEvidenceFixture(t, domain.AlertRuleCPUThreshold)
			switch name {
			case "stale":
				provider.metric.Status.State, provider.metric.Status.ReasonCode = domain.AvailabilityStale, "source.stale"
			case "denied":
				provider.metric.Status.State, provider.metric.Status.ReasonCode = domain.AvailabilityPermissionDenied, "source.denied"
				provider.metric.Samples = nil
			case "partial":
				provider.metric.Status.Completeness, provider.metric.Status.ReasonCode = domain.CompletenessPartial, "source.metric_history_limited"
			case "future":
				provider.metric.Samples[1].SampledAt = provider.at.Add(time.Second)
			case "missing":
				provider.metric.Samples = provider.metric.Samples[1:]
			case "capture_gap":
				collections.history.Observations = collections.history.Observations[1:]
			case "capture_denied":
				collections.history.Observations[1].MetricStatus.State = domain.AvailabilityPermissionDenied
				collections.history.Observations[1].MetricStatus.ReasonCode = "source.denied"
			case "capture_capped":
				collections.history.Complete = false
			}
			if _, err := newFixtureEvidence(t, provider, collections).ResolveAlertEvidence(context.Background(), task, provider.at); !errors.Is(err, ErrEvaluationUnavailable) {
				t.Fatalf("%s evidence = %v; want unknown", name, err)
			}
		})
	}
}

func TestSourceConnectionUsesRawResultAndRejectsDisabledDeniedStale(t *testing.T) {
	task, provider, _ := metricEvidenceFixture(t, domain.AlertRuleCPUThreshold)
	task.Rule.Kind, task.Rule.ResourceID, task.Rule.Comparison, task.Rule.ThresholdValue = domain.AlertRuleProviderConnectionLost, nil, nil, nil
	task.Rule.Window, task.Resource = 0, nil
	tests := []struct {
		state          domain.AvailabilityState
		reason         domain.ReasonCode
		known, matched bool
	}{
		{domain.AvailabilityAvailable, "", true, false},
		{domain.AvailabilityUnavailable, "observation.provider_unavailable", true, true},
		{domain.AvailabilityUnavailable, "observation.deadline_exceeded", true, true},
		{domain.AvailabilityUnavailable, "source.disabled", false, false},
		{domain.AvailabilityUnavailable, "observation.persistence_failed", false, false},
		{domain.AvailabilityUnavailable, "observation.profile_unavailable", false, false},
		{domain.AvailabilityPermissionDenied, "observation.permission_denied", false, false},
		{domain.AvailabilityUnsupported, "observation.unsupported", false, false},
		{domain.AvailabilityStale, "source.stale", false, false},
	}
	for _, test := range tests {
		t.Run(string(test.state)+"_"+string(test.reason), func(t *testing.T) {
			collections := collectionEvidenceStub{current: collectionOutcome(provider.at)}
			collections.current.Status.State, collections.current.Status.ReasonCode = test.state, test.reason
			result, err := newFixtureEvidence(t, provider, collections).ResolveAlertEvidence(context.Background(), task, provider.at)
			if (err == nil) != test.known || err == nil && result.Matched != test.matched {
				t.Fatalf("raw source condition = %#v, %v", result, err)
			}
		})
	}
}

func TestResourceAbsenceRequiresCompleteFreshCollection(t *testing.T) {
	task, provider, collections := metricEvidenceFixture(t, domain.AlertRuleCPUThreshold)
	task.Rule.Kind, task.Rule.Comparison, task.Rule.ThresholdValue, task.Rule.Window = domain.AlertRuleResourceUnavailable, nil, nil, 0
	provider.notFound = true
	result, err := newFixtureEvidence(t, provider, collections).ResolveAlertEvidence(context.Background(), task, provider.at)
	if err != nil || !result.Matched {
		t.Fatalf("known absence = %#v, %v", result, err)
	}
	collections.current.Status.Completeness, collections.current.Status.ReasonCode = domain.CompletenessPartial, "source.partial"
	if _, err := newFixtureEvidence(t, provider, collections).ResolveAlertEvidence(context.Background(), task, provider.at); !errors.Is(err, ErrEvaluationUnavailable) {
		t.Fatalf("partial absence = %v", err)
	}
}

func TestHealthConditionRejectsUnknownPartialAndFuture(t *testing.T) {
	for _, name := range []string{"healthy", "unhealthy", "unknown", "partial", "future", "other_resource", "old_evidence", "mixed_provenance"} {
		t.Run(name, func(t *testing.T) {
			task, provider, collections := metricEvidenceFixture(t, domain.AlertRuleCPUThreshold)
			task.Rule.Kind, task.Rule.Comparison, task.Rule.ThresholdValue, task.Rule.Window = domain.AlertRuleResourceUnhealthy, nil, nil, 0
			status := collectionOutcome(provider.at).Status
			provider.health = domain.HealthResult{Status: status, Snapshots: []domain.HealthSnapshot{{ResourceID: task.Resource.ID,
				State: domain.HealthHealthy, Status: status, Evidence: []domain.HealthEvidence{{ID: "evidence_ready", Code: "health.ready", SourceCode: "demo.ready",
					Status: domain.EvidencePositive, ObservedAt: provider.at, Synthetic: true}}}}}
			snapshot := &provider.health.Snapshots[0]
			switch name {
			case "unhealthy":
				snapshot.State, snapshot.Evidence[0].Status = domain.HealthUnhealthy, domain.EvidenceNegative
			case "unknown":
				snapshot.State, snapshot.Evidence = domain.HealthUnknown, nil
			case "partial":
				snapshot.Status.Completeness, snapshot.Status.ReasonCode = domain.CompletenessPartial, "source.partial"
			case "future":
				snapshot.Status.ObservedAt = provider.at.Add(time.Second)
			case "other_resource":
				snapshot.ResourceID = *fixtureRule(0).ResourceID
			case "old_evidence":
				snapshot.Evidence[0].ObservedAt = provider.at.Add(-defaultMaximumCollectionGap - time.Microsecond)
			case "mixed_provenance":
				snapshot.Evidence[0].Synthetic = false
			}
			result, err := newFixtureEvidence(t, provider, collections).ResolveAlertEvidence(context.Background(), task, provider.at)
			known := name == "healthy" || name == "unhealthy"
			if (err == nil) != known || err == nil && result.Matched != (name == "unhealthy") {
				t.Fatalf("%s health condition = %#v, %v", name, result, err)
			}
		})
	}
}

func TestEvidenceRequiresBoundedRawCollectionAndOwnedSource(t *testing.T) {
	task, provider, collections := metricEvidenceFixture(t, domain.AlertRuleCPUThreshold)
	if _, err := NewProviderEvidence([]EvidenceProvider{provider}); err == nil {
		t.Fatal("cache-only source evidence accepted")
	}
	for _, options := range []ProviderEvidenceOptions{
		{Providers: []EvidenceProvider{provider}, Collections: collections},
		{Providers: []EvidenceProvider{provider}, Collections: collections, MaximumCollectionGap: 6 * time.Minute},
		{Providers: []EvidenceProvider{provider, provider}, Collections: collections, MaximumCollectionGap: time.Minute},
		{Providers: []EvidenceProvider{nil}, Collections: collections, MaximumCollectionGap: time.Minute},
	} {
		if _, err := NewProviderEvidenceWithOptions(options); err == nil {
			t.Fatal("unbounded or invalid evidence options accepted")
		}
	}
	resolver := newFixtureEvidence(t, provider, collections)
	if _, err := resolver.ResolveAlertEvidence(nil, task, provider.at); !errors.Is(err, ErrEvaluationUnavailable) {
		t.Fatalf("nil context = %v", err)
	}
	task.Synthetic = false
	if _, err := resolver.ResolveAlertEvidence(context.Background(), task, provider.at); !errors.Is(err, ErrEvaluationUnavailable) {
		t.Fatalf("wrong synthetic provenance = %v", err)
	}
}

type sourceEvidenceProvider struct {
	EvidenceProvider
	source domain.InfrastructureSource
}

func (provider sourceEvidenceProvider) Source() domain.InfrastructureSource { return provider.source }

func TestConnectionCannotClearFromFreshCaptureWithOldStatus(t *testing.T) {
	task, provider, collections := metricEvidenceFixture(t, domain.AlertRuleCPUThreshold)
	task.Rule.Kind, task.Rule.ResourceID, task.Rule.Comparison, task.Rule.ThresholdValue = domain.AlertRuleProviderConnectionLost, nil, nil, nil
	task.Rule.Window, task.Resource, task.Synthetic = 0, nil, false
	task.Source.Provider = domain.ProviderDocker
	owned := sourceEvidenceProvider{EvidenceProvider: provider,
		source: domain.InfrastructureSource{ID: task.Source.SourceID, Provider: domain.ProviderDocker, Name: "Engine"}}
	collections.current.Status.ObservedAt = provider.at.Add(-defaultMaximumCollectionGap - time.Microsecond)
	if _, err := newFixtureEvidence(t, owned, collections).ResolveAlertEvidence(context.Background(), task, provider.at); !errors.Is(err, ErrEvaluationUnavailable) {
		t.Fatalf("fresh capture with old raw status = %v; want unknown", err)
	}
	collections.current.Status.ObservedAt = provider.at.Add(-defaultMaximumCollectionGap)
	result, err := newFixtureEvidence(t, owned, collections).ResolveAlertEvidence(context.Background(), task, provider.at)
	if err != nil || result.Matched {
		t.Fatalf("bounded raw status = %#v, %v; want known recovery", result, err)
	}
}

func TestCollectionCoverageRejectsOversizedCompleteHistory(t *testing.T) {
	task, provider, collections := metricEvidenceFixture(t, domain.AlertRuleCPUThreshold)
	collections.history.Observations = make([]domain.SourceCollectionOutcome, domain.MaximumCollectionHistory+1)
	for index := range collections.history.Observations {
		collections.history.Observations[index] = collectionOutcome(provider.at.Add(-time.Minute).Add(time.Duration(index) * 10 * time.Millisecond))
	}
	if _, err := newFixtureEvidence(t, provider, collections).ResolveAlertEvidence(context.Background(), task, provider.at); !errors.Is(err, ErrEvaluationUnavailable) {
		t.Fatalf("oversized history = %v; want bounded unknown", err)
	}
}

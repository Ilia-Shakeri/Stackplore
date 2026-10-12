package alerting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type workerStoreStub struct {
	tasks     []EvaluationTask
	claimErr  error
	mutations []domain.AlertMutation
	saveErr   error
}

func (store *workerStoreStub) ClaimDueAlertEvaluations(context.Context, time.Time, int) ([]EvaluationTask, error) {
	return store.tasks, store.claimErr
}

func (store *workerStoreStub) SaveAlertMutation(_ context.Context, mutation domain.AlertMutation) error {
	store.mutations = append(store.mutations, mutation)
	return store.saveErr
}

type evidenceStub struct {
	result ConditionResult
	err    error
}

func (stub evidenceStub) ResolveAlertEvidence(context.Context, EvaluationTask, time.Time) (ConditionResult, error) {
	return stub.result, stub.err
}

type identifiersStub struct{}

func (identifiersStub) NewAlertID() (domain.AlertID, error) { return "alert_worker", nil }
func (identifiersStub) NewTransitionID() (domain.AlertTransitionID, error) {
	return "transition_worker", nil
}
func (identifiersStub) NewNotificationID() (domain.NotificationID, error) {
	return "notification_worker", nil
}

func TestWorkerOpensFiringAlertAtomically(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 18, 10, 0, 0, 0, time.UTC)
	task := workerTask(t, now)
	store := &workerStoreStub{tasks: []EvaluationTask{task}}
	worker := newTestWorker(t, store, evidenceStub{result: ConditionResult{
		Matched: true,
		EvidenceRange: domain.TimeRange{
			Start: now.Add(-time.Minute), End: now.Add(-time.Microsecond),
		},
		SummaryCode: "alert.resource_unhealthy.matched",
	}}, now, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(store.mutations) != 1 {
		t.Fatalf("saved mutations = %d; want 1", len(store.mutations))
	}
	mutation := store.mutations[0]
	if mutation.Alert.State != domain.AlertFiring || mutation.Transition == nil ||
		mutation.Transition.From != nil || mutation.Notification == nil ||
		mutation.Notification.Kind != "notification.alert_firing" {
		t.Fatalf("saved mutation = %#v; want firing transition and notification", mutation)
	}
	if mutation.ExpectedState != nil || mutation.ExpectedUpdatedAt != nil {
		t.Fatal("new alert unexpectedly used optimistic update fields")
	}
}

func TestWorkerResolvesCurrentAlertWithOptimisticGuard(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 18, 10, 0, 0, 0, time.UTC)
	task := workerTask(t, now)
	started := now.Add(-10 * time.Minute)
	resourceID := task.Resource.ID
	task.Current = &domain.Alert{
		ID: "alert_current", RuleID: task.Rule.ID, SourceID: task.Rule.SourceID,
		ResourceID: &resourceID, State: domain.AlertFiring, Severity: domain.SeverityError,
		EvidenceRange: domain.TimeRange{Start: started, End: now.Add(-2 * time.Minute)},
		StartedAt:     started, UpdatedAt: now.Add(-time.Minute),
		SummaryCode: "alert.resource_unhealthy.matched", Synthetic: true,
	}
	store := &workerStoreStub{tasks: []EvaluationTask{task}}
	worker := newTestWorker(t, store, evidenceStub{result: ConditionResult{
		Matched: false,
		EvidenceRange: domain.TimeRange{
			Start: now.Add(-time.Minute), End: now.Add(-time.Microsecond),
		},
		SummaryCode: "alert.resource_unhealthy.clear",
	}}, now, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(store.mutations) != 1 {
		t.Fatalf("saved mutations = %d; want 1", len(store.mutations))
	}
	mutation := store.mutations[0]
	if mutation.Alert.State != domain.AlertResolved || mutation.Alert.ResolvedAt == nil ||
		mutation.Transition == nil || mutation.Transition.From == nil ||
		*mutation.Transition.From != domain.AlertFiring || mutation.Notification == nil ||
		mutation.Notification.Kind != "notification.alert_resolved" ||
		mutation.ExpectedState == nil || *mutation.ExpectedState != domain.AlertFiring ||
		mutation.ExpectedUpdatedAt == nil || !mutation.ExpectedUpdatedAt.Equal(task.Current.UpdatedAt) {
		t.Fatalf("saved mutation = %#v; want guarded resolution", mutation)
	}
}

func TestWorkerReportsEvidenceFailureWithoutWrite(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 18, 10, 0, 0, 0, time.UTC)
	store := &workerStoreStub{tasks: []EvaluationTask{workerTask(t, now)}}
	reported := make([]error, 0, 1)
	worker := newTestWorker(t, store, evidenceStub{err: errors.New("provider failed")}, now, func(err error) {
		reported = append(reported, err)
	})

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(store.mutations) != 0 || len(reported) != 1 || !errors.Is(reported[0], ErrEvaluationUnavailable) {
		t.Fatalf("writes = %d, reports = %#v; want one safe report and no write", len(store.mutations), reported)
	}
}

func newTestWorker(
	t *testing.T,
	store TaskStore,
	evidence EvidenceResolver,
	now time.Time,
	onError func(error),
) *Worker {
	t.Helper()
	worker, err := NewWorker(WorkerOptions{
		Store: store, Evidence: evidence, Identifiers: identifiersStub{}, Now: func() time.Time { return now },
		OnError: onError, PollInterval: time.Second, ReadTimeout: time.Second, BatchSize: 4,
	})
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}
	return worker
}

func workerTask(t *testing.T, claimedAt time.Time) EvaluationTask {
	t.Helper()
	identity, err := domain.NewResourceIdentity(
		domain.ProviderDemo, "source_demo", "env_demo", "demo.service",
		domain.Scope{Kind: domain.ScopeProject, Name: "demo"}, "service",
	)
	if err != nil {
		t.Fatal(err)
	}
	resource, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal(err)
	}
	resourceID := resource.ID
	return EvaluationTask{
		Rule: domain.AlertRule{
			ID: "rule_worker", SourceID: "source_demo", ResourceID: &resourceID,
			DisplayName: "Worker health", Kind: domain.AlertRuleResourceUnhealthy,
			Enabled: true, Severity: domain.SeverityError, EvaluationInterval: time.Minute,
		},
		Source:   domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "source_demo"},
		Resource: &resource, DedupeKey: "rule_worker", Synthetic: true,
		ClaimedAt: claimedAt, NextEvaluationAt: claimedAt.Add(time.Minute),
	}
}

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
	completed int
	saveErr   error
}

func (store *workerStoreStub) ClaimDueAlertEvaluations(_ context.Context, _ time.Time, limit int) ([]EvaluationTask, error) {
	if store.claimErr != nil {
		return nil, store.claimErr
	}
	count := min(limit, len(store.tasks))
	tasks := store.tasks[:count]
	store.tasks = store.tasks[count:]
	return tasks, nil
}

func (store *workerStoreStub) CompleteAlertEvaluation(_ context.Context, _ EvaluationTask, mutation *domain.AlertMutation) error {
	store.completed++
	if mutation != nil {
		store.mutations = append(store.mutations, *mutation)
	}
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

func TestWorkerReopensResolvedEpisodeWithNewIdentityAndNoOldAcknowledgement(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	task := workerTask(t, now)
	started, acknowledged, resolved := now.Add(-10*time.Minute), now.Add(-2*time.Minute), now.Add(-time.Minute)
	userID := domain.UserID("user_admin")
	task.Current = &domain.Alert{ID: "alert_old", RuleID: task.Rule.ID, SourceID: task.Rule.SourceID,
		ResourceID: task.Rule.ResourceID, State: domain.AlertResolved, Severity: domain.SeverityError,
		EvidenceRange: domain.TimeRange{Start: started, End: resolved}, StartedAt: started, UpdatedAt: resolved,
		AcknowledgedAt: &acknowledged, ResolvedAt: &resolved, SummaryCode: "alert.resource_unhealthy.clear", Synthetic: true}
	task.AcknowledgedByUserID = &userID
	store := &workerStoreStub{tasks: []EvaluationTask{task}}
	worker := newTestWorker(t, store, evidenceStub{result: condition(true, now)}, now, func(err error) { t.Errorf("unexpected error: %v", err) })
	if err := worker.RunOnce(context.Background()); err != nil || len(store.mutations) != 1 {
		t.Fatalf("reopened episode = %v, mutations = %d", err, len(store.mutations))
	}
	mutation := store.mutations[0]
	if mutation.Alert.ID == task.Current.ID || mutation.Alert.ID != "alert_worker" || mutation.Alert.AcknowledgedAt != nil ||
		mutation.AcknowledgedByUserID != nil || mutation.ExpectedUpdatedAt != nil || mutation.ExpectedState != nil ||
		mutation.Transition == nil || mutation.Transition.From != nil || mutation.Notification == nil {
		t.Fatalf("new episode inherited old identity or acknowledgement: %#v", mutation)
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
	if len(store.mutations) != 0 || store.completed != 1 || len(reported) != 1 || !errors.Is(reported[0], ErrEvaluationUnavailable) {
		t.Fatalf("writes = %d, reports = %#v; want one safe report and no write", len(store.mutations), reported)
	}
}

type callbackTaskStore struct {
	claim    func(context.Context, time.Time, int) ([]EvaluationTask, error)
	complete func(context.Context, EvaluationTask, *domain.AlertMutation) error
}

func (store callbackTaskStore) ClaimDueAlertEvaluations(ctx context.Context, at time.Time, limit int) ([]EvaluationTask, error) {
	return store.claim(ctx, at, limit)
}

func (store callbackTaskStore) CompleteAlertEvaluation(ctx context.Context, task EvaluationTask, mutation *domain.AlertMutation) error {
	return store.complete(ctx, task, mutation)
}

type callbackEvidence func(context.Context, EvaluationTask, time.Time) (ConditionResult, error)

func (resolve callbackEvidence) ResolveAlertEvidence(ctx context.Context, task EvaluationTask, at time.Time) (ConditionResult, error) {
	return resolve(ctx, task, at)
}

func TestWorkerClaimsEachTaskWhenReadyAndHonorsBatchBound(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	claims, completions := 0, 0
	store := callbackTaskStore{
		claim: func(_ context.Context, at time.Time, limit int) ([]EvaluationTask, error) {
			claims++
			if limit != 1 || !at.Equal(now) {
				t.Fatalf("claim limit = %d, time = %v; need one fresh task", limit, at)
			}
			return []EvaluationTask{workerTask(t, at)}, nil
		},
		complete: func(_ context.Context, task EvaluationTask, mutation *domain.AlertMutation) error {
			completions++
			if mutation == nil || !now.Before(task.LeaseExpiresAt) {
				t.Fatalf("task completed after lease = %v; now = %v", task.LeaseExpiresAt, now)
			}
			return nil
		},
	}
	worker := newTestWorker(t, store, callbackEvidence(func(_ context.Context, _ EvaluationTask, at time.Time) (ConditionResult, error) {
		// Advance the injected clock without slowing the test. Bulk claiming would
		// expire later tasks before this second bounded read finishes.
		now = now.Add(40 * time.Second)
		return condition(true, at), nil
	}), now, func(err error) { t.Errorf("unexpected worker error: %v", err) })
	worker.now = func() time.Time { return now }
	worker.batchSize = 2
	if err := worker.RunOnce(context.Background()); err != nil || claims != 2 || completions != 2 {
		t.Fatalf("bounded fresh claims = %d, completions = %d, err = %v", claims, completions, err)
	}
}

func TestWorkerReleasesClaimWhenConditionIsInvalid(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	store := &workerStoreStub{tasks: []EvaluationTask{workerTask(t, now)}}
	reports := 0
	worker := newTestWorker(t, store, evidenceStub{result: condition(true, now.Add(time.Second))}, now,
		func(error) { reports++ })
	if err := worker.RunOnce(context.Background()); err != nil || reports != 1 || store.completed != 1 || len(store.mutations) != 0 {
		t.Fatalf("invalid condition: releases = %d, mutations = %d, reports = %d, err = %v", store.completed, len(store.mutations), reports, err)
	}
}

func TestWorkerRejectsLateSuccessAndReleasesUnknownClaim(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	store := &workerStoreStub{tasks: []EvaluationTask{workerTask(t, now)}}
	var reported error
	worker := newTestWorker(t, store, callbackEvidence(func(ctx context.Context, _ EvaluationTask, at time.Time) (ConditionResult, error) {
		<-ctx.Done()
		return condition(true, at), nil
	}), now, func(err error) { reported = err })
	worker.readTimeout = minimumReadTimeout
	if err := worker.RunOnce(context.Background()); err != nil || !errors.Is(reported, context.DeadlineExceeded) ||
		store.completed != 1 || len(store.mutations) != 0 {
		t.Fatalf("late success: releases = %d, mutations = %d, report = %v, err = %v", store.completed, len(store.mutations), reported, err)
	}
}

func TestWorkerStopsClaimsAfterCancellation(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &workerStoreStub{tasks: []EvaluationTask{workerTask(t, now), workerTask(t, now)}}
	worker := newTestWorker(t, store, callbackEvidence(func(context.Context, EvaluationTask, time.Time) (ConditionResult, error) {
		cancel()
		return ConditionResult{}, context.Canceled
	}), now, nil)
	if err := worker.RunOnce(ctx); !errors.Is(err, context.Canceled) || len(store.tasks) != 1 || len(store.mutations) != 0 {
		t.Fatalf("cancelled worker: tasks left = %d, mutations = %d, err = %v", len(store.tasks), len(store.mutations), err)
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
		RuleVersion: 1, SourceConfigurationVersion: 1, ClaimGeneration: 1, LeaseExpiresAt: claimedAt.Add(time.Minute),
	}
}

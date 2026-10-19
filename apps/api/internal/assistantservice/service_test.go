package assistantservice

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
)

func TestAdmittedRunHasOneBoundedReadAndOneReservedDispatch(t *testing.T) {
	t.Parallel()
	service, repository, authorizer, model := fixtureService(t, nil)
	input := inputFixture(t, 1)
	run, err := service.Submit(context.Background(), input)
	if err != nil || run.State != assistant.Queued || model.calls.Load() != 0 {
		t.Fatal("submission performed work or failed admission")
	}
	if found, err := service.runOne(context.Background(), "worker"); err != nil || !found {
		t.Fatal("worker failed bounded explanation")
	}
	snapshot, err := service.Get(context.Background(), input.Admission.Actor, run.ID)
	if err != nil || snapshot.Run.State != assistant.Completed || snapshot.Answer == nil || len(snapshot.Citations) != 1 || model.calls.Load() != 1 || authorizer.calls.Load() < 5 {
		t.Fatal("grounded result was not safely published")
	}
	want := []string{"begin:inventory", "read:inventory", "finish:inventory", "begin:inference", "finish:inference", "terminal:completed"}
	if !slices.Equal(repository.trace, want) {
		t.Fatal("network work crossed transaction ordering")
	}
	snapshot.Answer.Sections[0].Text = "Changed"
	second, err := service.Get(context.Background(), input.Admission.Actor, run.ID)
	if err != nil || second.Answer == nil || second.Answer.Sections[0].Text == "Changed" {
		t.Fatal("result snapshot mutated the registry")
	}
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal("matching idempotent submission rejected")
	}
	if found, err := service.runOne(context.Background(), "worker"); err != nil || found || model.calls.Load() != 1 {
		t.Fatal("replayed submission restarted inference")
	}
	input.Question = "A changed question."
	if _, err := service.Submit(context.Background(), input); err != assistant.ErrConflict {
		t.Fatal("changed material reused an idempotent run")
	}
	for _, value := range []any{input, snapshot, service} {
		if strings.Contains(fmt.Sprintf("%#v", value), "Explain the selected") {
			t.Fatal("raw question escaped formatting")
		}
	}
}

func TestAuthorityDenialStopsBeforeAnyRead(t *testing.T) {
	t.Parallel()
	service, repository, authorizer, model := fixtureService(t, nil)
	input := inputFixture(t, 1)
	authorizer.denied.Store(true)
	if _, err := service.Submit(context.Background(), input); err != assistant.ErrDenied || repository.admissions != 0 {
		t.Fatal("denied actor reached admission")
	}
	authorizer.denied.Store(false)
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	authorizer.denied.Store(true)
	if found, err := service.runOne(context.Background(), "worker"); !found || err != assistant.ErrDenied || model.calls.Load() != 0 {
		t.Fatal("denied authority reached a tool or inference")
	}
	if len(repository.trace) != 1 || repository.trace[0] != "terminal:interrupted" {
		t.Fatal("authority loss was misreported as success")
	}
}

func TestSourceInvalidationClearsMemoryBeforeCancellingLateOutput(t *testing.T) {
	t.Parallel()
	started, cancelled := make(chan struct{}), make(chan struct{})
	var service *Service
	service, _, _, _ = fixtureService(t, func(_ *Options, model *modelFixture) {
		model.hook = func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			service.mu.Lock()
			for _, entry := range service.entries {
				if entry.input.Question != "" || entry.answer != nil || !entry.revoked {
					t.Error("cancel preceded memory fencing")
				}
			}
			service.mu.Unlock()
			close(cancelled)
			return nil
		}
	})
	input := inputFixture(t, 1)
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := service.runOne(context.Background(), "worker"); done <- err }()
	waitSignal(t, started)
	service.InvalidateSource(input.Admission.SourceID)
	waitSignal(t, cancelled)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("revoked worker did not finish")
	}
	snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
	if err != nil || snapshot.Answer != nil || snapshot.Volatile {
		t.Fatal("late revoked answer was published")
	}
}

func TestExplicitCancelAndShutdownLeaveNoWorker(t *testing.T) {
	t.Parallel()
	for _, shutdown := range []bool{false, true} {
		started := make(chan struct{})
		service, _, _, model := fixtureService(t, func(_ *Options, model *modelFixture) {
			model.hook = func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() }
		})
		input := inputFixture(t, 1)
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- service.Run(ctx) }()
		waitSignal(t, started)
		if err := service.Run(ctx); err != assistant.ErrConflict {
			t.Fatal("duplicate worker pool started")
		}
		if shutdown {
			service.Close()
		} else {
			if run, err := service.Cancel(context.Background(), input.Admission.Actor, input.Admission.ID); err != nil || run.State != assistant.Cancelled {
				t.Fatal("explicit cancel failed")
			}
			cancel()
		}
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("shutdown left an owned worker")
		}
		cancel()
		if model.calls.Load() != 1 {
			t.Fatal("cancelled inference retried")
		}
		if _, err := service.Submit(context.Background(), inputFixture(t, 2)); err == nil {
			t.Fatal("closed service accepted new work")
		}
	}
}

func TestQuotaExpiryAndConversationBounds(t *testing.T) {
	t.Parallel()
	var offset atomic.Int64
	service, repository, _, _ := fixtureService(t, func(options *Options, _ *modelFixture) {
		options.Now = func() time.Time { return time.Now().Add(time.Duration(offset.Load())) }
	})
	input := inputFixture(t, 1)
	input.Admission.ReservedUnits = 1
	if _, err := service.Submit(context.Background(), input); err != assistant.ErrQuota || repository.admissions != 0 {
		t.Fatal("insufficient dispatch reserve was admitted")
	}
	for index := range MaximumTurns {
		input = inputFixture(t, index+1)
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal("bounded turn failed")
		}
		if _, err := service.runOne(context.Background(), "worker"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.Submit(context.Background(), inputFixture(t, 10)); err != assistant.ErrQuota {
		t.Fatal("conversation exceeded eight turns")
	}
	offset.Store(int64(MemoryLifetime + time.Second))
	snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
	if err != nil || snapshot.Answer != nil || snapshot.Volatile || len(service.entries) != 0 {
		t.Fatal("expired private history survived")
	}
	for index := range MaximumMemoryRuns {
		input := inputFixture(t, index+20)
		input.Admission.ConversationID = fmt.Sprintf("conversation_%d", index)
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal("bounded registry rejected available slot")
		}
	}
	input = inputFixture(t, 70)
	input.Admission.ConversationID = "overflow"
	if _, err := service.Submit(context.Background(), input); err != assistant.ErrQuota {
		t.Fatal("registry grew beyond its hard cap")
	}
}

func TestFailedAdmissionDoesNotConsumeMemoryCapacity(t *testing.T) {
	t.Parallel()
	service, repository, _, _ := fixtureService(t, nil)
	repository.admitError = assistant.ErrQuota
	for index := range MaximumMemoryRuns + 1 {
		input := inputFixture(t, index+1)
		input.Admission.ConversationID = fmt.Sprintf("conversation_%d", index)
		if _, err := service.Submit(context.Background(), input); err != assistant.ErrQuota {
			t.Fatal("quota failure changed")
		}
	}
	if len(service.entries) != 0 || len(service.conversations) != 0 {
		t.Fatal("failed admission consumed bounded memory")
	}
}

func TestUncertainInferenceIsNeverRepeated(t *testing.T) {
	t.Parallel()
	service, repository, _, model := fixtureService(t, func(_ *Options, model *modelFixture) {
		model.hook = func(context.Context) error { return errors.New("fixture interruption") }
	})
	input := inputFixture(t, 1)
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if _, err := service.runOne(context.Background(), "worker"); err == nil {
		t.Fatal("uncertain model call succeeded")
	}
	if repository.runs[input.Admission.ID].State != assistant.Uncertain {
		t.Fatal("uncertain usage was silently refunded")
	}
	if found, err := service.runOne(context.Background(), "worker"); err != nil || found || model.calls.Load() != 1 {
		t.Fatal("uncertain inference was automatically repeated")
	}
}

func TestCollectorCannotExpandScopeOrToolKinds(t *testing.T) {
	t.Parallel()
	service, _, _, model := fixtureService(t, func(options *Options, _ *modelFixture) {
		options.Collector = collectorFunc(func(_ context.Context, scope contextbuilder.Scope, _ assistant.StepKind) (contextbuilder.Request, error) {
			scope.Source.SourceID = "other_source"
			return contextbuilder.Request{Scope: scope, Destination: contextbuilder.Remote}, nil
		})
	})
	if _, err := service.Submit(context.Background(), inputFixture(t, 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.runOne(context.Background(), "worker"); err != assistant.ErrDenied || model.calls.Load() != 0 {
		t.Fatal("collector expanded authorized scope")
	}
}

func TestMissingVolatileQuestionBecomesInterrupted(t *testing.T) {
	t.Parallel()
	service, repository, _, model := fixtureService(t, nil)
	input := inputFixture(t, 1)
	run, err := service.Submit(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	service.eraseLocked(service.entries[run.ID])
	delete(service.entries, run.ID)
	service.mu.Unlock()
	if found, err := service.runOne(context.Background(), "worker"); err != nil || !found || model.calls.Load() != 0 || repository.runs[run.ID].State != assistant.Interrupted {
		t.Fatal("lost question was reconstructed or retried")
	}
}

func waitSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("bounded fixture did not progress")
	}
}

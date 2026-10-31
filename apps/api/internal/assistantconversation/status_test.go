package assistantconversation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

type statusRepositoryStub struct {
	lookupFunc
	events func(context.Context) ([]assistant.Event, error)
	calls  int
}

func (repository *statusRepositoryStub) AssistantEvents(ctx context.Context, _ assistant.Actor, _ string, _ int64, limit int, _ time.Time) ([]assistant.Event, error) {
	repository.calls++
	if limit != 65 {
		return nil, assistant.ErrInvalid
	}
	return repository.events(ctx)
}

func statusFixture(t *testing.T) (Options, assistant.Actor, assistant.Run, *statusRepositoryStub) {
	t.Helper()
	options, request, actor, _ := conversationFixture(t)
	at := options.Now()
	run := assistant.Run{ID: "ar_" + strings.Repeat("a", 64), ConversationID: "ac_" + strings.Repeat("b", 64), OwnerID: actor.UserID, SourceID: string(request.SourceID), ResourceID: string(request.ResourceID), Goal: assistant.ExplainResource, State: assistant.Running, Version: 3, CreatedAt: at.Add(-time.Minute), UpdatedAt: at, Deadline: at.Add(time.Minute), SourceVersion: 1, PolicyVersion: 1, ProfileID: "profile", ProfileVersion: 1, ConfigurationVersion: 1, Volatile: true, From: at.Add(-16 * time.Minute), Until: at.Add(-time.Minute)}
	repository := &statusRepositoryStub{lookupFunc: func(context.Context, assistant.Actor, string, time.Time) (assistant.Run, error) { return run, nil }, events: func(context.Context) ([]assistant.Event, error) {
		return []assistant.Event{{Sequence: 2, RunVersion: 2, State: assistant.Queued, At: at.Add(-time.Second)}, {Sequence: 3, RunVersion: 3, State: assistant.Running, At: at}}, nil
	}}
	options.Repository = repository
	return options, actor, run, repository
}

func TestStatusSnapshotDoesNotReadEventsOrVolatileContent(t *testing.T) {
	options, actor, run, repository := statusFixture(t)
	service, err := New(options)
	if err != nil {
		t.Fatal("construct metadata service")
	}
	defer service.Close()
	page, err := service.Status(context.Background(), actor, run.ID, 0)
	if err != nil || page.Run != run || len(page.Events) != 1 || page.Events[0].Sequence != 3 || page.Events[0].State != assistant.Running || repository.calls != 0 || page.Reset != "" {
		t.Fatal("fresh metadata snapshot crossed its boundary")
	}
	formatted := fmt.Sprintf("%v %+v %#v", page, page, page)
	wire, err := json.Marshal(page)
	if err != nil || string(wire) != "{}" || strings.Contains(formatted, run.ID) || strings.Contains(formatted, actor.UserID) {
		t.Fatal("internal status page serialized private metadata")
	}
}

func TestStatusReplayDetachesEventsAndRechecksCurrentAuthority(t *testing.T) {
	options, actor, run, repository := statusFixture(t)
	events, _ := repository.events(context.Background())
	lookups := 0
	repository.lookupFunc = func(ctx context.Context, got assistant.Actor, id string, _ time.Time) (assistant.Run, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 3*time.Second || got != actor || id != run.ID {
			t.Fatal("status lookup lost actor or bound")
		}
		lookups++
		return run, nil
	}
	repository.events = func(context.Context) ([]assistant.Event, error) { return events, nil }
	service, err := New(options)
	if err != nil {
		t.Fatal("construct replay service")
	}
	defer service.Close()
	page, err := service.Status(context.Background(), actor, run.ID, 1)
	if err != nil || len(page.Events) != 2 || page.Reset != "" || lookups != 2 || repository.calls != 1 {
		t.Fatal("status replay lost fresh authority or events")
	}
	events[0].State = assistant.Failed
	if page.Events[0].State != assistant.Queued {
		t.Fatal("status replay retained mutable repository slices")
	}
}

func TestStatusReplayClosedResetReasons(t *testing.T) {
	for _, test := range []struct {
		name   string
		after  int64
		reason ResetReason
		change func(*assistant.Run, *statusRepositoryStub)
	}{
		{"future", 4, ResetCursorAhead, func(*assistant.Run, *statusRepositoryStub) {}},
		{"gap", 1, ResetReplayGap, func(_ *assistant.Run, r *statusRepositoryStub) {
			r.events = func(context.Context) ([]assistant.Event, error) { return nil, assistant.ErrConflict }
		}},
		{"expired", 1, ResetReplayExpired, func(run *assistant.Run, r *statusRepositoryStub) {
			run.UpdatedAt = run.UpdatedAt.Add(-25 * time.Hour)
			run.CreatedAt = run.UpdatedAt.Add(-time.Minute)
			r.events = func(context.Context) ([]assistant.Event, error) { return nil, assistant.ErrConflict }
		}},
		{"missing_middle", 1, ResetReplayGap, func(run *assistant.Run, r *statusRepositoryStub) {
			r.events = func(context.Context) ([]assistant.Event, error) {
				return []assistant.Event{{Sequence: 3, RunVersion: 3, State: run.State, At: run.UpdatedAt}}, nil
			}
		}},
		{"missing_tail", 1, ResetReplayGap, func(_ *assistant.Run, r *statusRepositoryStub) {
			r.events = func(context.Context) ([]assistant.Event, error) { return nil, nil }
		}},
		{"overflow", 1, ResetQueueOverflow, func(_ *assistant.Run, r *statusRepositoryStub) {
			r.events = func(context.Context) ([]assistant.Event, error) { return make([]assistant.Event, 65), nil }
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			options, actor, run, repository := statusFixture(t)
			test.change(&run, repository)
			repository.lookupFunc = func(context.Context, assistant.Actor, string, time.Time) (assistant.Run, error) { return run, nil }
			service, err := New(options)
			if err != nil {
				t.Fatal("construct reset service")
			}
			defer service.Close()
			page, err := service.Status(context.Background(), actor, run.ID, test.after)
			if err != nil || page.Reset != test.reason || !page.Reset.Valid() || len(page.Events) != 0 {
				t.Fatal("retained replay fault did not produce its closed reset")
			}
		})
	}
	if ResetReason("private_unknown").Valid() {
		t.Fatal("unknown reset reason accepted")
	}
}

func TestStatusRejectsMalformedReplayWithoutReturningPrefix(t *testing.T) {
	for _, change := range []func([]assistant.Event){
		func(e []assistant.Event) { e[1].Sequence = 2 }, func(e []assistant.Event) { e[0].RunVersion = 64 }, func(e []assistant.Event) { e[1].RunVersion = 1 },
		func(e []assistant.Event) { e[0].State = "private_unknown" }, func(e []assistant.Event) { e[0].At = time.Time{} }, func(e []assistant.Event) { e[0].At = e[1].At.Add(time.Second) }, func(e []assistant.Event) { e[1].State = assistant.Queued },
	} {
		options, actor, run, repository := statusFixture(t)
		events, _ := repository.events(context.Background())
		change(events)
		repository.events = func(context.Context) ([]assistant.Event, error) { return events, nil }
		service, err := New(options)
		if err != nil {
			t.Fatal("construct malformed replay service")
		}
		page, err := service.Status(context.Background(), actor, run.ID, 1)
		service.Close()
		if err != assistant.ErrUnavailable || page.Run.ID != "" || len(page.Events) != 0 {
			t.Fatal("malformed replay released a status prefix")
		}
	}
}

func TestStatusFencesRevocationReplacementAndLateCancellation(t *testing.T) {
	for _, change := range []func(*assistant.Run){func(r *assistant.Run) { r.SourceID = "other" }, func(r *assistant.Run) { r.Version = 2 }, func(r *assistant.Run) { r.OwnerID = "other" }} {
		options, actor, run, repository := statusFixture(t)
		lookups := 0
		repository.lookupFunc = func(context.Context, assistant.Actor, string, time.Time) (assistant.Run, error) {
			lookups++
			current := run
			if lookups > 1 {
				change(&current)
			}
			return current, nil
		}
		service, err := New(options)
		if err != nil {
			t.Fatal("construct authority service")
		}
		if page, err := service.Status(context.Background(), actor, run.ID, 1); err != assistant.ErrUnavailable || page.Run.ID != "" {
			t.Fatal("replaced or regressed run released replay")
		}
		service.Close()
	}
	options, actor, run, repository := statusFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repository.events = func(context.Context) ([]assistant.Event, error) { cancel(); return nil, nil }
	service, err := New(options)
	if err != nil {
		t.Fatal("construct cancellation service")
	}
	defer service.Close()
	if page, err := service.Status(ctx, actor, run.ID, 1); err != assistant.ErrUnavailable || page.Run.ID != "" {
		t.Fatal("late cancellation returned metadata")
	}
	service.Close()
	if _, err := service.Status(context.Background(), actor, run.ID, 0); err != assistant.ErrUnavailable {
		t.Fatal("closed coordinator returned status")
	}
	if _, err := service.Status(nil, actor, run.ID, 0); err != assistant.ErrInvalid {
		t.Fatal("missing status context accepted")
	}
	if _, err := service.Status(context.Background(), actor, "wrong", 0); err != assistant.ErrInvalid {
		t.Fatal("wrong status identity accepted")
	}
}

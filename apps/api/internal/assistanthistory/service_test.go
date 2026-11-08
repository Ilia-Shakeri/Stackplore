package assistanthistory

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

type repositoryFunc func(context.Context, assistant.Actor, assistant.HistoryQuery, time.Time) (assistant.HistoryPage, error)

func (read repositoryFunc) ListAssistantHistory(ctx context.Context, actor assistant.Actor, query assistant.HistoryQuery, at time.Time) (assistant.HistoryPage, error) {
	return read(ctx, actor, query, at)
}

func fixture(t *testing.T) (*Service, assistant.Actor, *time.Time, *int, *[]assistant.HistoryQuery) {
	t.Helper()
	at := time.Now().UTC().Truncate(time.Microsecond)
	actor := assistant.Actor{UserID: "owner", SessionDigest: [32]byte{1}, DigestKeyID: "key", AuthRevision: 1}
	calls := 0
	var seen []assistant.HistoryQuery
	repository := repositoryFunc(func(ctx context.Context, received assistant.Actor, query assistant.HistoryQuery, readAt time.Time) (assistant.HistoryPage, error) {
		calls++
		seen = append(seen, query)
		if received != actor {
			t.Error("unexpected actor reached repository")
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Error("unbounded metadata read")
		}
		entry := assistant.HistoryEntry{ID: "run_b", SourceID: "source", Goal: assistant.ExplainResource, State: query.State, CreatedAt: at.Add(-time.Second), UpdatedAt: at, From: at.Add(-time.Minute), Until: at.Add(-time.Second)}
		more := query.BeforeID == ""
		if !more {
			entry.ID = "run_a"
		}
		return assistant.HistoryPage{Entries: []assistant.HistoryEntry{entry}, HasMore: more, Generation: 0}, nil
	})
	key := append([]byte{1}, make([]byte, 31)...)
	service, err := New(repository, key, func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	return service, actor, &at, &calls, &seen
}

func TestMetadataPagesBindExactScopeWithoutTextReplay(t *testing.T) {
	service, actor, _, calls, seen := fixture(t)
	first, err := service.List(context.Background(), actor, assistant.Cancelled, 1, "")
	if err != nil || len(first.Entries) != 1 || first.NextCursor == "" || *calls != 1 {
		t.Fatal("first bounded page failed", err)
	}
	second, err := service.List(context.Background(), actor, assistant.Cancelled, 1, first.NextCursor)
	if err != nil || len(second.Entries) != 1 || second.NextCursor != "" || second.Entries[0].ID != "run_a" || *calls != 2 {
		t.Fatal("keyset did not advance", err)
	}
	if (*seen)[1].UpperBound != (*seen)[0].UpperBound || (*seen)[1].BeforeID != "run_b" || (*seen)[1].Generation != 0 {
		t.Fatal("cursor lost anchor or restore fence")
	}
	encoded, err := json.Marshal(first)
	if err != nil || string(encoded) != "{}" || strings.Contains(fmt.Sprintf("%+v", first), "run_b") {
		t.Fatal("ordinary formatting exposed metadata")
	}
	if fmt.Sprintf("%+v", service) != "[protected history service]" {
		t.Fatal("ordinary formatting exposed service key")
	}
	first.Entries[0].ID = "mutated"
	if second.Entries[0].ID == "mutated" {
		t.Fatal("page alias crossed request")
	}
}

func TestCursorRejectsChangedOwnerSessionFilterExpiryAndMalformedInputBeforeIO(t *testing.T) {
	service, actor, at, calls, _ := fixture(t)
	first, err := service.List(context.Background(), actor, assistant.Cancelled, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, changed := range []func(*assistant.Actor){func(a *assistant.Actor) { a.UserID = "other" }, func(a *assistant.Actor) { a.SessionDigest[0]++ }, func(a *assistant.Actor) { a.DigestKeyID = "new_key" }, func(a *assistant.Actor) { a.AuthRevision++ }} {
		other := actor
		changed(&other)
		if _, err := service.List(context.Background(), other, assistant.Cancelled, 1, first.NextCursor); err != assistant.ErrInvalid {
			t.Fatal("foreign cursor accepted", err)
		}
	}
	for _, encoded := range []string{"not-a-cursor", first.NextCursor + "x", strings.Repeat("x", MaximumCursorBytes+1), ".", "!.!", "e30.AA"} {
		if _, err := service.List(context.Background(), actor, assistant.Cancelled, 1, encoded); err != assistant.ErrInvalid {
			t.Fatal("malformed cursor accepted", err)
		}
	}
	if _, err := service.List(context.Background(), actor, assistant.Completed, 1, first.NextCursor); err != assistant.ErrInvalid {
		t.Fatal("cursor changed state")
	}
	if _, err := service.List(context.Background(), actor, assistant.Cancelled, 2, first.NextCursor); err != assistant.ErrInvalid {
		t.Fatal("cursor changed page size")
	}
	*at = at.Add(CursorLifetime)
	if _, err := service.List(context.Background(), actor, assistant.Cancelled, 1, first.NextCursor); err != assistant.ErrInvalid {
		t.Fatal("expired cursor accepted")
	}
	if *calls != 1 {
		t.Fatal("rejected cursor reached repository")
	}
}

func TestSignedMalformedPayloadAndForeignGenerationAreStillRejected(t *testing.T) {
	service, actor, at, calls, _ := fixture(t)
	for _, body := range []string{"{", "{}", `{"a":1,"a":2}`, `{"a":1,"b":2,"i":"run","s":"cancelled","l":1,"g":-1}`} {
		encoded := base64.RawURLEncoding.EncodeToString([]byte(body)) + "." + base64.RawURLEncoding.EncodeToString(service.signature(actor, []byte(body)))
		if _, err := service.List(context.Background(), actor, assistant.Cancelled, 1, encoded); err != assistant.ErrInvalid {
			t.Fatal("signed invalid body accepted")
		}
	}
	if *calls != 0 {
		t.Fatal("invalid signed payload performed I/O")
	}
	first, err := service.List(context.Background(), actor, assistant.Cancelled, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	service.repository = repositoryFunc(func(context.Context, assistant.Actor, assistant.HistoryQuery, time.Time) (assistant.HistoryPage, error) {
		return assistant.HistoryPage{Generation: 1}, nil
	})
	if _, err := service.List(context.Background(), actor, assistant.Cancelled, 1, first.NextCursor); err != assistant.ErrUnavailable {
		t.Fatal("changed restore generation accepted")
	}
	*at = time.Time{}
	if _, err := service.List(context.Background(), actor, assistant.Cancelled, 1, ""); err != assistant.ErrInvalid {
		t.Fatal("invalid clock accepted")
	}
}

func TestMalformedRepositoryResultsAndFailuresNeverExposePartialMetadata(t *testing.T) {
	for _, cause := range []error{assistant.ErrDenied, errors.New("private repository diagnostic")} {
		service, actor, _, _, _ := fixture(t)
		service.repository = repositoryFunc(func(context.Context, assistant.Actor, assistant.HistoryQuery, time.Time) (assistant.HistoryPage, error) {
			return assistant.HistoryPage{Entries: []assistant.HistoryEntry{{ID: "private"}}}, cause
		})
		page, err := service.List(context.Background(), actor, assistant.Cancelled, 1, "")
		if err == nil || err != assistant.ErrDenied && err != assistant.ErrUnavailable || len(page.Entries) != 0 {
			t.Fatal("failed repository exposed metadata")
		}
	}
	for _, mutate := range []func(*assistant.HistoryPage){func(p *assistant.HistoryPage) { p.Generation = -1 }, func(p *assistant.HistoryPage) { p.HasMore = true }, func(p *assistant.HistoryPage) { p.Entries = []assistant.HistoryEntry{{ID: "bad"}} }, func(p *assistant.HistoryPage) { p.Entries = make([]assistant.HistoryEntry, 11) }} {
		service, actor, _, _, _ := fixture(t)
		service.repository = repositoryFunc(func(context.Context, assistant.Actor, assistant.HistoryQuery, time.Time) (assistant.HistoryPage, error) {
			var result assistant.HistoryPage
			mutate(&result)
			return result, nil
		})
		if _, err := service.List(context.Background(), actor, assistant.Cancelled, 1, ""); err != assistant.ErrUnavailable {
			t.Fatal("invalid repository metadata accepted")
		}
	}
	service, actor, _, calls, _ := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, input := range []struct {
		ctx   context.Context
		actor assistant.Actor
		state assistant.State
		limit int
	}{{nil, actor, assistant.Completed, 1}, {ctx, actor, assistant.Completed, 1}, {context.Background(), assistant.Actor{}, assistant.Completed, 1}, {context.Background(), actor, "unknown", 1}, {context.Background(), actor, assistant.Completed, 11}} {
		if _, err := service.List(input.ctx, input.actor, input.state, input.limit, ""); err != assistant.ErrInvalid {
			t.Fatal("invalid request accepted")
		}
	}
	if *calls != 0 {
		t.Fatal("invalid request reached I/O")
	}
	var absent *Service
	if _, err := absent.List(context.Background(), actor, assistant.Completed, 1, ""); err != assistant.ErrInvalid {
		t.Fatal("nil service accepted")
	}
	if _, err := New(nil, make([]byte, 32), time.Now); err != assistant.ErrInvalid {
		t.Fatal("missing repository accepted")
	}
	if _, err := New(service.repository, []byte{1}, time.Now); err != assistant.ErrInvalid {
		t.Fatal("short key accepted")
	}
	if _, err := New(service.repository, make([]byte, 32), time.Now); err != assistant.ErrInvalid {
		t.Fatal("zero key accepted")
	}
	if _, err := New(service.repository, append([]byte{1}, make([]byte, 31)...), nil); err != assistant.ErrInvalid {
		t.Fatal("missing clock accepted")
	}
}

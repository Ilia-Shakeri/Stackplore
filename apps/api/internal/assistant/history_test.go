package assistant

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestHistoryBoundsAndProtectedMetadata(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Microsecond)
	query := HistoryQuery{State: Completed, UpperBound: at, Limit: 10}
	entry := HistoryEntry{ID: "private_run", SourceID: "source", Goal: ExplainResource, State: Completed, CreatedAt: at.Add(-time.Second), UpdatedAt: at, From: at.Add(-time.Minute), Until: at.Add(-time.Second)}
	if query.Validate(at) != nil || entry.Validate(query, at) != nil {
		t.Fatal("bounded metadata rejected")
	}
	for _, mutate := range []func(*HistoryQuery){func(q *HistoryQuery) { q.Limit = 0 }, func(q *HistoryQuery) { q.Limit = MaximumHistoryPage + 1 }, func(q *HistoryQuery) { q.State = "unknown" }, func(q *HistoryQuery) { q.UpperBound = at.Add(time.Second) }, func(q *HistoryQuery) { q.BeforeAt = at }, func(q *HistoryQuery) { q.BeforeID = "run" }, func(q *HistoryQuery) { q.BeforeAt, q.BeforeID = at.Add(time.Second), "run" }} {
		bad := query
		mutate(&bad)
		if bad.Validate(at) != ErrInvalid {
			t.Fatal("unbounded metadata query accepted")
		}
	}
	query.BeforeAt, query.BeforeID = entry.CreatedAt, entry.ID
	if entry.Validate(query, at) != ErrInvalid {
		t.Fatal("inclusive cursor repeated its own row")
	}
	query.BeforeID = "z_after"
	if entry.Validate(query, at) != nil {
		t.Fatal("same-time lexical keyset rejected")
	}
	for _, mutate := range []func(*HistoryEntry){func(e *HistoryEntry) { e.CreatedAt = at.Add(-MetadataRetention - time.Second) }, func(e *HistoryEntry) { e.Until = at }, func(e *HistoryEntry) { e.State = Queued }, func(e *HistoryEntry) { e.Goal = ExplainAlert }, func(e *HistoryEntry) { e.ToolCalls = MaximumToolCalls + 1 }, func(e *HistoryEntry) { e.ID = "../file" }} {
		bad := entry
		mutate(&bad)
		if bad.Validate(query, at) != ErrInvalid {
			t.Fatal("invalid metadata accepted")
		}
	}
	for _, value := range []any{entry, HistoryPage{Entries: []HistoryEntry{entry}}} {
		encoded, err := json.Marshal(value)
		if err != nil || string(encoded) != "{}" || strings.Contains(fmt.Sprintf("%+v", value), entry.ID) {
			t.Fatal("ordinary formatting exposed metadata")
		}
	}
}

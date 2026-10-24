package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/jackc/pgx/v5"
)

func TestAssistantHotPathQueryPlansAndRetentionBoundsOnRealPostgreSQL(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	for _, id := range []string{"plan_queued_a", "plan_queued_b"} {
		if _, err := store.AdmitAssistant(ctx, assistantAdmission(actor, id, at)); err != nil {
			t.Fatal("admit bounded query-plan fixtures")
		}
	}
	unknown := seedAssistantPlanHistory(t, ctx, store, actor, at)
	baseline := assistantPlanCounts(t, ctx, store.pool)
	plans := []struct {
		name, statement string
		arguments       []any
		rows, removed   int64
	}{
		{"claim", assistantClaimCandidatesSQL, []any{at.Add(time.Minute).UnixMicro()}, 2, 0},
		{"replay", assistantReplaySQL, []any{"plan_history_4095", int64(0), at.Add(-assistant.ReplayRetention).UnixMicro(), 20}, 4, 0},
		{"prune_events", assistantPruneEventsSQL, []any{at.Add(-assistant.ReplayRetention).UnixMicro(), 5}, 0, 5},
		{"prune_steps", assistantPruneStepsSQL, []any{at.Add(-assistant.MetadataRetention).UnixMicro(), 5}, 0, 5},
		{"prune_runs", assistantPruneRunsSQL, []any{at.Add(-assistant.MetadataRetention).UnixMicro(), 5}, 0, 5},
		{"prune_settled_usage", assistantPruneUsageSQL, []any{at.Add(-assistant.MetadataRetention).UnixMicro(), 5}, 0, 5},
		{"unknown_tombstone", assistantTombstoneSQL, []any{actor.UserID, unknown[:]}, 1, 0},
	}
	for _, plan := range plans {
		t.Run(plan.name, func(t *testing.T) {
			work, stop := context.WithTimeout(ctx, 5*time.Second)
			defer stop()
			tx, err := store.pool.Begin(work)
			if err != nil {
				t.Fatal("begin bounded query-plan transaction")
			}
			defer func() {
				cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
				defer stop()
				if err := tx.Rollback(cleanup); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
					t.Error("query-plan rollback did not complete")
				}
			}()
			var output []byte
			if err := tx.QueryRow(work, `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) `+plan.statement, plan.arguments...).Scan(&output); err != nil || len(output) > 256*1024 {
				t.Fatal("bounded query-plan execution failed")
			}
			var document []struct {
				Plan map[string]any `json:"Plan"`
			}
			if json.Unmarshal(output, &document) != nil || len(document) != 1 || document[0].Plan == nil {
				t.Fatal("query-plan document rejected")
			}
			actualRows, ok := document[0].Plan["Actual Rows"].(float64)
			if !ok || actualRows != float64(plan.rows) {
				t.Fatal("query-plan result cardinality changed")
			}
			after := assistantPlanCounts(t, work, tx)
			removed := baseline.metadata - after.metadata
			if removed != plan.removed || after.unknown != baseline.unknown || after.held != baseline.held || after.audits != baseline.audits {
				t.Fatal("query plan exceeded its batch or changed protected state")
			}
			// All subjects and predicates in these plans belong to synthetic fixtures.
			// Record actual plans without an index hint or hardware latency claim.
			t.Logf("query_plan %s %s", plan.name, string(output))
		})
	}
	if after := assistantPlanCounts(t, ctx, store.pool); after != baseline {
		t.Fatal("query-plan execution escaped its rollback")
	}
	for _, limit := range []int{1, 3, 17} {
		before := assistantPlanCounts(t, ctx, store.pool)
		removed, err := store.PruneAssistant(ctx, at, limit)
		after := assistantPlanCounts(t, ctx, store.pool)
		if err != nil || removed < 1 || removed > limit || before.metadata-after.metadata != int64(removed) || after.unknown != baseline.unknown || after.held != baseline.held || after.audits != baseline.audits {
			t.Fatal("aggregate retention exceeded its batch or pruned protected state")
		}
	}
	retry := assistantAdmission(actor, "plan_unknown_replay", at)
	retry.IdempotencyDigest = unknown
	if _, err := store.AdmitAssistant(ctx, retry); !errors.Is(err, assistant.ErrConflict) {
		t.Fatal("retention allowed unknown exposure to be admitted again")
	}
}

type assistantPlanCounter interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type assistantPlanCount struct{ metadata, unknown, held, audits int64 }

func assistantPlanCounts(t *testing.T, ctx context.Context, connection assistantPlanCounter) assistantPlanCount {
	t.Helper()
	var counts assistantPlanCount
	err := connection.QueryRow(ctx, `SELECT
(SELECT count(*) FROM assistant_runs)+(SELECT count(*) FROM assistant_steps)+(SELECT count(*) FROM assistant_status_events)+(SELECT count(*) FROM assistant_usage_ledger),
(SELECT count(*) FROM assistant_usage_ledger WHERE state='unknown'),
(SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation'),
(SELECT count(*) FROM audit_entries)`).Scan(&counts.metadata, &counts.unknown, &counts.held, &counts.audits)
	if err != nil {
		t.Fatal("count bounded query-plan fixture state")
	}
	return counts
}

func seedAssistantPlanHistory(t *testing.T, ctx context.Context, store *Store, actor assistant.Actor, at time.Time) [32]byte {
	t.Helper()
	const historyRows = 4096
	runs := make([][]any, 0, historyRows)
	steps := make([][]any, 0, 2*historyRows)
	events := make([][]any, 0, 4*historyRows)
	ledger := make([][]any, 0, historyRows+1)
	for index := range historyRows {
		created := at.Add(-12 * time.Hour)
		if index < historyRows/8 {
			created = at.Add(-8 * 24 * time.Hour)
		}
		created = created.Add(time.Duration(index) * time.Microsecond)
		updated := created.Add(time.Minute)
		a := assistantAdmission(actor, fmt.Sprintf("plan_history_%04d", index), created)
		runs = append(runs, []any{a.ID, a.ConversationID, actor.UserID, actor.SessionDigest[:], actor.DigestKeyID, actor.AuthRevision,
			a.IdempotencyDigest[:], a.Fingerprint[:], a.Goal, a.SourceID, a.SourceVersion, a.From.UnixMicro(), a.Until.UnixMicro(),
			int64(0), int64(2), a.PolicyVersion, a.ProfileID, a.ProfileVersion, false, false, "completed", int64(4), int64(1), int64(100), int64(10),
			created.UnixMicro(), updated.UnixMicro(), a.Deadline.UnixMicro(), int64(1), int64(1)})
		// A subset already had child metadata removed by earlier bounded batches.
		if index%8 != 0 {
			for step, kind := range []string{"inventory", "inference"} {
				units, maximum := int64(0), int64(0)
				if kind == "inference" {
					units, maximum = 10, 100
				}
				steps = append(steps, []any{a.ID, int64(step + 1), kind, "completed", int64(1), units, maximum, created.UnixMicro(), updated.UnixMicro()})
			}
			for sequence, state := range []string{"queued", "running", "running", "completed"} {
				events = append(events, []any{a.ID, int64(sequence + 1), int64(sequence + 1), state, updated.UnixMicro()})
			}
		}
		ledger = append(ledger, []any{a.ID, a.ID, actor.UserID, a.IdempotencyDigest[:], a.Fingerprint[:], int64(100), int64(10), "settled", int64(0), created.UnixMicro(), updated.UnixMicro(), "observed"})
	}
	for _, copy := range []struct {
		table   string
		columns []string
		rows    [][]any
	}{
		{"assistant_runs", []string{"id", "conversation_id", "owner_id", "session_digest", "digest_key_id", "auth_revision", "idempotency_digest", "fingerprint", "goal", "source_id", "source_version", "from_us", "until_us", "restore_generation", "configuration_version", "policy_version", "profile_id", "profile_version", "volatile", "paid", "state", "row_version", "fence", "reserved_units", "used_units", "created_at_us", "updated_at_us", "deadline_us", "tool_calls", "inference_calls"}, runs},
		{"assistant_steps", []string{"run_id", "step_number", "kind", "state", "fence", "units", "maximum_units", "started_at_us", "finished_at_us"}, steps},
		{"assistant_status_events", []string{"run_id", "sequence", "run_version", "state", "occurred_at_us"}, events},
		{"assistant_usage_ledger", []string{"reservation_id", "run_reference", "owner_id", "idempotency_digest", "fingerprint", "reserved_units", "settled_units", "state", "restore_generation", "created_at_us", "updated_at_us", "settlement_kind"}, ledger},
	} {
		if _, err := store.pool.CopyFrom(ctx, pgx.Identifier{copy.table}, copy.columns, pgx.CopyFromRows(copy.rows)); err != nil {
			t.Fatal("seed bounded synthetic assistant history")
		}
	}
	unknown := sha256.Sum256([]byte("synthetic_plan_unknown_tombstone"))
	if _, err := store.pool.Exec(ctx, `INSERT INTO assistant_usage_ledger(reservation_id,owner_id,idempotency_digest,reserved_units,state,restore_generation,created_at_us,updated_at_us)
VALUES('plan_unknown',$1,$2,100,'unknown',0,$3,$3)`, actor.UserID, unknown[:], at.Add(-8*24*time.Hour).UnixMicro()); err != nil {
		t.Fatal("seed bounded unknown-usage tombstone")
	}
	if _, err := store.pool.Exec(ctx, `UPDATE assistant_usage_accounts SET reserved_units=reserved_units+100 WHERE account_key IN ('installation',$1)`, "user:"+actor.UserID); err != nil {
		t.Fatal("reserve bounded unknown exposure")
	}
	if _, err := store.pool.Exec(ctx, `ANALYZE assistant_runs; ANALYZE assistant_steps; ANALYZE assistant_status_events; ANALYZE assistant_usage_ledger`); err != nil {
		t.Fatal("analyze bounded assistant plan fixtures")
	}
	return unknown
}

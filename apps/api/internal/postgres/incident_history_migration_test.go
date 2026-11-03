package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIncidentHistoryMigrationAcceptsOnlyClosedStepKinds(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	admission := assistantAdmission(actor, "history_step_schema", at)
	if _, err := store.AdmitAssistant(ctx, admission); err != nil {
		t.Fatal("admit owned migration fixture", err)
	}
	statement := `INSERT INTO assistant_steps(run_id,step_number,kind,state,fence,started_at_us,finished_at_us,maximum_units) VALUES($1,$2,$3,'completed',1,$4,$4,0)`
	for index, kind := range []string{"inventory", "health", "metrics", "events", "topology", "logs", "inference", "alert_history", "workload_history"} {
		if _, err := store.pool.Exec(ctx, statement, admission.ID, index+1, kind, at.UnixMicro()); err != nil {
			t.Fatal("closed step kind rejected after migration", err)
		}
	}
	_, err := store.pool.Exec(ctx, statement, admission.ID, 10, "arbitrary_history", at.UnixMicro())
	var failure *pgconn.PgError
	if !errors.As(err, &failure) || failure.Code != "23514" {
		t.Fatal("unknown step kind bypassed the SQL constraint")
	}
}

func TestIncidentHistoryMigrationReservesSeparateDurableReads(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	run := assistantAdmitClaim(t, store, actor, "history_reservations", at)
	for _, kind := range []assistant.StepKind{assistant.AlertHistory, assistant.WorkloadHistory} {
		updated, step, err := store.BeginAssistantStep(ctx, assistant.StepStart{Lease: assistantLease(run, run.UpdatedAt), Kind: kind})
		if err != nil || step.Kind != kind || updated.ToolCalls != run.ToolCalls+1 || updated.InferenceCalls != 0 {
			t.Fatal("history did not reserve its own durable tool call", err)
		}
		run, err = store.FinishAssistantStep(ctx, assistant.StepFinish{Lease: assistantLease(updated, updated.UpdatedAt), Number: step.Number})
		if err != nil {
			t.Fatal("history reservation did not settle", err)
		}
	}
	var completed int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM assistant_steps WHERE run_id=$1 AND state='completed' AND kind IN ('alert_history','workload_history')`, run.ID).Scan(&completed); err != nil || completed != 2 || run.ToolCalls != 2 {
		t.Fatal("separate completed history reservations were not retained", err)
	}
}

package postgres

import (
	"context"
	"errors"
	"testing"

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

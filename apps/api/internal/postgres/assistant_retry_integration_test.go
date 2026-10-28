package postgres

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestAssistantExistingOnlyFlagDoesNotChangeMaterial(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.October, 21, 0, 0, 0, 0, time.UTC)
	a := assistantAdmission(assistant.Actor{UserID: "fixture_user", SessionDigest: [32]byte{1}, DigestKeyID: "fixture_key", AuthRevision: 1}, "material_run", at)
	original := assistantMaterial(a)
	a.RequireExisting = true
	if assistantMaterial(a) != original {
		t.Fatal("trusted retry guard changed the original material fingerprint")
	}
}

func TestAssistantExistingOnlyAdmissionChecksMaterialAndCurrentAuthority(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	a := assistantAdmission(actor, "existing_only_run", at)
	a.RequireExisting = true
	if _, err := store.AdmitAssistant(ctx, a); err != assistant.ErrConflict {
		t.Fatal("existing-only retry admitted a missing run")
	}
	assertAssistantRetryStorage(t, ctx, store, 0, 0, 0)
	a.RequireExisting = false
	original, err := store.AdmitAssistant(ctx, a)
	if err != nil {
		t.Fatal("initial admission failed")
	}
	a.RequireExisting = true
	for range 2 {
		run, err := store.AdmitAssistant(ctx, a)
		if err != nil || run != original {
			t.Fatal("matching existing-only retry changed the original run")
		}
	}
	assertAssistantRetryStorage(t, ctx, store, 1, 1, a.ReservedUnits)
	changed := a
	changed.Fingerprint = sha256.Sum256([]byte("changed_fixture_material"))
	if _, err = store.AdmitAssistant(ctx, changed); err != assistant.ErrConflict {
		t.Fatal("existing-only retry bypassed material conflict")
	}
	if _, err = store.pool.Exec(ctx, `UPDATE infrastructure_sources SET enabled=0 WHERE id=$1`, a.SourceID); err != nil {
		t.Fatal("disable owned source fixture")
	}
	if _, err = store.AdmitAssistant(ctx, a); err != assistant.ErrDenied {
		t.Fatal("existing-only retry bypassed current source authority")
	}
	assertAssistantRetryStorage(t, ctx, store, 1, 1, a.ReservedUnits)
}

func TestAssistantExistingOnlyRetryCannotRecreateRunRemovedAfterRead(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	a := assistantAdmission(actor, "removed_retry_run", at)
	if _, err := store.AdmitAssistant(ctx, a); err != nil {
		t.Fatal("initial retry fixture admission failed")
	}
	if run, err := store.CancelAssistant(ctx, actor, a.ID, at); err != nil || run.State != assistant.Cancelled {
		t.Fatal("cancel owned retry fixture")
	}
	if run, err := store.GetAssistant(ctx, actor, a.ID, at); err != nil || run.State != assistant.Cancelled {
		t.Fatal("preflight did not observe the owned terminal run")
	}
	// Fault injection removes only settled fixture history after the successful
	// preflight. No unknown usage is removed and this is not a retention claim.
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatal("start owned retry deletion fixture")
	}
	defer func() {
		rollback, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		_ = transaction.Rollback(rollback)
	}()
	for _, statement := range []string{
		`DELETE FROM assistant_status_events WHERE run_id=$1`,
		`DELETE FROM assistant_runs WHERE id=$1 AND state='cancelled'`,
		`DELETE FROM assistant_usage_ledger WHERE reservation_id=$1 AND state='settled'`,
	} {
		command, deleteErr := transaction.Exec(ctx, statement, a.ID)
		if deleteErr != nil || command.RowsAffected() < 1 {
			t.Fatal("remove only settled owned retry fixture")
		}
	}
	if err = transaction.Commit(ctx); err != nil {
		t.Fatal("commit owned retry deletion fixture")
	}
	a.RequireExisting = true
	if _, err = store.AdmitAssistant(ctx, a); err != assistant.ErrConflict {
		t.Fatal("successful preflight authorized recreation after deletion")
	}
	assertAssistantRetryStorage(t, ctx, store, 0, 0, 0)
}

func assertAssistantRetryStorage(t *testing.T, ctx context.Context, store *Store, runs, ledger int, reserved int64) {
	t.Helper()
	var actualRuns, actualLedger int
	var actualReserved int64
	err := store.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM assistant_runs),(SELECT count(*) FROM assistant_usage_ledger),(SELECT reserved_units FROM assistant_usage_accounts WHERE account_key='installation')`).Scan(&actualRuns, &actualLedger, &actualReserved)
	if err != nil || actualRuns != runs || actualLedger != ledger || actualReserved != reserved {
		t.Fatal("retry changed run count, protected usage or reservation")
	}
}

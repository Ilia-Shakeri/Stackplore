package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
)

func TestAssistantMetadataHistoryIsBoundedOwnerPrivateAndRestoreFenced(t *testing.T) {
	store, actor, at := assistantFixture(t)
	ctx := context.Background()
	user := authRepositoryTestUser("history_viewer", "historyviewer", auth.RoleViewer, at)
	session := authRepositoryTestSession(t, store, user, "history_viewer_session", "historyviewer", at)
	if err := store.withinTransaction(ctx, func(ctx context.Context, tx *transaction) error {
		if err := insertUserTx(ctx, tx.transaction, user); err != nil {
			return err
		}
		return insertSessionTx(ctx, tx.transaction, session)
	}); err != nil {
		t.Fatal(err)
	}
	other := actor
	other.UserID, other.AuthRevision = string(user.ID), int64(user.AuthRevision)
	var digest []byte
	if err := store.pool.QueryRow(ctx, `SELECT token_digest,token_digest_key_id FROM sessions WHERE user_id=$1`, other.UserID).Scan(&digest, &other.DigestKeyID); err != nil {
		t.Fatal(err)
	}
	copy(other.SessionDigest[:], digest)
	for _, seed := range []struct {
		actor assistant.Actor
		id    string
	}{{actor, "history_a"}, {actor, "history_b"}, {other, "foreign_history"}} {
		run, err := store.AdmitAssistant(ctx, assistantAdmission(seed.actor, seed.id, at))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.CancelAssistant(ctx, seed.actor, run.ID, time.Now().UTC().Truncate(time.Microsecond)); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	created := now.Add(-time.Second)
	if _, err := store.pool.Exec(ctx, `UPDATE assistant_runs SET created_at_us=$1,from_us=$2,until_us=$1,deadline_us=$3 WHERE id IN ('history_a','history_b')`, created.UnixMicro(), created.Add(-time.Minute).UnixMicro(), created.Add(assistant.MaximumRunDuration).UnixMicro()); err != nil {
		t.Fatal(err)
	}
	query := assistant.HistoryQuery{State: assistant.Cancelled, UpperBound: now, Limit: 1, Generation: -1}
	page, err := store.ListAssistantHistory(ctx, actor, query, now)
	if err != nil || len(page.Entries) != 1 || !page.HasMore || page.Entries[0].ID != "history_b" {
		t.Fatal("owner/state/keyset bound failed", err)
	}
	query.BeforeAt, query.BeforeID = page.Entries[0].CreatedAt, page.Entries[0].ID
	next, err := store.ListAssistantHistory(ctx, actor, query, now)
	if err != nil || len(next.Entries) != 1 || next.HasMore || next.Entries[0].ID != "history_a" {
		t.Fatal("same-time page repeated or leaked another owner", err)
	}
	query.BeforeAt, query.BeforeID = time.Time{}, ""
	foreign, err := store.ListAssistantHistory(ctx, other, query, now)
	if err != nil || len(foreign.Entries) != 1 || foreign.Entries[0].ID != "foreign_history" {
		t.Fatal("viewer could not read only their own metadata", err)
	}
	forged := other
	forged.UserID = actor.UserID
	if _, err := store.ListAssistantHistory(ctx, forged, query, now); err != assistant.ErrDenied {
		t.Fatal("foreign session borrowed administrator ownership", err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE installation_metadata SET restore_generation=restore_generation+1,restored_at_us=$1 WHERE singleton_id=1`, now.UnixMicro()); err != nil {
		t.Fatal(err)
	}
	query.Generation = page.Generation
	if _, err := store.ListAssistantHistory(ctx, actor, query, now); err != assistant.ErrDenied {
		t.Fatal("old cursor crossed restore generation", err)
	}
	query.Generation = -1
	cleared, err := store.ListAssistantHistory(ctx, actor, query, now)
	if err != nil || len(cleared.Entries) != 0 || cleared.HasMore {
		t.Fatal("restored old metadata crossed generation fence", err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE sessions SET revoked_at_us=$1,revoke_reason_code='session.revoked' WHERE token_digest=$2`, now.UnixMicro(), actor.SessionDigest[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ListAssistantHistory(ctx, actor, query, now); err != assistant.ErrDenied {
		t.Fatal("revoked actor read metadata", err)
	}
	query.Limit = assistant.MaximumHistoryPage + 1
	if _, err := store.ListAssistantHistory(ctx, other, query, now); err != assistant.ErrInvalid {
		t.Fatal("oversized metadata page accepted", err)
	}
}

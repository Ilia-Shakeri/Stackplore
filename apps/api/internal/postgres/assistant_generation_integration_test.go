package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestAssistantRestoreGenerationReadKeepsCurrentFence(t *testing.T) {
	store, _, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var before int64
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries`).Scan(&before); err != nil {
		t.Fatal("read owned audit fixture")
	}
	for range 2 {
		if generation, err := store.AssistantRestoreGeneration(ctx); err != nil || generation != 0 {
			t.Fatal("initial restore metadata read failed")
		}
	}
	if _, err := store.pool.Exec(ctx, `UPDATE installation_metadata SET restore_generation=1,restored_at_us=$1 WHERE singleton_id=1`, at.UnixMicro()); err != nil {
		t.Fatal("advance owned restore fixture")
	}
	if generation, err := store.AssistantRestoreGeneration(ctx); err != nil || generation != 1 {
		t.Fatal("current restore fence was cached or changed")
	}
	var after int64
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries`).Scan(&after); err != nil || after != before {
		t.Fatal("restore metadata reads mutated audit state")
	}
	if _, err := store.AssistantRestoreGeneration(nil); err != assistant.ErrInvalid {
		t.Fatal("missing generation context accepted")
	}
	if _, err := (*Store)(nil).AssistantRestoreGeneration(ctx); err != assistant.ErrUnavailable {
		t.Fatal("nil generation store accepted")
	}
	if err := store.Close(); err != nil {
		t.Fatal("close owned generation fixture")
	}
	if _, err := store.AssistantRestoreGeneration(ctx); err != assistant.ErrUnavailable {
		t.Fatal("closed generation store returned stale metadata")
	}
}

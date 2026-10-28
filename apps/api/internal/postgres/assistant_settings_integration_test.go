package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestAssistantSettingsReadDoesNotMutateControlOrAudit(t *testing.T) {
	store, _, at := assistantFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var before int64
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries`).Scan(&before); err != nil {
		t.Fatal("read owned settings audit count")
	}
	want := assistant.Settings{Version: 2, Enabled: true, PolicyVersion: 1, ProfileID: "local_test", ProfileVersion: 1, UserQuota: 1000, InstallationQuota: 10000}
	for range 2 {
		got, err := store.GetAssistantSettings(ctx)
		if err != nil || got != want {
			t.Fatal("trusted settings snapshot changed control metadata")
		}
	}
	var after int64
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries`).Scan(&after); err != nil || before != after {
		t.Fatal("settings read emitted an audit mutation")
	}
	want.Enabled = false
	want, err := store.ConfigureAssistant(ctx, want, assistantOperator("settings_read_disable", at))
	if err != nil {
		t.Fatal("update owned settings fixture")
	}
	got, err := store.GetAssistantSettings(ctx)
	if err != nil || got != want || got.Enabled {
		t.Fatal("settings read did not observe the committed control change")
	}
	if got, err := store.GetAssistantSettings(nil); err != assistant.ErrInvalid || got != (assistant.Settings{}) {
		t.Fatal("missing read context returned metadata")
	}
	if err := store.Close(); err != nil {
		t.Fatal("close owned settings fixture")
	}
	if got, err := store.GetAssistantSettings(ctx); err != assistant.ErrUnavailable || got != (assistant.Settings{}) {
		t.Fatal("closed settings reader returned stale metadata")
	}
}

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestAuditLifecycleRequiresVerifiedArchiveAndPrunesBoundedOldRows(t *testing.T) {
	store, _, _ := sourceSettingsFixture(t)
	ctx := context.Background()
	policy := domain.AuditLifecyclePolicy{RetentionDays: 90, QuotaRows: 1000}
	if err := store.ConfigureAuditLifecyclePolicy(ctx, policy); err != nil {
		t.Fatal(err)
	}
	removed, err := store.PruneAuditHistory(ctx, 10)
	if err != nil || removed != 0 {
		t.Fatalf("without archive removed=%d err=%v", removed, err)
	}
	archive := time.Now().UTC().Add(-91 * 24 * time.Hour).Truncate(time.Microsecond)
	policy.ArchiveVerifiedThrough = archive
	if err = store.ConfigureAuditLifecyclePolicy(ctx, policy); err != nil {
		t.Fatal(err)
	}
	entry, err := systemAuditEntry("audit.policy_update", "audit.fixture")
	if err != nil {
		t.Fatal(err)
	}
	entry.OccurredAt = archive.Add(-time.Microsecond)
	if err = insertAuditFixture(ctx, store, entry); err != nil {
		t.Fatal(err)
	}
	removed, err = store.PruneAuditHistory(ctx, 1)
	if err != nil || removed != 1 {
		t.Fatalf("bounded prune removed=%d err=%v", removed, err)
	}
	status, err := store.ReadAuditPolicyStatus(ctx)
	if err != nil || status.ArchiveVerifiedThrough == nil || !status.ArchiveVerifiedThrough.Equal(archive) {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func insertAuditFixture(ctx context.Context, store *Store, entry domain.AuditEntry) error {
	return store.withinAuthTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		return insertAuthAuditTx(ctx, transaction.transaction, entry)
	})
}

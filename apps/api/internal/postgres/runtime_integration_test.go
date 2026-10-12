package postgres

import (
	"context"
	"errors"
	"io/fs"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
)

func TestRuntimeOwnershipRejectsBeforePendingMigration(t *testing.T) {
	store, options, _ := openIntegrationStore(t)
	files := migrationFixture(t)
	files["000007_blocked.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE must_not_exist (id bigint PRIMARY KEY);")}
	second, err := Open(context.Background(), options, files)
	if second != nil {
		_ = second.Close()
	}
	if !errors.Is(err, ErrOwnershipUnavailable) {
		t.Fatal("second runtime was not denied by ownership")
	}
	var exists bool
	if err := store.pool.QueryRow(context.Background(), `SELECT to_regclass('must_not_exist') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatal("denied second runtime changed the active schema")
	}
}

func TestOwnershipLossSealsStoreAndSignalsRuntime(t *testing.T) {
	store, _, _ := openIntegrationStore(t)
	var unlocked bool
	if err := store.runtimeLock.QueryRow(context.Background(), `SELECT pg_advisory_unlock($1)`, int64(runtimeLockKey)).Scan(&unlocked); err != nil || !unlocked {
		t.Fatal("could not release fixture ownership")
	}
	if err := store.Health(context.Background()); !errors.Is(err, ErrOwnershipUnavailable) {
		t.Fatal("missing ownership was not rejected")
	}
	select {
	case <-store.OwnershipLost():
	default:
		t.Fatal("ownership loss did not signal the runtime")
	}
	if _, err := store.SetupRequired(context.Background()); !errors.Is(err, auth.ErrAuthRepositoryUnavailable) {
		t.Fatal("store accepted an auth read after ownership loss")
	}
	if _, _, err := store.AuthDigestKey(); !errors.Is(err, ErrStoreClosed) {
		t.Fatal("store returned a digest key after ownership loss")
	}
}

func TestHealthChecksShareBoundedOwnershipSession(t *testing.T) {
	store, _, _ := openIntegrationStore(t)
	var wait sync.WaitGroup
	errorsCh := make(chan error, 24)
	for range 24 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errorsCh <- store.Health(context.Background())
		}()
	}
	wait.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal("concurrent health checks collided")
		}
	}
}

func TestConcurrentCloseAndDigestKeyReadsRemainSealed(t *testing.T) {
	store, _, _ := openIntegrationStore(t)
	var wait sync.WaitGroup
	start := make(chan struct{})
	for range 16 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			for range 100 {
				_, key, err := store.AuthDigestKey()
				if err != nil && !errors.Is(err, ErrStoreClosed) {
					t.Error("unexpected concurrent digest-key read error")
				}
				clear(key)
				_ = store.snapshotAuthDigestKey()
			}
		}()
	}
	close(start)
	if err := store.Close(); err != nil {
		t.Fatal("close concurrent digest-key fixture")
	}
	wait.Wait()
	if _, key, err := store.AuthDigestKey(); !errors.Is(err, ErrStoreClosed) || key != nil {
		t.Fatal("closed store returned a digest key")
	}
}

func TestTransactionAndAuthReadsUseStoreDeadline(t *testing.T) {
	store, _, admin := openIntegrationStore(t)
	store.queryTimeout = 100 * time.Millisecond
	err := store.withinTransaction(context.Background(), func(ctx context.Context, tx *transaction) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 150*time.Millisecond {
			t.Fatal("transaction work lacks a bounded deadline")
		}
		_, err := tx.transaction.Exec(ctx, `SELECT pg_sleep(1)`)
		return err
	})
	if err == nil {
		t.Fatal("slow transaction escaped its deadline")
	}
	identifier := store.pool.Config().ConnConfig.RuntimeParams["search_path"]
	if _, err := admin.Exec(context.Background(), "BEGIN; LOCK TABLE "+identifier+".installation_metadata IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal("could not lock auth read fixture")
	}
	defer func() { _, _ = admin.Exec(context.Background(), "ROLLBACK") }()
	start := time.Now()
	if _, err := store.SetupRequired(context.Background()); !errors.Is(err, auth.ErrAuthRepositoryUnavailable) {
		t.Fatal("locked auth read escaped its store deadline")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("auth read deadline was not bounded")
	}
}

func TestMigrationUpgradeFailureAndChangedHistoryAreRejected(t *testing.T) {
	store, options, _ := openIntegrationStore(t)
	if err := store.Close(); err != nil {
		t.Fatal("close migration fixture")
	}
	files := migrationFixture(t)
	files["000007_failed.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE rollback_probe (id bigint PRIMARY KEY); SELECT missing_column;")}
	if candidate, err := Open(context.Background(), options, files); !errors.Is(err, ErrMigrationRejected) {
		if candidate != nil {
			_ = candidate.Close()
		}
		t.Fatal("failed migration was accepted")
	}
	reopened, err := Open(context.Background(), options, postgresmigrations.FS)
	if err != nil {
		t.Fatal("failed migration damaged valid history")
	}
	var exists bool
	if err := reopened.pool.QueryRow(context.Background(), `SELECT to_regclass('rollback_probe') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatal("failed migration was not rolled back")
	}
	_ = reopened.Close()
	changed := migrationFixture(t)
	changed["000001_core_identity.sql"].Data = append(changed["000001_core_identity.sql"].Data, []byte("\n-- changed\n")...)
	if candidate, err := Open(context.Background(), options, changed); !errors.Is(err, ErrMigrationRejected) {
		if candidate != nil {
			_ = candidate.Close()
		}
		t.Fatal("changed migration history was accepted")
	}
}

func TestOutboxCursorWaitsForNotificationCommit(t *testing.T) {
	store, _, _ := openIntegrationStore(t)
	ctx := context.Background()
	started := make(chan struct{})
	release := make(chan struct{})
	writerDone := make(chan error, 1)
	now := time.Now().UTC().Truncate(time.Microsecond)
	go func() {
		writerDone <- store.withinTransaction(ctx, func(ctx context.Context, tx *transaction) error {
			if _, err := tx.transaction.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, notificationOutboxLockKey); err != nil {
				return err
			}
			if _, err := tx.transaction.Exec(ctx, insertNotificationStatement,
				"notification_commit_order", nil, nil, nil, "notification.alert_firing", "error", "alert.condition_matched", now.UnixMicro(), int64(0)); err != nil {
				return err
			}
			close(started)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	<-started
	shortContext, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	_, err := store.EnqueueWebhookDeliveries(shortContext, "webhook_commit_order", 3, now, 10)
	cancel()
	if err == nil {
		close(release)
		<-writerDone
		t.Fatal("outbox cursor ran ahead of uncommitted notification")
	}
	close(release)
	if err := <-writerDone; err != nil {
		t.Fatal("notification transaction failed")
	}
	created, err := store.EnqueueWebhookDeliveries(ctx, "webhook_commit_order", 3, now, 10)
	if err != nil || created != 1 {
		t.Fatal("committed notification was skipped")
	}
	var count int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM notification_deliveries WHERE notification_id=$1`, "notification_commit_order").Scan(&count); err != nil || count != 1 {
		t.Fatal("outbox delivery did not persist exactly once")
	}
}

func TestUnknownMigrationLedgerRefusesReadinessAndReopen(t *testing.T) {
	store, options, _ := openIntegrationStore(t)
	if _, err := store.pool.Exec(context.Background(), `INSERT INTO schema_migrations
(version, filename, checksum_sha256, applied_at)
VALUES(7, '000007_unknown.sql', $1, $2)`, make([]byte, 32), time.Now().UTC()); err != nil {
		t.Fatal("seed future migration ledger")
	}
	if err := store.Health(context.Background()); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatal("future migration ledger passed readiness")
	}
	_ = store.Close()
	if candidate, err := Open(context.Background(), options, postgresmigrations.FS); !errors.Is(err, ErrMigrationRejected) {
		if candidate != nil {
			_ = candidate.Close()
		}
		t.Fatal("future migration ledger passed startup")
	}
}

func TestConcurrentSelfDemotionsKeepLastAdministrator(t *testing.T) {
	store, _, _ := openIntegrationStore(t)
	createdAt := authRepositoryTestTime()
	users := []auth.User{
		authRepositoryTestUser("usr_self_one", "selfone", auth.RoleAdministrator, createdAt),
		authRepositoryTestUser("usr_self_two", "selftwo", auth.RoleAdministrator, createdAt),
	}
	sessions := []auth.Session{
		authRepositoryTestSession(t, store, users[0], "sess_self_one", "self_one", createdAt),
		authRepositoryTestSession(t, store, users[1], "sess_self_two", "self_two", createdAt),
	}
	seedAuthRepositoryUsers(t, store, users, sessions)
	commits := make([]auth.UserUpdateCommit, 2)
	for index, user := range users {
		actor := auth.Principal{UserID: user.ID, SessionID: sessions[index].ID, Role: user.Role, AuthRevision: user.AuthRevision}
		at := createdAt.Add(2 * time.Second)
		grant := issueAuthRepositoryGrant(t, store, actor, string(user.ID), auth.RecentAuthPurposeUserUpdate, createdAt.Add(time.Second), at)
		next := user
		next.Role, next.AuthRevision, next.Version, next.UpdatedAt = auth.RoleViewer, user.AuthRevision+1, user.Version+1, at
		commits[index] = auth.UserUpdateCommit{
			Actor: actor, Grant: grant, User: next, ExpectedVersion: user.Version,
			Audit: authRepositoryTestAudit("audit_self_"+string(user.ID), domain.AuditActor{Kind: domain.AuditActorUser, ID: string(user.ID)}, "auth.user_update", user.ID, "auth.user_updated", at),
		}
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, commit := range commits {
		go func() { <-start; results <- store.CommitUserUpdate(context.Background(), commit) }()
	}
	close(start)
	success, refused := 0, 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			success++
		case errors.Is(err, auth.ErrAuthRepositoryLastAdministrator):
			refused++
		default:
			t.Fatal("self-demotion race returned an unexpected result")
		}
	}
	if success != 1 || refused != 1 {
		t.Fatal("self-demotion race did not protect last administrator")
	}
}

func migrationFixture(t *testing.T) fstest.MapFS {
	t.Helper()
	files := fstest.MapFS{}
	entries, err := fs.ReadDir(postgresmigrations.FS, ".")
	if err != nil {
		t.Fatal("read embedded migrations")
	}
	for _, entry := range entries {
		data, err := fs.ReadFile(postgresmigrations.FS, entry.Name())
		if err != nil {
			t.Fatal("read embedded migration")
		}
		files[entry.Name()] = &fstest.MapFile{Data: data}
	}
	return files
}

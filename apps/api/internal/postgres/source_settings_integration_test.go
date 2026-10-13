package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/sourcesettings"
)

func sourceSettingsFixture(t *testing.T, offset ...time.Duration) (*Store, auth.Principal, time.Time) {
	t.Helper()
	store := openAuthRepositoryTestStore(t)
	at := authRepositoryTestTime()
	if len(offset) > 0 {
		at = time.Now().UTC().Add(offset[0]).Truncate(time.Microsecond)
		if _, err := store.pool.Exec(context.Background(), `UPDATE installation_metadata SET created_at_us=$1 WHERE singleton_id=1`, at.Add(-time.Second).UnixMicro()); err != nil {
			t.Fatal(err)
		}
	}
	user := authRepositoryTestUser("usr_source_admin", "sourceadmin", auth.RoleAdministrator, at)
	session := authRepositoryTestSession(t, store, user, "sess_source_admin", "source_admin", at)
	seedAuthRepositoryUsers(t, store, []auth.User{user}, []auth.Session{session})
	actor := auth.Principal{UserID: user.ID, SessionID: session.ID, Role: user.Role, AuthRevision: user.AuthRevision}
	if err := store.EnsureSourceConfigurations(context.Background(), []domain.SourceConfiguration{
		{Source: domain.InfrastructureSource{ID: "source_demo", Provider: domain.ProviderDemo, Name: "Demo", Synthetic: true},
			ProfileID: domain.SourceProfileDemoFixture, Enabled: true, Version: 1, CreatedAt: at, UpdatedAt: at},
		{Source: domain.InfrastructureSource{ID: "source_docker", Provider: domain.ProviderDocker, Name: "Docker"},
			ProfileID: domain.SourceProfileDockerSocketDevelopment, Enabled: true, Version: 1, CreatedAt: at, UpdatedAt: at},
	}); err != nil {
		t.Fatal(err)
	}
	return store, actor, at
}

func sourcePreviewCommand(actor auth.Principal, at time.Time) sourcesettings.PreviewCommand {
	return sourcesettings.PreviewCommand{Actor: actor, SourceID: "source_demo", ExpectedVersion: 1, Enabled: false, ProfileAvailable: true,
		Digest: sha256.Sum256([]byte("synthetic-source-confirmation")), CreatedAt: at, ExpiresAt: at.Add(sourcesettings.ConfirmationLifetime)}
}

func sourceUpdateCommand(t *testing.T, store *Store, preview sourcesettings.PreviewCommand, at time.Time, suffix string) sourcesettings.UpdateCommand {
	t.Helper()
	grant := issueAuthRepositoryGrant(t, store, preview.Actor, suffix, auth.RecentAuthPurposeSourceUpdate, preview.CreatedAt, at)
	return sourcesettings.UpdateCommand{Actor: preview.Actor, SourceID: preview.SourceID, ExpectedVersion: preview.ExpectedVersion,
		Enabled: preview.Enabled, ProfileAvailable: preview.ProfileAvailable, ConfirmationDigest: preview.Digest, Grant: grant,
		Audit: domain.AuditEntry{ID: domain.AuditEntryID("audit_source_" + suffix), OccurredAt: at,
			Actor: domain.AuditActor{Kind: domain.AuditActorUser, ID: string(preview.Actor.UserID)}, Action: "source.update",
			Target: domain.AuditTarget{Kind: "infrastructure.source", ID: string(preview.SourceID)}, Outcome: domain.AuditSuccess,
			RequestID: domain.RequestID("request_source_" + suffix), ReasonCode: "source.collection_disabled"}}
}

func TestSourceUpdateConcurrentReplayCommitsOneToggleAndAudit(t *testing.T) {
	store, actor, at := sourceSettingsFixture(t)
	preview := sourcePreviewCommand(actor, at.Add(time.Second))
	if _, err := store.CreateSourcePreview(context.Background(), preview); err != nil {
		t.Fatal(err)
	}
	command := sourceUpdateCommand(t, store, preview, at.Add(2*time.Second), "race")
	var wait sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := store.CommitSourceUpdate(context.Background(), command)
			results <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	wins, conflicts := 0, 0
	for err := range results {
		if err == nil {
			wins++
		} else if errors.Is(err, sourcesettings.ErrVersionConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("source update replay did not serialize")
	}
	configuration, err := store.GetSourceConfiguration(context.Background(), "source_demo")
	if err != nil || configuration.Enabled || configuration.Version != 2 {
		t.Fatal("source toggle not committed")
	}
	var auditCount, confirmationCount, usedGrantCount int
	if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_entries WHERE action='source.update'`).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM source_update_confirmations`).Scan(&confirmationCount); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM recent_auth_grants WHERE purpose='source.update' AND consumed_at_us IS NOT NULL`).Scan(&usedGrantCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 || confirmationCount != 0 || usedGrantCount != 1 {
		t.Fatal("source update was not atomic")
	}
}

func TestSourceUpdateRejectsWrongTargetStateExpiryAndPreservesGrant(t *testing.T) {
	for _, testCase := range []string{"target", "state", "expiry", "superseded", "purpose", "revoked", "session", "role_revision"} {
		t.Run(testCase, func(t *testing.T) {
			store, actor, at := sourceSettingsFixture(t)
			preview := sourcePreviewCommand(actor, at.Add(time.Second))
			if _, err := store.CreateSourcePreview(context.Background(), preview); err != nil {
				t.Fatal(err)
			}
			operationAt := at.Add(2 * time.Second)
			if testCase == "expiry" {
				operationAt = preview.ExpiresAt
			}
			command := sourceUpdateCommand(t, store, preview, operationAt, testCase)
			switch testCase {
			case "target":
				command.SourceID = "source_docker"
				command.Audit.Target.ID = "source_docker"
			case "state":
				command.Enabled = true
				command.Audit.ReasonCode = "source.collection_enabled"
			case "superseded":
				second := preview
				second.Digest = sha256.Sum256([]byte("synthetic-new-confirmation"))
				if _, err := store.CreateSourcePreview(context.Background(), second); err != nil {
					t.Fatal(err)
				}
			case "purpose":
				command.Grant.Purpose = auth.RecentAuthPurposeNotificationTest
			case "revoked":
				if _, err := store.pool.Exec(context.Background(), `UPDATE sessions SET revoked_at_us=$1,revoke_reason_code='auth.logout' WHERE id=$2`, operationAt.UnixMicro(), string(actor.SessionID)); err != nil {
					t.Fatal(err)
				}
			case "session":
				command.Actor.SessionID = "sess_other"
				command.Grant.Actor = command.Actor
			case "role_revision":
				if _, err := store.pool.Exec(context.Background(), `UPDATE users SET role='viewer',auth_revision=auth_revision+1 WHERE id=$1`, string(actor.UserID)); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := store.CommitSourceUpdate(context.Background(), command); err == nil {
				t.Fatal("unsafe source update accepted")
			}
			configuration, err := store.GetSourceConfiguration(context.Background(), "source_demo")
			if err != nil || !configuration.Enabled || configuration.Version != 1 {
				t.Fatal("rejected update changed source")
			}
			var consumed int
			if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM recent_auth_grants WHERE consumed_at_us IS NOT NULL`).Scan(&consumed); err != nil || consumed != 0 {
				t.Fatal("rejected update spent grant")
			}
		})
	}
}

func TestSourceUpdateChecksExpiryAfterTransactionLockWithActualDatabaseClock(t *testing.T) {
	store, actor, at := sourceSettingsFixture(t, -3*time.Minute)
	preview := sourcePreviewCommand(actor, at.Add(time.Second))
	if _, err := store.CreateSourcePreview(context.Background(), preview); err != nil {
		t.Fatal(err)
	}
	command := sourceUpdateCommand(t, store, preview, at.Add(2*time.Second), "late_clock")
	// The caller's prepared time predates expiry; the transaction now occurs later.
	if _, err := store.CommitSourceUpdate(context.Background(), command); !errors.Is(err, sourcesettings.ErrConfirmationRejected) {
		t.Fatal("expired preview accepted with old authorization time")
	}
	configuration, err := store.GetSourceConfiguration(context.Background(), preview.SourceID)
	if err != nil || !configuration.Enabled || configuration.Version != 1 {
		t.Fatal("expired preview changed source")
	}
}

func TestSourceUpdateAuditFailureRollsBackConfirmationGrantAndConfiguration(t *testing.T) {
	store, actor, at := sourceSettingsFixture(t)
	preview := sourcePreviewCommand(actor, at.Add(time.Second))
	if _, err := store.CreateSourcePreview(context.Background(), preview); err != nil {
		t.Fatal(err)
	}
	command := sourceUpdateCommand(t, store, preview, at.Add(2*time.Second), "rollback")
	if err := store.withinTransaction(context.Background(), func(ctx context.Context, tx *transaction) error {
		return insertAuthAuditTx(ctx, tx.transaction, command.Audit)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CommitSourceUpdate(context.Background(), command); !errors.Is(err, sourcesettings.ErrUnavailable) {
		t.Fatal("audit failure was hidden")
	}
	var previews, grants int
	if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM source_update_confirmations`).Scan(&previews); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM recent_auth_grants WHERE consumed_at_us IS NOT NULL`).Scan(&grants); err != nil {
		t.Fatal(err)
	}
	configuration, err := store.GetSourceConfiguration(context.Background(), "source_demo")
	if err != nil || !configuration.Enabled || configuration.Version != 1 || previews != 1 || grants != 0 {
		t.Fatal("source transaction did not roll back")
	}
	command.Audit.ID = "audit_source_after_rollback"
	if _, err := store.CommitSourceUpdate(context.Background(), command); err != nil {
		t.Fatal("rolled back proof could not be retried")
	}
}

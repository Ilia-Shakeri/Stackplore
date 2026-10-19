package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/appsettings"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func applicationTestPreview(actor auth.Principal, kind appsettings.PreviewKind, at time.Time) appsettings.PreviewCommand {
	preview := appsettings.PreviewCommand{Actor: actor, Kind: kind, ExpectedVersion: 1,
		Digest: sha256.Sum256([]byte("owned-application-confirmation")), CreatedAt: at, ExpiresAt: at.Add(appsettings.ConfirmationLifetime)}
	if kind == appsettings.PreviewSettings {
		preview.Values = domain.DefaultApplicationValues()
		preview.Values.MetricRetentionHours = 1
	} else {
		preview.Classes = []domain.HistoryClass{domain.HistoryEvents}
	}
	return preview
}

func applicationTestAudit(actor auth.Principal, action domain.AuditAction, suffix string, at time.Time) domain.AuditEntry {
	return domain.AuditEntry{ID: domain.AuditEntryID("audit_application_" + suffix), OccurredAt: at,
		Actor: domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor.UserID)}, Action: action,
		Target: domain.AuditTarget{Kind: "application.settings", ID: "application"}, Outcome: domain.AuditSuccess,
		RequestID: domain.RequestID("request_application_" + suffix), ReasonCode: "settings.updated"}
}

func applicationTestCommit(t *testing.T, store *Store, preview appsettings.PreviewCommand, suffix string) appsettings.CommitCommand {
	t.Helper()
	purpose, action := auth.RecentAuthPurposeSettingsUpdate, domain.AuditAction("settings.update")
	if preview.Kind == appsettings.PreviewCleanup {
		purpose, action = auth.RecentAuthPurposeDataCleanup, "data.cleanup"
	}
	grant := issueAuthRepositoryGrant(t, store, preview.Actor, suffix, purpose, preview.CreatedAt, preview.CreatedAt.Add(time.Microsecond))
	return appsettings.CommitCommand{Actor: preview.Actor, ExpectedVersion: preview.ExpectedVersion, Values: preview.Values,
		ConfirmationDigest: preview.Digest, BackupAcknowledged: true, Grant: grant,
		Audit: applicationTestAudit(preview.Actor, action, suffix, grant.ConsumedAt)}
}

func applicationTestState(t *testing.T, store *Store, action domain.AuditAction) (int64, int, int, int) {
	t.Helper()
	var version int64
	var previews, grants, audits int
	if err := store.pool.QueryRow(context.Background(), `SELECT
(SELECT row_version FROM application_settings WHERE singleton=1),
(SELECT count(*) FROM application_change_confirmations),
(SELECT count(*) FROM recent_auth_grants WHERE consumed_at_us IS NOT NULL),
(SELECT count(*) FROM audit_entries WHERE action=$1)`, string(action)).Scan(&version, &previews, &grants, &audits); err != nil {
		t.Fatal("inspect application transaction state")
	}
	return version, previews, grants, audits
}

func TestApplicationSettingsGuardedCommitIsAtomicAndOneUse(t *testing.T) {
	store, actor, _ := sourceSettingsFixture(t, -time.Minute)
	preview := applicationTestPreview(actor, appsettings.PreviewSettings, time.Now().UTC().Truncate(time.Microsecond))
	preview.Values.HealthRetentionHours = 12
	preview.Values.EventRetentionHours = 24
	preview.Values.DeliveryRetentionHours = 48
	preview.Values.LogLineLimit = 100
	preview.Values.DefaultAlertWindowSeconds = 120
	preview.Values.DefaultAlertPendingSeconds = 30
	stored, err := store.CreateApplicationPreview(context.Background(), preview)
	if err != nil || !stored.Impact.BackupRequired || !stored.Impact.ExpiryPolicyChanged || len(stored.Impact.Classes) != 4 {
		t.Fatal("settings decrease preview lacks guarded impact")
	}
	command := applicationTestCommit(t, store, preview, "settings_race")
	results := make(chan error, 2)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := store.CommitApplicationSettings(context.Background(), command)
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
		} else if errors.Is(err, appsettings.ErrVersion) {
			conflicts++
		} else {
			t.Fatal("unexpected settings replay outcome")
		}
	}
	if version, previews, grants, audits := applicationTestState(t, store, "settings.update"); wins != 1 || conflicts != 1 || version != 2 || previews != 0 || grants != 1 || audits != 1 {
		t.Fatal("settings commit failed version, confirmation, grant or audit atomicity")
	}
	current, err := store.ReadApplicationSettings(context.Background())
	if err != nil || current.Values != preview.Values {
		t.Fatal("committed settings differ from preview")
	}
	preview.ExpectedVersion = 2
	preview.Digest = sha256.Sum256([]byte("owned-second-confirmation"))
	if _, err := store.CreateApplicationPreview(context.Background(), preview); err != nil {
		t.Fatal("create second settings preview")
	}
	command.ExpectedVersion, command.ConfirmationDigest = 2, preview.Digest
	command.Audit.ID = "audit_application_reused_grant"
	if _, err := store.CommitApplicationSettings(context.Background(), command); !errors.Is(err, auth.ErrRecentAuthRejected) {
		t.Fatal("spent recent-auth grant accepted for a new settings preview")
	}
}

func TestApplicationSettingsRejectedCommitsPreserveAuthorizationAndValues(t *testing.T) {
	for _, check := range []string{"version", "material", "confirmation", "backup", "expired", "purpose", "audit"} {
		t.Run(check, func(t *testing.T) {
			store, actor, _ := sourceSettingsFixture(t, -4*time.Minute)
			at := time.Now().UTC().Truncate(time.Microsecond)
			if check == "expired" {
				at = at.Add(-3 * time.Minute)
			}
			preview := applicationTestPreview(actor, appsettings.PreviewSettings, at)
			if _, err := store.CreateApplicationPreview(context.Background(), preview); err != nil {
				t.Fatal("create guarded application preview")
			}
			command := applicationTestCommit(t, store, preview, "settings_"+check)
			want := appsettings.ErrConfirmation
			priorAudits := 0
			switch check {
			case "version":
				command.ExpectedVersion++
				want = appsettings.ErrVersion
			case "material":
				command.Values.LogLineLimit = 100
			case "confirmation":
				command.ConfirmationDigest = sha256.Sum256([]byte("different-confirmation"))
			case "backup":
				command.BackupAcknowledged = false
				want = appsettings.ErrBackup
			case "purpose":
				command.Grant.Purpose = auth.RecentAuthPurposeConfigurationExport
				want = appsettings.ErrInvalid
			case "audit":
				if err := store.withinTransaction(context.Background(), func(ctx context.Context, tx *transaction) error {
					return insertAuthAuditTx(ctx, tx.transaction, command.Audit)
				}); err != nil {
					t.Fatal("seed conflicting owned audit")
				}
				priorAudits, want = 1, appsettings.ErrUnavailable
			}
			if _, err := store.CommitApplicationSettings(context.Background(), command); !errors.Is(err, want) {
				t.Fatalf("unsafe settings commit returned unexpected error for %s", check)
			}
			if version, previews, grants, audits := applicationTestState(t, store, "settings.update"); version != 1 || previews != 1 || grants != 0 || audits != priorAudits {
				t.Fatal("rejected settings commit changed values or spent authorization")
			}
			current, err := store.ReadApplicationSettings(context.Background())
			if err != nil || current.Values != domain.DefaultApplicationValues() {
				t.Fatal("rejected settings values persisted")
			}
		})
	}
}

func TestApplicationCleanupPreviewAndCommitAreBoundedAtomicAndOneUse(t *testing.T) {
	store, actor, _ := sourceSettingsFixture(t, -time.Minute)
	at := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := store.pool.Exec(context.Background(), `INSERT INTO infrastructure_events
(id,origin_kind,kind,severity,reason_code,summary,occurred_at_us,observed_at_us,synthetic)
SELECT 'event_cleanup_'||n,'system','system.test','info','system.test','Owned cleanup fixture',$1,$1,0 FROM generate_series(1,1002) AS n`, at.Add(-31*24*time.Hour).UnixMicro()); err != nil {
		t.Fatal("seed bounded cleanup history")
	}
	preview := applicationTestPreview(actor, appsettings.PreviewCleanup, at)
	stored, err := store.CreateApplicationPreview(context.Background(), preview)
	if err != nil || !stored.Impact.BackupRequired || len(stored.Impact.Classes) != 1 || stored.Impact.Classes[0].EligibleRows != 1002 || stored.Impact.MaximumRows != 1000 {
		t.Fatal("cleanup preview impact differs from eligible history")
	}
	command := applicationTestCommit(t, store, preview, "cleanup")
	command.BackupAcknowledged = false
	if _, err := store.CommitHistoryCleanup(context.Background(), command); !errors.Is(err, appsettings.ErrBackup) {
		t.Fatal("cleanup accepted missing backup acknowledgement")
	}
	if version, previews, grants, audits := applicationTestState(t, store, "data.cleanup"); version != 1 || previews != 1 || grants != 0 || audits != 0 {
		t.Fatal("rejected cleanup spent authorization")
	}
	// A failed audit must roll back the deletion as well as both one-use proofs.
	if err := store.withinTransaction(context.Background(), func(ctx context.Context, tx *transaction) error {
		return insertAuthAuditTx(ctx, tx.transaction, command.Audit)
	}); err != nil {
		t.Fatal("seed conflicting cleanup audit")
	}
	command.BackupAcknowledged = true
	if _, err := store.CommitHistoryCleanup(context.Background(), command); !errors.Is(err, appsettings.ErrUnavailable) {
		t.Fatal("cleanup hid audit failure")
	}
	var rows int
	if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM infrastructure_events WHERE synthetic=0`).Scan(&rows); err != nil || rows != 1002 {
		t.Fatal("failed cleanup audit did not roll back history")
	}
	if _, previews, grants, _ := applicationTestState(t, store, "data.cleanup"); previews != 1 || grants != 0 {
		t.Fatal("failed cleanup audit spent authorization")
	}
	command.Audit.ID = "audit_application_cleanup_success"
	result, err := store.CommitHistoryCleanup(context.Background(), command)
	if err != nil || result.RemovedRows != 1000 || !result.RemainingRows || result.PolicyVersion != 1 || len(result.Scope) != 1 || result.Scope[0] != domain.HistoryEvents || len(result.Cutoffs) != 1 || result.Cutoffs[0].Cutoff != stored.Impact.Classes[0].Cutoff {
		t.Fatal("cleanup failed its exact preview scope or aggregate bound")
	}
	if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM infrastructure_events WHERE synthetic=0`).Scan(&rows); err != nil || rows != 2 {
		t.Fatal("cleanup removed more than its bounded batch")
	}
	if _, err := store.CommitHistoryCleanup(context.Background(), command); !errors.Is(err, appsettings.ErrConfirmation) {
		t.Fatal("cleanup confirmation replay accepted")
	}
	var removed string
	if err := store.pool.QueryRow(context.Background(), `SELECT value_text FROM audit_attributes WHERE audit_entry_id=$1 AND attribute_key='cleanup.removed_rows'`, string(command.Audit.ID)).Scan(&removed); err != nil || removed != "1000" {
		t.Fatal("cleanup result audit missing exact removal count")
	}
	if version, previews, grants, audits := applicationTestState(t, store, "data.cleanup"); version != 1 || previews != 0 || grants != 1 || audits != 2 {
		t.Fatal("cleanup commit was not atomic")
	}
}

func TestApplicationConfigurationExportMasksAndAuditsWithOneUseGrant(t *testing.T) {
	store, actor, _ := sourceSettingsFixture(t, -time.Minute)
	at := time.Now().UTC().Truncate(time.Microsecond)
	grant := issueAuthRepositoryGrant(t, store, actor, "export", auth.RecentAuthPurposeConfigurationExport, at, at.Add(time.Microsecond))
	command := appsettings.ExportCommand{Actor: actor, Grant: grant, Audit: applicationTestAudit(actor, "configuration.export", "export", grant.ConsumedAt), ApplicationVersion: "owned-test-version",
		NotificationIntegrations: []appsettings.ExportIntegration{{Kind: "webhook", Enabled: true, Target: "[masked]", CredentialReference: "[masked]"}}}
	command.NotificationIntegrations[0].Target = "invalid-unmasked-fixture"
	if _, err := store.CommitConfigurationExport(context.Background(), command); !errors.Is(err, appsettings.ErrInvalid) {
		t.Fatal("unmasked export target accepted")
	}
	command.NotificationIntegrations[0].Target = "[masked]"
	if err := store.withinTransaction(context.Background(), func(ctx context.Context, tx *transaction) error {
		return insertAuthAuditTx(ctx, tx.transaction, command.Audit)
	}); err != nil {
		t.Fatal("seed conflicting export audit")
	}
	if _, err := store.CommitConfigurationExport(context.Background(), command); !errors.Is(err, appsettings.ErrUnavailable) {
		t.Fatal("export hid audit failure")
	}
	if _, _, grants, audits := applicationTestState(t, store, "configuration.export"); grants != 0 || audits != 1 {
		t.Fatal("failed export spent grant")
	}
	command.Audit.ID = "audit_application_export_success"
	exported, err := store.CommitConfigurationExport(context.Background(), command)
	if err != nil || exported.ApplicationVersion != command.ApplicationVersion || exported.Settings != domain.DefaultApplicationValues() || len(exported.Sources) != 2 || len(exported.NotificationIntegrations) != 1 {
		t.Fatal("configuration export lacks effective settings or sources")
	}
	for _, source := range exported.Sources {
		if source.Host != "[masked]" || source.CredentialReference != "[masked]" {
			t.Fatal("configuration export exposed source material")
		}
	}
	if _, err := store.CommitConfigurationExport(context.Background(), command); !errors.Is(err, auth.ErrRecentAuthRejected) {
		t.Fatal("configuration export grant replay accepted")
	}
	if version, previews, grants, audits := applicationTestState(t, store, "configuration.export"); version != 1 || previews != 0 || grants != 1 || audits != 2 {
		t.Fatal("configuration export changed settings or failed atomic audit")
	}
}

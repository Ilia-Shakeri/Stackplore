package postgres

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/testpostgres"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
	"github.com/jackc/pgx/v5"
)

func TestReleaseUpgradeFromShippedPrefixesPreservesDataAndImmutableLedger(t *testing.T) {
	for _, previousVersion := range []int{14, 16} {
		t.Run(strconv.Itoa(previousVersion), func(t *testing.T) {
			testReleaseUpgradePreservesDataAndImmutableLedger(t, previousVersion)
		})
	}
}

func testReleaseUpgradePreservesDataAndImmutableLedger(t *testing.T, previousVersion int) {
	connectionString := os.Getenv(integrationDatabaseURLKey)
	if connectionString == "" {
		t.Skip("real PostgreSQL URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := testpostgres.OpenGate(ctx, connectionString)
	if err != nil {
		t.Fatal("open upgrade PostgreSQL gate")
	}
	schema := uniqueTestSchema(t)
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		_ = admin.Close(ctx)
		t.Fatal("create isolated upgrade schema")
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_, _ = admin.Exec(cleanupContext, "DROP SCHEMA "+identifier+" CASCADE")
		_ = admin.Close(cleanupContext)
	})

	all, err := loadMigrations(postgresmigrations.FS)
	if err != nil || len(all) != 17 {
		t.Fatal("upgrade fixture requires the exact 17-migration release manifest")
	}
	previousFiles := fstest.MapFS{}
	for _, migration := range all[:previousVersion] {
		previousFiles[migration.filename] = &fstest.MapFile{Data: []byte(migration.contents)}
	}
	options := validTestOptions()
	options.ConnectionString = connectionString
	options.Schema = schema
	previous, err := Open(ctx, options, previousFiles)
	if err != nil {
		t.Fatal("initialize exact shipped migration prefix")
	}
	t.Cleanup(func() { _ = previous.Close() })
	at := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := previous.pool.Exec(ctx, `INSERT INTO users(id,login_name,canonical_login_name,display_name,role,enabled,auth_revision,created_at_us,updated_at_us,row_version)
VALUES('user_release_upgrade','release-upgrade','release-upgrade','Upgrade administrator','administrator',1,3,$1,$1,4)`, at.UnixMicro()); err != nil {
		t.Fatal("seed previous administrator")
	}
	configuration := domain.SourceConfiguration{Source: domain.InfrastructureSource{ID: "source_release_upgrade", Provider: domain.ProviderKubernetes, Name: "Upgrade cluster"}, ProfileID: domain.SourceProfileKubernetesInCluster, Enabled: false, Version: 1, CreatedAt: at, UpdatedAt: at}
	if err := previous.EnsureSourceConfigurations(ctx, []domain.SourceConfiguration{configuration}); err != nil {
		t.Fatal("seed previous disabled source and protected creation audit")
	}
	if _, err := previous.pool.Exec(ctx, `INSERT INTO source_connection_references(source_id,purpose,reference_kind,reference_id,display_label,created_at_us,updated_at_us)
VALUES($1,'observation','operator_profile','profile_release_upgrade','Upgrade reference',$2,$2)`, string(configuration.Source.ID), at.UnixMicro()); err != nil {
		t.Fatal("seed previous operator reference")
	}
	if _, err := previous.pool.Exec(ctx, `UPDATE application_settings SET row_version=7,updated_at_us=$1,metric_retention_hours=96,health_retention_hours=144,event_retention_hours=72,delivery_retention_hours=48,log_line_limit=80,default_alert_window_seconds=600,default_alert_pending_seconds=60 WHERE singleton=1`, at.UnixMicro()); err != nil {
		t.Fatal("seed previous application policy")
	}
	entry := domain.AuditEntry{ID: "audit_release_upgrade", OccurredAt: at, Actor: domain.AuditActor{Kind: domain.AuditActorUser, ID: "user_release_upgrade"}, Action: "source.update", Target: domain.AuditTarget{Kind: "source.configuration", ID: string(configuration.Source.ID)}, Outcome: domain.AuditSuccess, RequestID: "request_release_upgrade", ReasonCode: "source.disabled"}
	if err := previous.AppendAuditEntries(ctx, []domain.AuditEntry{entry}); err != nil {
		t.Fatal("seed previous protected user evidence")
	}
	readSnapshot := func(store *Store, table string) string {
		t.Helper()
		var snapshot string
		if err := store.pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(item) ORDER BY to_jsonb(item)::text),'[]'::jsonb)::text FROM `+table+` AS item`).Scan(&snapshot); err != nil {
			t.Fatalf("read upgrade snapshot for %s", table)
		}
		return snapshot
	}
	protected := make(map[string]string)
	for _, table := range []string{"users", "infrastructure_sources", "source_connection_references", "application_settings", "audit_entries", "audit_lifecycle_state", "installation_metadata"} {
		protected[table] = readSnapshot(previous, table)
	}
	ledgerBefore := readSnapshot(previous, "schema_migrations")
	var previousCount int
	if err := previous.pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&previousCount); err != nil || previousCount != previousVersion {
		t.Fatal("fixture did not start at the exact shipped migration prefix")
	}
	if err := previous.Close(); err != nil {
		t.Fatal("close previous process ownership")
	}

	upgraded, err := Open(ctx, options, postgresmigrations.FS)
	if err != nil {
		t.Fatal("forward upgrade from shipped migration prefix to 17 failed")
	}
	t.Cleanup(func() { _ = upgraded.Close() })
	if err := upgraded.Health(ctx); err != nil {
		t.Fatal("upgraded store is not healthy")
	}
	for table, expected := range protected {
		if actual := readSnapshot(upgraded, table); actual != expected {
			t.Fatalf("forward upgrade changed protected %s data", table)
		}
	}
	var prefixAfter string
	if err := upgraded.pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(item) ORDER BY to_jsonb(item)::text),'[]'::jsonb)::text FROM schema_migrations AS item WHERE version<=$1`, previousVersion).Scan(&prefixAfter); err != nil || prefixAfter != ledgerBefore {
		t.Fatal("forward upgrade changed a prior ledger filename, checksum or application timestamp")
	}
	applied, err := readAppliedMigrations(ctx, upgraded.pool, len(all)+1)
	if err != nil || !migrationHistoryMatches(applied, all) {
		t.Fatal("upgraded ledger does not match the complete release manifest")
	}
	var indexValid bool
	if err := upgraded.pool.QueryRow(ctx, `SELECT index.indisvalid AND index.indisready FROM pg_index AS index JOIN pg_class AS definition ON definition.oid=index.indexrelid JOIN pg_namespace AS namespace ON namespace.oid=definition.relnamespace WHERE namespace.nspname=$1 AND definition.relname='alerts_source_order_idx'`, schema).Scan(&indexValid); err != nil || !indexValid {
		t.Fatal("migration 16 did not create the valid alert feed index")
	}
	for _, index := range []string{"notifications_time_idx", "alert_rules_updated_order_idx"} {
		if err := upgraded.pool.QueryRow(ctx, `SELECT index.indisvalid AND index.indisready AND pg_get_indexdef(index.indexrelid) LIKE '%COLLATE "C"%' FROM pg_index AS index JOIN pg_class AS definition ON definition.oid=index.indexrelid JOIN pg_namespace AS namespace ON namespace.oid=definition.relnamespace WHERE namespace.nspname=$1 AND definition.relname=$2`, schema, index).Scan(&indexValid); err != nil || !indexValid {
			t.Fatal("migration 17 did not create the valid canonical feed index")
		}
	}
	source := domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: configuration.Source.ID}
	if err := upgraded.ReconcileOperatorProfile(ctx, source, domain.SourceProfileKubernetesKubeconfig); err != nil {
		t.Fatal("migration 15 did not permit the mounted Kubernetes profile")
	}
	actual, err := upgraded.GetSourceConfiguration(ctx, source.SourceID)
	if err != nil || actual.Enabled || actual.Version != 2 || actual.ProfileID != domain.SourceProfileKubernetesKubeconfig {
		t.Fatal("mounted profile reconciliation lost administrator choice after upgrade")
	}
	ledgerAfter := readSnapshot(upgraded, "schema_migrations")
	if err := upgraded.Close(); err != nil {
		t.Fatal("close upgraded process ownership")
	}
	reopened, err := Open(ctx, options, postgresmigrations.FS)
	if err != nil {
		t.Fatal("same-release restart rejected upgraded database")
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if readSnapshot(reopened, "schema_migrations") != ledgerAfter {
		t.Fatal("same-release restart reapplied migrations")
	}
	if err := reopened.Close(); err != nil {
		t.Fatal("close restarted process ownership")
	}
	downgraded, err := Open(ctx, options, previousFiles)
	if downgraded != nil {
		_ = downgraded.Close()
	}
	if !errors.Is(err, ErrMigrationRejected) {
		t.Fatal("previous migration manifest accepted the upgraded ledger")
	}
}

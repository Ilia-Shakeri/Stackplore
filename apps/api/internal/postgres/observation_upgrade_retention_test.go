package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
	"github.com/jackc/pgx/v5"
)

func TestObservationMigrationUpgradesSixVersionSchemaWithoutLoss(t *testing.T) {
	store, options, admin := openIntegrationStore(t)
	if err := store.Close(); err != nil {
		t.Fatal("close upgrade fixture")
	}
	identifier := pgx.Identifier{options.Schema}.Sanitize()
	if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+identifier+" CASCADE; CREATE SCHEMA "+identifier); err != nil {
		t.Fatal("reset isolated upgrade schema")
	}
	files := fstest.MapFS{}
	entries, err := fs.ReadDir(postgresmigrations.FS, ".")
	if err != nil {
		t.Fatal("read upgrade migration files")
	}
	for _, entry := range entries {
		version, valid := migrationVersion(entry.Name())
		if !valid || version > 6 {
			continue
		}
		data, err := fs.ReadFile(postgresmigrations.FS, entry.Name())
		if err != nil {
			t.Fatal("read old migration file")
		}
		files[entry.Name()] = &fstest.MapFile{Data: data}
	}
	legacy, err := Open(context.Background(), options, files)
	if err != nil {
		t.Fatalf("open six-version schema: %v", err)
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := legacy.pool.Exec(context.Background(), `INSERT INTO infrastructure_sources
(id,provider,display_name,synthetic,enabled,connection_kind,created_at_us,updated_at_us)
VALUES('source_legacy','demo','Keep legacy label',1,1,'fixture',$1,$1)`, at.UnixMicro()); err != nil {
		t.Fatal("seed legacy source")
	}
	if err := legacy.Close(); err != nil {
		t.Fatal("close six-version schema")
	}
	upgraded, err := Open(context.Background(), options, postgresmigrations.FS)
	if err != nil {
		t.Fatalf("upgrade six-version schema: %v", err)
	}
	defer upgraded.Close()
	configuration, err := upgraded.GetSourceConfiguration(context.Background(), "source_legacy")
	if err != nil || configuration.Source.Name != "Keep legacy label" || configuration.ProfileID != domain.SourceProfileDemoFixture || configuration.Version != 1 {
		t.Fatalf("legacy source did not survive: %v", err)
	}
	configuration.Source.Name = "Deployment default changed"
	if err := upgraded.EnsureSourceConfigurations(context.Background(), []domain.SourceConfiguration{configuration}); err != nil {
		t.Fatal("ensure upgraded configuration")
	}
	stored, err := upgraded.GetSourceConfiguration(context.Background(), "source_legacy")
	if err != nil || stored.Source.Name != "Keep legacy label" {
		t.Fatal("startup defaults replaced durable source label")
	}
}

func TestObservationRetentionPreservesSyntheticAndProtectedAudit(t *testing.T) {
	fixture := newObservationFixture(t)
	ctx := fixture.ctx
	at := fixture.at
	for index := range 5 {
		if _, err := fixture.store.pool.Exec(ctx, `INSERT INTO infrastructure_events
(id,origin_kind,provider,source_id,environment_id,resource_id,kind,severity,reason_code,summary,occurred_at_us,observed_at_us,synthetic)
VALUES($1,'system',NULL,NULL,NULL,NULL,'system.test','info','system.test','System fixture',$2,$2,0)`, fmt.Sprintf("event_retention_real_%d", index), at.Add(-time.Hour).UnixMicro()); err != nil {
			t.Fatal("seed non-synthetic history")
		}
	}
	audit := domain.AuditEntry{ID: "audit_observation_retention", OccurredAt: at.Add(-24 * time.Hour), Actor: domain.AuditActor{Kind: domain.AuditActorSystem}, Action: "collector.persist", Target: domain.AuditTarget{Kind: "source.observation", ID: string(fixture.source.Source.ID)}, Outcome: domain.AuditSuccess, RequestID: "request_observation_retention", ReasonCode: "collector.persisted"}
	if err := fixture.store.AppendAuditEntries(ctx, []domain.AuditEntry{audit}); err != nil {
		t.Fatal("seed protected observation audit")
	}
	seedObservationTerminalDelivery(t, fixture, "delivery_protected", string(audit.ID))
	removed, err := fixture.store.PruneObservationHistory(ctx, at.Add(31*24*time.Hour), 8)
	if err != nil || removed != 2 {
		t.Fatalf("aggregate retention bound: %v", err)
	}
	var syntheticEvents, syntheticMetrics, syntheticHealth, realEvents, audits, protectedDeliveries int
	if err := fixture.store.pool.QueryRow(ctx, `SELECT
(SELECT count(*) FROM infrastructure_events WHERE synthetic=1),
(SELECT count(*) FROM metric_samples WHERE synthetic=1),
(SELECT count(*) FROM health_history WHERE synthetic=1),
(SELECT count(*) FROM infrastructure_events WHERE synthetic=0),
(SELECT count(*) FROM audit_entries WHERE id=$1),
(SELECT count(*) FROM notification_deliveries WHERE audit_intent_id=$1)`, string(audit.ID)).Scan(&syntheticEvents, &syntheticMetrics, &syntheticHealth, &realEvents, &audits, &protectedDeliveries); err != nil {
		t.Fatal("inspect retained observations")
	}
	if syntheticEvents != 3 || syntheticMetrics != 3 || syntheticHealth != 3 || realEvents != 3 || audits != 1 || protectedDeliveries != 1 {
		t.Fatal("retention removed synthetic fixtures or protected audit")
	}
}

func TestObservationRetentionFairQuotaKeepsNewestMetricAndCurrentHealth(t *testing.T) {
	fixture := newObservationFixture(t)
	configuration := domain.SourceConfiguration{Source: domain.InfrastructureSource{ID: "source_retention_native", Provider: domain.ProviderDocker, Name: "Native retention fixture"},
		ProfileID: domain.SourceProfileDockerSocketDevelopment, Enabled: true, Version: 1, CreatedAt: fixture.at.Add(-24 * time.Hour), UpdatedAt: fixture.at.Add(-24 * time.Hour)}
	if err := fixture.store.EnsureSourceConfigurations(fixture.ctx, []domain.SourceConfiguration{configuration}); err != nil {
		t.Fatal("seed native retention source")
	}
	token, err := fixture.store.BeginCapture(fixture.ctx, configuration.Source.ID, 1, fixture.at)
	if err != nil {
		t.Fatal("begin native retention capture")
	}
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: fixture.at.Add(-time.Hour)}
	environment := domain.Environment{ID: "environment_retention_native", SourceID: configuration.Source.ID, Provider: domain.ProviderDocker, Name: "Native fixture", Status: status}
	identity, err := domain.NewResourceIdentity(domain.ProviderDocker, configuration.Source.ID, environment.ID, "docker.container", domain.Scope{Kind: domain.ScopeGlobal}, "retention_native_container")
	if err != nil {
		t.Fatal("make native retention identity")
	}
	ref, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal("make native retention reference")
	}
	batch := domain.CaptureBatch{Token: token, Environments: []domain.Environment{environment},
		Resources: []domain.ResourceSnapshot{{Resource: domain.Resource{Ref: ref, DisplayName: "Native fixture"}, Status: status}},
		Health: []domain.HealthSnapshot{{ResourceID: ref.ID, State: domain.HealthHealthy, Status: status,
			Evidence: []domain.HealthEvidence{{ID: "evidence_retention_ready", Code: "health.ready", SourceCode: "docker.state", Status: domain.EvidencePositive, ObservedAt: status.ObservedAt}}}}}
	for index := range 10 {
		batch.Metrics = append(batch.Metrics, domain.MetricSample{ID: domain.MetricSampleID(fmt.Sprintf("metric_retention_native_%d", index)), ResourceID: ref.ID,
			Name: domain.MetricCPUUsageCores, Unit: domain.MetricUnitCores, Value: float64(index) / 10, SampledAt: status.ObservedAt.Add(time.Duration(index) * time.Second), Interval: time.Second})
		batch.Events = append(batch.Events, domain.InfrastructureEvent{ID: domain.EventID(fmt.Sprintf("event_retention_native_%d", index)), Provider: domain.ProviderDocker,
			SourceID: configuration.Source.ID, Kind: "docker.lifecycle", Severity: domain.SeverityInfo, ReasonCode: "docker.lifecycle", Summary: "Native fixture event", OccurredAt: status.ObservedAt, ObservedAt: status.ObservedAt})
		seedObservationTerminalDelivery(t, fixture, fmt.Sprintf("delivery_retention_native_%d", index), "")
	}
	batch.Metrics = append(batch.Metrics, domain.MetricSample{ID: "metric_retention_memory", ResourceID: ref.ID, Name: domain.MetricMemoryUsageBytes,
		Unit: domain.MetricUnitBytes, Value: 100, SampledAt: status.ObservedAt, Interval: time.Second})
	if err := fixture.store.SaveCapture(fixture.ctx, batch); err != nil {
		t.Fatalf("save native retention capture: %v", err)
	}
	// Add history after its resource exists. Current health remains a separate row.
	if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO health_history
(id,resource_id,health_state,availability,completeness,observed_at_us,synthetic)
SELECT 'health_retention_native_'||item,$1,'healthy','available','complete',$2,0 FROM generate_series(1,9) AS item`, string(ref.ID), status.ObservedAt.UnixMicro()); err != nil {
		t.Fatal("seed native health change history")
	}
	capabilities := domain.CapabilitySet{Source: token.Source}
	for _, name := range domain.CapabilityNames() {
		portStatus := status
		if name == domain.CapabilityMetricCurrent || name == domain.CapabilityMetricHistory {
			portStatus.ObservedAt = fixture.at.Add(-time.Minute)
		}
		capabilities.Capabilities = append(capabilities.Capabilities, domain.CapabilityStatus{Name: name, Status: portStatus})
	}
	if err := fixture.store.FinishCapture(fixture.ctx, domain.CaptureCompletion{Token: token, Status: status, Capabilities: capabilities, Complete: true}); err != nil {
		t.Fatal("finish native retention capture")
	}
	pruneAt := fixture.at.Add(31 * 24 * time.Hour)
	if removed, err := fixture.store.PruneObservationHistory(fixture.ctx, pruneAt, 8); err != nil || removed != 8 {
		t.Fatalf("fair aggregate retention: removed=%d err=%v", removed, err)
	}
	var metrics, health, events, deliveries int
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT
(SELECT count(*) FROM metric_samples WHERE synthetic=0),
(SELECT count(*) FROM health_history WHERE synthetic=0),
(SELECT count(*) FROM infrastructure_events WHERE synthetic=0),
(SELECT count(*) FROM notification_deliveries)`).Scan(&metrics, &health, &events, &deliveries); err != nil {
		t.Fatal("inspect fair retention quotas")
	}
	if metrics != 9 || health != 8 || events != 8 || deliveries != 8 {
		t.Fatal("metric backlog starved another retention class")
	}
	for range 20 {
		removed, err := fixture.store.PruneObservationHistory(fixture.ctx, pruneAt, 8)
		if err != nil {
			t.Fatal("drain native observation history")
		}
		if removed == 0 {
			break
		}
	}
	current, err := fixture.store.GetCurrentMetrics(fixture.ctx, domain.CurrentMetricQuery{Source: token.Source, Resources: []domain.ResourceRef{ref}, Names: []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes}})
	if err != nil || len(current.Sets) != 1 || len(current.Sets[0].Samples) != 2 {
		t.Fatalf("retained current metric read: err=%v sets=%d", err, len(current.Sets))
	}
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT
(SELECT count(*) FROM metric_samples WHERE synthetic=0),
(SELECT count(*) FROM resource_health_current WHERE resource_id=$1)`, string(ref.ID)).Scan(&metrics, &health); err != nil || metrics != 2 || health != 1 {
		t.Fatal("retention failed to keep newest point per metric and current health")
	}
}

func seedObservationTerminalDelivery(t *testing.T, fixture observationFixture, id, auditIntent string) {
	t.Helper()
	if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO notifications
(id,notification_kind,severity,summary_code,created_at_us,synthetic) VALUES($1,'system.test','info','system.test',$2,0)`, "notification_"+id, fixture.at.Add(-time.Hour).UnixMicro()); err != nil {
		t.Fatal("seed retention notification")
	}
	purpose := "alert"
	if auditIntent != "" {
		purpose = "test"
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO notification_deliveries
(id,notification_id,adapter_kind,delivery_purpose,canonical_payload_bytes,payload_sha256,state,attempt_count,max_attempts,created_at_us,updated_at_us,terminal_at_us,audit_intent_id)
VALUES($1,$2,'in_app',$3,'{}'::bytea,decode(repeat('00',32),'hex'),'succeeded',1,1,$4,$4,$4,$5)`, id, "notification_"+id, purpose, fixture.at.Add(-time.Hour).UnixMicro(), optionalString(auditIntent)); err != nil {
		t.Fatal("seed retention terminal delivery")
	}
}

func TestObservationSourceRegistryKeysetAndEventContinuation(t *testing.T) {
	fixture := newObservationFixture(t)
	configurations := []domain.SourceConfiguration{}
	for _, id := range []domain.SourceID{"source_alpha", "source_zeta"} {
		configuration := fixture.source
		configuration.Source.ID = id
		configurations = append(configurations, configuration)
	}
	if err := fixture.store.EnsureSourceConfigurations(fixture.ctx, configurations); err != nil {
		t.Fatal("seed source registry")
	}
	request := domain.PageRequest{Limit: 1}
	count := 0
	for range 4 {
		page, err := fixture.store.ListSourceConfigurations(fixture.ctx, request)
		if err != nil {
			t.Fatal("read source registry page")
		}
		count += len(page.Configurations)
		if !page.Page.HasMore {
			break
		}
		request.Cursor = page.Page.NextCursor
	}
	if count != 3 {
		t.Fatal("source registry skipped or repeated rows")
	}
	query := domain.EventQuery{Source: fixture.token.Source, Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: fixture.at}, Page: domain.PageRequest{Limit: 1}}
	seen := map[domain.EventID]bool{}
	for range 4 {
		page, err := fixture.store.ListEvents(fixture.ctx, query)
		if err != nil {
			t.Fatalf("read event keyset page: %v", err)
		}
		for _, event := range page.Events {
			if seen[event.ID] {
				t.Fatal("event keyset repeated a row")
			}
			seen[event.ID] = true
		}
		if !page.Page.HasMore {
			break
		}
		query.Page.Cursor = page.Page.NextCursor
	}
	if len(seen) != 3 {
		t.Fatal("event keyset skipped rows")
	}
}

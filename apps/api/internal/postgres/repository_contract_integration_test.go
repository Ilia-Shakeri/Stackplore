package postgres

import (
	"context"
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v5"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestRepositoryContractPersistsExactRows(t *testing.T) {

	fixture := newRepositoryFixture(t)
	repositories := domain.ProductRepositories(fixture.store)

	metric := fixture.metric("metric_exact")
	if err := repositories.InsertMetricSamples(fixture.ctx, []domain.MetricSample{metric}); err != nil {
		t.Fatalf("InsertMetricSamples() error = %v", err)
	}
	requireStoredMetric(t, fixture, metric)

	event := fixture.event("event_exact")
	if err := repositories.InsertInfrastructureEvents(fixture.ctx, []domain.InfrastructureEvent{event}); err != nil {
		t.Fatalf("InsertInfrastructureEvents() error = %v", err)
	}
	requireStoredEvent(t, fixture, event)

	entries := []domain.AuditEntry{
		fixture.audit("audit_operator", domain.AuditActor{Kind: domain.AuditActorOperator, ID: "operator_one"}),
		fixture.audit("audit_system", domain.AuditActor{Kind: domain.AuditActorSystem}),
		fixture.audit("audit_user", domain.AuditActor{Kind: domain.AuditActorUser, ID: "user_one"}),
	}
	if err := repositories.AppendAuditEntries(fixture.ctx, entries); err != nil {
		t.Fatalf("AppendAuditEntries() error = %v", err)
	}
	for _, entry := range entries {
		requireStoredAuditEntry(t, fixture, entry)
	}
}

func TestRepositoryBatchesAreAtomicAndMapConflicts(t *testing.T) {

	fixture := newRepositoryFixture(t)
	existing := fixture.metric("metric_existing")
	if err := fixture.store.InsertMetricSamples(fixture.ctx, []domain.MetricSample{existing}); err != nil {
		t.Fatalf("seed metric error = %v", err)
	}

	newSample := fixture.metric("metric_new")
	err := fixture.store.InsertMetricSamples(fixture.ctx, []domain.MetricSample{newSample, existing})
	if !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatalf("duplicate batch error = %v, want conflict", err)
	}
	if got := repositoryRowCount(t, fixture, "metric_samples", newSample.ID); got != 0 {
		t.Fatalf("rolled-back metric count = %d, want 0", got)
	}
	if strings.Contains(strings.ToLower(err.Error()), "constraint") {
		t.Fatalf("conflict exposed driver detail: %q", err)
	}
}

func TestRepositoryMetricWriteBindsSyntheticStateToStoredResource(t *testing.T) {

	fixture := newRepositoryFixture(t)

	mismatched := fixture.metric("metric_mismatch")
	mismatched.Synthetic = false
	err := fixture.store.InsertMetricSamples(fixture.ctx, []domain.MetricSample{mismatched})
	if !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatalf("synthetic mismatch error = %v, want conflict", err)
	}
	if got := repositoryRowCount(t, fixture, "metric_samples", mismatched.ID); got != 0 {
		t.Fatalf("mismatched metric count = %d, want 0", got)
	}

	missing := fixture.metric("metric_missing_resource")
	missing.ResourceID = repositoryResourceID(t, "native-missing")
	err = fixture.store.InsertMetricSamples(fixture.ctx, []domain.MetricSample{missing})
	if !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatalf("missing resource error = %v, want conflict", err)
	}
}

func TestRepositoryEventRejectsResourceFromAnotherEnvironment(t *testing.T) {

	fixture := newRepositoryFixture(t)
	secondEnvironment := domain.EnvironmentID("environment_two")
	now := fixture.now.UnixMicro()
	if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO environments (
id, source_id, provider, display_name, availability, completeness, observed_at_us,
reason_code, synthetic, first_seen_at_us, last_seen_at_us, retired_at_us, row_version
) VALUES ($1, $2, 'demo', 'Second', 'available', 'complete', $3, NULL, 1, $4, $5, NULL, 1)`,
		secondEnvironment,
		fixture.sourceID,
		now,
		now,
		now,
	); err != nil {
		t.Fatal(err)
	}
	event := fixture.event("event_wrong_environment")
	event.EnvironmentID = &secondEnvironment
	err := fixture.store.InsertInfrastructureEvents(fixture.ctx, []domain.InfrastructureEvent{event})
	if !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatalf("environment-resource mismatch error = %v, want conflict", err)
	}
	if got := repositoryRowCount(t, fixture, "infrastructure_events", event.ID); got != 0 {
		t.Fatalf("mismatched event count = %d, want 0", got)
	}
}

func TestRepositoryUnitOfWorkCommitsAcrossPorts(t *testing.T) {

	fixture := newRepositoryFixture(t)
	metric := fixture.metric("metric_commit")
	event := fixture.event("event_commit")
	audit := fixture.audit("audit_commit", domain.AuditActor{Kind: domain.AuditActorSystem})

	err := fixture.store.WithinProductUnitOfWork(fixture.ctx, func(repositories domain.ProductRepositories) error {
		if err := repositories.InsertMetricSamples(fixture.ctx, []domain.MetricSample{metric}); err != nil {
			return err
		}
		if err := repositories.InsertInfrastructureEvents(fixture.ctx, []domain.InfrastructureEvent{event}); err != nil {
			return err
		}
		return repositories.AppendAuditEntries(fixture.ctx, []domain.AuditEntry{audit})
	})
	if err != nil {
		t.Fatalf("WithinProductUnitOfWork() error = %v", err)
	}
	requireStoredMetric(t, fixture, metric)
	requireStoredEvent(t, fixture, event)
	requireStoredAuditEntry(t, fixture, audit)
}

func TestRepositoryUnitOfWorkRollsBackAcrossPorts(t *testing.T) {

	fixture := newRepositoryFixture(t)
	metric := fixture.metric("metric_rollback")
	event := fixture.event("event_rollback")
	audit := fixture.audit("audit_rollback", domain.AuditActor{Kind: domain.AuditActorSystem})
	workRejected := errors.New("work rejected")

	err := fixture.store.WithinProductUnitOfWork(fixture.ctx, func(repositories domain.ProductRepositories) error {
		if err := repositories.InsertMetricSamples(fixture.ctx, []domain.MetricSample{metric}); err != nil {
			return err
		}
		if err := repositories.InsertInfrastructureEvents(fixture.ctx, []domain.InfrastructureEvent{event}); err != nil {
			return err
		}
		if err := repositories.AppendAuditEntries(fixture.ctx, []domain.AuditEntry{audit}); err != nil {
			return err
		}
		return workRejected
	})
	if !errors.Is(err, workRejected) {
		t.Fatalf("WithinProductUnitOfWork() error = %v, want callback error", err)
	}
	if got := repositoryRowCount(t, fixture, "metric_samples", metric.ID); got != 0 {
		t.Fatalf("metric count after rollback = %d, want 0", got)
	}
	if got := repositoryRowCount(t, fixture, "infrastructure_events", event.ID); got != 0 {
		t.Fatalf("event count after rollback = %d, want 0", got)
	}
	if got := repositoryRowCount(t, fixture, "audit_entries", audit.ID); got != 0 {
		t.Fatalf("audit count after rollback = %d, want 0", got)
	}
}

func TestRepositoryUnitOfWorkEnforcesAggregateWriteLimit(t *testing.T) {

	fixture := newRepositoryFixture(t)
	first := make([]domain.MetricSample, 60)
	second := make([]domain.MetricSample, 41)
	for index := range first {
		first[index] = fixture.metric(domain.MetricSampleID("metric_first_" + strconv.Itoa(index)))
	}
	for index := range second {
		second[index] = fixture.metric(domain.MetricSampleID("metric_second_" + strconv.Itoa(index)))
	}

	err := fixture.store.WithinProductUnitOfWork(fixture.ctx, func(repositories domain.ProductRepositories) error {
		if err := repositories.InsertMetricSamples(fixture.ctx, first); err != nil {
			return err
		}
		return repositories.InsertMetricSamples(fixture.ctx, second)
	})
	if !errors.Is(err, domain.ErrRepositoryInvalidInput) {
		t.Fatalf("aggregate write error = %v, want invalid input", err)
	}
	if got := repositoryRowCount(t, fixture, "metric_samples", first[0].ID); got != 0 {
		t.Fatalf("metric count after aggregate rollback = %d, want 0", got)
	}
}

func TestRepositoryUnitOfWorkRollsBackIgnoredRepositoryError(t *testing.T) {

	fixture := newRepositoryFixture(t)
	existing := fixture.metric("metric_latched_existing")
	if err := fixture.store.InsertMetricSamples(fixture.ctx, []domain.MetricSample{existing}); err != nil {
		t.Fatalf("seed metric error = %v", err)
	}
	partial := fixture.metric("metric_latched_partial")

	err := fixture.store.WithinProductUnitOfWork(fixture.ctx, func(repositories domain.ProductRepositories) error {
		_ = repositories.InsertMetricSamples(fixture.ctx, []domain.MetricSample{partial, existing})
		return nil
	})
	if !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatalf("ignored repository error = %v, want conflict", err)
	}
	if got := repositoryRowCount(t, fixture, "metric_samples", partial.ID); got != 0 {
		t.Fatalf("partial metric count after rollback = %d, want 0", got)
	}
}

func TestRepositoryErrorsStayOwnedByDomain(t *testing.T) {

	fixture := newRepositoryFixture(t)

	if err := fixture.store.InsertMetricSamples(nil, []domain.MetricSample{fixture.metric("metric_nil_context")}); !errors.Is(err, domain.ErrRepositoryInvalidInput) {
		t.Fatalf("nil context error = %v", err)
	}
	if err := fixture.store.WithinProductUnitOfWork(fixture.ctx, nil); !errors.Is(err, domain.ErrRepositoryInvalidInput) {
		t.Fatalf("nil work error = %v", err)
	}

	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()
	err := fixture.store.InsertMetricSamples(canceledContext, []domain.MetricSample{fixture.metric("metric_canceled")})
	if !errors.Is(err, domain.ErrRepositoryUnavailable) || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled write error = %v", err)
	}

	if err := fixture.store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	err = fixture.store.AppendAuditEntries(fixture.ctx, []domain.AuditEntry{
		fixture.audit("audit_closed", domain.AuditActor{Kind: domain.AuditActorSystem}),
	})
	if !errors.Is(err, domain.ErrRepositoryUnavailable) {
		t.Fatalf("closed store error = %v, want unavailable", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "closed") {
		t.Fatalf("closed store error exposed adapter detail: %q", err)
	}
}

type repositoryFixture struct {
	ctx           context.Context
	store         *Store
	sourceID      domain.SourceID
	environmentID domain.EnvironmentID
	resourceID    domain.ResourceID
	now           time.Time
}

func newRepositoryFixture(t *testing.T) repositoryFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	store, _, _ := openIntegrationStore(t)

	fixture := repositoryFixture{
		ctx:           ctx,
		store:         store,
		sourceID:      "source_one",
		environmentID: "environment_one",
		resourceID:    repositoryResourceID(t, "native-one"),
		now:           time.Date(2026, time.August, 1, 12, 0, 0, 123000000, time.UTC),
	}
	seedRepositoryDependencies(t, fixture)
	return fixture
}

func seedRepositoryDependencies(t *testing.T, fixture repositoryFixture) {
	t.Helper()
	transaction, err := fixture.store.pool.Begin(fixture.ctx)
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	defer func() { _ = transaction.Rollback(fixture.ctx) }()
	now := fixture.now.UnixMicro()
	repositorySeedExec(t, fixture.ctx, transaction, `INSERT INTO users (
id, login_name, canonical_login_name, display_name, role, enabled, auth_revision,
created_at_us, updated_at_us, disabled_at_us, deleted_at_us, row_version
) VALUES ($1, $2, $3, $4, $5, 1, 1, $6, $7, NULL, NULL, 1)`,
		"user_one", "operator", "operator", "Operator", "administrator", now, now)
	repositorySeedExec(t, fixture.ctx, transaction, `INSERT INTO infrastructure_sources (
id, provider, display_name, synthetic, enabled, connection_kind, config_revision,
created_by_user_id, created_at_us, updated_at_us, deleted_at_us, row_version
) VALUES ($1, 'demo', $2, 1, 1, $3, 1, $4, $5, $6, NULL, 1)`,
		fixture.sourceID, "Demo source", "demo.local", "user_one", now, now)
	repositorySeedExec(t, fixture.ctx, transaction, `INSERT INTO environments (
id, source_id, provider, display_name, availability, completeness, observed_at_us,
reason_code, synthetic, first_seen_at_us, last_seen_at_us, retired_at_us, row_version
) VALUES ($1, $2, 'demo', $3, 'available', 'complete', $4, NULL, 1, $5, $6, NULL, 1)`,
		fixture.environmentID, fixture.sourceID, "Primary", now, now, now)
	repositorySeedExec(t, fixture.ctx, transaction, `INSERT INTO resources (
id, source_id, provider, environment_id, kind, scope_kind, scope_name, native_id,
synthetic, first_seen_at_us, last_seen_at_us, retired_at_us, row_version
) VALUES ($1, $2, 'demo', $3, 'demo.service', 'global', '', $4, 1, $5, $6, NULL, 1)`,
		fixture.resourceID, fixture.sourceID, fixture.environmentID, "native-one", now, now)
	if err := transaction.Commit(fixture.ctx); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
}

func repositorySeedExec(t *testing.T, ctx context.Context, transaction pgx.Tx, query string, arguments ...any) {
	t.Helper()
	if _, err := transaction.Exec(ctx, query, arguments...); err != nil {
		t.Fatalf("seed statement error = %v", err)
	}
}

func repositoryResourceID(t *testing.T, nativeID domain.NativeResourceID) domain.ResourceID {
	t.Helper()
	identity, err := domain.NewResourceIdentity(
		domain.ProviderDemo,
		"source_one",
		"environment_one",
		"demo.service",
		domain.Scope{Kind: domain.ScopeGlobal},
		nativeID,
	)
	if err != nil {
		t.Fatalf("NewResourceIdentity() error = %v", err)
	}
	resourceID, err := domain.NewResourceID(identity)
	if err != nil {
		t.Fatalf("NewResourceID() error = %v", err)
	}
	return resourceID
}

func (fixture repositoryFixture) metric(id domain.MetricSampleID) domain.MetricSample {
	return domain.MetricSample{
		ID:         id,
		ResourceID: fixture.resourceID,
		Name:       domain.MetricCPUUsageCores,
		Unit:       domain.MetricUnitCores,
		Value:      0.75,
		SampledAt:  fixture.now,
		Interval:   15 * time.Second,
		Synthetic:  true,
	}
}

func (fixture repositoryFixture) event(id domain.EventID) domain.InfrastructureEvent {
	environmentID := fixture.environmentID
	resourceID := fixture.resourceID
	return domain.InfrastructureEvent{
		ID:            id,
		Provider:      domain.ProviderDemo,
		SourceID:      fixture.sourceID,
		EnvironmentID: &environmentID,
		ResourceID:    &resourceID,
		Kind:          "resource.changed",
		Severity:      domain.SeverityWarning,
		ReasonCode:    "resource.updated",
		Summary:       "Resource changed",
		OccurredAt:    fixture.now.Add(-time.Minute),
		ObservedAt:    fixture.now,
		Synthetic:     true,
	}
}

func (fixture repositoryFixture) audit(id domain.AuditEntryID, actor domain.AuditActor) domain.AuditEntry {
	return domain.AuditEntry{
		ID:         id,
		OccurredAt: fixture.now,
		Actor:      actor,
		Action:     "source.test",
		Target:     domain.AuditTarget{Kind: "source.connection", ID: string(fixture.sourceID)},
		Outcome:    domain.AuditSuccess,
		RequestID:  domain.RequestID("request_" + string(id)),
		ReasonCode: "source.test_succeeded",
	}
}

func requireStoredMetric(t *testing.T, fixture repositoryFixture, want domain.MetricSample) {
	t.Helper()
	var id, resourceID, name, unit string
	var value float64
	var sampledAt, interval int64
	var synthetic int
	err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT
id, resource_id, metric_name, unit, value, sampled_at_us, interval_seconds, synthetic
FROM metric_samples WHERE id = $1`, want.ID).Scan(
		&id, &resourceID, &name, &unit, &value, &sampledAt, &interval, &synthetic,
	)
	if err != nil {
		t.Fatalf("metric query error = %v", err)
	}
	if id != string(want.ID) || resourceID != string(want.ResourceID) || name != string(want.Name) ||
		unit != string(want.Unit) || value != want.Value || sampledAt != want.SampledAt.UnixMicro() ||
		interval != int64(want.Interval/time.Second) || int64(synthetic) != boolInteger(want.Synthetic) {
		t.Fatalf("stored metric differs from input")
	}
}

func requireStoredEvent(t *testing.T, fixture repositoryFixture, want domain.InfrastructureEvent) {
	t.Helper()
	var id, origin, provider, sourceID, kind, severity, reasonCode, summary string
	var environmentID, resourceID sql.NullString
	var occurredAt, observedAt int64
	var synthetic int
	err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT
id, origin_kind, provider, source_id, environment_id, resource_id, kind, severity,
reason_code, summary, occurred_at_us, observed_at_us, synthetic
FROM infrastructure_events WHERE id = $1`, want.ID).Scan(
		&id, &origin, &provider, &sourceID, &environmentID, &resourceID, &kind, &severity,
		&reasonCode, &summary, &occurredAt, &observedAt, &synthetic,
	)
	if err != nil {
		t.Fatalf("event query error = %v", err)
	}
	if id != string(want.ID) || origin != "provider" || provider != string(want.Provider) ||
		sourceID != string(want.SourceID) || !environmentID.Valid || environmentID.String != string(*want.EnvironmentID) ||
		!resourceID.Valid || resourceID.String != string(*want.ResourceID) || kind != string(want.Kind) ||
		severity != string(want.Severity) || reasonCode != string(want.ReasonCode) || summary != want.Summary ||
		occurredAt != want.OccurredAt.UnixMicro() || observedAt != want.ObservedAt.UnixMicro() ||
		int64(synthetic) != boolInteger(want.Synthetic) {
		t.Fatalf("stored event differs from input")
	}
}

func requireStoredAuditEntry(t *testing.T, fixture repositoryFixture, want domain.AuditEntry) {
	t.Helper()
	var id, actorKind, action, targetKind, outcome, requestID, reasonCode string
	var occurredAt int64
	var actorUserID, operatorID, targetID, sourceID sql.NullString
	err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT
id, occurred_at_us, actor_kind, actor_user_id, operator_id, action, target_kind,
target_id, outcome, request_id, reason_code, source_id
FROM audit_entries WHERE id = $1`, want.ID).Scan(
		&id, &occurredAt, &actorKind, &actorUserID, &operatorID, &action, &targetKind,
		&targetID, &outcome, &requestID, &reasonCode, &sourceID,
	)
	if err != nil {
		t.Fatalf("audit query error = %v", err)
	}
	wantActorUserID, wantOperatorID := auditActorColumns(want.Actor)
	if id != string(want.ID) || occurredAt != want.OccurredAt.UnixMicro() || actorKind != string(want.Actor.Kind) ||
		actorUserID != repositoryNullString(wantActorUserID) || operatorID != repositoryNullString(wantOperatorID) ||
		action != string(want.Action) || targetKind != string(want.Target.Kind) ||
		targetID != repositoryNullString(optionalString(want.Target.ID)) || outcome != string(want.Outcome) ||
		requestID != string(want.RequestID) || reasonCode != string(want.ReasonCode) || sourceID.Valid {
		t.Fatalf("stored audit entry differs from input")
	}
}

func repositoryNullString(value any) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: value.(string), Valid: true}
}

func repositoryRowCount(t *testing.T, fixture repositoryFixture, table string, id any) int {
	t.Helper()
	var query string
	switch table {
	case "audit_entries":
		query = "SELECT count(*) FROM audit_entries WHERE id = $1"
	case "infrastructure_events":
		query = "SELECT count(*) FROM infrastructure_events WHERE id = $1"
	case "metric_samples":
		query = "SELECT count(*) FROM metric_samples WHERE id = $1"
	default:
		t.Fatal("unknown repository count table")
	}
	var count int
	if err := fixture.store.pool.QueryRow(fixture.ctx, query, id).Scan(&count); err != nil {
		t.Fatalf("row count error = %v", err)
	}
	return count
}

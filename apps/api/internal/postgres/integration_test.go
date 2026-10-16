package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/notification"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/testpostgres"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
	"github.com/jackc/pgx/v5"
)

const integrationDatabaseURLKey = "STACKPLORE_TEST_POSTGRES_URL"

func TestPostgreSQLMigrationsOnRealDatabase(t *testing.T) {
	databaseURL := os.Getenv(integrationDatabaseURLKey)
	if databaseURL == "" {
		t.Skip("real PostgreSQL URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	admin, err := testpostgres.OpenGate(ctx, databaseURL)
	if err != nil {
		t.Fatal("connect to PostgreSQL test database")
	}
	defer func() { _ = admin.Close(context.Background()) }()

	schema := uniqueTestSchema(t)
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal("create isolated PostgreSQL test schema")
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_, _ = admin.Exec(cleanupContext, "DROP SCHEMA "+identifier+" CASCADE")
	})

	options := validTestOptions()
	options.ConnectionString = databaseURL
	options.Schema = schema
	store, err := Open(ctx, options, postgresmigrations.FS)
	if err != nil {
		t.Fatalf("Open() rejected PostgreSQL schema: %v", err)
	}
	defer store.Close()

	if err := store.Health(ctx); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	contender, contenderErr := Open(ctx, options, postgresmigrations.FS)
	if contender != nil {
		_ = contender.Close()
	}
	if !errors.Is(contenderErr, ErrOwnershipUnavailable) {
		t.Fatalf("concurrent Open() error = %v; want ownership rejection", contenderErr)
	}
	var migrationsApplied int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&migrationsApplied); err != nil || migrationsApplied != len(store.expectedMigrations) {
		t.Fatalf("migration count = %d, error = %v; want %d", migrationsApplied, err, len(store.expectedMigrations))
	}
	var installations int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM installation_metadata`).Scan(&installations); err != nil || installations != 1 {
		t.Fatalf("installation count = %d, error = %v; want 1", installations, err)
	}
	for _, table := range []string{"users", "sessions", "resources", "metric_samples", "infrastructure_events", "alert_rules", "alerts", "alert_changes", "alert_change_retention_positions", "source_collection_outcomes", "workload_replica_history", "demo_alert_seeds", "notification_deliveries", "notification_delivery_cursors", "audit_entries"} {
		var exists bool
		if err := store.pool.QueryRow(ctx, `SELECT EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = $1 AND table_name = $2
)`, schema, table).Scan(&exists); err != nil || !exists {
			t.Fatalf("table %s missing, error = %v", table, err)
		}
	}
	setupRequired, err := store.SetupRequired(ctx)
	if err != nil || !setupRequired {
		t.Fatalf("SetupRequired() = (%t, %v); want true", setupRequired, err)
	}
	keyID, digestKey, err := store.AuthDigestKey()
	if err != nil {
		t.Fatalf("AuthDigestKey() error = %v", err)
	}
	defer clear(digestKey)
	hasher, err := auth.NewPasswordHasher(auth.DefaultPasswordPolicy())
	if err != nil {
		t.Fatalf("NewPasswordHasher() error = %v", err)
	}
	authentication, err := auth.NewService(auth.ServiceOptions{
		Repository: store, PasswordHasher: hasher, DigestKey: digestKey, DigestKeyID: keyID,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	password := []byte("integration recovery password")
	defer clear(password)
	recovered, err := authentication.RecoverAdministrator(ctx, auth.OperatorRecoveryInput{
		OperatorID: "integration_console", LoginName: "operator", DisplayName: "Primary Operator",
		Password: password, RequestID: "request_integration_recovery",
	})
	if err != nil || recovered.Role != auth.RoleAdministrator || !recovered.Enabled {
		t.Fatalf("RecoverAdministrator() = (%#v, %v)", recovered, err)
	}
	setupRequired, err = store.SetupRequired(ctx)
	if err != nil || setupRequired {
		t.Fatalf("SetupRequired() after recovery = (%t, %v); want false", setupRequired, err)
	}
	account, err := store.FindLoginAccount(ctx, "operator")
	if err != nil {
		t.Fatalf("FindLoginAccount() error = %v", err)
	}
	verification, err := hasher.Verify(ctx, password, account.Credential)
	if err != nil || !verification.Match {
		t.Fatalf("Verify(recovered password) = (%#v, %v)", verification, err)
	}
	cookieToken, err := auth.GenerateOpaqueToken()
	if err != nil {
		t.Fatal("generate preauth cookie")
	}
	proofToken, err := auth.GenerateOpaqueToken()
	if err != nil {
		t.Fatal("generate preauth proof")
	}
	cookieDigest, err := auth.DigestPreauthCookie(digestKey, cookieToken)
	if err != nil {
		t.Fatal("digest preauth cookie")
	}
	proofDigest, err := auth.DigestPreauthProof(digestKey, proofToken)
	if err != nil {
		t.Fatal("digest preauth proof")
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	preauth := auth.Preauth{
		ID:           "preauth_integration_1",
		Purpose:      auth.PreauthPurposeLogin,
		CookieDigest: cookieDigest,
		ProofDigest:  proofDigest,
		DigestKeyID:  keyID,
		CreatedAt:    now,
		ExpiresAt:    now.Add(time.Minute),
	}
	if err := store.InsertPreauth(ctx, preauth); err != nil {
		t.Fatalf("InsertPreauth() error = %v", err)
	}
	usedAt := now.Add(time.Second)
	use := auth.PreauthUse{
		Purpose:      preauth.Purpose,
		CookieDigest: preauth.CookieDigest,
		ProofDigest:  preauth.ProofDigest,
		DigestKeyID:  preauth.DigestKeyID,
		ConsumedAt:   usedAt,
	}
	if err := store.ConsumePreauth(ctx, use); err != nil {
		t.Fatalf("ConsumePreauth() error = %v", err)
	}
	if err := store.ConsumePreauth(ctx, use); !errors.Is(err, auth.ErrAuthRepositoryRejected) {
		t.Fatalf("replayed ConsumePreauth() error = %v; want rejection", err)
	}
	role := auth.RoleViewer
	enabled := true
	page, err := store.ListUsers(ctx, auth.UserListQuery{Limit: 10, Search: "nobody", Role: &role, Enabled: &enabled})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("ListUsers() = (%d items, %v); want empty page", len(page.Items), err)
	}
	verifyObservationRepositories(t, ctx, store, now, account.User.ID)

	store.Close()
	second, err := Open(ctx, options, postgresmigrations.FS)
	if err != nil {
		t.Fatalf("second Open() was not idempotent: %v", err)
	}
	second.Close()
}

func verifyObservationRepositories(t *testing.T, ctx context.Context, store *Store, now time.Time, userID auth.UserID) {
	t.Helper()
	identity, err := domain.NewResourceIdentity(
		domain.ProviderDemo,
		"source_integration",
		"env_integration",
		"demo.service",
		domain.Scope{Kind: domain.ScopeProject, Name: "integration"},
		"service-one",
	)
	if err != nil {
		t.Fatal("build integration resource identity")
	}
	resource, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal("build integration resource reference")
	}
	timestamp := now.UnixMicro()
	for _, statement := range []string{
		`INSERT INTO infrastructure_sources
            (id, provider, display_name, synthetic, enabled, connection_kind, config_revision, created_at_us, updated_at_us)
         VALUES ('source_integration', 'demo', 'Integration source', 1, 1, 'fixture', 1, $1, $1)`,
		`INSERT INTO environments
            (id, source_id, provider, display_name, availability, completeness, observed_at_us, synthetic,
             first_seen_at_us, last_seen_at_us)
         VALUES ('env_integration', 'source_integration', 'demo', 'Integration environment', 'available',
                 'complete', $1, 1, $1, $1)`,
	} {
		if _, err := store.pool.Exec(ctx, statement, timestamp); err != nil {
			t.Fatal("seed observation repository parent")
		}
	}
	if _, err := store.pool.Exec(ctx, `INSERT INTO resources
        (id, source_id, provider, environment_id, kind, scope_kind, scope_name, native_id, synthetic,
         first_seen_at_us, last_seen_at_us)
     VALUES ($1, 'source_integration', 'demo', 'env_integration', 'demo.service', 'project',
             'integration', 'service-one', 1, $2, $2)`, resource.ID, timestamp); err != nil {
		t.Fatal("seed observation repository resource")
	}
	if _, err := store.pool.Exec(ctx, `INSERT INTO alert_rules
        (id, source_id, resource_id, display_name, rule_kind, enabled, severity,
         evaluation_interval_seconds, pending_for_seconds, created_by_user_id, updated_by_user_id,
         created_at_us, updated_at_us)
     VALUES ('rule_integration', 'source_integration', $1, 'Integration health', 'resource_unhealthy',
             1, 'error', 60, 0, $2, $2, $3, $3)`, resource.ID, userID, timestamp); err != nil {
		t.Fatal("seed integration alert rule")
	}
	claimedAt := now.Add(time.Minute)
	tasks, err := store.ClaimDueAlertEvaluations(ctx, claimedAt, 4)
	if err != nil || len(tasks) != 1 || tasks[0].Rule.ID != "rule_integration" ||
		tasks[0].Resource == nil || tasks[0].Resource.ID != resource.ID || tasks[0].Current != nil ||
		!tasks[0].NextEvaluationAt.Equal(claimedAt.Add(time.Minute)) {
		t.Fatalf("ClaimDueAlertEvaluations() = (%#v, %v); want one claimed rule", tasks, err)
	}
	tasks, err = store.ClaimDueAlertEvaluations(ctx, claimedAt, 4)
	if err != nil || len(tasks) != 0 {
		t.Fatalf("second ClaimDueAlertEvaluations() = (%#v, %v); want no duplicate claim", tasks, err)
	}
	environmentID := domain.EnvironmentID("env_integration")
	resourceID := resource.ID
	metric := domain.MetricSample{
		ID: "metric_integration", ResourceID: resource.ID, Name: domain.MetricCPUUsageCores,
		Unit: domain.MetricUnitCores, Value: 0.5, SampledAt: now, Interval: 30 * time.Second, Synthetic: true,
	}
	event := domain.InfrastructureEvent{
		ID: "event_integration", Provider: domain.ProviderDemo, SourceID: "source_integration",
		EnvironmentID: &environmentID, ResourceID: &resourceID, Kind: "demo.health.changed",
		Severity: domain.SeverityWarning, ReasonCode: "demo.health.degraded", Summary: "service degraded",
		OccurredAt: now, ObservedAt: now, Synthetic: true,
	}
	alert := domain.Alert{
		ID: "alert_integration", RuleID: "rule_integration", SourceID: "source_integration",
		ResourceID: &resourceID, State: domain.AlertFiring, Severity: domain.SeverityError,
		EvidenceRange: domain.TimeRange{Start: now.Add(-2 * time.Minute), End: now.Add(-time.Minute)},
		StartedAt:     now.Add(-2 * time.Minute), UpdatedAt: now,
		SummaryCode: "alert.resource_unhealthy", Synthetic: true,
	}
	transition := domain.AlertTransition{
		ID: "transition_integration", AlertID: alert.ID, To: domain.AlertFiring, OccurredAt: now,
		Actor: domain.AuditActor{Kind: domain.AuditActorSystem}, ReasonCode: "alert.condition_matched",
		EvidenceRange: alert.EvidenceRange,
	}
	notification := domain.Notification{
		ID: "notification_integration", SourceID: &alert.SourceID, AlertID: &alert.ID,
		ResourceID: alert.ResourceID, Kind: "notification.alert_firing", Severity: alert.Severity,
		SummaryCode: alert.SummaryCode, CreatedAt: now, Synthetic: true,
	}
	mutation := domain.AlertMutation{
		Alert: alert, DedupeKey: "resource-health", Transition: &transition, Notification: &notification,
	}
	audit := domain.AuditEntry{
		ID: "audit_integration", OccurredAt: now, Actor: domain.AuditActor{Kind: domain.AuditActorSystem},
		Action: "collector.persist", Target: domain.AuditTarget{Kind: "source.observation", ID: "source_integration"},
		Outcome: domain.AuditSuccess, RequestID: "request_integration_observation", ReasonCode: "collector.persisted",
	}
	if err := store.WithinProductUnitOfWork(ctx, func(repositories domain.ProductRepositories) error {
		if err := repositories.InsertMetricSamples(ctx, []domain.MetricSample{metric}); err != nil {
			return err
		}
		if err := repositories.InsertInfrastructureEvents(ctx, []domain.InfrastructureEvent{event}); err != nil {
			return err
		}
		if err := repositories.SaveAlertMutation(ctx, mutation); err != nil {
			return err
		}
		return repositories.AppendAuditEntries(ctx, []domain.AuditEntry{audit})
	}); err != nil {
		t.Fatalf("persist observation unit: %v", err)
	}
	for table, identifier := range map[string]string{
		"metric_samples": "metric_integration", "infrastructure_events": "event_integration",
		"alerts": "alert_integration", "alert_transitions": "transition_integration",
		"notifications": "notification_integration", "audit_entries": "audit_integration",
	} {
		var count int
		if err := store.pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE id = $1", identifier).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s persisted count = %d, error = %v", table, count, err)
		}
	}
	auditPage, err := store.ListAuditEntries(ctx, domain.PageRequest{Limit: 10})
	if err != nil || len(auditPage.Entries) == 0 || auditPage.Entries[0].ID != audit.ID || auditPage.Page.Limit != 10 {
		t.Fatalf("ListAuditEntries() = (%#v, %v); want persisted audit", auditPage, err)
	}
	alertPage, err := store.ListAlerts(ctx, "source_integration", domain.PageRequest{Limit: 10})
	if err != nil || len(alertPage.Alerts) != 1 || alertPage.Alerts[0].ID != "alert_integration" || alertPage.Page.Limit != 10 {
		t.Fatalf("ListAlerts() = (%#v, %v); want persisted alert", alertPage, err)
	}
	notificationPage, err := store.ListNotifications(ctx, domain.PageRequest{Limit: 10})
	if err != nil || len(notificationPage.Notifications) != 1 ||
		notificationPage.Notifications[0].ID != "notification_integration" || notificationPage.Page.Limit != 10 {
		t.Fatalf("ListNotifications() = (%#v, %v); want persisted notification", notificationPage, err)
	}
	verifyWebhookDeliveryRepository(t, ctx, store, now, domain.UserID(userID))
	acknowledgedAt := now.Add(time.Minute)
	acknowledgement := domain.AlertAcknowledgement{
		AlertID: "alert_integration", UserID: domain.UserID(userID), TransitionID: "transition_ack_integration",
		OccurredAt: acknowledgedAt,
		Audit: domain.AuditEntry{
			ID: "audit_ack_integration", OccurredAt: acknowledgedAt,
			Actor:  domain.AuditActor{Kind: domain.AuditActorUser, ID: string(userID)},
			Action: "alert.acknowledge", Target: domain.AuditTarget{Kind: "alert.instance", ID: "alert_integration"},
			Outcome: domain.AuditSuccess, RequestID: "request_ack_integration", ReasonCode: "alert.acknowledged",
		},
	}
	acknowledged, err := store.AcknowledgeAlert(ctx, acknowledgement)
	if err != nil || acknowledged.State != domain.AlertAcknowledged || acknowledged.AcknowledgedAt == nil ||
		!acknowledged.AcknowledgedAt.Equal(acknowledgedAt) {
		t.Fatalf("AcknowledgeAlert() = (%#v, %v); want acknowledged alert", acknowledged, err)
	}
	if _, err := store.AcknowledgeAlert(ctx, acknowledgement); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatalf("second AcknowledgeAlert() error = %v; want conflict", err)
	}
}

func verifyWebhookDeliveryRepository(t *testing.T, ctx context.Context, store *Store, now time.Time, actor domain.UserID) {
	t.Helper()
	created, err := store.EnqueueWebhookDeliveries(ctx, "webhook_integration", 3, now, 4)
	if err != nil || created != 0 {
		t.Fatal("synthetic notification escaped to outbound delivery")
	}
	if _, err := store.pool.Exec(ctx, insertNotificationStatement, "notification_real_delivery_integration", nil, nil, nil, "notification.alert_firing", "error", "alert.condition_matched", now.UnixMicro(), 0); err != nil {
		t.Fatal("seed isolated nonsynthetic delivery fixture")
	}
	created, err = store.EnqueueWebhookDeliveries(ctx, "webhook_integration", 3, now, 4)
	if err != nil || created != 1 {
		t.Fatalf("EnqueueWebhookDeliveries() = (%d, %v); want one", created, err)
	}
	created, err = store.EnqueueWebhookDeliveries(ctx, "webhook_integration", 3, now, 4)
	if err != nil || created != 0 {
		t.Fatalf("second EnqueueWebhookDeliveries() = (%d, %v); want stable no-op", created, err)
	}
	tasks, err := store.ClaimWebhookDeliveries(ctx, "worker_integration", now, time.Minute, 4)
	if err != nil || len(tasks) != 1 || tasks[0].AttemptNumber != 1 ||
		tasks[0].NotificationID != "notification_real_delivery_integration" || tasks[0].Validate(now) != nil {
		t.Fatalf("ClaimWebhookDeliveries() = (%#v, %v); want one", tasks, err)
	}
	firstID := tasks[0].ID
	firstPayload := string(tasks[0].Payload)
	tasks, err = store.ClaimWebhookDeliveries(ctx, "worker_other", now, time.Minute, 4)
	if err != nil || len(tasks) != 0 {
		t.Fatalf("concurrent ClaimWebhookDeliveries() = (%#v, %v); want none", tasks, err)
	}
	retryAt := now.Add(5 * time.Second)
	status := 503
	err = store.CompleteWebhookDelivery(ctx, notification.DeliveryCompletion{
		DeliveryID: firstID, AttemptNumber: 1, LeaseOwner: "worker_integration", FinishedAt: now,
		Outcome: notification.DeliveryFailed, ResponseStatus: &status,
		ErrorCode: "notification.remote_rejected", RetryAt: &retryAt,
	})
	if err != nil {
		t.Fatalf("CompleteWebhookDelivery(retry) error = %v", err)
	}
	tasks, err = store.ClaimWebhookDeliveries(ctx, "worker_integration", retryAt.Add(-time.Microsecond), time.Minute, 4)
	if err != nil || len(tasks) != 0 {
		t.Fatalf("early ClaimWebhookDeliveries() = (%#v, %v); want none", tasks, err)
	}
	tasks, err = store.ClaimWebhookDeliveries(ctx, "worker_integration", retryAt, time.Minute, 4)
	if err != nil || len(tasks) != 1 || tasks[0].AttemptNumber != 2 || tasks[0].ID != firstID ||
		string(tasks[0].Payload) != firstPayload {
		t.Fatalf("retry ClaimWebhookDeliveries() = (%#v, %v); want fixed retry", tasks, err)
	}
	status = 204
	err = store.CompleteWebhookDelivery(ctx, notification.DeliveryCompletion{
		DeliveryID: firstID, AttemptNumber: 2, LeaseOwner: "worker_integration", FinishedAt: retryAt,
		Outcome: notification.DeliverySucceeded, ResponseStatus: &status,
	})
	if err != nil {
		t.Fatalf("CompleteWebhookDelivery(success) error = %v", err)
	}
	var state string
	var attempts int
	if err := store.pool.QueryRow(ctx, `SELECT state, attempt_count FROM notification_deliveries WHERE id = $1`, firstID).Scan(&state, &attempts); err != nil || state != "succeeded" || attempts != 2 {
		t.Fatalf("delivery state = (%q, %d, %v); want succeeded after two", state, attempts, err)
	}

	leaseNotificationAt := retryAt.Add(time.Second)
	if _, err := store.pool.Exec(ctx, `INSERT INTO notifications (
    id, source_id, alert_id, resource_id, notification_kind, severity,
    summary_code, created_at_us, synthetic
) SELECT 'notification_lease_integration', source_id, alert_id, resource_id,
         notification_kind, severity, summary_code, $1, synthetic
  FROM notifications WHERE id = 'notification_real_delivery_integration'`, leaseNotificationAt.UnixMicro()); err != nil {
		t.Fatal("seed lease recovery notification")
	}
	created, err = store.EnqueueWebhookDeliveries(ctx, "webhook_integration", 3, leaseNotificationAt, 4)
	if err != nil || created != 1 {
		t.Fatalf("lease EnqueueWebhookDeliveries() = (%d, %v); want one", created, err)
	}
	tasks, err = store.ClaimWebhookDeliveries(ctx, "worker_lost", leaseNotificationAt, time.Second, 4)
	if err != nil || len(tasks) != 1 || tasks[0].AttemptNumber != 1 {
		t.Fatalf("lease ClaimWebhookDeliveries() = (%#v, %v); want first attempt", tasks, err)
	}
	recoveredAt := leaseNotificationAt.Add(time.Second)
	tasks, err = store.ClaimWebhookDeliveries(ctx, "worker_recovery", recoveredAt, time.Minute, 4)
	if err != nil || len(tasks) != 1 || tasks[0].AttemptNumber != 2 || tasks[0].LeaseOwner != "worker_recovery" {
		t.Fatalf("recovered ClaimWebhookDeliveries() = (%#v, %v); want second attempt", tasks, err)
	}
	var firstOutcome string
	if err := store.pool.QueryRow(ctx, `SELECT outcome FROM notification_delivery_attempts
WHERE delivery_id = $1 AND attempt_number = 1`, tasks[0].ID).Scan(&firstOutcome); err != nil || firstOutcome != "uncertain" {
		t.Fatalf("expired attempt outcome = (%q, %v); want uncertain", firstOutcome, err)
	}
	status = 204
	if err := store.CompleteWebhookDelivery(ctx, notification.DeliveryCompletion{
		DeliveryID: tasks[0].ID, AttemptNumber: 2, LeaseOwner: "worker_recovery", FinishedAt: recoveredAt,
		Outcome: notification.DeliverySucceeded, ResponseStatus: &status,
	}); err != nil {
		t.Fatalf("CompleteWebhookDelivery(recovered) error = %v", err)
	}

	testAt := recoveredAt.Add(time.Second)
	command := notification.WebhookTestCommand{
		Notification: domain.Notification{
			ID: "notification_test_integration", Kind: notification.TestNotificationKind,
			Severity: domain.SeverityInfo, SummaryCode: notification.TestNotificationReason, CreatedAt: testAt,
		},
		DestinationReferenceID: "webhook_integration", MaximumAttempts: 1,
		Audit: domain.AuditEntry{
			ID: "audit_notification_test_integration", OccurredAt: testAt,
			Actor:  domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor)},
			Action: "notification.test", Target: domain.AuditTarget{Kind: "notification.destination", ID: "webhook_integration"},
			Outcome: domain.AuditIntent, RequestID: "request_notification_test_integration",
			ReasonCode: notification.TestNotificationReason,
		},
	}
	if err := store.CreateWebhookTest(ctx, command); err != nil {
		t.Fatalf("CreateWebhookTest() error = %v", err)
	}
	tasks, err = store.ClaimWebhookDeliveries(ctx, "worker_test", testAt, time.Minute, 4)
	if err != nil || len(tasks) != 1 || tasks[0].NotificationID != "notification_test_integration" || tasks[0].MaximumAttempts != 1 {
		t.Fatalf("test ClaimWebhookDeliveries() = (%#v, %v); want one test", tasks, err)
	}
	var purpose string
	if err := store.pool.QueryRow(ctx, `SELECT delivery_purpose FROM notification_deliveries WHERE id = $1`, tasks[0].ID).Scan(&purpose); err != nil || purpose != "test" {
		t.Fatalf("test delivery purpose = (%q, %v); want test", purpose, err)
	}
	testStatus := 204
	if err := store.CompleteWebhookDelivery(ctx, notification.DeliveryCompletion{
		DeliveryID: tasks[0].ID, AttemptNumber: 1, LeaseOwner: "worker_test",
		FinishedAt: testAt.Add(time.Second), Outcome: notification.DeliverySucceeded, ResponseStatus: &testStatus,
	}); err != nil {
		t.Fatalf("CompleteWebhookDelivery(test) error = %v", err)
	}
	var auditOutcome, auditReason, auditRequest string
	if err := store.pool.QueryRow(ctx, `SELECT outcome, reason_code, request_id
FROM audit_entries
WHERE action = 'notification.test' AND outcome = 'success' AND target_id = 'webhook_integration'`).Scan(
		&auditOutcome, &auditReason, &auditRequest,
	); err != nil || auditOutcome != "success" || auditReason != "notification.test_delivery_succeeded" || auditRequest != "request_notification_test_integration" {
		t.Fatalf("test result audit = (%q, %q, %q, %v)", auditOutcome, auditReason, auditRequest, err)
	}
	leaseTestAt := testAt.Add(2 * time.Second)
	leaseCommand := notification.WebhookTestCommand{
		Notification: domain.Notification{
			ID: "notification_test_lease_integration", Kind: notification.TestNotificationKind,
			Severity: domain.SeverityInfo, SummaryCode: notification.TestNotificationReason, CreatedAt: leaseTestAt,
		},
		DestinationReferenceID: "webhook_integration", MaximumAttempts: 1,
		Audit: domain.AuditEntry{
			ID: "audit_notification_test_lease_integration", OccurredAt: leaseTestAt,
			Actor:  domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor)},
			Action: "notification.test", Target: domain.AuditTarget{Kind: "notification.destination", ID: "webhook_integration"},
			Outcome: domain.AuditIntent, RequestID: "request_notification_test_lease_integration",
			ReasonCode: notification.TestNotificationReason,
		},
	}
	if err := store.CreateWebhookTest(ctx, leaseCommand); err != nil {
		t.Fatalf("CreateWebhookTest(lease) error = %v", err)
	}
	tasks, err = store.ClaimWebhookDeliveries(ctx, "worker_test_lost", leaseTestAt, time.Second, 4)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("lease test claim = (%#v, %v); want one", tasks, err)
	}
	tasks, err = store.ClaimWebhookDeliveries(ctx, "worker_test_recovery", leaseTestAt.Add(time.Second), time.Minute, 4)
	if err != nil || len(tasks) != 0 {
		t.Fatalf("lease test recovery claim = (%#v, %v); want terminal test", tasks, err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT outcome, reason_code, request_id
FROM audit_entries
WHERE action = 'notification.test' AND outcome = 'uncertain' AND target_id = 'webhook_integration'`).Scan(
		&auditOutcome, &auditReason, &auditRequest,
	); err != nil || auditReason != "notification.test_delivery_uncertain" || auditRequest != "request_notification_test_lease_integration" {
		t.Fatalf("test lease audit = (%q, %q, %q, %v)", auditOutcome, auditReason, auditRequest, err)
	}
}

func TestPostgreSQLRejectsForeignNonEmptySchema(t *testing.T) {
	databaseURL := os.Getenv(integrationDatabaseURLKey)
	if databaseURL == "" {
		t.Skip("real PostgreSQL URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	admin, err := testpostgres.OpenGate(ctx, databaseURL)
	if err != nil {
		t.Fatal("connect to PostgreSQL test database")
	}
	defer func() { _ = admin.Close(context.Background()) }()
	schema := uniqueTestSchema(t)
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal("create isolated PostgreSQL test schema")
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_, _ = admin.Exec(cleanupContext, "DROP SCHEMA "+identifier+" CASCADE")
	})
	if _, err := admin.Exec(ctx, "CREATE TABLE "+identifier+`.foreign_table (id bigint PRIMARY KEY)`); err != nil {
		t.Fatal("create foreign table")
	}
	options := validTestOptions()
	options.ConnectionString = databaseURL
	options.Schema = schema
	store, err := Open(ctx, options, postgresmigrations.FS)
	if store != nil {
		store.Close()
	}
	if !errors.Is(err, ErrMigrationRejected) {
		t.Fatalf("Open() error = %v; want migration rejection", err)
	}
}

func uniqueTestSchema(t *testing.T) string {
	t.Helper()
	var randomBytes [8]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		t.Fatal("make isolated PostgreSQL test schema name")
	}
	return "stackplore_test_" + hex.EncodeToString(randomBytes[:])
}

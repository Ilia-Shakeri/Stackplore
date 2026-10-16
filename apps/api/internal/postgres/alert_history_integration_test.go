package postgres

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
	"github.com/jackc/pgx/v5"
)

func durableDemoSeed(t *testing.T, fixture observationFixture) domain.DemoAlertSeed {
	t.Helper()
	_, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO users(id,login_name,canonical_login_name,display_name,role,enabled,created_at_us,updated_at_us) VALUES('user_alert_seed','seed','seed','Fixture administrator','administrator',1,$1,$1)`, fixture.at.Add(-time.Hour).UnixMicro())
	if err != nil {
		t.Fatal("seed administrator metadata")
	}
	resourceID := fixture.batch.Resources[1].Resource.Ref.ID
	alert := domain.Alert{ID: "alert_seed_fixture", RuleID: "rule_seed_fixture", SourceID: fixture.source.Source.ID, ResourceID: &resourceID, State: domain.AlertFiring, Severity: domain.SeverityError, EvidenceRange: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: fixture.at.Add(-time.Second)}, StartedAt: fixture.at.Add(-time.Minute), UpdatedAt: fixture.at, SummaryCode: "demo.alert.checkout_unhealthy", Synthetic: true}
	seed := domain.DemoAlertSeed{FixtureVersion: "demo-v1", Rule: domain.AlertRule{ID: alert.RuleID, SourceID: alert.SourceID, ResourceID: alert.ResourceID, DisplayName: "Fixture health", Kind: domain.AlertRuleResourceUnhealthy, Enabled: true, Severity: domain.SeverityError, EvaluationInterval: time.Second}, Mutation: domain.AlertMutation{Alert: alert, DedupeKey: string(alert.RuleID), Transition: &domain.AlertTransition{ID: "transition_seed_fixture", AlertID: alert.ID, To: domain.AlertFiring, OccurredAt: alert.UpdatedAt, Actor: domain.AuditActor{Kind: domain.AuditActorSystem}, ReasonCode: "alert.condition_matched", EvidenceRange: alert.EvidenceRange}}}
	if seed.Validate() != nil {
		t.Fatal("invalid fixture alert seed")
	}
	return seed
}

func TestDurableDemoAlertSeedPreservesAcknowledgementRestartAndDeletedRule(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatalf("seed durable demo alert: %v", err)
	}
	page, err := fixture.store.ListAlerts(fixture.ctx, fixture.source.Source.ID, domain.PageRequest{Limit: 10})
	if err != nil || len(page.Alerts) != 1 {
		t.Fatal("seed did not create durable alert")
	}
	at := fixture.at.Add(time.Second)
	command := domain.AlertAcknowledgement{AlertID: seed.Mutation.Alert.ID, UserID: "user_alert_seed", TransitionID: "transition_seed_ack", OccurredAt: at, Audit: domain.AuditEntry{ID: "audit_seed_ack", OccurredAt: at, Actor: domain.AuditActor{Kind: domain.AuditActorUser, ID: "user_alert_seed"}, Action: "alert.acknowledge", Target: domain.AuditTarget{Kind: "alert.instance", ID: string(seed.Mutation.Alert.ID)}, Outcome: domain.AuditSuccess, RequestID: "request_seed_ack", ReasonCode: "alert.acknowledged"}}
	if _, err := fixture.store.AcknowledgeAlert(fixture.ctx, command); err != nil {
		t.Fatalf("ack seed alert: %v", err)
	}
	if err := fixture.store.Close(); err != nil {
		t.Fatal("close seeded fixture")
	}
	reopened, err := Open(context.Background(), fixture.options, postgresmigrations.FS)
	if err != nil {
		t.Fatal("reopen seeded fixture")
	}
	defer reopened.Close()
	if err := reopened.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("repeat seed after restart")
	}
	page, err = reopened.ListAlerts(fixture.ctx, fixture.source.Source.ID, domain.PageRequest{Limit: 10})
	if err != nil || len(page.Alerts) != 1 || page.Alerts[0].State != domain.AlertAcknowledged {
		t.Fatal("restart reset acknowledgement")
	}
	rule, err := reopened.GetAlertRule(fixture.ctx, seed.Rule.ID)
	if err != nil {
		t.Fatal("read seed rule")
	}
	at = at.Add(time.Second)
	del := domain.AlertRuleDelete{RuleID: rule.Rule.ID, ExpectedVersion: rule.Version, UpdatedByUserID: "user_alert_seed", DeletedAt: at, Audit: domain.AuditEntry{ID: "audit_seed_delete", OccurredAt: at, Actor: domain.AuditActor{Kind: domain.AuditActorUser, ID: "user_alert_seed"}, Action: "alert.rule_delete", Target: domain.AuditTarget{Kind: "alert.rule", ID: string(rule.Rule.ID)}, Outcome: domain.AuditSuccess, RequestID: "request_seed_delete", ReasonCode: "alert.rule_deleted"}}
	if err := reopened.DeleteAlertRule(fixture.ctx, del); err != nil {
		t.Fatalf("delete seed rule: %v", err)
	}
	if err := reopened.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("repeat seed after delete")
	}
	if _, err := reopened.GetAlertRule(fixture.ctx, seed.Rule.ID); !errors.Is(err, domain.ErrRepositoryNotFound) {
		t.Fatal("seed resurrected deleted rule")
	}
	page, err = reopened.ListAlerts(fixture.ctx, fixture.source.Source.ID, domain.PageRequest{Limit: 10})
	if err != nil || page.Alerts[0].State != domain.AlertResolved {
		t.Fatal("deleted rule left active episode")
	}
	var notices, deliveries int
	if err := reopened.pool.QueryRow(fixture.ctx, `SELECT (SELECT count(*) FROM notifications),(SELECT count(*) FROM notification_deliveries)`).Scan(&notices, &deliveries); err != nil || notices != 0 || deliveries != 0 {
		t.Fatal("seed created external notification work")
	}
	changes, err := reopened.ListAlertChanges(fixture.ctx, fixture.source.Source.ID, 0, 20)
	if err != nil || changes.Reset || len(changes.Changes) != 5 {
		t.Fatalf("durable seed/ack/delete changes: count=%d err=%v", len(changes.Changes), err)
	}
}

func TestAlertEvaluationLeasesFenceRecoveryAndRuleEdits(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("seed leased rule")
	}
	at := time.Now().UTC().Truncate(time.Microsecond).Add(2 * time.Second)
	tasks, err := fixture.store.ClaimDueAlertEvaluations(fixture.ctx, at, 10)
	if err != nil || len(tasks) != 1 {
		t.Fatal("claim leased rule")
	}
	first := tasks[0]
	if first.RuleVersion != 1 || first.ClaimGeneration != 1 {
		t.Fatal("claim changed rule version")
	}
	duplicates, err := fixture.store.ClaimDueAlertEvaluations(fixture.ctx, at.Add(2*time.Second), 10)
	if err != nil || len(duplicates) != 0 {
		t.Fatal("lease allowed parallel evaluation")
	}
	recovered, err := fixture.store.ClaimDueAlertEvaluations(fixture.ctx, at.Add(time.Minute), 10)
	if err != nil || len(recovered) != 1 || recovered[0].ClaimGeneration <= first.ClaimGeneration {
		t.Fatal("expired lease did not recover")
	}
	if err := fixture.store.CompleteAlertEvaluation(fixture.ctx, first, nil); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("stale worker completion changed current lease")
	}
	if err := fixture.store.CompleteAlertEvaluation(fixture.ctx, recovered[0], nil); err != nil {
		t.Fatalf("complete current lease: %v", err)
	}
	edited, err := fixture.store.ClaimDueAlertEvaluations(fixture.ctx, at.Add(2*time.Minute), 10)
	if err != nil || len(edited) != 1 {
		t.Fatal("claim before edit")
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE alert_rules SET row_version=row_version+1,evaluation_generation=evaluation_generation+1,evaluation_lease_until_us=NULL WHERE id=$1`, string(seed.Rule.ID)); err != nil {
		t.Fatal("edit lease fixture")
	}
	if err := fixture.store.CompleteAlertEvaluation(fixture.ctx, edited[0], nil); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("edited rule accepted old evaluation")
	}
}

func TestRawCollectionOutcomeDoesNotPretendCacheIsConnected(t *testing.T) {
	fixture := newObservationFixture(t)
	first, err := fixture.store.GetSourceCollectionStatus(fixture.ctx, fixture.token.Source)
	if err != nil || first.Status.State != domain.AvailabilityAvailable || !first.CollectedAt.Equal(fixture.at) {
		t.Fatal("raw collection success unavailable")
	}
	token, err := fixture.store.BeginCapture(fixture.ctx, fixture.source.Source.ID, 1, fixture.at.Add(time.Second))
	if err != nil {
		t.Fatal("begin failed capture")
	}
	completion := fixture.completion
	completion.Token = token
	completion.Complete = false
	completion.Status.State = domain.AvailabilityUnavailable
	completion.Status.ReasonCode = "observation.provider_unavailable"
	if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
		t.Fatal("finish failed capture")
	}
	cached, err := fixture.store.GetObservationStatus(fixture.ctx, token.Source)
	if err != nil || cached.State != domain.AvailabilityStale {
		t.Fatal("cache did not show stale data")
	}
	raw, err := fixture.store.GetSourceCollectionStatus(fixture.ctx, token.Source)
	if err != nil || raw.Status.State != domain.AvailabilityUnavailable || raw.Status.ReasonCode != "observation.provider_unavailable" {
		t.Fatal("raw collection outage became cachedstale")
	}
	history, err := fixture.store.ListSourceCollectionHistory(fixture.ctx, token.Source, domain.TimeRange{Start: fixture.at.Add(-time.Second), End: fixture.at.Add(2 * time.Second)}, 1)
	if err != nil || history.Complete || len(history.Observations) != 1 {
		t.Fatal("collection history cap was hidden")
	}
}

func TestRawCollectionOutcomeCannotCrossConfigurationVersion(t *testing.T) {
	fixture := newObservationFixture(t)
	source := fixture.token.Source
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE infrastructure_sources SET enabled=0,row_version=row_version+1 WHERE id=$1`, string(source.SourceID)); err != nil {
		t.Fatal("disable source fixture")
	}
	disabled, err := fixture.store.GetSourceCollectionStatus(fixture.ctx, source)
	if err != nil || disabled.Status.ReasonCode != "source.disabled" || disabled.Status.AllowsData() {
		t.Fatal("disabled source reused successful collection")
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE infrastructure_sources SET enabled=1,row_version=row_version+1 WHERE id=$1`, string(source.SourceID)); err != nil {
		t.Fatal("enable source fixture")
	}
	pending, err := fixture.store.GetSourceCollectionStatus(fixture.ctx, source)
	if err != nil || pending.Status.ReasonCode != "source.collection_pending" || pending.Status.AllowsData() {
		t.Fatal("enabled source reused prior configuration success")
	}
	period := domain.TimeRange{Start: fixture.at.Add(-time.Second), End: fixture.at.Add(time.Minute)}
	history, err := fixture.store.ListSourceCollectionHistory(fixture.ctx, source, period, 10)
	if err != nil || !history.Complete || len(history.Observations) != 0 {
		t.Fatal("new configuration reused old collection window")
	}
	token, err := fixture.store.BeginCapture(fixture.ctx, source.SourceID, 3, fixture.at.Add(time.Second))
	if err != nil {
		t.Fatal("begin new configuration capture")
	}
	completion := fixture.completion
	completion.Token = token
	if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
		t.Fatal("finish new configuration capture")
	}
	current, err := fixture.store.GetSourceCollectionStatus(fixture.ctx, source)
	if err != nil || current.Status.State != domain.AvailabilityAvailable || !current.CollectedAt.Equal(token.ObservedAt) {
		t.Fatal("new configuration has no own result")
	}
	history, err = fixture.store.ListSourceCollectionHistory(fixture.ctx, source, period, 10)
	if err != nil || len(history.Observations) != 1 || !history.Observations[0].CollectedAt.Equal(token.ObservedAt) {
		t.Fatal("configuration window mixed old collection")
	}
}

func TestAlertChangesSerializeCommitOrderAndResetPrunedCursor(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("seed replay fixture")
	}
	position, err := fixture.store.AlertChangePosition(fixture.ctx, fixture.source.Source.ID)
	if err != nil || position < 1 {
		t.Fatal("read replay position")
	}
	ctx, cancel := context.WithTimeout(fixture.ctx, 5*time.Second)
	defer cancel()
	first, err := fixture.store.pool.Begin(ctx)
	if err != nil {
		t.Fatal("begin first change transaction")
	}
	defer first.Rollback(context.Background())
	holder := &transaction{transaction: first}
	if err := lockAlertChanges(ctx, holder); err != nil {
		t.Fatal("lock first change")
	}
	if err := appendAlertChange(ctx, holder, seed.Rule.SourceID, seed.Rule.ID, nil, domain.AlertChangeRuleChanged, fixture.at.Add(time.Millisecond)); err != nil {
		t.Fatal("append first change")
	}
	mutation := seed.Mutation
	mutation.Notification = nil
	mutation.Transition = nil
	previousAt, previousState := mutation.Alert.UpdatedAt, mutation.Alert.State
	mutation.ExpectedUpdatedAt = &previousAt
	mutation.ExpectedState = &previousState
	mutation.Alert.UpdatedAt = fixture.at.Add(time.Second)
	finished := make(chan error, 1)
	go func() { finished <- fixture.store.SaveAlertMutation(ctx, mutation) }()
	select {
	case err := <-finished:
		t.Fatalf("second change passed uncommitted first: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := first.Commit(ctx); err != nil {
		t.Fatal("commit first change")
	}
	if err := <-finished; err != nil {
		t.Fatalf("commit second change: %v", err)
	}
	page, err := fixture.store.ListAlertChanges(ctx, fixture.source.Source.ID, position, 1)
	if err != nil || page.Reset || len(page.Changes) != 1 || page.Changes[0].Kind != domain.AlertChangeRuleChanged {
		t.Fatal("replay skipped firstcommitted change")
	}
	next, err := fixture.store.ListAlertChanges(ctx, fixture.source.Source.ID, page.NextID, 1)
	if err != nil || next.Reset || len(next.Changes) != 1 || next.Changes[0].Kind != domain.AlertChangeAlert || next.NextID <= page.NextID {
		t.Fatal("replay failed ordered keyset")
	}
	removed, err := fixture.store.PruneAlertHistory(ctx, fixture.at.Add(2*time.Second), 100)
	if err != nil || removed < 1 || removed > 100 {
		t.Fatal("bounded replay prune failed")
	}
	reset, err := fixture.store.ListAlertChanges(ctx, fixture.source.Source.ID, position, 10)
	if err != nil || !reset.Reset || reset.NextID != next.NextID {
		t.Fatal("pruned cursor did not demand reset")
	}
	latest, err := fixture.store.AlertChangePosition(ctx, fixture.source.Source.ID)
	if err != nil || latest != next.NextID {
		t.Fatal("prune lost latest durable position")
	}
}

func TestAlertChangesResetWhenBackdatedCommitIsPrunedBetweenRetainedIDs(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("seed backdated replay")
	}
	position, err := fixture.store.AlertChangePosition(fixture.ctx, seed.Rule.SourceID)
	if err != nil {
		t.Fatal("read backdated replay position")
	}
	for _, at := range []time.Time{fixture.at.Add(time.Second), fixture.at.Add(-time.Minute), fixture.at.Add(2 * time.Second)} {
		if err := fixture.store.withinTransaction(fixture.ctx, func(ctx context.Context, transaction *transaction) error {
			if err := lockAlertChanges(ctx, transaction); err != nil {
				return err
			}
			return appendAlertChange(ctx, transaction, seed.Rule.SourceID, seed.Rule.ID, nil, domain.AlertChangeRuleChanged, at)
		}); err != nil {
			t.Fatal("append variably timed replay change")
		}
	}
	newest, err := fixture.store.AlertChangePosition(fixture.ctx, seed.Rule.SourceID)
	if err != nil {
		t.Fatal("read last backdated replay position")
	}
	if removed, err := fixture.store.PruneAlertHistory(fixture.ctx, fixture.at, 100); err != nil || removed != 1 {
		t.Fatalf("prune only middle backdated change: rows=%d err=%v", removed, err)
	}
	var oldest, prunedThrough int64
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT (SELECT MIN(id) FROM alert_changes WHERE source_id=$1),pruned_through_id FROM alert_change_retention_positions WHERE source_id=$1`, string(seed.Rule.SourceID)).Scan(&oldest, &prunedThrough); err != nil || oldest >= position || prunedThrough != newest-1 {
		t.Fatal("backdated replay fixture lacks retained lower IDs")
	}
	if err := fixture.store.Close(); err != nil {
		t.Fatal("close pruned replay store")
	}
	reopened, err := Open(context.Background(), fixture.options, postgresmigrations.FS)
	if err != nil {
		t.Fatal("reopen pruned replay store")
	}
	defer reopened.Close()
	page, err := reopened.ListAlertChanges(fixture.ctx, seed.Rule.SourceID, position, 10)
	if err != nil || !page.Reset || page.NextID != newest || len(page.Changes) != 0 {
		t.Fatal("middle prune silently skipped durable change after restart")
	}
	current, err := reopened.ListAlertChanges(fixture.ctx, seed.Rule.SourceID, prunedThrough, 10)
	if err != nil || !current.Reset || len(current.Changes) != 0 || current.NextID != newest {
		t.Fatal("absent cursor at prune watermark did not demand reset")
	}
}

func TestAlertChangesResetWrongSourceAndRolledBackCursorInsideBounds(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("seed source-bound replay")
	}
	otherSource := domain.SourceConfiguration{Source: domain.InfrastructureSource{ID: "source_alert_other", Provider: domain.ProviderDemo, Name: "Other fixture", Synthetic: true}, ProfileID: domain.SourceProfileDemoFixture, Enabled: true, Version: 1, CreatedAt: fixture.at, UpdatedAt: fixture.at}
	if err := fixture.store.EnsureSourceConfigurations(fixture.ctx, []domain.SourceConfiguration{otherSource}); err != nil {
		t.Fatal("create other replay source")
	}
	otherRule := domain.AlertRuleRecord{Rule: domain.AlertRule{ID: "rule_alert_other", SourceID: otherSource.Source.ID, DisplayName: "Other connection", Kind: domain.AlertRuleProviderConnectionLost, Enabled: true, Severity: domain.SeverityWarning, EvaluationInterval: time.Second}, CreatedByUserID: "user_alert_seed", UpdatedByUserID: "user_alert_seed", CreatedAt: fixture.at, UpdatedAt: fixture.at, Version: 1}
	create := domain.AlertRuleCreate{Record: otherRule, Audit: alertCommandAudit("audit_alert_other_rule", fixture.at, "alert.rule_create", "alert.rule", string(otherRule.Rule.ID), "alert.rule_created")}
	if err := fixture.store.CreateAlertRule(fixture.ctx, create); err != nil {
		t.Fatalf("create other source rule: %v", err)
	}
	otherPosition, err := fixture.store.AlertChangePosition(fixture.ctx, otherSource.Source.ID)
	if err != nil {
		t.Fatal("read other source position")
	}
	rolledBack, err := fixture.store.pool.Begin(fixture.ctx)
	if err != nil {
		t.Fatal("begin rolled-back replay change")
	}
	defer rolledBack.Rollback(context.Background())
	if err := lockAlertChanges(fixture.ctx, &transaction{transaction: rolledBack}); err != nil {
		t.Fatal("lock rolled-back replay change")
	}
	var gap int64
	if err := rolledBack.QueryRow(fixture.ctx, `INSERT INTO alert_changes(source_id,rule_id,change_kind,occurred_at_us) VALUES($1,$2,'rule.changed',$3) RETURNING id`, string(seed.Rule.SourceID), string(seed.Rule.ID), fixture.at.UnixMicro()).Scan(&gap); err != nil {
		t.Fatal("append rolled-back replay change")
	}
	if err := rolledBack.Rollback(fixture.ctx); err != nil {
		t.Fatal("roll back replay change")
	}
	if err := fixture.store.withinTransaction(fixture.ctx, func(ctx context.Context, transaction *transaction) error {
		if err := lockAlertChanges(ctx, transaction); err != nil {
			return err
		}
		return appendAlertChange(ctx, transaction, seed.Rule.SourceID, seed.Rule.ID, nil, domain.AlertChangeRuleChanged, fixture.at.Add(time.Second))
	}); err != nil {
		t.Fatal("append own source upper replay bound")
	}
	newest, err := fixture.store.AlertChangePosition(fixture.ctx, seed.Rule.SourceID)
	if err != nil || otherPosition <= 2 || otherPosition >= newest || gap <= otherPosition || gap >= newest {
		t.Fatal("replay fixture has no internal cursor gaps")
	}
	for _, after := range []int64{otherPosition, gap} {
		page, err := fixture.store.ListAlertChanges(fixture.ctx, seed.Rule.SourceID, after, 10)
		if err != nil || !page.Reset || page.NextID != newest || len(page.Changes) != 0 {
			t.Fatal("foreign or uncommitted cursor silently accepted within source bounds")
		}
	}
	valid, err := fixture.store.ListAlertChanges(fixture.ctx, seed.Rule.SourceID, 2, 10)
	if err != nil || valid.Reset || valid.NextID != newest || len(valid.Changes) != 1 {
		t.Fatal("valid source cursor lost replay across unrelated sequence gaps")
	}
}

func TestWorkloadReplicaHistoryBoundsAndMissingCapture(t *testing.T) {
	fixture := newObservationFixture(t)
	resource := fixture.batch.Resources[0].Resource.Ref
	query := domain.WorkloadReplicaHistoryQuery{Source: fixture.token.Source, Resource: resource, Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: fixture.at.Add(time.Minute)}, Limit: 10}
	initial, err := fixture.store.GetWorkloadReplicaHistory(fixture.ctx, query)
	if err != nil || len(initial.Samples) != 1 || initial.Samples[0].Desired != 3 || initial.Samples[0].Ready != 2 {
		t.Fatal("persisted replica history differs")
	}
	token, err := fixture.store.BeginCapture(fixture.ctx, fixture.source.Source.ID, 1, fixture.at.Add(time.Second))
	if err != nil {
		t.Fatal("begin missing replica capture")
	}
	batch := fixture.batch
	batch.Token = token
	batch.WorkloadReplicas = nil
	if err := fixture.store.SaveCapture(fixture.ctx, batch); err != nil {
		t.Fatalf("save missing replica capture: %v", err)
	}
	completion := fixture.completion
	completion.Token = token
	if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
		t.Fatal("finish missing replica capture")
	}
	missing, err := fixture.store.GetWorkloadReplicaHistory(fixture.ctx, query)
	if err != nil || missing.Status.AllowsData() {
		t.Fatal("missing replica capture kept prior readycounts")
	}
	token, err = fixture.store.BeginCapture(fixture.ctx, fixture.source.Source.ID, 1, fixture.at.Add(2*time.Second))
	if err != nil {
		t.Fatal("begin recoveredreplica capture")
	}
	batch.Token = token
	batch.WorkloadReplicas = fixture.batch.WorkloadReplicas
	batch.WorkloadReplicas[0].Status.ObservedAt = fixture.at.Add(2 * time.Second)
	if err := fixture.store.SaveCapture(fixture.ctx, batch); err != nil {
		t.Fatal("save recovered replica capture")
	}
	completion.Token = token
	if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
		t.Fatal("finish recovered replica capture")
	}
	recovered, err := fixture.store.GetWorkloadReplicaHistory(fixture.ctx, query)
	if err != nil || len(recovered.Samples) != 3 || recovered.Samples[1].Status.AllowsData() {
		t.Fatal("recovery hid missingpoll history")
	}
	query.Limit = 1
	capped, err := fixture.store.GetWorkloadReplicaHistory(fixture.ctx, query)
	if err != nil || len(capped.Samples) != 1 || capped.Status.Completeness != domain.CompletenessPartial {
		t.Fatal("replica history cap hidden")
	}
}

func TestAlertMigrationUpgradesEightVersionLedgerWithoutLoss(t *testing.T) {
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
		t.Fatal("read migration inventory")
	}
	for _, entry := range entries {
		version, valid := migrationVersion(entry.Name())
		if !valid || version > 8 {
			continue
		}
		data, err := fs.ReadFile(postgresmigrations.FS, entry.Name())
		if err != nil {
			t.Fatal("read released migration")
		}
		files[entry.Name()] = &fstest.MapFile{Data: data}
	}
	legacy, err := Open(context.Background(), options, files)
	if err != nil {
		t.Fatalf("open eight-version schema: %v", err)
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	_, err = legacy.pool.Exec(context.Background(), `INSERT INTO infrastructure_sources(id,provider,display_name,synthetic,enabled,connection_kind,created_at_us,updated_at_us) VALUES('source_eight','demo','Keep source metadata',1,1,'fixture',$1,$1)`, at.UnixMicro())
	if err != nil {
		t.Fatal("seed eight-version metadata")
	}
	if err := legacy.Close(); err != nil {
		t.Fatal("close eight-version fixture")
	}
	upgraded, err := Open(context.Background(), options, postgresmigrations.FS)
	if err != nil {
		t.Fatalf("upgrade eight-version ledger: %v", err)
	}
	defer upgraded.Close()
	source, err := upgraded.GetSourceConfiguration(context.Background(), "source_eight")
	if err != nil || source.Source.Name != "Keep source metadata" || source.Version != 1 {
		t.Fatal("migration lost source metadata")
	}
	var ledger int
	if err := upgraded.pool.QueryRow(context.Background(), `SELECT count(*) FROM schema_migrations`).Scan(&ledger); err != nil || ledger != len(upgraded.expectedMigrations) {
		t.Fatal("upgraded migration ledger mismatch")
	}
}

package postgres

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func alertCommandAudit(id string, at time.Time, action domain.AuditAction, targetKind domain.AuditTargetKind, targetID string, reason domain.ReasonCode) domain.AuditEntry {
	return domain.AuditEntry{ID: domain.AuditEntryID(id), OccurredAt: at,
		Actor: domain.AuditActor{Kind: domain.AuditActorUser, ID: "user_alert_seed"}, Action: action,
		Target: domain.AuditTarget{Kind: targetKind, ID: targetID}, Outcome: domain.AuditSuccess,
		RequestID: "request_alert_atomic", ReasonCode: reason}
}

func alertAckCommand(seed domain.DemoAlertSeed, at time.Time) domain.AlertAcknowledgement {
	return domain.AlertAcknowledgement{AlertID: seed.Mutation.Alert.ID, UserID: "user_alert_seed",
		TransitionID: "transition_alert_atomic_ack", OccurredAt: at,
		Audit: alertCommandAudit("audit_alert_atomic_ack", at, "alert.acknowledge", "alert.instance", string(seed.Mutation.Alert.ID), "alert.acknowledged")}
}

func TestAlertAcknowledgementChangeTransitionAndAuditAreAtomic(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("seed atomic acknowledgement")
	}
	command := alertAckCommand(seed, fixture.at.Add(time.Second))
	if err := fixture.store.AppendAuditEntries(fixture.ctx, []domain.AuditEntry{command.Audit}); err != nil {
		t.Fatal("seed audit conflict")
	}
	position, err := fixture.store.AlertChangePosition(fixture.ctx, seed.Rule.SourceID)
	if err != nil {
		t.Fatal("read acknowledgement position")
	}
	if _, err := fixture.store.AcknowledgeAlert(fixture.ctx, command); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatalf("audit conflict did not reject acknowledgement: %v", err)
	}
	page, err := fixture.store.ListAlerts(fixture.ctx, seed.Rule.SourceID, domain.PageRequest{Limit: 10})
	if err != nil || len(page.Alerts) != 1 || page.Alerts[0].State != domain.AlertFiring {
		t.Fatal("audit failure committed acknowledgement")
	}
	changes, err := fixture.store.ListAlertChanges(fixture.ctx, seed.Rule.SourceID, position, 10)
	if err != nil || changes.Reset || len(changes.Changes) != 0 {
		t.Fatal("audit failure committed replay change")
	}
	var transitions int
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM alert_transitions WHERE id=$1`, string(command.TransitionID)).Scan(&transitions); err != nil || transitions != 0 {
		t.Fatal("audit failure committed transition")
	}
	var timeline int
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM infrastructure_events WHERE kind='system.alert' AND reason_code='alert.acknowledged'`).Scan(&timeline); err != nil || timeline != 0 {
		t.Fatal("failed acknowledgement leaked timeline event")
	}
	command.Audit.ID = "audit_alert_atomic_ack_success"
	if _, err := fixture.store.AcknowledgeAlert(fixture.ctx, command); err != nil {
		t.Fatalf("acknowledge after rollback: %v", err)
	}
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM infrastructure_events WHERE origin_kind='system' AND kind='system.alert' AND reason_code='alert.acknowledged' AND summary='Alert state changed to acknowledged.'`).Scan(&timeline); err != nil || timeline != 1 {
		t.Fatal("successful acknowledgement lacks one safe timeline event")
	}
	changes, err = fixture.store.ListAlertChanges(fixture.ctx, seed.Rule.SourceID, position, 10)
	if err != nil || len(changes.Changes) != 1 || changes.Changes[0].Kind != domain.AlertChangeAlert || changes.Changes[0].AlertID == nil || *changes.Changes[0].AlertID != command.AlertID {
		t.Fatal("acknowledgement did not commit one durable change")
	}
	if _, err := fixture.store.AcknowledgeAlert(fixture.ctx, command); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("acknowledgement replay did not conflict")
	}
	latest, err := fixture.store.AlertChangePosition(fixture.ctx, seed.Rule.SourceID)
	if err != nil || latest != changes.NextID {
		t.Fatal("acknowledgement replay added change")
	}
}

func TestConcurrentAlertAcknowledgementsCommitOneChange(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("seed concurrent acknowledgement")
	}
	position, err := fixture.store.AlertChangePosition(fixture.ctx, seed.Rule.SourceID)
	if err != nil {
		t.Fatal("read concurrent acknowledgement position")
	}
	command := alertAckCommand(seed, fixture.at.Add(time.Second))
	start := make(chan struct{})
	results := make(chan error, 2)
	var pending sync.WaitGroup
	for range 2 {
		pending.Add(1)
		go func() {
			defer pending.Done()
			<-start
			_, err := fixture.store.AcknowledgeAlert(fixture.ctx, command)
			results <- err
		}()
	}
	close(start)
	pending.Wait()
	close(results)
	succeeded, conflicted := 0, 0
	for err := range results {
		if err == nil {
			succeeded++
		} else if errors.Is(err, domain.ErrRepositoryConflict) {
			conflicted++
		} else {
			t.Fatalf("unexpected concurrent acknowledgement: %v", err)
		}
	}
	changes, err := fixture.store.ListAlertChanges(fixture.ctx, seed.Rule.SourceID, position, 10)
	if succeeded != 1 || conflicted != 1 || err != nil || len(changes.Changes) != 1 {
		t.Fatal("concurrent acknowledgement did not commit exactly once")
	}
}

func TestRuleReplacementAndDeletionFenceLeasesAndResolvePendingEpisodes(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := durableDemoSeed(t, fixture)
	seed.Mutation.Alert.State = domain.AlertPending
	seed.Mutation.Transition.To = domain.AlertPending
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("seed pending episode")
	}
	at := time.Now().UTC().Truncate(time.Microsecond).Add(2 * time.Second)
	tasks, err := fixture.store.ClaimDueAlertEvaluations(fixture.ctx, at, 1)
	if err != nil || len(tasks) != 1 {
		t.Fatal("claim pending episode")
	}
	record, err := fixture.store.GetAlertRule(fixture.ctx, seed.Rule.ID)
	if err != nil || record.Version != 1 {
		t.Fatal("read pending rule")
	}
	record.Rule.DisplayName = "Changed health rule"
	record.Version = 2
	record.UpdatedAt = at.Add(time.Second)
	replace := domain.AlertRuleReplace{Record: record, ExpectedVersion: 1,
		Audit: alertCommandAudit("audit_alert_atomic_replace", record.UpdatedAt, "alert.rule_replace", "alert.rule", string(record.Rule.ID), "alert.rule_replaced")}
	if err := fixture.store.ReplaceAlertRule(fixture.ctx, replace); err != nil {
		t.Fatalf("replace pending rule: %v", err)
	}
	if err := fixture.store.CompleteAlertEvaluation(fixture.ctx, tasks[0], nil); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("rule replacement accepted earlier worker")
	}
	page, err := fixture.store.ListAlerts(fixture.ctx, seed.Rule.SourceID, domain.PageRequest{Limit: 10})
	if err != nil || len(page.Alerts) != 1 || page.Alerts[0].State != domain.AlertResolved {
		t.Fatal("rule replacement left earlier pending episode")
	}
	mutation := seed.Mutation
	mutation.Alert.ID = "alert_pending_second"
	mutation.Alert.StartedAt = record.UpdatedAt
	mutation.Alert.UpdatedAt = record.UpdatedAt
	transition := *mutation.Transition
	transition.ID, transition.AlertID, transition.OccurredAt = "transition_pending_second", mutation.Alert.ID, mutation.Alert.UpdatedAt
	mutation.Transition = &transition
	if err := fixture.store.SaveAlertMutation(fixture.ctx, mutation); err != nil {
		t.Fatalf("start new pending episode: %v", err)
	}
	tasks, err = fixture.store.ClaimDueAlertEvaluations(fixture.ctx, record.UpdatedAt.Add(time.Second), 1)
	if err != nil || len(tasks) != 1 || tasks[0].RuleVersion != 2 || tasks[0].Current == nil || tasks[0].Current.ID != mutation.Alert.ID {
		t.Fatal("new rule episode claim differs")
	}
	deletedAt := record.UpdatedAt.Add(2 * time.Second)
	command := domain.AlertRuleDelete{RuleID: record.Rule.ID, ExpectedVersion: 2, UpdatedByUserID: "user_alert_seed", DeletedAt: deletedAt,
		Audit: alertCommandAudit("audit_alert_atomic_delete", deletedAt, "alert.rule_delete", "alert.rule", string(record.Rule.ID), "alert.rule_deleted")}
	if err := fixture.store.DeleteAlertRule(fixture.ctx, command); err != nil {
		t.Fatalf("delete pending rule: %v", err)
	}
	if err := fixture.store.CompleteAlertEvaluation(fixture.ctx, tasks[0], nil); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("rule deletion accepted earlier worker")
	}
	page, err = fixture.store.ListAlerts(fixture.ctx, seed.Rule.SourceID, domain.PageRequest{Limit: 10})
	if err != nil || len(page.Alerts) != 2 || page.Alerts[0].State != domain.AlertResolved || page.Alerts[1].State != domain.AlertResolved {
		t.Fatal("rule deletion left active episode")
	}
	if _, err := fixture.store.GetAlertRule(fixture.ctx, record.Rule.ID); !errors.Is(err, domain.ErrRepositoryNotFound) {
		t.Fatal("deleted rule remains visible")
	}
}

func TestRuleAuditConflictRollsBackResolutionLeaseAndReplay(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("seed rule rollback")
	}
	at := time.Now().UTC().Truncate(time.Microsecond).Add(2 * time.Second)
	tasks, err := fixture.store.ClaimDueAlertEvaluations(fixture.ctx, at, 1)
	if err != nil || len(tasks) != 1 {
		t.Fatal("claim rule rollback")
	}
	record, err := fixture.store.GetAlertRule(fixture.ctx, seed.Rule.ID)
	if err != nil {
		t.Fatal("read rule rollback")
	}
	record.Version++
	record.Rule.Enabled = false
	record.UpdatedAt = at.Add(time.Second)
	command := domain.AlertRuleReplace{Record: record, ExpectedVersion: 1,
		Audit: alertCommandAudit("audit_alert_atomic_rule_conflict", record.UpdatedAt, "alert.rule_replace", "alert.rule", string(record.Rule.ID), "alert.rule_replaced")}
	if err := fixture.store.AppendAuditEntries(fixture.ctx, []domain.AuditEntry{command.Audit}); err != nil {
		t.Fatal("seed rule audit conflict")
	}
	position, err := fixture.store.AlertChangePosition(fixture.ctx, seed.Rule.SourceID)
	if err != nil {
		t.Fatal("read rule rollback position")
	}
	if err := fixture.store.ReplaceAlertRule(fixture.ctx, command); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatalf("rule audit conflict did not reject update: %v", err)
	}
	unchanged, err := fixture.store.GetAlertRule(fixture.ctx, seed.Rule.ID)
	if err != nil || unchanged.Version != 1 || !unchanged.Rule.Enabled {
		t.Fatal("failed audit committed rule update")
	}
	page, err := fixture.store.ListAlerts(fixture.ctx, seed.Rule.SourceID, domain.PageRequest{Limit: 10})
	if err != nil || len(page.Alerts) != 1 || page.Alerts[0].State != domain.AlertFiring {
		t.Fatal("failed audit committed episode resolution")
	}
	changes, err := fixture.store.ListAlertChanges(fixture.ctx, seed.Rule.SourceID, position, 10)
	if err != nil || len(changes.Changes) != 0 {
		t.Fatal("failed audit committed rule replay")
	}
	if err := fixture.store.CompleteAlertEvaluation(fixture.ctx, tasks[0], nil); err != nil {
		t.Fatalf("failed audit invalidated prior lease: %v", err)
	}
}

func TestExpiredAlertEvaluationLeaseCannotCompleteAndCanRecover(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("seed expired evaluation")
	}
	claimedAt := time.Now().UTC().Truncate(time.Microsecond).Add(-2 * time.Minute)
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE alert_rules SET next_evaluation_at_us=$1::bigint,created_at_us=$1::bigint-1000000 WHERE id=$2`, claimedAt.UnixMicro(), string(seed.Rule.ID)); err != nil {
		t.Fatalf("prepare expired evaluation: %v", err)
	}
	tasks, err := fixture.store.ClaimDueAlertEvaluations(fixture.ctx, claimedAt, 1)
	if err != nil || len(tasks) != 1 {
		t.Fatal("claim expired evaluation")
	}
	if err := fixture.store.CompleteAlertEvaluation(fixture.ctx, tasks[0], nil); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatal("expired lease accepted completion")
	}
	recovered, err := fixture.store.ClaimDueAlertEvaluations(fixture.ctx, time.Now().UTC().Truncate(time.Microsecond), 1)
	if err != nil || len(recovered) != 1 || recovered[0].ClaimGeneration <= tasks[0].ClaimGeneration {
		t.Fatal("expired evaluation lease did not recover")
	}
	if err := fixture.store.CompleteAlertEvaluation(fixture.ctx, recovered[0], nil); err != nil {
		t.Fatalf("recovered evaluation completion: %v", err)
	}
}

func TestSourceConfigurationChangesFenceAlertEvidenceCommit(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "changed_version"}[enabled], func(t *testing.T) {
			fixture := newObservationFixture(t)
			seed := durableDemoSeed(t, fixture)
			if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
				t.Fatal("seed source-fenced evaluation")
			}
			at := time.Now().UTC().Truncate(time.Microsecond).Add(2 * time.Second)
			tasks, err := fixture.store.ClaimDueAlertEvaluations(fixture.ctx, at, 1)
			if err != nil || len(tasks) != 1 || tasks[0].SourceConfigurationVersion != 1 {
				t.Fatal("claim source-fenced evaluation")
			}
			if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE infrastructure_sources SET row_version=row_version+1,enabled=$1 WHERE id=$2`, boolInteger(enabled), string(seed.Rule.SourceID)); err != nil {
				t.Fatal("change source during evaluation")
			}
			if err := fixture.store.CompleteAlertEvaluation(fixture.ctx, tasks[0], nil); !errors.Is(err, domain.ErrRepositoryConflict) {
				t.Fatal("source change accepted prior evidence")
			}
		})
	}
}

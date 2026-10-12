package alerting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type managementRepositoryStub struct {
	command        domain.AlertAcknowledgement
	createCommand  domain.AlertRuleCreate
	replaceCommand domain.AlertRuleReplace
	deleteCommand  domain.AlertRuleDelete
	currentRule    domain.AlertRuleRecord
	alert          domain.Alert
	err            error
}

func (stub *managementRepositoryStub) AcknowledgeAlert(_ context.Context, command domain.AlertAcknowledgement) (domain.Alert, error) {
	stub.command = command
	return stub.alert, stub.err
}
func (stub *managementRepositoryStub) ListAlertRules(context.Context, domain.PageRequest) (domain.AlertRulePage, error) {
	return domain.AlertRulePage{}, stub.err
}
func (stub *managementRepositoryStub) GetAlertRule(context.Context, domain.AlertRuleID) (domain.AlertRuleRecord, error) {
	return stub.currentRule, stub.err
}
func (stub *managementRepositoryStub) CreateAlertRule(_ context.Context, command domain.AlertRuleCreate) error {
	stub.createCommand = command
	return stub.err
}
func (stub *managementRepositoryStub) ReplaceAlertRule(_ context.Context, command domain.AlertRuleReplace) error {
	stub.replaceCommand = command
	return stub.err
}
func (stub *managementRepositoryStub) DeleteAlertRule(_ context.Context, command domain.AlertRuleDelete) error {
	stub.deleteCommand = command
	return stub.err
}

type managementIDsStub struct{}

func (managementIDsStub) NewTransitionID() (domain.AlertTransitionID, error) {
	return "transition_ack", nil
}
func (managementIDsStub) NewAuditID() (domain.AuditEntryID, error) { return "audit_ack", nil }
func (managementIDsStub) NewRuleID() (domain.AlertRuleID, error)   { return "rule_new", nil }

func TestManagementServiceBuildsAuditedAcknowledgement(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 20, 9, 0, 0, 0, time.UTC)
	repository := &managementRepositoryStub{alert: domain.Alert{ID: "alert_one"}}
	service, err := NewManagementService(repository, managementIDsStub{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Acknowledge(context.Background(), "alert_one", "user_admin", "request_ack"); err != nil {
		t.Fatalf("Acknowledge() error = %v", err)
	}
	command := repository.command
	if command.AlertID != "alert_one" || command.UserID != "user_admin" || command.TransitionID != "transition_ack" ||
		command.Audit.ID != "audit_ack" || command.Audit.Action != "alert.acknowledge" ||
		command.Audit.RequestID != "request_ack" || command.Validate() != nil {
		t.Fatalf("command = %#v; want valid bound audit", command)
	}
}

func TestManagementServiceMapsRepositoryState(t *testing.T) {
	t.Parallel()
	service, err := NewManagementService(
		&managementRepositoryStub{err: domain.ErrRepositoryConflict}, managementIDsStub{}, time.Now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Acknowledge(context.Background(), "alert_one", "user_admin", "request_ack")
	if !errors.Is(err, ErrAlertStateConflict) {
		t.Fatalf("Acknowledge() error = %v; want state conflict", err)
	}
}

func TestManagementServiceRejectsInvalidRulePage(t *testing.T) {
	t.Parallel()
	service, err := NewManagementService(&managementRepositoryStub{}, managementIDsStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ListRules(context.Background(), domain.PageRequest{Limit: 10})
	if !errors.Is(err, ErrManagementUnavailable) {
		t.Fatalf("ListRules() error = %v; want unavailable", err)
	}
}

func TestManagementServiceBuildsAuditedRuleLifecycle(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	comparison := domain.ComparisonGreaterThanOrEqual
	threshold := 80.0
	resourceID := domain.ResourceID("r1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	input := RuleInput{
		SourceID: "source_one", ResourceID: &resourceID, DisplayName: "High CPU",
		Kind: domain.AlertRuleCPUThreshold, Enabled: true, Severity: domain.SeverityWarning,
		Comparison: &comparison, ThresholdValue: &threshold, Window: 5 * time.Minute,
		EvaluationInterval: time.Minute, PendingFor: 2 * time.Minute,
	}
	repository := &managementRepositoryStub{}
	service, err := NewManagementService(repository, managementIDsStub{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateRule(context.Background(), input, "user_admin", "request_create")
	if err != nil {
		t.Fatalf("CreateRule() error = %v", err)
	}
	if created.Rule.ID != "rule_new" || repository.createCommand.Validate() != nil ||
		repository.createCommand.Audit.Action != "alert.rule_create" || repository.createCommand.Audit.RequestID != "request_create" {
		t.Fatalf("create command = %#v", repository.createCommand)
	}
	repository.currentRule = created
	replaced, err := service.ReplaceRule(context.Background(), "rule_new", 1, input, "user_admin", "request_replace")
	if err != nil {
		t.Fatalf("ReplaceRule() error = %v", err)
	}
	if replaced.Version != 2 || repository.replaceCommand.Validate() != nil ||
		repository.replaceCommand.Audit.Action != "alert.rule_replace" || repository.replaceCommand.Audit.RequestID != "request_replace" {
		t.Fatalf("replace command = %#v", repository.replaceCommand)
	}
	if err := service.DeleteRule(context.Background(), "rule_new", 2, "user_admin", "request_delete"); err != nil {
		t.Fatalf("DeleteRule() error = %v", err)
	}
	if repository.deleteCommand.Validate() != nil || repository.deleteCommand.Audit.Action != "alert.rule_delete" ||
		repository.deleteCommand.Audit.RequestID != "request_delete" {
		t.Fatalf("delete command = %#v", repository.deleteCommand)
	}
}

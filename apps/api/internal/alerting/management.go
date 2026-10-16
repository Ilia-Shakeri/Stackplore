package alerting

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

var (
	ErrManagementInvalid     = errors.New("alert management input is invalid")
	ErrAlertNotFound         = errors.New("alert was not found")
	ErrAlertStateConflict    = errors.New("alert state conflicts with the request")
	ErrManagementUnavailable = errors.New("alert management is unavailable")
)

type ManagementRepository interface {
	AcknowledgeAlert(context.Context, domain.AlertAcknowledgement) (domain.Alert, error)
	ListAlertRules(context.Context, domain.PageRequest) (domain.AlertRulePage, error)
	GetAlertRule(context.Context, domain.AlertRuleID) (domain.AlertRuleRecord, error)
	CreateAlertRule(context.Context, domain.AlertRuleCreate) error
	ReplaceAlertRule(context.Context, domain.AlertRuleReplace) error
	DeleteAlertRule(context.Context, domain.AlertRuleDelete) error
}

type ManagementIdentifierFactory interface {
	NewTransitionID() (domain.AlertTransitionID, error)
	NewAuditID() (domain.AuditEntryID, error)
	NewRuleID() (domain.AlertRuleID, error)
}

type RuleInput struct {
	SourceID           domain.SourceID
	EnvironmentID      *domain.EnvironmentID
	ResourceID         *domain.ResourceID
	DisplayName        string
	Kind               domain.AlertRuleKind
	Enabled            bool
	Severity           domain.Severity
	Comparison         *domain.ComparisonOperator
	ThresholdValue     *float64
	ThresholdCount     *uint64
	Window             time.Duration
	MinimumSeverity    *domain.Severity
	EvaluationInterval time.Duration
	PendingFor         time.Duration
}

type ManagementService struct {
	repository  ManagementRepository
	identifiers ManagementIdentifierFactory
	now         func() time.Time
}

func NewManagementService(repository ManagementRepository, identifiers ManagementIdentifierFactory, now func() time.Time) (*ManagementService, error) {
	if repository == nil || identifiers == nil || now == nil {
		return nil, ErrManagementInvalid
	}
	return &ManagementService{repository: repository, identifiers: identifiers, now: now}, nil
}

func (service *ManagementService) Acknowledge(ctx context.Context, alertID domain.AlertID, userID domain.UserID, requestID domain.RequestID) (domain.Alert, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || alertID.Validate() != nil || userID.Validate() != nil || requestID.Validate() != nil {
		return domain.Alert{}, ErrManagementInvalid
	}
	now := service.now().UTC().Truncate(time.Microsecond)
	transitionID, err := service.identifiers.NewTransitionID()
	if err != nil {
		return domain.Alert{}, ErrManagementUnavailable
	}
	auditID, err := service.identifiers.NewAuditID()
	if err != nil {
		return domain.Alert{}, ErrManagementUnavailable
	}
	command := domain.AlertAcknowledgement{
		AlertID: alertID, UserID: userID, TransitionID: transitionID, OccurredAt: now,
		Audit: domain.AuditEntry{
			ID: auditID, OccurredAt: now,
			Actor:  domain.AuditActor{Kind: domain.AuditActorUser, ID: string(userID)},
			Action: "alert.acknowledge", Target: domain.AuditTarget{Kind: "alert.instance", ID: string(alertID)},
			Outcome: domain.AuditSuccess, RequestID: requestID, ReasonCode: "alert.acknowledged",
		},
	}
	if command.Validate() != nil {
		return domain.Alert{}, ErrManagementUnavailable
	}
	alert, err := service.repository.AcknowledgeAlert(ctx, command)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrRepositoryNotFound):
			return domain.Alert{}, ErrAlertNotFound
		case errors.Is(err, domain.ErrRepositoryConflict):
			return domain.Alert{}, ErrAlertStateConflict
		case errors.Is(err, domain.ErrRepositoryInvalidInput):
			return domain.Alert{}, ErrManagementInvalid
		default:
			return domain.Alert{}, ErrManagementUnavailable
		}
	}
	if alert.Validate() != nil || alert.ID != alertID || alert.State != domain.AlertAcknowledged ||
		alert.AcknowledgedAt == nil || !alert.AcknowledgedAt.Equal(now) {
		return domain.Alert{}, ErrManagementUnavailable
	}
	return alert, nil
}

func (service *ManagementService) ListRules(ctx context.Context, request domain.PageRequest) (domain.AlertRulePage, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || request.Validate() != nil {
		return domain.AlertRulePage{}, ErrManagementInvalid
	}
	page, err := service.repository.ListAlertRules(ctx, request)
	if err != nil {
		return domain.AlertRulePage{}, mapManagementError(err)
	}
	if page.ValidateFor(request) != nil {
		return domain.AlertRulePage{}, ErrManagementUnavailable
	}
	return page, nil
}

func (service *ManagementService) CreateRule(ctx context.Context, input RuleInput, userID domain.UserID, requestID domain.RequestID) (domain.AlertRuleRecord, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || userID.Validate() != nil || requestID.Validate() != nil {
		return domain.AlertRuleRecord{}, ErrManagementInvalid
	}
	ruleID, err := service.identifiers.NewRuleID()
	if err != nil {
		return domain.AlertRuleRecord{}, ErrManagementUnavailable
	}
	rule := input.rule(ruleID)
	if rule.Validate() != nil {
		return domain.AlertRuleRecord{}, ErrManagementInvalid
	}
	now := service.now().UTC().Truncate(time.Microsecond)
	record := domain.AlertRuleRecord{
		Rule: rule, CreatedByUserID: userID, UpdatedByUserID: userID,
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}
	audit, err := service.ruleAudit(ruleID, userID, requestID, now, "alert.rule_create", "alert.rule_created")
	if err != nil {
		return domain.AlertRuleRecord{}, err
	}
	if err := service.repository.CreateAlertRule(ctx, domain.AlertRuleCreate{Record: record, Audit: audit}); err != nil {
		return domain.AlertRuleRecord{}, mapManagementError(err)
	}
	return record, nil
}

func (service *ManagementService) ReplaceRule(ctx context.Context, ruleID domain.AlertRuleID, expectedVersion int64, input RuleInput, userID domain.UserID, requestID domain.RequestID) (domain.AlertRuleRecord, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || ruleID.Validate() != nil || expectedVersion < 1 ||
		userID.Validate() != nil || requestID.Validate() != nil {
		return domain.AlertRuleRecord{}, ErrManagementInvalid
	}
	current, err := service.repository.GetAlertRule(ctx, ruleID)
	if err != nil {
		return domain.AlertRuleRecord{}, mapManagementError(err)
	}
	if current.Validate() != nil || current.Rule.ID != ruleID {
		return domain.AlertRuleRecord{}, ErrManagementUnavailable
	}
	if current.Version != expectedVersion {
		return domain.AlertRuleRecord{}, ErrAlertStateConflict
	}
	if input.SourceID != current.Rule.SourceID {
		return domain.AlertRuleRecord{}, ErrManagementInvalid
	}
	rule := input.rule(ruleID)
	if rule.Validate() != nil {
		return domain.AlertRuleRecord{}, ErrManagementInvalid
	}
	now := service.now().UTC().Truncate(time.Microsecond)
	if !now.After(current.UpdatedAt) {
		now = current.UpdatedAt.Add(time.Microsecond)
	}
	record := domain.AlertRuleRecord{
		Rule: rule, CreatedByUserID: current.CreatedByUserID, UpdatedByUserID: userID,
		CreatedAt: current.CreatedAt, UpdatedAt: now, Version: expectedVersion + 1,
	}
	audit, err := service.ruleAudit(ruleID, userID, requestID, now, "alert.rule_replace", "alert.rule_replaced")
	if err != nil {
		return domain.AlertRuleRecord{}, err
	}
	if err := service.repository.ReplaceAlertRule(ctx, domain.AlertRuleReplace{Record: record, ExpectedVersion: expectedVersion, Audit: audit}); err != nil {
		return domain.AlertRuleRecord{}, mapManagementError(err)
	}
	return record, nil
}

func (service *ManagementService) DeleteRule(ctx context.Context, ruleID domain.AlertRuleID, expectedVersion int64, userID domain.UserID, requestID domain.RequestID) error {
	if service == nil || ctx == nil || ctx.Err() != nil || ruleID.Validate() != nil || expectedVersion < 1 ||
		userID.Validate() != nil || requestID.Validate() != nil {
		return ErrManagementInvalid
	}
	now := service.now().UTC().Truncate(time.Microsecond)
	audit, err := service.ruleAudit(ruleID, userID, requestID, now, "alert.rule_delete", "alert.rule_deleted")
	if err != nil {
		return err
	}
	if err := service.repository.DeleteAlertRule(ctx, domain.AlertRuleDelete{
		RuleID: ruleID, UpdatedByUserID: userID, ExpectedVersion: expectedVersion, DeletedAt: now, Audit: audit,
	}); err != nil {
		return mapManagementError(err)
	}
	return nil
}

func (input RuleInput) rule(id domain.AlertRuleID) domain.AlertRule {
	return domain.AlertRule{
		ID: id, SourceID: input.SourceID, EnvironmentID: input.EnvironmentID, ResourceID: input.ResourceID,
		DisplayName: input.DisplayName, Kind: input.Kind, Enabled: input.Enabled, Severity: input.Severity,
		Comparison: input.Comparison, ThresholdValue: input.ThresholdValue, ThresholdCount: input.ThresholdCount,
		Window: input.Window, MinimumSeverity: input.MinimumSeverity,
		EvaluationInterval: input.EvaluationInterval, PendingFor: input.PendingFor,
	}
}

func (service *ManagementService) ruleAudit(ruleID domain.AlertRuleID, userID domain.UserID, requestID domain.RequestID, at time.Time, action domain.AuditAction, reason domain.ReasonCode) (domain.AuditEntry, error) {
	auditID, err := service.identifiers.NewAuditID()
	if err != nil {
		return domain.AuditEntry{}, ErrManagementUnavailable
	}
	return domain.AuditEntry{
		ID: auditID, OccurredAt: at, Actor: domain.AuditActor{Kind: domain.AuditActorUser, ID: string(userID)},
		Action: action, Target: domain.AuditTarget{Kind: "alert.rule", ID: string(ruleID)},
		Outcome: domain.AuditSuccess, RequestID: requestID, ReasonCode: reason,
	}, nil
}

func mapManagementError(err error) error {
	switch {
	case errors.Is(err, domain.ErrRepositoryNotFound):
		return ErrAlertNotFound
	case errors.Is(err, domain.ErrRepositoryConflict):
		return ErrAlertStateConflict
	case errors.Is(err, domain.ErrRepositoryInvalidInput):
		return ErrManagementInvalid
	default:
		return ErrManagementUnavailable
	}
}

func (SecureIdentifierFactory) NewAuditID() (domain.AuditEntryID, error) {
	value, err := secureIdentifier("audit_")
	return domain.AuditEntryID(value), err
}

func (SecureIdentifierFactory) NewRuleID() (domain.AlertRuleID, error) {
	value, err := secureIdentifier("rule_")
	return domain.AlertRuleID(value), err
}

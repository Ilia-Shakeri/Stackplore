package domain

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type AlertRuleKind string

const (
	AlertRuleResourceUnavailable         AlertRuleKind = "resource_unavailable"
	AlertRuleResourceUnhealthy           AlertRuleKind = "resource_unhealthy"
	AlertRuleRestartThreshold            AlertRuleKind = "restart_threshold"
	AlertRuleCPUThreshold                AlertRuleKind = "cpu_threshold"
	AlertRuleMemoryThreshold             AlertRuleKind = "memory_threshold"
	AlertRuleWorkloadUnavailableReplicas AlertRuleKind = "workload_unavailable_replicas"
	AlertRuleEventSeverityMatch          AlertRuleKind = "event_severity_match"
	AlertRuleProviderConnectionLost      AlertRuleKind = "provider_connection_lost"
)

func (kind AlertRuleKind) Validate() error {
	switch kind {
	case AlertRuleResourceUnavailable, AlertRuleResourceUnhealthy, AlertRuleRestartThreshold,
		AlertRuleCPUThreshold, AlertRuleMemoryThreshold, AlertRuleWorkloadUnavailableReplicas,
		AlertRuleEventSeverityMatch, AlertRuleProviderConnectionLost:
		return nil
	default:
		return fmt.Errorf("unknown alert rule kind %q", kind)
	}
}

type ComparisonOperator string

const (
	ComparisonGreaterThan        ComparisonOperator = "greater_than"
	ComparisonGreaterThanOrEqual ComparisonOperator = "greater_than_or_equal"
	ComparisonLessThan           ComparisonOperator = "less_than"
	ComparisonLessThanOrEqual    ComparisonOperator = "less_than_or_equal"
	ComparisonEqual              ComparisonOperator = "equal"
)

func (operator ComparisonOperator) Validate() error {
	switch operator {
	case ComparisonGreaterThan, ComparisonGreaterThanOrEqual, ComparisonLessThan,
		ComparisonLessThanOrEqual, ComparisonEqual:
		return nil
	default:
		return fmt.Errorf("unknown comparison operator %q", operator)
	}
}

type AlertRule struct {
	ID                 AlertRuleID
	SourceID           SourceID
	EnvironmentID      *EnvironmentID
	ResourceID         *ResourceID
	DisplayName        string
	Kind               AlertRuleKind
	Enabled            bool
	Severity           Severity
	Comparison         *ComparisonOperator
	ThresholdValue     *float64
	ThresholdCount     *uint64
	Window             time.Duration
	MinimumSeverity    *Severity
	EvaluationInterval time.Duration
	PendingFor         time.Duration
}

type AlertRuleRecord struct {
	Rule            AlertRule
	CreatedByUserID UserID
	UpdatedByUserID UserID
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Version         int64
}

func (record AlertRuleRecord) Validate() error {
	if record.Rule.Validate() != nil || record.CreatedByUserID.Validate() != nil || record.UpdatedByUserID.Validate() != nil ||
		validateUTCTime("alert rule creation time", record.CreatedAt) != nil ||
		validateUTCTime("alert rule update time", record.UpdatedAt) != nil ||
		record.UpdatedAt.Before(record.CreatedAt) || record.Version < 1 {
		return fmt.Errorf("stored alert rule is invalid")
	}
	return nil
}

type AlertRulePage struct {
	Rules []AlertRuleRecord
	Page  PageInfo
}

func (page AlertRulePage) ValidateFor(request PageRequest) error {
	if page.Page.ValidateFor(request) != nil || len(page.Rules) > request.Limit {
		return fmt.Errorf("alert rule page is invalid")
	}
	seen := make(map[AlertRuleID]struct{}, len(page.Rules))
	for index, record := range page.Rules {
		if record.Validate() != nil {
			return fmt.Errorf("alert rule page has invalid rule")
		}
		if _, exists := seen[record.Rule.ID]; exists {
			return fmt.Errorf("alert rule page has duplicate rule")
		}
		if index > 0 {
			previous := page.Rules[index-1]
			if previous.UpdatedAt.Before(record.UpdatedAt) ||
				(previous.UpdatedAt.Equal(record.UpdatedAt) && strings.Compare(string(previous.Rule.ID), string(record.Rule.ID)) > 0) {
				return fmt.Errorf("alert rule page order is invalid")
			}
		}
		seen[record.Rule.ID] = struct{}{}
	}
	return nil
}

type AlertRuleCreate struct {
	Record AlertRuleRecord
	Audit  AuditEntry
}

func (command AlertRuleCreate) Validate() error {
	if command.Record.Validate() != nil || command.Record.CreatedAt != command.Record.UpdatedAt || command.Record.Version != 1 ||
		command.Record.CreatedByUserID != command.Record.UpdatedByUserID ||
		!alertRuleAuditMatches(command.Audit, command.Record.Rule.ID, command.Record.UpdatedByUserID, command.Record.UpdatedAt, "alert.rule_create", "alert.rule_created") {
		return fmt.Errorf("alert rule create is invalid")
	}
	return nil
}

type AlertRuleReplace struct {
	Record          AlertRuleRecord
	ExpectedVersion int64
	Audit           AuditEntry
}

func (command AlertRuleReplace) Validate() error {
	if command.Record.Validate() != nil || command.ExpectedVersion < 1 || command.Record.Version != command.ExpectedVersion+1 ||
		!command.Record.UpdatedAt.After(command.Record.CreatedAt) ||
		!alertRuleAuditMatches(command.Audit, command.Record.Rule.ID, command.Record.UpdatedByUserID, command.Record.UpdatedAt, "alert.rule_replace", "alert.rule_replaced") {
		return fmt.Errorf("alert rule replacement is invalid")
	}
	return nil
}

type AlertRuleDelete struct {
	RuleID          AlertRuleID
	UpdatedByUserID UserID
	ExpectedVersion int64
	DeletedAt       time.Time
	Audit           AuditEntry
}

func (command AlertRuleDelete) Validate() error {
	if command.RuleID.Validate() != nil || command.UpdatedByUserID.Validate() != nil || command.ExpectedVersion < 1 ||
		validateUTCTime("alert rule deletion time", command.DeletedAt) != nil ||
		!alertRuleAuditMatches(command.Audit, command.RuleID, command.UpdatedByUserID, command.DeletedAt, "alert.rule_delete", "alert.rule_deleted") {
		return fmt.Errorf("alert rule deletion is invalid")
	}
	return nil
}

func alertRuleAuditMatches(audit AuditEntry, ruleID AlertRuleID, userID UserID, at time.Time, action AuditAction, reason ReasonCode) bool {
	return audit.Validate() == nil && audit.OccurredAt == at && audit.Actor.Kind == AuditActorUser &&
		audit.Actor.ID == string(userID) && audit.Action == action && audit.Target.Kind == "alert.rule" &&
		audit.Target.ID == string(ruleID) && audit.Outcome == AuditSuccess && audit.ReasonCode == reason
}

func (rule AlertRule) Validate() error {
	if err := rule.ID.Validate(); err != nil {
		return err
	}
	if err := rule.SourceID.Validate(); err != nil {
		return err
	}
	if rule.EnvironmentID != nil {
		if err := rule.EnvironmentID.Validate(); err != nil {
			return err
		}
	}
	if rule.ResourceID != nil {
		if err := rule.ResourceID.Validate(); err != nil {
			return err
		}
	}
	if err := validateDisplayText("alert rule name", rule.DisplayName, 256, false); err != nil {
		return err
	}
	if err := rule.Kind.Validate(); err != nil {
		return err
	}
	if err := rule.Severity.Validate(); err != nil {
		return err
	}
	if rule.EvaluationInterval < time.Second || rule.EvaluationInterval > 24*time.Hour ||
		rule.EvaluationInterval%time.Second != 0 {
		return fmt.Errorf("alert evaluation interval is outside its allowed range")
	}
	if rule.PendingFor < 0 || rule.PendingFor > 24*time.Hour || rule.PendingFor%time.Second != 0 {
		return fmt.Errorf("alert pending duration is outside its allowed range")
	}
	if rule.ThresholdValue != nil && (math.IsNaN(*rule.ThresholdValue) || math.IsInf(*rule.ThresholdValue, 0) ||
		*rule.ThresholdValue < 0 || *rule.ThresholdValue > 1<<53-1) {
		return fmt.Errorf("alert threshold value must be finite and non-negative")
	}
	if rule.ThresholdCount != nil && (*rule.ThresholdCount == 0 || *rule.ThresholdCount > uint64(1<<63-1)) {
		return fmt.Errorf("alert threshold count must be positive")
	}
	if rule.Comparison != nil {
		if err := rule.Comparison.Validate(); err != nil {
			return err
		}
	}
	if rule.MinimumSeverity != nil {
		if err := rule.MinimumSeverity.Validate(); err != nil {
			return err
		}
	}
	return rule.validateShape()
}

func (rule AlertRule) validateShape() error {
	noThreshold := rule.Comparison == nil && rule.ThresholdValue == nil && rule.ThresholdCount == nil &&
		rule.Window == 0 && rule.MinimumSeverity == nil
	switch rule.Kind {
	case AlertRuleResourceUnavailable, AlertRuleResourceUnhealthy:
		if rule.ResourceID == nil || !noThreshold {
			return fmt.Errorf("resource health rule has invalid fields")
		}
	case AlertRuleCPUThreshold, AlertRuleMemoryThreshold:
		if rule.ResourceID == nil || rule.Comparison == nil || rule.ThresholdValue == nil ||
			rule.ThresholdCount != nil || !validAlertWindow(rule.Window) || rule.MinimumSeverity != nil {
			return fmt.Errorf("metric threshold rule has invalid fields")
		}
	case AlertRuleRestartThreshold, AlertRuleWorkloadUnavailableReplicas:
		if rule.ResourceID == nil || rule.Comparison == nil || rule.ThresholdValue != nil ||
			rule.ThresholdCount == nil || !validAlertWindow(rule.Window) || rule.MinimumSeverity != nil {
			return fmt.Errorf("count threshold rule has invalid fields")
		}
	case AlertRuleEventSeverityMatch:
		if rule.Comparison != nil || rule.ThresholdValue != nil || rule.ThresholdCount != nil ||
			rule.Window != 0 || rule.MinimumSeverity == nil {
			return fmt.Errorf("event severity rule has invalid fields")
		}
	case AlertRuleProviderConnectionLost:
		if rule.EnvironmentID != nil || rule.ResourceID != nil || !noThreshold {
			return fmt.Errorf("provider connection rule has invalid fields")
		}
	}
	return nil
}

func validAlertWindow(value time.Duration) bool {
	return value >= time.Second && value <= 24*time.Hour && value%time.Second == 0
}

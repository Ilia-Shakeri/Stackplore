package alerting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func releaseRuleInput() RuleInput {
	rule := fixtureRule(0)
	return RuleInput{SourceID: rule.SourceID, ResourceID: rule.ResourceID, DisplayName: rule.DisplayName,
		Kind: rule.Kind, Enabled: true, Severity: rule.Severity, EvaluationInterval: rule.EvaluationInterval}
}

func TestManagementRepositoryFailuresPreservePublicErrorBoundary(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name            string
		repositoryError error
		want            error
	}{
		{"missing", domain.ErrRepositoryNotFound, ErrAlertNotFound},
		{"conflict", domain.ErrRepositoryConflict, ErrAlertStateConflict},
		{"invalid", domain.ErrRepositoryInvalidInput, ErrManagementInvalid},
		{"unavailable", errors.New("internal storage detail"), ErrManagementUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
			repository := &managementRepositoryStub{err: test.repositoryError}
			service, err := NewManagementService(repository, managementIDsStub{}, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			operations := map[string]func() error{
				"list": func() error {
					_, err := service.ListRules(context.Background(), domain.PageRequest{Limit: 10})
					return err
				},
				"create": func() error {
					_, err := service.CreateRule(context.Background(), releaseRuleInput(), "user_admin", "request_create")
					return err
				},
				"replace": func() error {
					_, err := service.ReplaceRule(context.Background(), "rule_one", 1, releaseRuleInput(), "user_admin", "request_replace")
					return err
				},
				"delete": func() error {
					return service.DeleteRule(context.Background(), "rule_one", 1, "user_admin", "request_delete")
				},
				"acknowledge": func() error {
					_, err := service.Acknowledge(context.Background(), "alert_one", "user_admin", "request_ack")
					return err
				},
			}
			for name, operation := range operations {
				if got := operation(); got != test.want {
					t.Errorf("%s error = %v; want fixed public error %v", name, got, test.want)
				}
			}
		})
	}
}

type unavailableManagementIdentifiers struct{ transition, audit, rule bool }

func (ids unavailableManagementIdentifiers) NewTransitionID() (domain.AlertTransitionID, error) {
	if ids.transition {
		return "", errors.New("identifier unavailable")
	}
	return "transition_ack", nil
}
func (ids unavailableManagementIdentifiers) NewAuditID() (domain.AuditEntryID, error) {
	if ids.audit {
		return "", errors.New("identifier unavailable")
	}
	return "audit_ack", nil
}
func (ids unavailableManagementIdentifiers) NewRuleID() (domain.AlertRuleID, error) {
	if ids.rule {
		return "", errors.New("identifier unavailable")
	}
	return "rule_new", nil
}

func TestManagementIdentifierFailureNeverCreatesUnauditedWrite(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	for _, ids := range []unavailableManagementIdentifiers{{transition: true}, {audit: true}} {
		repository := &managementRepositoryStub{}
		service, err := NewManagementService(repository, ids, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Acknowledge(context.Background(), "alert_one", "user_admin", "request_ack"); err != ErrManagementUnavailable || repository.command.AlertID != "" {
			t.Fatalf("acknowledgement without identifier = %v; write must be absent", err)
		}
	}
	for _, ids := range []unavailableManagementIdentifiers{{rule: true}, {audit: true}} {
		repository := &managementRepositoryStub{}
		service, err := NewManagementService(repository, ids, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.CreateRule(context.Background(), releaseRuleInput(), "user_admin", "request_create"); err != ErrManagementUnavailable || repository.createCommand.Record.Rule.ID != "" {
			t.Fatalf("rule creation without identifier = %v; write must be absent", err)
		}
	}
}

func TestSecureAlertIdentifiersAreValidDistinctAndDomainSeparated(t *testing.T) {
	t.Parallel()
	factory := SecureIdentifierFactory{}
	seen := make(map[string]bool)
	for range 32 {
		alertID, err := factory.NewAlertID()
		if err != nil || alertID.Validate() != nil {
			t.Fatal("alert identifier invalid")
		}
		transitionID, err := factory.NewTransitionID()
		if err != nil || transitionID.Validate() != nil {
			t.Fatal("transition identifier invalid")
		}
		notificationID, err := factory.NewNotificationID()
		if err != nil || notificationID.Validate() != nil {
			t.Fatal("notification identifier invalid")
		}
		auditID, err := factory.NewAuditID()
		if err != nil || auditID.Validate() != nil {
			t.Fatal("audit identifier invalid")
		}
		ruleID, err := factory.NewRuleID()
		if err != nil || ruleID.Validate() != nil {
			t.Fatal("rule identifier invalid")
		}
		for _, value := range []string{string(alertID), string(transitionID), string(notificationID), string(auditID), string(ruleID)} {
			if seen[value] {
				t.Fatal("identifier collision")
			}
			seen[value] = true
		}
	}
}

func TestWorkerRunReportsActiveFailureAndStopsWhenCancelled(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reports := 0
	worker := newTestWorker(t, &workerStoreStub{claimErr: errors.New("storage unavailable")}, evidenceStub{}, now, func(err error) {
		if !errors.Is(err, ErrEvaluationUnavailable) {
			t.Errorf("worker error lost public boundary: %v", err)
		}
		reports++
		cancel()
	})
	worker.Run(ctx)
	if reports != 1 {
		t.Fatalf("reported %d failures; want 1", reports)
	}
}

func TestWorkerRunPollsWithBoundAndSuppressesCancellationNoise(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	claims := 0
	store := callbackTaskStore{
		claim: func(context.Context, time.Time, int) ([]EvaluationTask, error) {
			claims++
			if claims == 2 {
				cancel()
				return nil, context.Canceled
			}
			return nil, nil
		},
		complete: func(context.Context, EvaluationTask, *domain.AlertMutation) error {
			t.Fatal("empty poll must not complete a task")
			return nil
		},
	}
	worker := newTestWorker(t, store, evidenceStub{}, now, func(err error) { t.Errorf("cancellation should not report failure: %v", err) })
	worker.pollInterval = minimumPollInterval
	worker.Run(ctx)
	if claims != 2 {
		t.Fatalf("polls = %d; want immediate poll and one bounded retry", claims)
	}
}

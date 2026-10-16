package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/alerting"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

func buildDemoAlertSeed(ctx context.Context, provider *demo.Provider) (domain.DemoAlertSeed, error) {
	fixtures := provider.Alerts()
	if len(fixtures) != 1 || fixtures[0].ResourceID == nil {
		return domain.DemoAlertSeed{}, errObservationStart
	}
	fixture := fixtures[0]
	rule := domain.AlertRule{ID: fixture.RuleID, SourceID: fixture.SourceID, EnvironmentID: fixture.EnvironmentID,
		ResourceID: fixture.ResourceID, DisplayName: "Demo checkout health", Kind: domain.AlertRuleResourceUnhealthy,
		Enabled: true, Severity: fixture.Severity, EvaluationInterval: 30 * time.Second}
	source := domain.ProviderRef{SourceID: provider.Source().ID, Provider: provider.Source().Provider}
	snapshot, err := provider.GetResource(ctx, domain.ResourceDetailQuery{Source: source, ResourceID: *fixture.ResourceID})
	if err != nil {
		return domain.DemoAlertSeed{}, err
	}
	health, err := provider.GetHealth(ctx, domain.HealthQuery{Source: source, Resources: []domain.ResourceRef{snapshot.Resource.Ref}})
	if err != nil || len(health.Snapshots) != 1 {
		return domain.DemoAlertSeed{}, errObservationStart
	}
	state := health.Snapshots[0].State
	at := provider.ObservationTime()
	condition, err := alerting.Evaluate(rule, alerting.EvaluationInput{SourceID: rule.SourceID, EnvironmentID: rule.EnvironmentID,
		ResourceID: rule.ResourceID, Health: &state, EvidenceRange: domain.TimeRange{Start: at.Add(-time.Microsecond), End: at}})
	if err != nil || !condition.Matched {
		return domain.DemoAlertSeed{}, errObservationStart
	}
	result, err := alerting.Advance(alerting.AdvanceRequest{Rule: rule, Condition: condition, CandidateID: fixture.ID, EvaluatedAt: at, Synthetic: true})
	if err != nil || result.Alert == nil || result.Alert.State != domain.AlertFiring {
		return domain.DemoAlertSeed{}, errObservationStart
	}
	alert := *result.Alert
	mutation := domain.AlertMutation{Alert: alert, DedupeKey: string(rule.ID), Transition: &domain.AlertTransition{
		ID: "transition_demo_checkout_seed", AlertID: alert.ID, To: alert.State, OccurredAt: at,
		Actor: domain.AuditActor{Kind: domain.AuditActorSystem}, ReasonCode: condition.SummaryCode, EvidenceRange: condition.EvidenceRange,
	}, Notification: &domain.Notification{ID: "notification_demo_checkout_seed", SourceID: &alert.SourceID, AlertID: &alert.ID,
		ResourceID: alert.ResourceID, Kind: "alert.firing", Severity: alert.Severity, SummaryCode: alert.SummaryCode, CreatedAt: at, Synthetic: true}}
	seed := domain.DemoAlertSeed{FixtureVersion: provider.FixtureVersion(), Rule: rule, Mutation: mutation}
	if seed.Validate() != nil {
		return domain.DemoAlertSeed{}, errObservationStart
	}
	return seed, nil
}

// Maintenance shares a bounded lifecycle with observation retention.
func (runtime observationRuntime) runRetention(ctx context.Context, logger *slog.Logger) {
	done := make(chan struct{})
	go func() { defer close(done); runtime.retention.Run(ctx) }()
	defer func() { <-done }()
	seedTicker := time.NewTicker(5 * time.Second)
	pruneTicker := time.NewTicker(time.Minute)
	defer seedTicker.Stop()
	defer pruneTicker.Stop()
	remaining := append([]domain.DemoAlertSeed(nil), runtime.demoSeeds...)
	seed := func() {
		for index := 0; index < len(remaining); {
			writeContext, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := runtime.alertHistory.EnsureDemoAlerts(writeContext, remaining[index])
			cancel()
			if err == nil {
				remaining = append(remaining[:index], remaining[index+1:]...)
				continue
			}
			if !errors.Is(err, domain.ErrRepositoryNotFound) && !errors.Is(err, domain.ErrRepositoryConflict) && ctx.Err() == nil {
				logger.Warn("demo alert seed delayed", "reason", "seed_unavailable")
			}
			index++
		}
	}
	seed()
	for {
		select {
		case <-ctx.Done():
			return
		case <-seedTicker.C:
			seed()
		case <-pruneTicker.C:
			writeContext, cancel := context.WithTimeout(ctx, 10*time.Second)
			_, err := runtime.alertHistory.PruneAlertHistory(writeContext, time.Now().UTC().Add(-7*24*time.Hour).Truncate(time.Microsecond), 1000)
			cancel()
			if err != nil && ctx.Err() == nil {
				logger.Warn("alert history retention delayed", "reason", "bounded_retention_failed")
			}
		}
	}
}

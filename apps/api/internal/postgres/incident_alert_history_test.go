package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func seedIncidentTransitions(t *testing.T, fixture observationFixture) domain.DemoAlertSeed {
	t.Helper()
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("seed selected incident alert")
	}
	previous := seed.Mutation.Alert
	for index := 1; index <= 9; index++ {
		current := previous
		at := fixture.at.Add(time.Duration(index) * time.Second)
		current.UpdatedAt = at
		current.State, current.ResolvedAt = domain.AlertFiring, nil
		reason := domain.ReasonCode("alert.condition_matched")
		if index%2 == 1 {
			current.State, current.ResolvedAt = domain.AlertResolved, &at
			reason = "alert.condition_cleared"
		}
		from, expected := previous.State, previous.UpdatedAt
		mutation := domain.AlertMutation{Alert: current, DedupeKey: seed.Mutation.DedupeKey, ExpectedState: &from, ExpectedUpdatedAt: &expected,
			Transition: &domain.AlertTransition{ID: domain.AlertTransitionID(fmt.Sprintf("incident_transition_%02d", index)), AlertID: current.ID, From: &from, To: current.State, OccurredAt: at, Actor: domain.AuditActor{Kind: domain.AuditActorSystem}, ReasonCode: reason, EvidenceRange: current.EvidenceRange}}
		if err := fixture.store.SaveAlertMutation(fixture.ctx, mutation); err != nil {
			t.Fatal("seed bounded transition chronology", err)
		}
		previous = current
	}
	return seed
}

func TestIncidentAlertHistoryFiltersInSQLAndPreservesHalfOpenChronology(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := seedIncidentTransitions(t, fixture)
	query := domain.IncidentAlertHistoryQuery{Source: fixture.token.Source, Resource: fixture.batch.Resources[1].Resource.Ref, Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: fixture.at.Add(time.Minute)}, Limit: domain.MaximumIncidentHistory}
	history, err := fixture.store.GetIncidentAlertHistory(fixture.ctx, query)
	if err != nil || history.ValidateFor(query) != nil || len(history.Transitions) != query.Limit || !history.MoreAvailable || history.Status.Completeness != domain.CompletenessPartial || history.Transitions[0].ID != seed.Mutation.Transition.ID {
		t.Fatal("selected bounded history or partial coverage failed", err)
	}
	if history.Transitions[1].To != domain.AlertResolved || !history.Transitions[1].OccurredAt.Equal(fixture.at.Add(time.Second)) || history.Transitions[1].From == nil || *history.Transitions[1].From != domain.AlertFiring {
		t.Fatal("current alert state replaced an actual historical transition")
	}
	window := query
	window.Range = domain.TimeRange{Start: fixture.at.Add(time.Second), End: fixture.at.Add(2 * time.Second)}
	bounded, err := fixture.store.GetIncidentAlertHistory(fixture.ctx, window)
	if err != nil || len(bounded.Transitions) != 1 || bounded.MoreAvailable || bounded.Transitions[0].ID != "incident_transition_01" {
		t.Fatal("history widened its half-open selected window")
	}
	neighbor := query
	neighbor.Resource = fixture.batch.Resources[0].Resource.Ref
	empty, err := fixture.store.GetIncidentAlertHistory(fixture.ctx, neighbor)
	if err != nil || len(empty.Transitions) != 0 || empty.MoreAvailable {
		t.Fatal("source-wide or adjacent-resource history escaped SQL predicate")
	}
	foreign := query
	foreign.Source.SourceID = "foreign_source"
	if _, err := fixture.store.GetIncidentAlertHistory(fixture.ctx, foreign); !errors.Is(err, domain.ErrRepositoryInvalidInput) {
		t.Fatal("foreign resource/source binding reached a history read")
	}
}

func TestIncidentAlertHistoryStopsOnCurrentPolicyLossCancellationAndClosedStore(t *testing.T) {
	fixture := newObservationFixture(t)
	_ = seedIncidentTransitions(t, fixture)
	query := domain.IncidentAlertHistoryQuery{Source: fixture.token.Source, Resource: fixture.batch.Resources[1].Resource.Ref, Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: fixture.at.Add(time.Minute)}, Limit: domain.MaximumIncidentHistory}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE infrastructure_sources SET operator_policy_pending=true WHERE id=$1`, string(query.Source.SourceID)); err != nil {
		t.Fatal("set current policy fence")
	}
	history, err := fixture.store.GetIncidentAlertHistory(fixture.ctx, query)
	if err != nil || history.Status.State != domain.AvailabilityPermissionDenied || len(history.Transitions) != 0 || history.MoreAvailable {
		t.Fatal("policy loss materialized private transition history")
	}
	ctx, cancel := context.WithCancel(fixture.ctx)
	cancel()
	history, err = fixture.store.GetIncidentAlertHistory(ctx, query)
	if err == nil || len(history.Transitions) != 0 {
		t.Fatal("cancelled history read returned content")
	}
	if err := fixture.store.Close(); err != nil {
		t.Fatal("close owned history fixture")
	}
	if _, err := fixture.store.GetIncidentAlertHistory(fixture.ctx, query); !errors.Is(err, domain.ErrRepositoryUnavailable) {
		t.Fatal("closed repository accepted a history read")
	}
}

package contextbuilder

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestIncidentHistoryProjectionKeepsActualTimeAndHidesUnavailableCounts(t *testing.T) {
	request := requestFixture(t)
	request.Scope.Resources = request.Scope.Resources[:1]
	request.Resources, request.Health, request.Metrics = nil, nil, nil
	request.Events, request.Relationships = nil, nil
	resource := request.Scope.Resources[0]
	from := domain.AlertPending
	at := request.Scope.ObservedAt
	request.AlertHistory = &domain.IncidentAlertHistory{Status: request.Status, Transitions: []domain.IncidentAlertTransition{
		{ID: "private_transition", AlertID: "private_alert", ResourceID: resource.ID, From: &from, To: domain.AlertFiring, OccurredAt: at.Add(-time.Second), Synthetic: true},
	}}
	missing := request.Status
	missing.State, missing.ReasonCode, missing.ObservedAt = domain.AvailabilityUnavailable, "source.missing", at.Add(-2*time.Second)
	available := request.Status
	available.ObservedAt = at.Add(-3 * time.Second)
	request.WorkloadHistory = &domain.IncidentWorkloadHistory{Status: request.Status, Samples: []domain.WorkloadReplicaSnapshot{
		{Resource: resource, Desired: 3, Ready: 2, Status: available},
		{Resource: resource, Desired: 8, Ready: 7, Status: missing},
	}}
	bundle, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal("typed history projection failed", err)
	}
	items, err := bundle.Items()
	if err != nil || len(items) != 3 {
		t.Fatal("history items lost scoped references", err)
	}
	for _, item := range items {
		if strings.Contains(item.Text, "private_") || !json.Valid([]byte(item.Text)) {
			t.Fatal("private history identifier escaped remote projection")
		}
	}
	observations, err := bundle.Observations()
	if err != nil || len(observations) != 3 || observations[0].AlertFrom != domain.AlertPending || !observations[0].AlertHasFrom || observations[0].AlertTo != domain.AlertFiring || !observations[0].Time.Equal(at.Add(-time.Second)) || observations[0].TimeBasis != "occurred" {
		t.Fatal("alert transition lost its historical facts", err)
	}
	if !observations[1].WorkloadCounts || observations[1].Desired != 3 || observations[1].Ready != 2 || observations[1].TimeBasis != "sampled" || observations[2].WorkloadCounts || observations[2].Desired != 0 || observations[2].Ready != 0 {
		t.Fatal("missing sample exposed counts or actual sample changed")
	}
	from = domain.AlertResolved
	request.WorkloadHistory.Samples[0].Ready = 9
	again, err := bundle.Observations()
	if err != nil || again[0].AlertFrom != domain.AlertPending || again[1].Ready != 2 {
		t.Fatal("history projection retained mutable input")
	}
}

func TestIncidentHistoryProjectionRejectsForeignResourceAndWindow(t *testing.T) {
	request := requestFixture(t)
	request.Resources, request.Health, request.Metrics = nil, nil, nil
	request.Events, request.Relationships = nil, nil
	request.Scope.Resources = request.Scope.Resources[:1]
	request.AlertHistory = &domain.IncidentAlertHistory{Status: request.Status, Transitions: []domain.IncidentAlertTransition{
		{ID: "transition", AlertID: "alert", ResourceID: "foreign", To: domain.AlertFiring, OccurredAt: request.Scope.Range.End, Synthetic: true},
	}}
	if _, err := Build(context.Background(), request); err == nil {
		t.Fatal("foreign and out-of-window history projected")
	}
}

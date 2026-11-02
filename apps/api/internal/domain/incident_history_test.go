package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func incidentQuery(t *testing.T) IncidentAlertHistoryQuery {
	t.Helper()
	return IncidentAlertHistoryQuery{Source: fixtureProvider(), Resource: fixtureResourceRef(t, "private_incident_native"), Range: TimeRange{Start: fixtureTime().Add(-5 * time.Minute), End: fixtureTime().Add(5 * time.Minute)}, Limit: MaximumIncidentHistory}
}

func TestIncidentHistoryHasOneBoundedResourceWindowAndProtectedFormatting(t *testing.T) {
	query := incidentQuery(t)
	if query.Validate() != nil {
		t.Fatal("valid selected history query rejected")
	}
	for _, mutate := range []func(*IncidentAlertHistoryQuery){
		func(value *IncidentAlertHistoryQuery) { value.Limit = 0 },
		func(value *IncidentAlertHistoryQuery) { value.Limit++ },
		func(value *IncidentAlertHistoryQuery) { value.Source.SourceID = "other_source" },
		func(value *IncidentAlertHistoryQuery) { value.Resource.ID = "" },
		func(value *IncidentAlertHistoryQuery) { value.Range.End = value.Range.Start },
		func(value *IncidentAlertHistoryQuery) {
			value.Range.End = value.Range.Start.Add(24*time.Hour + time.Microsecond)
		},
		func(value *IncidentAlertHistoryQuery) { value.Range.Start = value.Range.Start.Add(time.Nanosecond) },
	} {
		changed := query
		mutate(&changed)
		if changed.Validate() == nil {
			t.Fatal("unbounded, foreign or noncanonical history query accepted")
		}
	}
	for _, value := range []any{query, IncidentAlertHistory{}} {
		encoded, err := json.Marshal(value)
		if err != nil || string(encoded) != "{}" || strings.Contains(fmt.Sprintf("%+v", value), "private_incident_native") {
			t.Fatal("ordinary history formatting disclosed private scope")
		}
	}
}

func TestIncidentHistoryPreservesActualTransitionsWithoutCurrentRuleMetadata(t *testing.T) {
	query := incidentQuery(t)
	from := AlertPending
	history := IncidentAlertHistory{Status: fixtureStatus(), Transitions: []IncidentAlertTransition{
		{ID: "transition_A", AlertID: "alert_first", ResourceID: query.Resource.ID, To: AlertPending, OccurredAt: query.Range.Start, Synthetic: true},
		{ID: "transition_B", AlertID: "alert_first", ResourceID: query.Resource.ID, From: &from, To: AlertFiring, OccurredAt: fixtureTime().Add(time.Second), Synthetic: true},
	}}
	// Local transitions can be newer than the last provider collection. Their
	// actual occurrence must not be replaced with that collection timestamp.
	if history.ValidateFor(query) != nil || !history.Transitions[1].OccurredAt.After(history.Status.ObservedAt) {
		t.Fatal("actual local transition time was rejected or altered")
	}
	for _, mutate := range []func(*IncidentAlertHistory){
		func(value *IncidentAlertHistory) { value.Transitions[0].ResourceID = "other_resource" },
		func(value *IncidentAlertHistory) { value.Transitions[0].ID = "" },
		func(value *IncidentAlertHistory) { value.Transitions[0].AlertID = "" },
		func(value *IncidentAlertHistory) { value.Transitions[0].To = "unknown" },
		func(value *IncidentAlertHistory) { value.Transitions[0].Synthetic = false },
		func(value *IncidentAlertHistory) { value.Transitions[0].OccurredAt = query.Range.End },
		func(value *IncidentAlertHistory) {
			value.Transitions[0].OccurredAt = query.Range.Start.Add(-time.Microsecond)
		},
		func(value *IncidentAlertHistory) {
			value.Transitions[0].OccurredAt = query.Range.Start.Add(time.Nanosecond)
		},
		func(value *IncidentAlertHistory) { value.Transitions[0].ID = value.Transitions[1].ID },
		func(value *IncidentAlertHistory) {
			value.Transitions[0], value.Transitions[1] = value.Transitions[1], value.Transitions[0]
		},
		func(value *IncidentAlertHistory) { value.MoreAvailable = true },
		func(value *IncidentAlertHistory) {
			value.Status.State, value.Status.ReasonCode = AvailabilityUnsupported, "source.unsupported"
		},
		func(value *IncidentAlertHistory) { value.Status.ObservedAt = time.Time{} },
	} {
		changed := history
		changed.Transitions = append([]IncidentAlertTransition{}, history.Transitions...)
		mutate(&changed)
		if changed.ValidateFor(query) == nil {
			t.Fatal("forged history scope, chronology, pagination or state accepted")
		}
	}
	same := AlertFiring
	history.Transitions[1].From = &same
	if history.ValidateFor(query) == nil {
		t.Fatal("non-transition accepted")
	}
	history.Transitions[1].From = &from
	history.MoreAvailable, history.Status.Completeness, history.Status.ReasonCode = true, CompletenessPartial, "source.alert_history_limited"
	if history.ValidateFor(query) != nil {
		t.Fatal("honest bounded partial history rejected")
	}
}

func TestIncidentHistoryRejectsDuplicateTieOrderAndNoDataFabrication(t *testing.T) {
	query := incidentQuery(t)
	history := IncidentAlertHistory{Status: fixtureStatus(), Transitions: []IncidentAlertTransition{
		{ID: "transition_A", AlertID: "alert_A", ResourceID: query.Resource.ID, To: AlertPending, OccurredAt: fixtureTime(), Synthetic: true},
		{ID: "transition_a", AlertID: "alert_a", ResourceID: query.Resource.ID, To: AlertPending, OccurredAt: fixtureTime(), Synthetic: true},
	}}
	if history.ValidateFor(query) != nil {
		t.Fatal("canonical mixed-case tie order rejected")
	}
	history.Transitions[0], history.Transitions[1] = history.Transitions[1], history.Transitions[0]
	if history.ValidateFor(query) == nil {
		t.Fatal("wrong equal-time order accepted")
	}
	history.Status.State, history.Status.ReasonCode = AvailabilityPermissionDenied, "source.denied"
	history.Transitions = nil
	if history.ValidateFor(query) != nil {
		t.Fatal("truthful denied empty history rejected")
	}
	history.MoreAvailable = true
	if history.ValidateFor(query) == nil {
		t.Fatal("denied history advertised another page")
	}
}

package domain

import (
	"testing"
	"time"
)

func TestHealthHistoryBindsAuthorityTimeOrderAndCompleteness(t *testing.T) {
	ref := fixtureResourceRef(t, "history")
	query := HealthHistoryQuery{Source: fixtureProvider(), Resource: ref, Range: TimeRange{Start: fixtureTime().Add(-time.Hour), End: fixtureTime().Add(time.Second)}, Limit: 2}
	first := HealthHistorySample{ID: "health_a", ResourceID: ref.ID, State: HealthUnknown, Status: fixtureStatus(), Synthetic: true}
	second := first
	second.ID = "health_b"
	second.Status.ObservedAt = fixtureTime().Add(-time.Second)
	result := HealthHistoryResult{Status: fixtureStatus(), Samples: []HealthHistorySample{first, second}}
	if err := result.ValidateFor(query); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*HealthHistoryResult){"wrong resource": func(r *HealthHistoryResult) { r.Samples[0].ResourceID = fixtureResourceRef(t, "other").ID }, "duplicates": func(r *HealthHistoryResult) { r.Samples[1] = r.Samples[0] }, "order": func(r *HealthHistoryResult) { r.Samples[0], r.Samples[1] = r.Samples[1], r.Samples[0] }, "future": func(r *HealthHistoryResult) { r.Samples[0].Status.ObservedAt = fixtureTime().Add(time.Second) }, "denied result": func(r *HealthHistoryResult) {
		r.Status.State = AvailabilityPermissionDenied
		r.Status.ReasonCode = "source.denied"
	}, "known partial health": func(r *HealthHistoryResult) {
		r.Samples[0].State = HealthHealthy
		r.Samples[0].Status.Completeness = CompletenessPartial
		r.Samples[0].Status.ReasonCode = "source.partial"
	}} {
		t.Run(name, func(t *testing.T) {
			r := result
			r.Samples = append([]HealthHistorySample(nil), result.Samples...)
			change(&r)
			requireInvalid(t, r.ValidateFor(query))
		})
	}
	query.Source.SourceID = "other"
	requireInvalid(t, result.ValidateFor(query))
	query.Source = fixtureProvider()
	query.Limit = 0
	requireInvalid(t, result.ValidateFor(query))
}

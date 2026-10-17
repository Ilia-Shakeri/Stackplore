package domain

import (
	"strings"
	"testing"
	"time"
)

func TestEventFiltersAreBoundedAndApplyToEveryPage(t *testing.T) {
	at := time.Unix(1700000000, 0).UTC()
	environment := EnvironmentID("env_shop")
	event := InfrastructureEvent{ID: "event_pool", SourceID: "source_demo", Provider: ProviderDemo, EnvironmentID: &environment, Kind: "demo.pool", Severity: SeverityError, ReasonCode: "demo.pool_full", Summary: "Pool full", OccurredAt: at, ObservedAt: at, Synthetic: true}
	query := EventQuery{Source: ProviderRef{Provider: ProviderDemo, SourceID: "source_demo"}, Range: TimeRange{Start: at.Add(-time.Hour), End: at.Add(time.Second)}, EnvironmentID: environment, Search: "POOL", Page: PageRequest{Limit: 1}}
	page := EventPage{Status: ObservationStatus{State: AvailabilityAvailable, Completeness: CompletenessComplete, ObservedAt: at}, Events: []InfrastructureEvent{event}, Page: PageInfo{Limit: 1}}
	if query.Validate() != nil || !query.MatchesTextAndEnvironment(event) || page.ValidateFor(query) != nil {
		t.Fatal("valid compound event filter rejected")
	}
	for _, search := range []string{"event_pool", "demo.pool_full", "demo.pool", "pool full"} {
		query.Search = search
		if !query.MatchesTextAndEnvironment(event) {
			t.Fatal("missing searchable event field")
		}
	}
	query.Search = "absent"
	if query.MatchesTextAndEnvironment(event) || page.ValidateFor(query) == nil {
		t.Fatal("unmatched search escaped page boundary")
	}
	query.Search = ""
	query.EnvironmentID = "env_other"
	if query.MatchesTextAndEnvironment(event) || page.ValidateFor(query) == nil {
		t.Fatal("unmatched environment escaped page boundary")
	}
	event.EnvironmentID = nil
	if query.MatchesTextAndEnvironment(event) {
		t.Fatal("unscoped event matched an environment")
	}
	query.EnvironmentID = ""
	if !query.MatchesTextAndEnvironment(event) {
		t.Fatal("optional filters rejected unscoped event")
	}
	for _, invalid := range []string{strings.Repeat("x", MaxSearchLength+1), "bad\nline", " padding "} {
		query.Search = invalid
		if query.Validate() == nil {
			t.Fatal("invalid event search accepted")
		}
	}
	query.Search = ""
	query.EnvironmentID = "bad/environment"
	if query.Validate() == nil {
		t.Fatal("invalid environment accepted")
	}
}

package incidentreport

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func reportScope(t *testing.T) Scope {
	t.Helper()
	source := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: "report_source"}
	identity, err := domain.NewResourceIdentity(source.Provider, source.SourceID, "report_environment", "demo.service", domain.Scope{Kind: domain.ScopeProject, Name: "report_project"}, "report_native")
	if err != nil {
		t.Fatal(err)
	}
	resource, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	return Scope{Source: source, ResourceID: resource.ID, ConfigurationVersion: 1, PolicyVersion: 1, Range: domain.TimeRange{Start: at.Add(-5 * time.Minute), End: at}, ObservedAt: at}
}

func observation(scope Scope, index int, kind string, at time.Time) contextbuilder.Observation {
	ref := contextbuilder.CitationReference{ID: fmt.Sprintf("ev_%064x", index), Kind: kind, ObjectID: fmt.Sprintf("fact_%d", index), Source: scope.Source, ConfigurationVersion: scope.ConfigurationVersion, PolicyVersion: scope.PolicyVersion, RestoreGeneration: scope.RestoreGeneration, ResourceID: scope.ResourceID, ObservedAt: scope.ObservedAt, Range: scope.Range}
	item := contextbuilder.Observation{Reference: ref, Time: at, Availability: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, Synthetic: true}
	switch kind {
	case "resource", "health":
		item.Reference.ObjectID, item.Reference.ObservedAt = string(ref.ResourceID), at
		item.TimeBasis, item.Health = "observed", domain.HealthHealthy
	case "metric":
		item.TimeBasis, item.MetricName, item.MetricUnit = "sampled", domain.MetricRestartCount, domain.MetricUnitCount
		item.IntervalSeconds = 1
	case "event":
		item.TimeBasis, item.EventSeverity = "occurred", domain.SeverityWarning
	case "log":
		item.TimeBasis, item.LogSeverity = "occurred", domain.LogSeverityInfo
	case "relationship":
		item.Reference.ResourceID, item.Reference.ObservedAt, item.TimeBasis = "", at, "observed"
	}
	return item
}

func TestReportRejectsForgedSyntheticOrigin(t *testing.T) {
	scope := reportScope(t)
	item := observation(scope, 1, "health", scope.ObservedAt)
	if _, err := Build(scope, []contextbuilder.Observation{item}); err != nil {
		t.Fatal("valid demo origin rejected", err)
	}
	item.Synthetic = false
	if _, err := Build(scope, []contextbuilder.Observation{item}); !errors.Is(err, ErrInvalid) {
		t.Fatal("demo observation misrepresented as real")
	}
	scope.Source.Provider = domain.ProviderDocker
	item.Reference.Source, item.Synthetic = scope.Source, false
	if _, err := Build(scope, []contextbuilder.Observation{item}); err != nil {
		t.Fatal("valid real origin rejected", err)
	}
	item.Synthetic = true
	if _, err := Build(scope, []contextbuilder.Observation{item}); !errors.Is(err, ErrInvalid) {
		t.Fatal("real observation misrepresented as demo")
	}
}

func TestHistoryCorrelationRequiresCoverageAndKeepsRecovery(t *testing.T) {
	scope := reportScope(t)
	alert := observation(scope, 1, "alert", scope.Range.Start.Add(time.Second))
	alert.TimeBasis, alert.AlertHasFrom, alert.AlertFrom, alert.AlertTo = "occurred", true, domain.AlertPending, domain.AlertFiring
	short := observation(scope, 2, "workload", alert.Time.Add(time.Second))
	short.TimeBasis, short.WorkloadCounts, short.Desired, short.Ready = "sampled", true, 3, 2
	recovered := observation(scope, 3, "workload", short.Time.Add(time.Second))
	recovered.TimeBasis, recovered.WorkloadCounts, recovered.Desired, recovered.Ready = "sampled", true, 3, 3
	input := []contextbuilder.Observation{recovered, alert, short}
	if _, err := Build(scope, input); !errors.Is(err, ErrInvalid) {
		t.Fatal("history without a readable family was accepted")
	}
	coverage := HistoryCoverage{AlertsRead: true, WorkloadRead: true}
	report, err := BuildWithHistory(scope, input, coverage)
	if err != nil || len(report.Timeline) != 3 || len(report.Hypotheses) != 1 || report.Hypotheses[0].Code != "workload_shortfall_possible" ||
		!slices.Equal(report.Hypotheses[0].Supporting, []string{short.Reference.ID}) || !slices.Equal(report.Hypotheses[0].Contrary, []string{recovered.Reference.ID}) ||
		slices.Contains(report.Gaps, "alerts_not_read") || slices.Contains(report.Gaps, "workload_history_not_read") {
		t.Fatal("qualified shortfall or recovery lost", err)
	}
	refs := []contextbuilder.CitationReference{alert.Reference, short.Reference, recovered.Reference}
	if ValidateWithHistory(scope, report, refs, coverage) != nil {
		t.Fatal("exact authorized history rejected")
	}
	if !errors.Is(Validate(scope, report, refs), ErrInvalid) {
		t.Fatal("legacy publication accepted unverified history coverage")
	}
	forged := report.Clone()
	forged.history.AlertsRead = false
	if !errors.Is(ValidateWithHistory(scope, forged, refs, coverage), ErrInvalid) {
		t.Fatal("history report lost its family coverage")
	}
	empty, err := BuildWithHistory(scope, nil, coverage)
	if err != nil || slices.Contains(empty.Gaps, "alerts_not_read") || slices.Contains(empty.Gaps, "workload_history_not_read") || len(empty.Hypotheses) != 0 {
		t.Fatal("empty readable history became unread or invented a cause")
	}
}

func TestHistoryCorrelationRejectsConflictAndUnavailableCounts(t *testing.T) {
	scope := reportScope(t)
	first := observation(scope, 1, "workload", scope.Range.Start.Add(time.Second))
	first.TimeBasis, first.WorkloadCounts, first.Desired, first.Ready = "sampled", true, 3, 2
	conflict := first
	conflict.Reference.ID, conflict.Reference.ObjectID, conflict.Ready = fmt.Sprintf("ev_%064x", 2), "fact_2", 3
	coverage := HistoryCoverage{WorkloadRead: true}
	report, err := BuildWithHistory(scope, []contextbuilder.Observation{first, conflict}, coverage)
	if err != nil || len(report.Hypotheses) != 0 || !slices.Contains(report.Gaps, "conflicting_samples") {
		t.Fatal("conflicting workload samples invented a diagnosis")
	}
	missing := first
	missing.Availability, missing.WorkloadCounts = domain.AvailabilityUnavailable, false
	if _, err := BuildWithHistory(scope, []contextbuilder.Observation{missing}, coverage); !errors.Is(err, ErrInvalid) {
		t.Fatal("unavailable sample kept fabricated counts")
	}
	missing.Desired, missing.Ready = 0, 0
	report, err = BuildWithHistory(scope, []contextbuilder.Observation{missing}, coverage)
	if err != nil || len(report.Hypotheses) != 0 || !slices.Contains(report.Gaps, "limited_evidence") {
		t.Fatal("unavailable sample did not remain an explicit gap")
	}
	for _, change := range []func(*contextbuilder.Observation){
		func(item *contextbuilder.Observation) { item.Availability = domain.AvailabilityStale },
		func(item *contextbuilder.Observation) { item.Completeness = domain.CompletenessPartial },
	} {
		limited := first
		change(&limited)
		report, err := BuildWithHistory(scope, []contextbuilder.Observation{limited}, coverage)
		if err != nil || len(report.Hypotheses) != 0 || !slices.Contains(report.Gaps, "limited_evidence") {
			t.Fatal("stale or partial sample produced a qualified shortfall")
		}
	}
}

func TestQualifiedRestartAndHealthKeepActualChronologyAndContraryEvidence(t *testing.T) {
	scope := reportScope(t)
	early := observation(scope, 1, "metric", scope.Range.Start.Add(time.Second))
	early.MetricValue = 3
	later := observation(scope, 2, "metric", early.Time.Add(time.Second))
	later.MetricValue = 5
	bad := observation(scope, 3, "resource", later.Time)
	bad.Health = domain.HealthUnhealthy
	recovered := observation(scope, 4, "health", scope.Range.End)
	input := []contextbuilder.Observation{recovered, later, bad, early}
	before := slices.Clone(input)
	report, err := Build(scope, input)
	if err != nil || len(report.Hypotheses) != 2 || report.Timeline[0] != early || !reflect.DeepEqual(input, before) {
		t.Fatal("bounded chronology, hypotheses or immutable input failed")
	}
	if report.Hypotheses[0].Code != "restart_related_possible" || !slices.Equal(report.Hypotheses[0].Supporting, []string{early.Reference.ID, later.Reference.ID}) ||
		report.Hypotheses[1].Code != "health_problem_possible" || !slices.Equal(report.Hypotheses[1].Contrary, []string{recovered.Reference.ID}) {
		t.Fatal("qualified support or later contrary evidence lost")
	}
	for index, hypothesis := range report.Hypotheses {
		if !hypothesis.Qualified || hypothesis.Rank != index+1 {
			t.Fatal("rank became a probability or unqualified cause")
		}
	}
	cloned := report.Clone()
	cloned.Timeline[0].Reference.ID = "changed"
	cloned.Hypotheses[0].Supporting[0] = "changed"
	cloned.Hypotheses[1].Contrary[0] = "changed"
	cloned.Gaps[0], cloned.NextReads[0] = "changed", "changed"
	if report.Timeline[0].Reference.ID == "changed" || report.Hypotheses[0].Supporting[0] == "changed" || report.Hypotheses[1].Contrary[0] == "changed" || report.Gaps[0] == "changed" || report.NextReads[0] == "changed" {
		t.Fatal("nested report clone retained mutable aliases")
	}
	encoded, err := json.Marshal(report)
	if err != nil || string(encoded) != "{}" || strings.Contains(fmt.Sprintf("%+v", report), "report_source") {
		t.Fatal("ordinary formatting exposed report material")
	}
	report.Timeline[0].Reference.ID = "changed"
	report.Hypotheses[0].Supporting[0] = "changed"
	again, err := Build(scope, input)
	if err != nil || again.Timeline[0].Reference.ID == "changed" || again.Hypotheses[0].Supporting[0] == "changed" {
		t.Fatal("report mutation changed original facts")
	}
}

func TestPressureNeedsMatchingPositiveLimitAndKeepsLaterContraryPair(t *testing.T) {
	scope := reportScope(t)
	usage := observation(scope, 1, "metric", scope.Range.Start)
	usage.MetricName, usage.MetricUnit, usage.MetricValue = domain.MetricMemoryUsageBytes, domain.MetricUnitBytes, 1024
	limit := observation(scope, 2, "metric", usage.Time)
	limit.MetricName, limit.MetricUnit, limit.MetricValue = domain.MetricMemoryLimitBytes, domain.MetricUnitBytes, 512
	low, laterLimit := usage, limit
	low.Reference.ID, laterLimit.Reference.ID = fmt.Sprintf("ev_%064x", 3), fmt.Sprintf("ev_%064x", 4)
	low.Reference.ObjectID, laterLimit.Reference.ObjectID = "fact_3", "fact_4"
	low.Time, laterLimit.Time, low.MetricValue = usage.Time.Add(time.Second), usage.Time.Add(time.Second), 128
	report, err := Build(scope, []contextbuilder.Observation{laterLimit, low, limit, usage})
	if err != nil || len(report.Hypotheses) != 1 || report.Hypotheses[0].Code != "memory_pressure_possible" || len(report.Hypotheses[0].Supporting) != 2 || !slices.Equal(report.Hypotheses[0].Contrary, []string{low.Reference.ID, laterLimit.Reference.ID}) {
		t.Fatal("matching pressure evidence or later contrary samples lost")
	}
	for _, change := range []func(*contextbuilder.Observation){
		func(item *contextbuilder.Observation) { item.MetricValue = 0 },
		func(item *contextbuilder.Observation) { item.Time = item.Time.Add(time.Second) },
		func(item *contextbuilder.Observation) { item.IntervalSeconds++ },
		func(item *contextbuilder.Observation) { item.Availability = domain.AvailabilityStale },
	} {
		other := limit
		change(&other)
		report, err := Build(scope, []contextbuilder.Observation{usage, other})
		if err != nil || len(report.Hypotheses) != 0 || !slices.Contains(report.Gaps, "matching_limits_missing") {
			t.Fatal("missing, zero, stale or unmatched limit invented pressure")
		}
	}
}

func TestCounterLifetimeConflictAndInsufficientEvidenceDoNotInventCause(t *testing.T) {
	scope := reportScope(t)
	first := observation(scope, 1, "metric", scope.Range.Start)
	first.MetricValue = 50
	second := observation(scope, 2, "metric", first.Time.Add(time.Second))
	second.MetricValue = 1
	report, err := Build(scope, []contextbuilder.Observation{second, first})
	if err != nil || len(report.Hypotheses) != 0 || !slices.Contains(report.Gaps, "restart_counter_reset") {
		t.Fatal("counter reset became a window restart delta")
	}
	second.Time = first.Time
	report, err = Build(scope, []contextbuilder.Observation{second, first})
	if err != nil || len(report.Hypotheses) != 0 || !slices.Contains(report.Gaps, "conflicting_samples") {
		t.Fatal("conflicting simultaneous samples became chronology")
	}
	event := observation(scope, 3, "event", scope.Range.Start)
	report, err = Build(scope, []contextbuilder.Observation{event})
	if err != nil || len(report.Hypotheses) != 0 || !slices.Contains(report.Gaps, "metrics_missing") || !slices.Contains(report.Gaps, "alerts_not_read") || !slices.Contains(report.Gaps, "cause_unproven") {
		t.Fatal("warning or absent metrics invented OOM, connectivity or cause")
	}
	report, err = Build(scope, nil)
	if err != nil || len(report.Timeline) != 0 || len(report.Hypotheses) != 0 {
		t.Fatal("empty input fabricated an observation")
	}
}

func TestRejectCrossScopeMalformedHiddenFieldsAndBudgets(t *testing.T) {
	scope := reportScope(t)
	base := observation(scope, 1, "metric", scope.Range.Start)
	for _, mutate := range []func(*contextbuilder.Observation){
		func(item *contextbuilder.Observation) { item.Reference.Source.SourceID = "other_source" },
		func(item *contextbuilder.Observation) { item.Reference.ConfigurationVersion++ },
		func(item *contextbuilder.Observation) { item.Reference.PolicyVersion++ },
		func(item *contextbuilder.Observation) { item.Reference.RestoreGeneration++ },
		func(item *contextbuilder.Observation) {
			item.Reference.Range.End = item.Reference.Range.End.Add(time.Second)
		},
		func(item *contextbuilder.Observation) { item.Reference.ResourceID = "" },
		func(item *contextbuilder.Observation) { item.Reference.ID = "ev_unknown" },
		func(item *contextbuilder.Observation) { item.Reference.ObservedAt = scope.ObservedAt.Add(time.Second) },
		func(item *contextbuilder.Observation) { item.Time = scope.Range.Start.Add(-time.Second) },
		func(item *contextbuilder.Observation) { item.TimeBasis = "observed" },
		func(item *contextbuilder.Observation) { item.MetricValue = math.NaN() },
		func(item *contextbuilder.Observation) { item.IntervalSeconds = math.MaxInt64 },
		func(item *contextbuilder.Observation) { item.Health = domain.HealthUnhealthy },
		func(item *contextbuilder.Observation) { item.Availability = domain.AvailabilityPermissionDenied },
	} {
		other := base
		mutate(&other)
		if _, err := Build(scope, []contextbuilder.Observation{other}); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid scope, hidden field or observation accepted")
		}
	}
	if _, err := Build(scope, []contextbuilder.Observation{base, base}); !errors.Is(err, ErrInvalid) {
		t.Fatal("duplicate reference accepted")
	}
	if _, err := Build(scope, make([]contextbuilder.Observation, MaximumObservations+1)); !errors.Is(err, ErrInvalid) {
		t.Fatal("unbounded report accepted")
	}
	bounded := make([]contextbuilder.Observation, MaximumObservations)
	for index := range bounded {
		bounded[index] = observation(scope, index+1, "metric", scope.Range.Start)
	}
	if report, err := Build(scope, bounded); err != nil || len(report.Timeline) != MaximumObservations {
		t.Fatal("exact observation bound rejected")
	}
}

func TestSimultaneousHealthConflictAndLimitedEvidenceRemainExplicit(t *testing.T) {
	scope := reportScope(t)
	good := observation(scope, 1, "health", scope.Range.End)
	bad := observation(scope, 2, "resource", good.Time)
	bad.Health = domain.HealthUnhealthy
	report, err := Build(scope, []contextbuilder.Observation{bad, good})
	if err != nil || len(report.Hypotheses) != 1 || !slices.Contains(report.Gaps, "conflicting_health") || !slices.Equal(report.Hypotheses[0].Contrary, []string{good.Reference.ID}) {
		t.Fatal("tie-break order suppressed simultaneous contrary health")
	}
	for _, limited := range []contextbuilder.Observation{
		func() contextbuilder.Observation {
			item := bad
			item.Availability = domain.AvailabilityStale
			return item
		}(),
		func() contextbuilder.Observation {
			item := bad
			item.Completeness = domain.CompletenessPartial
			return item
		}(),
	} {
		report, err := Build(scope, []contextbuilder.Observation{limited})
		if err != nil || len(report.Hypotheses) != 0 || !slices.Contains(report.Gaps, "limited_evidence") {
			t.Fatal("limited observation became a fresh complete diagnosis")
		}
	}
	bad.Time = scope.Range.Start.Add(-time.Second)
	bad.Reference.ObservedAt = bad.Time
	report, err = Build(scope, []contextbuilder.Observation{bad})
	if err != nil || len(report.Hypotheses) != 0 || !slices.Contains(report.Gaps, "out_of_window_observation") {
		t.Fatal("old health became a selected-window explanation")
	}
}

func TestIndependentReportValidationBindsClaimsAndExactReferences(t *testing.T) {
	scope := reportScope(t)
	first := observation(scope, 1, "metric", scope.Range.Start)
	second := observation(scope, 2, "metric", first.Time.Add(time.Second))
	second.MetricValue = 1
	report, err := Build(scope, []contextbuilder.Observation{first, second})
	if err != nil {
		t.Fatal(err)
	}
	refs := []contextbuilder.CitationReference{second.Reference, first.Reference}
	if Validate(scope, report, refs) != nil {
		t.Fatal("valid report rejected or reference order affected authority")
	}
	for _, mutate := range []func(*Report){
		func(value *Report) { value.Hypotheses[0].Qualified = false },
		func(value *Report) { value.Hypotheses[0].Rank = 2 },
		func(value *Report) { value.Hypotheses[0].Code = "oom_proven" },
		func(value *Report) { value.Hypotheses[0].Supporting[0] = "ev_unknown" },
		func(value *Report) { value.Hypotheses[0].Contrary = []string{first.Reference.ID} },
		func(value *Report) { value.Gaps = nil },
		func(value *Report) { value.NextReads[0] = "run_shell" },
		func(value *Report) {
			value.Timeline[0].Reference.ObservedAt = value.Timeline[0].Reference.ObservedAt.Add(-time.Second)
		},
		func(value *Report) { slices.Reverse(value.Timeline) },
		func(value *Report) { value.Timeline = value.Timeline[:1] },
	} {
		changed := report.Clone()
		mutate(&changed)
		if !errors.Is(Validate(scope, changed, refs), ErrInvalid) {
			t.Fatal("forged, reordered, missing or unqualified report accepted")
		}
	}
	if !errors.Is(Validate(scope, report, []contextbuilder.CitationReference{first.Reference, first.Reference}), ErrInvalid) {
		t.Fatal("duplicate authority set accepted")
	}
	second.Time = scope.Range.End
	if _, err := Build(scope, []contextbuilder.Observation{second}); !errors.Is(err, ErrInvalid) {
		t.Fatal("exclusive evidence end became a closed interval")
	}
}

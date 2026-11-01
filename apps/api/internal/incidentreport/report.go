// Package incidentreport correlates bounded typed observations without I/O.
package incidentreport

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const MaximumObservations = 64

var ErrInvalid = errors.New("incident_report.invalid")

// Scope contains only the already authorized selected identity and versions.
// Native provider handles and browser-selected query arguments are absent.
type Scope struct {
	Source               domain.ProviderRef
	ResourceID           domain.ResourceID
	ConfigurationVersion int64
	PolicyVersion        int64
	RestoreGeneration    int64
	Range                domain.TimeRange
	ObservedAt           time.Time
}

type Hypothesis struct {
	Code       string
	Rank       int
	Qualified  bool
	Supporting []string
	Contrary   []string
}

// Report is volatile internal material. Explicit authorized transport conversion
// must retain the parent scope and share the canonical answer's byte budget.
type Report struct {
	Timeline   []contextbuilder.Observation
	Hypotheses []Hypothesis
	Gaps       []string
	NextReads  []string
}

func (Report) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected incident report]")
}

func (Report) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (report Report) Clone() Report {
	report.Timeline = slices.Clone(report.Timeline)
	report.Gaps, report.NextReads = slices.Clone(report.Gaps), slices.Clone(report.NextReads)
	report.Hypotheses = slices.Clone(report.Hypotheses)
	for index := range report.Hypotheses {
		report.Hypotheses[index].Supporting = slices.Clone(report.Hypotheses[index].Supporting)
		report.Hypotheses[index].Contrary = slices.Clone(report.Hypotheses[index].Contrary)
	}
	return report
}

// Build grants no authority. The owning service must retain and freshly check
// every sealed read proof before invoking it and before any publication.
func Build(scope Scope, observations []contextbuilder.Observation) (Report, error) {
	if scope.Source.Validate() != nil || scope.ConfigurationVersion < 1 || scope.PolicyVersion < 1 || scope.RestoreGeneration < 0 || scope.ResourceID.Validate() != nil ||
		scope.Range.ValidateLookback(24*time.Hour) != nil || !timestamp(scope.ObservedAt) || scope.Range.End.After(scope.ObservedAt) || len(observations) > MaximumObservations {
		return Report{}, ErrInvalid
	}
	result := Report{Timeline: slices.Clone(observations), Hypotheses: []Hypothesis{},
		Gaps:      []string{"alerts_not_read", "workload_history_not_read", "cause_unproven"},
		NextReads: []string{"review_health", "review_metrics", "review_events", "review_alert_history", "review_workload_history"}}
	seen := make(map[string]bool, len(observations))
	for _, item := range observations {
		if !validObservation(scope, item) || seen[item.Reference.ID] {
			return Report{}, ErrInvalid
		}
		seen[item.Reference.ID] = true
		if item.Availability != domain.AvailabilityAvailable || item.Completeness != domain.CompletenessComplete {
			result.gap("limited_evidence")
		}
	}
	slices.SortFunc(result.Timeline, func(a, b contextbuilder.Observation) int {
		if order := a.Time.Compare(b.Time); order != 0 {
			return order
		}
		return strings.Compare(a.Reference.ID, b.Reference.ID)
	})
	result.pressure(domain.MetricMemoryUsageBytes, domain.MetricMemoryLimitBytes, "memory_pressure_possible")
	result.pressure(domain.MetricCPUUsageCores, domain.MetricCPULimitCores, "cpu_pressure_possible")
	result.restarts()
	result.health()
	if !slices.ContainsFunc(result.Timeline, func(item contextbuilder.Observation) bool { return item.Reference.Kind == "metric" }) {
		result.gap("metrics_missing")
	}
	if !slices.ContainsFunc(result.Timeline, func(item contextbuilder.Observation) bool { return item.Reference.Kind == "event" }) {
		result.gap("events_missing")
	}
	return result, nil
}

// Validate binds every observation to the exact freshly authorized reference
// set, then independently recomputes all claims, ranks, gaps and recommendations.
func Validate(scope Scope, report Report, references []contextbuilder.CitationReference) error {
	if len(references) > MaximumObservations || len(report.Timeline) != len(references) || len(report.Hypotheses) > 4 || len(report.Gaps) > 12 || len(report.NextReads) != 5 {
		return ErrInvalid
	}
	allowed := make(map[string]contextbuilder.CitationReference, len(references))
	for _, ref := range references {
		if _, duplicate := allowed[ref.ID]; duplicate {
			return ErrInvalid
		}
		allowed[ref.ID] = ref
	}
	for _, item := range report.Timeline {
		if expected, exists := allowed[item.Reference.ID]; !exists || expected != item.Reference {
			return ErrInvalid
		}
	}
	for _, hypothesis := range report.Hypotheses {
		if len(hypothesis.Supporting) > 2 || len(hypothesis.Contrary) > 2 {
			return ErrInvalid
		}
	}
	expected, err := Build(scope, report.Timeline)
	if err != nil || !reflect.DeepEqual(expected, report) {
		return ErrInvalid
	}
	return nil
}

func timestamp(value time.Time) bool {
	_, offset := value.Zone()
	return !value.IsZero() && offset == 0 && value.Year() >= 1970 && value.Year() <= 9999 && value.Nanosecond()%1000 == 0
}

func validObservation(scope Scope, item contextbuilder.Observation) bool {
	ref := item.Reference
	if len(ref.ID) != 67 || !strings.HasPrefix(ref.ID, "ev_") || strings.ToLower(ref.ID) != ref.ID {
		return false
	}
	if _, err := hex.DecodeString(ref.ID[3:]); err != nil {
		return false
	}
	if ref.Source != scope.Source || ref.ConfigurationVersion != scope.ConfigurationVersion || ref.PolicyVersion != scope.PolicyVersion || ref.RestoreGeneration != scope.RestoreGeneration ||
		ref.Range != scope.Range || !assistant.Identifier(ref.ObjectID) || !timestamp(ref.ObservedAt) || ref.ObservedAt.After(scope.ObservedAt) ||
		!timestamp(item.Time) || item.Time.After(ref.ObservedAt) || item.Availability != domain.AvailabilityAvailable && item.Availability != domain.AvailabilityStale || item.Completeness.Validate() != nil {
		return false
	}
	if ref.Kind == "relationship" {
		if ref.ResourceID != "" {
			return false
		}
	} else if ref.ResourceID != scope.ResourceID {
		return false
	}
	expected := contextbuilder.Observation{Reference: ref, Time: item.Time, TimeBasis: "observed", Availability: item.Availability, Completeness: item.Completeness, Synthetic: item.Synthetic}
	switch ref.Kind {
	case "resource", "health":
		if ref.ObjectID != string(ref.ResourceID) || item.Health.Validate() != nil || !item.Time.Equal(ref.ObservedAt) {
			return false
		}
		expected.Health = item.Health
	case "metric":
		if item.IntervalSeconds < 0 || item.IntervalSeconds > 86400 {
			return false
		}
		sample := domain.MetricSample{ID: domain.MetricSampleID(ref.ObjectID), ResourceID: ref.ResourceID, Name: item.MetricName, Unit: item.MetricUnit, Value: item.MetricValue, SampledAt: item.Time, Interval: time.Duration(item.IntervalSeconds) * time.Second}
		if sample.Validate() != nil {
			return false
		}
		expected.TimeBasis = "sampled"
		expected.MetricName, expected.MetricUnit = item.MetricName, item.MetricUnit
		expected.MetricValue, expected.IntervalSeconds = item.MetricValue, item.IntervalSeconds
	case "event":
		if item.EventSeverity.Validate() != nil {
			return false
		}
		expected.TimeBasis, expected.EventSeverity = "occurred", item.EventSeverity
	case "log":
		if item.LogSeverity.Validate() != nil || scope.Range.End.Sub(scope.Range.Start) > contextbuilder.MaximumLogWindow {
			return false
		}
		expected.TimeBasis, expected.LogSeverity = "occurred", item.LogSeverity
		expected.Redacted, expected.Truncated = item.Redacted, item.Truncated
	case "relationship":
		if !item.Time.Equal(ref.ObservedAt) {
			return false
		}
	default:
		return false
	}
	if expected.TimeBasis != "observed" && (item.Time.Before(scope.Range.Start) || !item.Time.Before(scope.Range.End)) {
		return false
	}
	return item == expected
}

func (report *Report) gap(code string) {
	if !slices.Contains(report.Gaps, code) {
		report.Gaps = append(report.Gaps, code)
	}
}

func usable(item contextbuilder.Observation) bool {
	return item.Availability == domain.AvailabilityAvailable && item.Completeness == domain.CompletenessComplete
}

func (report *Report) add(code string, support, contrary []string) {
	if len(support) == 0 {
		return
	}
	report.Hypotheses = append(report.Hypotheses, Hypothesis{Code: code, Rank: len(report.Hypotheses) + 1, Qualified: true, Supporting: slices.Clone(support), Contrary: slices.Clone(contrary)})
}

func (report *Report) metric(name domain.MetricName) []contextbuilder.Observation {
	var result []contextbuilder.Observation
	for _, item := range report.Timeline {
		if item.Reference.Kind == "metric" && item.MetricName == name && usable(item) {
			result = append(result, item)
		}
	}
	for index := 1; index < len(result); index++ {
		if result[index].Time.Equal(result[index-1].Time) && (result[index].MetricValue != result[index-1].MetricValue || result[index].IntervalSeconds != result[index-1].IntervalSeconds) {
			report.gap("conflicting_samples")
			return nil
		}
	}
	return result
}

func (report *Report) pressure(usageName, limitName domain.MetricName, code string) {
	usage, limits := report.metric(usageName), report.metric(limitName)
	var support, contrary []string
	var supportAt time.Time
	for _, item := range usage {
		matched := false
		for _, limit := range limits {
			if !item.Time.Equal(limit.Time) || item.IntervalSeconds != limit.IntervalSeconds || limit.MetricValue <= 0 {
				continue
			}
			matched = true
			ids := []string{item.Reference.ID, limit.Reference.ID}
			if item.MetricValue >= limit.MetricValue {
				support, supportAt, contrary = ids, item.Time, nil
			} else if len(support) > 0 && !item.Time.Before(supportAt) {
				contrary = ids
			}
			break
		}
		if !matched {
			report.gap("matching_limits_missing")
		}
	}
	report.add(code, support, contrary)
}

func (report *Report) restarts() {
	items := report.metric(domain.MetricRestartCount)
	var support, contrary []string
	for index := 1; index < len(items); index++ {
		previous, current := items[index-1], items[index]
		if !current.Time.After(previous.Time) {
			continue
		}
		ids := []string{previous.Reference.ID, current.Reference.ID}
		if current.MetricValue > previous.MetricValue {
			support, contrary = ids, nil
		} else if current.MetricValue < previous.MetricValue {
			report.gap("restart_counter_reset")
			// A reset breaks comparison with the prior counter lifetime.
			support, contrary = nil, nil
		} else if len(support) > 0 {
			contrary = ids
		}
	}
	report.add("restart_related_possible", support, contrary)
}

func (report *Report) health() {
	var support, contrary []string
	var supportAt time.Time
	var healthy *contextbuilder.Observation
	for _, item := range report.Timeline {
		if (item.Reference.Kind != "health" && item.Reference.Kind != "resource") || !usable(item) {
			continue
		}
		if item.Time.Before(item.Reference.Range.Start) {
			report.gap("out_of_window_observation")
			continue
		}
		if item.Health == domain.HealthUnhealthy || item.Health == domain.HealthDegraded {
			support, supportAt = []string{item.Reference.ID}, item.Time
		} else if item.Health == domain.HealthHealthy {
			copy := item
			healthy = &copy
		}
	}
	if healthy != nil && len(support) > 0 && !healthy.Time.Before(supportAt) {
		contrary = []string{healthy.Reference.ID}
		if healthy.Time.Equal(supportAt) {
			report.gap("conflicting_health")
		}
	}
	report.add("health_problem_possible", support, contrary)
}

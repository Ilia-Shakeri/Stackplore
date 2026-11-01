package contextbuilder

import (
	"fmt"
	"io"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// Observation is an internal scalar view of an already bounded bundle. It is
// not an authorization proof, transport DTO or independently publishable report.
// Prose, log messages, provider identities and arbitrary reason codes are absent.
type Observation struct {
	Reference       CitationReference
	Time            time.Time
	TimeBasis       string
	Availability    domain.AvailabilityState
	Completeness    domain.Completeness
	Synthetic       bool
	Health          domain.HealthState
	MetricName      domain.MetricName
	MetricUnit      domain.MetricUnit
	MetricValue     float64
	IntervalSeconds int64
	EventSeverity   domain.Severity
	LogSeverity     domain.LogSeverity
	Redacted        bool
	Truncated       bool
}

func (Observation) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected incident observation]")
}

func (Observation) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

// Observations preserves the exact reference/quality and actual sample/event
// time. An inventory or health observation never invents an incident start time.
// Callers must keep and freshly authorize the owning read proof before use.
func (bundle Bundle) Observations() ([]Observation, error) {
	if _, err := bundle.Items(); err != nil {
		return nil, err
	}
	result := make([]Observation, 0, len(bundle.document.Records))
	for index, record := range bundle.document.Records {
		item := Observation{
			Reference: bundle.references[index], Time: record.Quality.ObservedAt,
			TimeBasis: "observed", Availability: record.Quality.Availability,
			Completeness: record.Quality.Completeness, Synthetic: record.Quality.Synthetic,
		}
		switch record.Kind {
		case "resource", "health":
			item.Health = record.Health
		case "metric":
			if record.Metric == nil {
				return nil, ErrInvalid
			}
			item.Time, item.TimeBasis = record.Metric.SampledAt, "sampled"
			item.MetricName, item.MetricUnit = record.Metric.Name, record.Metric.Unit
			item.MetricValue, item.IntervalSeconds = record.Metric.Value, record.Metric.IntervalSeconds
		case "event":
			if record.Event == nil {
				return nil, ErrInvalid
			}
			item.Time, item.TimeBasis = record.Event.OccurredAt, "occurred"
			item.EventSeverity = record.Event.Severity
		case "log":
			if record.Log == nil {
				return nil, ErrInvalid
			}
			item.Time, item.TimeBasis = record.Log.Timestamp, "occurred"
			item.LogSeverity = record.Log.Severity
			item.Redacted, item.Truncated = record.Log.Redacted, record.Log.Truncated
		case "relationship":
			if record.Relationship == nil {
				return nil, ErrInvalid
			}
		default:
			return nil, ErrInvalid
		}
		result = append(result, item)
	}
	return result, nil
}

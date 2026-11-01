package contextbuilder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestObservationsPreserveExactQualityReferenceAndTimeWithoutProse(t *testing.T) {
	request := requestFixture(t)
	request.Destination = Local
	bundle, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	observations, err := bundle.Observations()
	if err != nil || len(observations) != 5 {
		t.Fatal("bounded typed observation projection failed")
	}
	for index, item := range observations {
		if item.Reference != bundle.Citations()[index] || item.Availability != domain.AvailabilityAvailable || item.Completeness != domain.CompletenessComplete || !item.Synthetic {
			t.Fatal("reference or quality changed")
		}
		encoded, err := json.Marshal(item)
		if err != nil || string(encoded) != "{}" || strings.Contains(fmt.Sprintf("%+v", item), "private") {
			t.Fatal("ordinary formatting exposed incident input")
		}
		switch item.Reference.Kind {
		case "resource", "health":
			if item.TimeBasis != "observed" || !item.Time.Equal(request.Status.ObservedAt) || item.Health != domain.HealthHealthy {
				t.Fatal("current health invented an occurrence time")
			}
		case "metric":
			if item.TimeBasis != "sampled" || !item.Time.Equal(request.Metrics[0].SampledAt) || item.MetricName != domain.MetricMemoryUsageBytes || item.MetricUnit != domain.MetricUnitBytes || item.MetricValue != 1024 || item.IntervalSeconds != 1 {
				t.Fatal("metric lost its unit, interval or actual sample time")
			}
		case "event":
			if item.TimeBasis != "occurred" || !item.Time.Equal(request.Events[0].OccurredAt) || item.EventSeverity != domain.SeverityWarning {
				t.Fatal("event observation time replaced its occurrence time")
			}
		case "relationship":
			if item.TimeBasis != "observed" || !item.Time.Equal(request.Status.ObservedAt) {
				t.Fatal("topology observation invented an incident time")
			}
		default:
			t.Fatal("unknown observation family")
		}
	}
	observations[0].Reference.ID = "changed"
	observations[0].Health = domain.HealthUnknown
	again, err := bundle.Observations()
	if err != nil || again[0].Reference.ID == "changed" || again[0].Health == domain.HealthUnknown {
		t.Fatal("projection allowed mutation of retained evidence")
	}
}

func TestObservationsRejectInvalidBundleAndPreserveEmptyCoverage(t *testing.T) {
	if _, err := (Bundle{}).Observations(); !errors.Is(err, ErrInvalid) {
		t.Fatal("zero bundle accepted")
	}
	request := requestFixture(t)
	request.Status.State = domain.AvailabilityUnsupported
	request.Status.ReasonCode = "source.unsupported"
	request.Resources, request.Health, request.Metrics = nil, nil, nil
	request.Events, request.Relationships = nil, nil
	bundle, err := Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	observations, err := bundle.Observations()
	if err != nil || len(observations) != 0 || bundle.Summary().Availability != domain.AvailabilityUnsupported {
		t.Fatal("empty unsupported family became a fabricated observation")
	}
}

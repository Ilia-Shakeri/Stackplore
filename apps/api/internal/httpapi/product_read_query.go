package httpapi

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

var logContainerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,255}$`)
var observationTimePattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,6})?Z$`)

func parseLogSelection(r *http.Request, resource domain.ResourceID) (string, bool, error) {
	query := r.URL.Query()
	container, previous := query.Get("container"), query.Get("previous")
	if container != "" && !logContainerPattern.MatchString(container) || previous != "" && previous != "true" && previous != "false" || resource == "" && (container != "" || previous != "") {
		return "", false, errors.New("invalid log target")
	}
	return container, previous == "true", nil
}

func parseObservationRange(r *http.Request, fallback domain.TimeRange, maximum time.Duration) (domain.TimeRange, error) {
	query := r.URL.Query()
	if !query.Has("from") && !query.Has("to") {
		return fallback, nil
	}
	if len(query["from"]) != 1 || len(query["to"]) != 1 {
		return domain.TimeRange{}, errors.New("paired time range required")
	}
	parse := func(text string) (time.Time, error) {
		value, err := time.Parse(time.RFC3339Nano, text)
		if err != nil || !observationTimePattern.MatchString(text) || strings.HasPrefix(text, "0000") || value.Nanosecond()%int(time.Microsecond) != 0 {
			return time.Time{}, errors.New("invalid observation time")
		}
		return value.UTC(), nil
	}
	start, err := parse(query.Get("from"))
	if err != nil {
		return domain.TimeRange{}, err
	}
	end, err := parse(query.Get("to"))
	if err != nil {
		return domain.TimeRange{}, err
	}
	result := domain.TimeRange{Start: start, End: end}
	if result.ValidateLookback(maximum) != nil || end.After(time.Now().UTC().Add(time.Minute)) {
		return domain.TimeRange{}, errors.New("observation range out of bounds")
	}
	return result, nil
}

func withSelectionStatus(status, selection domain.ObservationStatus) domain.ObservationStatus {
	if !status.AllowsData() {
		return status
	}
	if !selection.AllowsData() {
		return selection
	}
	if selection.State == domain.AvailabilityStale {
		status.State = domain.AvailabilityStale
		status.ReasonCode = selection.ReasonCode
	}
	if selection.Completeness == domain.CompletenessPartial {
		status.Completeness = domain.CompletenessPartial
		if status.ReasonCode == "" {
			status.ReasonCode = selection.ReasonCode
		}
	}
	return status
}

func emptyReadStatus(r *http.Request, provider ProductProvider, selection domain.ObservationStatus, name domain.CapabilityName) (domain.ObservationStatus, error) {
	if !selection.AllowsData() {
		return selection, nil
	}
	capabilities, err := provider.GetCapabilities(r.Context(), providerRef(provider))
	if err != nil {
		return domain.ObservationStatus{}, err
	}
	if capabilities.ValidateFor(providerRef(provider)) != nil {
		return domain.ObservationStatus{}, errors.New("invalid provider capabilities")
	}
	for _, capability := range capabilities.Capabilities {
		if capability.Name == name {
			return withSelectionStatus(capability.Status, selection), nil
		}
	}
	return domain.ObservationStatus{}, errors.New("missing provider capability")
}

package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	MaxEventTargets       = 100
	MaxEventLookback      = 31 * 24 * time.Hour
	MaxEventSummaryLength = 8192
)

type Severity string

const (
	SeverityDebug    Severity = "debug"
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

func (severity Severity) Validate() error {
	switch severity {
	case SeverityDebug, SeverityInfo, SeverityWarning, SeverityError, SeverityCritical:
		return nil
	default:
		return fmt.Errorf("unknown severity %q", severity)
	}
}

type EventKind string

func (kind EventKind) Validate() error {
	return validateNamespacedCode("event kind", string(kind))
}

type InfrastructureEvent struct {
	ID            EventID
	Provider      ProviderKind
	SourceID      SourceID
	EnvironmentID *EnvironmentID
	ResourceID    *ResourceID
	Kind          EventKind
	Severity      Severity
	ReasonCode    ReasonCode
	Summary       string
	OccurredAt    time.Time
	ObservedAt    time.Time
	Synthetic     bool
}

func (event InfrastructureEvent) Validate() error {
	if err := event.ID.Validate(); err != nil {
		return err
	}
	if err := event.Provider.Validate(); err != nil {
		return err
	}
	if err := event.SourceID.Validate(); err != nil {
		return err
	}
	if event.EnvironmentID != nil {
		if err := event.EnvironmentID.Validate(); err != nil {
			return err
		}
	}
	if event.ResourceID != nil {
		if err := event.ResourceID.Validate(); err != nil {
			return err
		}
	}
	if err := event.Kind.Validate(); err != nil {
		return err
	}
	if err := event.Severity.Validate(); err != nil {
		return err
	}
	if err := event.ReasonCode.Validate(); err != nil {
		return err
	}
	if err := validateDisplayText("event summary", event.Summary, MaxEventSummaryLength, true); err != nil {
		return err
	}
	if err := validateUTCTime("event occurrence time", event.OccurredAt); err != nil {
		return err
	}
	if err := validateUTCTime("event observation time", event.ObservedAt); err != nil {
		return err
	}
	if event.OccurredAt.After(event.ObservedAt) {
		return fmt.Errorf("event occurrence cannot follow observation")
	}
	return nil
}

func (event InfrastructureEvent) ValidateForWatch(query EventWatchQuery) error {
	if err := query.Validate(); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return err
	}
	if event.Provider != query.Source.Provider || event.SourceID != query.Source.SourceID {
		return fmt.Errorf("watched event belongs to another source")
	}
	if event.OccurredAt.Before(query.Since) {
		return fmt.Errorf("watched event predates requested replay window")
	}
	if len(query.Resources) > 0 {
		if event.ResourceID == nil {
			return fmt.Errorf("resource-filtered event watch contains an unscoped event")
		}
		matched := false
		for _, resource := range query.Resources {
			if *event.ResourceID == resource.ID {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("watched event belongs to an unrequested resource")
		}
	}
	if len(query.Severities) > 0 {
		matched := false
		for _, severity := range query.Severities {
			if event.Severity == severity {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("watched event has an unrequested severity")
		}
	}
	return nil
}

type EventQuery struct {
	Source        ProviderRef
	Resources     []ResourceRef
	Range         TimeRange
	Severities    []Severity
	EnvironmentID EnvironmentID
	Search        string
	Page          PageRequest
}

// MatchesTextAndEnvironment is shared by normalized providers and page validation.
func (query EventQuery) MatchesTextAndEnvironment(event InfrastructureEvent) bool {
	if query.EnvironmentID != "" && (event.EnvironmentID == nil || *event.EnvironmentID != query.EnvironmentID) {
		return false
	}
	return query.Search == "" || strings.Contains(strings.ToLower(event.Summary+" "+string(event.ReasonCode)+" "+string(event.Kind)+" "+string(event.ID)), strings.ToLower(query.Search))
}

func (query EventQuery) Validate() error {
	if query.EnvironmentID != "" && query.EnvironmentID.Validate() != nil {
		return fmt.Errorf("invalid event environment filter")
	}
	if validateDisplayText("event search", query.Search, MaxSearchLength, true) != nil {
		return fmt.Errorf("invalid event search")
	}
	if err := query.Source.Validate(); err != nil {
		return err
	}
	if err := validateResourceRefs(query.Source, query.Resources, MaxEventTargets, true); err != nil {
		return err
	}
	if err := query.Range.ValidateLookback(MaxEventLookback); err != nil {
		return err
	}
	if err := validateSeverities(query.Severities); err != nil {
		return err
	}
	return query.Page.Validate()
}

type EventPage struct {
	Status ObservationStatus
	Events []InfrastructureEvent
	Page   PageInfo
}

func (page EventPage) ValidateFor(query EventQuery) error {
	if err := query.Validate(); err != nil {
		return err
	}
	if err := page.Status.Validate(); err != nil {
		return err
	}
	if err := page.Page.ValidateFor(query.Page); err != nil {
		return err
	}
	if len(page.Events) > query.Page.Limit {
		return fmt.Errorf("event page exceeds requested limit")
	}
	if err := validatePageAvailability(page.Status, page.Page, len(page.Events)); err != nil {
		return err
	}
	resources := make(map[ResourceID]struct{}, len(query.Resources))
	for _, resource := range query.Resources {
		resources[resource.ID] = struct{}{}
	}
	seen := make(map[EventID]struct{}, len(page.Events))
	severities := make(map[Severity]struct{}, len(query.Severities))
	for _, severity := range query.Severities {
		severities[severity] = struct{}{}
	}
	for index, event := range page.Events {
		if !query.MatchesTextAndEnvironment(event) {
			return fmt.Errorf("event page contains an unrequested environment or search")
		}
		if err := event.Validate(); err != nil {
			return err
		}
		if event.ObservedAt.After(page.Status.ObservedAt) {
			return fmt.Errorf("event cannot be newer than its page")
		}
		if event.OccurredAt.Before(query.Range.Start) || !event.OccurredAt.Before(query.Range.End) {
			return fmt.Errorf("event page contains an event outside the requested range")
		}
		if event.Provider != query.Source.Provider || event.SourceID != query.Source.SourceID {
			return fmt.Errorf("event page contains an event from another source")
		}
		if len(resources) > 0 {
			if event.ResourceID == nil {
				return fmt.Errorf("resource-filtered event page contains an unscoped event")
			}
			if _, exists := resources[*event.ResourceID]; !exists {
				return fmt.Errorf("event page contains an unrequested resource")
			}
		}
		if len(severities) > 0 {
			if _, exists := severities[event.Severity]; !exists {
				return fmt.Errorf("event page contains an unrequested severity")
			}
		}
		if _, exists := seen[event.ID]; exists {
			return fmt.Errorf("event page contains a duplicate event")
		}
		if index > 0 {
			previous := page.Events[index-1]
			if previous.OccurredAt.Before(event.OccurredAt) ||
				(previous.OccurredAt.Equal(event.OccurredAt) && strings.Compare(string(previous.ID), string(event.ID)) > 0) {
				return fmt.Errorf("event page must use descending occurrence and ascending ID order")
			}
		}
		seen[event.ID] = struct{}{}
	}
	return nil
}

type EventWatchQuery struct {
	Source     ProviderRef
	Resources  []ResourceRef
	Severities []Severity
	Since      time.Time
	// RequestedAt is a trusted service timestamp, never client input.
	RequestedAt time.Time
	BufferSize  int
}

func (query EventWatchQuery) Validate() error {
	if err := query.Source.Validate(); err != nil {
		return err
	}
	if err := validateResourceRefs(query.Source, query.Resources, MaxEventTargets, true); err != nil {
		return err
	}
	if err := validateSeverities(query.Severities); err != nil {
		return err
	}
	if err := validateReplayWindow("event watch", query.Since, query.RequestedAt, MaxEventLookback); err != nil {
		return err
	}
	if query.BufferSize < 1 || query.BufferSize > MaxStreamBuffer {
		return fmt.Errorf("event watch buffer must be between 1 and %d", MaxStreamBuffer)
	}
	return nil
}

func validateResourceRefs(source ProviderRef, resources []ResourceRef, maximum int, allowEmpty bool) error {
	if !allowEmpty && len(resources) == 0 {
		return fmt.Errorf("at least one resource is required")
	}
	if len(resources) > maximum {
		return fmt.Errorf("resource count exceeds %d", maximum)
	}
	seen := make(map[ResourceID]struct{}, len(resources))
	for _, resource := range resources {
		if err := resource.Validate(); err != nil {
			return err
		}
		if !source.Matches(resource) {
			return fmt.Errorf("resource does not belong to query source")
		}
		if _, exists := seen[resource.ID]; exists {
			return fmt.Errorf("resource list contains a duplicate")
		}
		seen[resource.ID] = struct{}{}
	}
	return nil
}

func validateSeverities(severities []Severity) error {
	if len(severities) > 5 {
		return fmt.Errorf("severity filter exceeds 5 values")
	}
	seen := make(map[Severity]struct{}, len(severities))
	for _, severity := range severities {
		if err := severity.Validate(); err != nil {
			return err
		}
		if _, exists := seen[severity]; exists {
			return fmt.Errorf("severity filter contains a duplicate")
		}
		seen[severity] = struct{}{}
	}
	return nil
}

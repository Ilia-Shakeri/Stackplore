package domain

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

const MaximumIncidentHistory = 8

// IncidentAlertHistoryQuery is constructed by an authorized resource use case.
// There is no source-wide fallback, cursor, search string or arbitrary endpoint.
type IncidentAlertHistoryQuery struct {
	Source   ProviderRef
	Resource ResourceRef
	Range    TimeRange
	Limit    int
}

func (query IncidentAlertHistoryQuery) Validate() error {
	if query.Source.Validate() != nil || query.Resource.Validate() != nil || !query.Source.Matches(query.Resource) ||
		query.Range.ValidateLookback(24*time.Hour) != nil || !incidentHistoryTime(query.Range.Start) || !incidentHistoryTime(query.Range.End) ||
		query.Limit < 1 || query.Limit > MaximumIncidentHistory {
		return ErrRepositoryInvalidInput
	}
	return nil
}

func (IncidentAlertHistoryQuery) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected incident history query]")
}
func (IncidentAlertHistoryQuery) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

// Only historical transition fields are retained. The alert's current severity
// or rule kind must not be relabeled as their value at an earlier transition.
// Actor, acknowledgement identity, prose and raw reason codes are absent.
type IncidentAlertTransition struct {
	ID         AlertTransitionID
	AlertID    AlertID
	ResourceID ResourceID
	From       *AlertState
	To         AlertState
	OccurredAt time.Time
	Synthetic  bool
}

type IncidentAlertHistory struct {
	Status        ObservationStatus
	Transitions   []IncidentAlertTransition
	MoreAvailable bool
}

func (IncidentAlertHistory) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected incident alert history]")
}
func (IncidentAlertHistory) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (history IncidentAlertHistory) ValidateFor(query IncidentAlertHistoryQuery) error {
	if query.Validate() != nil || history.Status.Validate() != nil || !incidentHistoryTime(history.Status.ObservedAt) ||
		len(history.Transitions) > query.Limit || !history.Status.AllowsData() && (len(history.Transitions) != 0 || history.MoreAvailable) ||
		history.MoreAvailable && (history.Status.Completeness != CompletenessPartial || len(history.Transitions) != query.Limit) {
		return ErrRepositoryInvalidInput
	}
	seen := make(map[AlertTransitionID]bool, len(history.Transitions))
	for index, transition := range history.Transitions {
		if transition.ID.Validate() != nil || transition.AlertID.Validate() != nil || transition.ResourceID != query.Resource.ID ||
			transition.Synthetic != (query.Source.Provider == ProviderDemo) ||
			transition.To.Validate() != nil || transition.From != nil && (transition.From.Validate() != nil || *transition.From == transition.To) ||
			!incidentHistoryTime(transition.OccurredAt) || transition.OccurredAt.Before(query.Range.Start) || !transition.OccurredAt.Before(query.Range.End) || seen[transition.ID] {
			return ErrRepositoryInvalidInput
		}
		if index > 0 {
			previous := history.Transitions[index-1]
			if transition.OccurredAt.Before(previous.OccurredAt) || transition.OccurredAt.Equal(previous.OccurredAt) && strings.Compare(string(transition.ID), string(previous.ID)) <= 0 {
				return ErrRepositoryInvalidInput
			}
		}
		seen[transition.ID] = true
	}
	return nil
}

func incidentHistoryTime(value time.Time) bool {
	_, offset := value.Zone()
	return !value.IsZero() && offset == 0 && value.Year() >= 1970 && value.Year() <= 9999 && value.Nanosecond()%int(time.Microsecond) == 0
}

type IncidentAlertHistoryReader interface {
	GetIncidentAlertHistory(context.Context, IncidentAlertHistoryQuery) (IncidentAlertHistory, error)
}

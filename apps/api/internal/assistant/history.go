package assistant

import (
	"context"
	"fmt"
	"io"
	"time"
)

const MaximumHistoryPage = 10

// HistoryQuery is a bounded owner-metadata read, never a content replay.
type HistoryQuery struct {
	State State
	UpperBound time.Time
	BeforeAt time.Time
	BeforeID string
	Limit int
}

func (query HistoryQuery) Validate(at time.Time) error {
	if !Timestamp(at) || !Timestamp(query.UpperBound) || query.UpperBound.After(at) || query.State != Queued && query.State != Running && !query.State.Terminal() || query.Limit < 1 || query.Limit > MaximumHistoryPage { return ErrInvalid }
	if query.BeforeAt.IsZero() != (query.BeforeID == "") || !query.BeforeAt.IsZero() && (!Timestamp(query.BeforeAt) || query.BeforeAt.After(query.UpperBound) || !Identifier(query.BeforeID)) { return ErrInvalid }
	return nil
}

// HistoryEntry deliberately omits sessions, questions, replies and evidence bodies.
type HistoryEntry struct {
	ID string
	SourceID string
	ResourceID string
	Goal Goal
	State State
	CreatedAt time.Time
	UpdatedAt time.Time
	From time.Time
	Until time.Time
	ToolCalls int
	InferenceCalls int
	UsageUncertain bool
}

func (HistoryEntry) Format(state fmt.State, _ rune) { _, _ = io.WriteString(state, "[protected run metadata]") }
func (HistoryEntry) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (entry HistoryEntry) Validate(query HistoryQuery, at time.Time) error {
	if query.Validate(at) != nil || !Identifier(entry.ID) || !Identifier(entry.SourceID) || entry.ResourceID != "" && !Identifier(entry.ResourceID) || entry.Goal != ExplainResource && entry.Goal != InvestigateIncident || entry.State != query.State || !Timestamp(entry.CreatedAt) || !Timestamp(entry.UpdatedAt) || entry.UpdatedAt.Before(entry.CreatedAt) || entry.CreatedAt.After(query.UpperBound) || entry.CreatedAt.Before(at.Add(-MetadataRetention)) || !Timestamp(entry.From) || !Timestamp(entry.Until) || entry.Until.Before(entry.From) || entry.Until.After(entry.CreatedAt) || entry.Until.Sub(entry.From) > 24*time.Hour || entry.ToolCalls < 0 || entry.ToolCalls > MaximumToolCalls || entry.InferenceCalls < 0 || entry.InferenceCalls > MaximumInferenceCalls { return ErrInvalid }
	if !query.BeforeAt.IsZero() && (entry.CreatedAt.After(query.BeforeAt) || entry.CreatedAt.Equal(query.BeforeAt) && entry.ID >= query.BeforeID) { return ErrInvalid }
	return nil
}

type HistoryPage struct {
	Entries []HistoryEntry
	HasMore bool
}

func (HistoryPage) Format(state fmt.State, _ rune) { _, _ = io.WriteString(state, "[protected run metadata page]") }
func (HistoryPage) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type HistoryRepository interface {
	ListAssistantHistory(context.Context, Actor, HistoryQuery, time.Time) (HistoryPage, error)
}

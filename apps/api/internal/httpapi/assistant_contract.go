package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

// AssistantRunReader supplies freshly authorized snapshots and owner cancellation.
// A client cannot select this dependency or supply an actor digest.
type AssistantRunReader interface {
	Get(context.Context, assistant.Actor, string) (assistantservice.Snapshot, error)
	Cancel(context.Context, assistant.Actor, string) (assistant.Run, error)
}

var _ AssistantRunReader = (*assistantservice.Service)(nil)

var assistantPublicRunID = regexp.MustCompile(`^ar_[0-9a-f]{64}$`)
var assistantPublicConversationID = regexp.MustCompile(`^ac_[0-9a-f]{64}$`)
var assistantPublicCitationID = regexp.MustCompile(`^ev_[0-9a-f]{64}$`)
var assistantPublicSourceID = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,127}$`)

type assistantTimeRangeJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
}
type assistantScopeJSON struct {
	SourceID      string                 `json:"source_id"`
	ResourceID    string                 `json:"resource_id"`
	WindowMinutes int                    `json:"window_minutes"`
	Range         assistantTimeRangeJSON `json:"range"`
}
type assistantRunJSON struct {
	ID                string             `json:"id"`
	ConversationID    string             `json:"conversation_id"`
	Goal              assistant.Goal     `json:"goal"`
	State             assistant.State    `json:"state"`
	Version           int64              `json:"version"`
	Scope             assistantScopeJSON `json:"scope"`
	CreatedAt         string             `json:"created_at"`
	UpdatedAt         string             `json:"updated_at"`
	Deadline          string             `json:"deadline"`
	UsageUncertain    bool               `json:"usage_uncertain"`
	UsageConservative bool               `json:"usage_conservative"`
}
type assistantReceiptJSON struct {
	Run assistantRunJSON `json:"run"`
}
type assistantCitationJSON struct {
	ID         string                 `json:"id"`
	Kind       string                 `json:"kind"`
	SourceID   string                 `json:"source_id"`
	ResourceID string                 `json:"resource_id"`
	ObservedAt string                 `json:"observed_at"`
	Range      assistantTimeRangeJSON `json:"range"`
}
type assistantReplyJSON struct {
	Run          assistantRunJSON        `json:"run"`
	ContentState string                  `json:"content_state"`
	Answer       *inference.Answer       `json:"answer,omitempty"`
	Citations    []assistantCitationJSON `json:"citations,omitempty"`
}

func (assistantReplyJSON) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected conversation reply]")
}

func (assistantReplyJSON) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }
func (assistantReplyJSON) LogValue() slog.Value {
	return slog.StringValue("[protected conversation reply]")
}

// Deliberate wire conversion is confined to the freshly authorized response path.
func writeAssistantReply(w http.ResponseWriter, reply assistantReplyJSON) {
	type wireReply assistantReplyJSON
	writeJSON(w, http.StatusOK, wireReply(reply))
}

func assistantMetadata(run assistant.Run, owner, id string) (assistantRunJSON, error) {
	if run.OwnerID != owner || run.ID != id {
		return assistantRunJSON{}, assistant.ErrDenied
	}
	if !assistantPublicRunID.MatchString(run.ID) || !assistantPublicConversationID.MatchString(run.ConversationID) ||
		run.Goal != assistant.ExplainResource || run.Version < 1 || run.Version > assistant.MaximumStatusVersions ||
		run.SourceVersion < 1 || run.PolicyVersion < 1 || run.ProfileVersion < 1 || run.RestoreGeneration < 0 ||
		(!run.State.Terminal() && run.State != assistant.Queued && run.State != assistant.Running) ||
		!assistantPublicSourceID.MatchString(run.SourceID) || domain.ResourceID(run.ResourceID).Validate() != nil {
		return assistantRunJSON{}, assistant.ErrUnavailable
	}
	if !assistant.Timestamp(run.From) || !assistant.Timestamp(run.Until) || !assistant.Timestamp(run.CreatedAt) ||
		!assistant.Timestamp(run.UpdatedAt) || !assistant.Timestamp(run.Deadline) || run.UpdatedAt.Before(run.CreatedAt) ||
		!run.Deadline.After(run.CreatedAt) || run.Deadline.Sub(run.CreatedAt) > assistant.MaximumRunDuration {
		return assistantRunJSON{}, assistant.ErrUnavailable
	}
	duration := run.Until.Sub(run.From)
	window := int(duration / time.Minute)
	if duration != time.Duration(window)*time.Minute || !slices.Contains([]int{5, 15, 60, 360, 1440}, window) {
		return assistantRunJSON{}, assistant.ErrUnavailable
	}
	return assistantRunJSON{
		ID: run.ID, ConversationID: run.ConversationID, Goal: run.Goal, State: run.State, Version: run.Version,
		Scope: assistantScopeJSON{
			SourceID: run.SourceID, ResourceID: run.ResourceID, WindowMinutes: window,
			Range: assistantTimeRangeJSON{From: run.From.Format(time.RFC3339Nano), To: run.Until.Format(time.RFC3339Nano)},
		},
		CreatedAt: run.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: run.UpdatedAt.Format(time.RFC3339Nano),
		Deadline:       run.Deadline.Format(time.RFC3339Nano),
		UsageUncertain: run.UsageUncertain, UsageConservative: run.UsageConservative,
	}, nil
}

func assistantCitation(run assistant.Run, ref contextbuilder.CitationReference) (assistantCitationJSON, error) {
	_, offset := ref.ObservedAt.Zone()
	if !assistantPublicCitationID.MatchString(ref.ID) || ref.Kind != "resource" || ref.Source.Validate() != nil ||
		string(ref.Source.SourceID) != run.SourceID || string(ref.ResourceID) != run.ResourceID || ref.ObjectID != run.ResourceID ||
		ref.ConfigurationVersion != run.SourceVersion || ref.PolicyVersion != run.PolicyVersion || ref.RestoreGeneration != run.RestoreGeneration ||
		ref.ObservedAt.IsZero() || offset != 0 || ref.ObservedAt.Year() < 1970 || ref.ObservedAt.Year() > 9999 ||
		!ref.Range.Start.Equal(run.From) || !ref.Range.End.Equal(run.Until) {
		return assistantCitationJSON{}, assistant.ErrUnavailable
	}
	return assistantCitationJSON{ID: ref.ID, Kind: "resource", SourceID: run.SourceID, ResourceID: run.ResourceID, ObservedAt: ref.ObservedAt.Format(time.RFC3339Nano), Range: assistantTimeRangeJSON{From: run.From.Format(time.RFC3339Nano), To: run.Until.Format(time.RFC3339Nano)}}, nil
}

// Only the explicit publication path receives a serializable answer projection.
func assistantReply(snapshot assistantservice.Snapshot, owner, id string) (assistantReplyJSON, error) {
	run, err := assistantMetadata(snapshot.Run, owner, id)
	if err != nil {
		return assistantReplyJSON{}, err
	}
	reply := assistantReplyJSON{Run: run, ContentState: "unavailable"}
	if !snapshot.Volatile && (snapshot.Answer != nil || len(snapshot.Citations) != 0) {
		return assistantReplyJSON{}, assistant.ErrUnavailable
	}
	if snapshot.Run.State == assistant.Completed && !snapshot.Volatile {
		reply.ContentState = "volatile_lost"
		return reply, nil
	}
	if !snapshot.Volatile {
		return reply, nil
	}
	if snapshot.Run.State == assistant.Queued || snapshot.Run.State == assistant.Running {
		reply.ContentState = "pending"
		return reply, nil
	}
	if snapshot.Run.State != assistant.Completed || snapshot.Answer == nil {
		return reply, nil
	}
	if len(snapshot.Citations) != 1 || len(snapshot.Answer.Sections) < 1 || len(snapshot.Answer.Sections) > inference.MaximumSections {
		return assistantReplyJSON{}, assistant.ErrUnavailable
	}
	citation, err := assistantCitation(snapshot.Run, snapshot.Citations[0])
	if err != nil {
		return assistantReplyJSON{}, err
	}
	answer := inference.Answer{Sections: slices.Clone(snapshot.Answer.Sections)}
	for index := range answer.Sections {
		if len(answer.Sections[index].Citations) > inference.MaximumEvidence {
			return assistantReplyJSON{}, assistant.ErrUnavailable
		}
		answer.Sections[index].Citations = slices.Clone(answer.Sections[index].Citations)
	}
	if inference.ValidateAnswer(answer, map[string]struct{}{citation.ID: {}}) != nil {
		return assistantReplyJSON{}, assistant.ErrUnavailable
	}
	encoded, err := json.Marshal(answer)
	if err != nil || len(encoded) > inference.MaximumOutputBytes {
		clear(encoded)
		return assistantReplyJSON{}, assistant.ErrUnavailable
	}
	clear(encoded)
	reply.ContentState, reply.Answer, reply.Citations = "ready", &answer, []assistantCitationJSON{citation}
	return reply, nil
}

package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistanthistory"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type AssistantHistoryReader interface {
	History(context.Context, assistant.Actor, assistant.State, int, string) (assistanthistory.Page, error)
}

var assistantHistoryCursor = regexp.MustCompile(`^[A-Za-z0-9_-]{1,724}\.[A-Za-z0-9_-]{43}$`)

type assistantHistoryEntryJSON struct {
	ID             string                 `json:"id"`
	SourceID       string                 `json:"source_id"`
	ResourceID     *string                `json:"resource_id"`
	Goal           assistant.Goal         `json:"goal"`
	State          assistant.State        `json:"state"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
	Range          assistantTimeRangeJSON `json:"range"`
	ToolCalls      int                    `json:"tool_calls"`
	InferenceCalls int                    `json:"inference_calls"`
	UsageUncertain bool                   `json:"usage_uncertain"`
}

type assistantHistoryReplyJSON struct {
	Entries    []assistantHistoryEntryJSON `json:"entries"`
	NextCursor *string                     `json:"next_cursor"`
}

func (assistantHistoryReplyJSON) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected history reply]")
}
func (assistantHistoryReplyJSON) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func historyQuery(raw string) (assistant.State, int, string, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "", 0, "", assistant.ErrInvalid
	}
	for key, items := range values {
		if key != "state" && key != "limit" && key != "cursor" || len(items) != 1 || items[0] == "" {
			return "", 0, "", assistant.ErrInvalid
		}
	}
	state, limit, cursor := assistant.Completed, assistant.MaximumHistoryPage, values.Get("cursor")
	if value := values.Get("state"); value != "" {
		state = assistant.State(value)
	}
	if value := values.Get("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil || strconv.Itoa(limit) != value {
			return "", 0, "", assistant.ErrInvalid
		}
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	if (assistant.HistoryQuery{State: state, Limit: limit, UpperBound: at, Generation: -1}).Validate(at) != nil || len(cursor) > assistanthistory.MaximumCursorBytes || cursor != "" && !assistantHistoryCursor.MatchString(cursor) {
		return "", 0, "", assistant.ErrInvalid
	}
	return state, limit, cursor, nil
}

func historyReply(page assistanthistory.Page, state assistant.State, limit int) (assistantHistoryReplyJSON, error) {
	at := time.Now().UTC().Truncate(time.Microsecond)
	query := assistant.HistoryQuery{State: state, Limit: limit, UpperBound: at, Generation: -1}
	if len(page.Entries) > limit || page.NextCursor != "" && (len(page.NextCursor) > assistanthistory.MaximumCursorBytes || !assistantHistoryCursor.MatchString(page.NextCursor) || len(page.Entries) != limit) {
		return assistantHistoryReplyJSON{}, assistant.ErrUnavailable
	}
	reply := assistantHistoryReplyJSON{Entries: make([]assistantHistoryEntryJSON, 0, len(page.Entries))}
	seen := make(map[string]bool, len(page.Entries))
	for index, entry := range page.Entries {
		if seen[entry.ID] || entry.Validate(query, at) != nil || !assistantPublicRunID.MatchString(entry.ID) || !assistantPublicSourceID.MatchString(entry.SourceID) || entry.ResourceID != "" && domain.ResourceID(entry.ResourceID).Validate() != nil {
			return assistantHistoryReplyJSON{}, assistant.ErrUnavailable
		}
		seen[entry.ID] = true
		if index > 0 {
			prior := page.Entries[index-1]
			if entry.CreatedAt.After(prior.CreatedAt) || entry.CreatedAt.Equal(prior.CreatedAt) && entry.ID >= prior.ID {
				return assistantHistoryReplyJSON{}, assistant.ErrUnavailable
			}
		}
		item := assistantHistoryEntryJSON{ID: entry.ID, SourceID: entry.SourceID, Goal: entry.Goal, State: entry.State, CreatedAt: entry.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: entry.UpdatedAt.Format(time.RFC3339Nano), Range: assistantTimeRangeJSON{From: entry.From.Format(time.RFC3339Nano), To: entry.Until.Format(time.RFC3339Nano)}, ToolCalls: entry.ToolCalls, InferenceCalls: entry.InferenceCalls, UsageUncertain: entry.UsageUncertain}
		if entry.ResourceID != "" {
			value := entry.ResourceID
			item.ResourceID = &value
		}
		reply.Entries = append(reply.Entries, item)
	}
	if page.NextCursor != "" {
		value := page.NextCursor
		reply.NextCursor = &value
	}
	return reply, nil
}

func (handler assistantHTTP) listHistory(w http.ResponseWriter, r *http.Request) {
	identity, ok := handler.product.authenticateIdentity(w, r)
	if !ok {
		return
	}
	if !requestHasNoBody(r) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	state, limit, cursor, err := historyQuery(r.URL.RawQuery)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	actor, err := assistantActor(identity)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	page, err := handler.history.History(r.Context(), actor, state, limit, cursor)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	reply, err := historyReply(page, state, limit)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	type wireReply assistantHistoryReplyJSON
	writeJSON(w, http.StatusOK, wireReply(reply))
}

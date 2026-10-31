package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantconversation"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
)

const assistantStatusMaximumBatch = 64 * 1024

func isAssistantStatusPath(path string) bool {
	const prefix = "/api/v1/assistant/runs/"
	const suffix = "/events"
	return len(path) == len(prefix)+67+len(suffix) && strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) && assistantPublicRunID.MatchString(path[len(prefix):len(path)-len(suffix)])
}

type assistantStatusTimers struct{ life, poll, keepalive time.Duration }

func (handler assistantHTTP) streamStatus(w http.ResponseWriter, r *http.Request) {
	handler.streamStatusWithTimers(w, r, assistantStatusTimers{life: 120 * time.Second, poll: 2 * time.Second, keepalive: 10 * time.Second})
}

func (handler assistantHTTP) streamStatusWithTimers(w http.ResponseWriter, r *http.Request, timers assistantStatusTimers) {
	if handler.statuses == nil || handler.statusGate == nil || timers.life < time.Millisecond || timers.life > 120*time.Second || timers.poll < time.Millisecond || timers.poll > 2*time.Second || timers.keepalive < time.Millisecond || timers.keepalive > 10*time.Second {
		writeProblem(w, r, unavailableProblem)
		return
	}
	if !requireNoQuery(w, r) {
		return
	}
	if !requestHasNoBody(r) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	id, ok := assistantPath(w, r)
	if !ok {
		return
	}
	values := r.Header.Values("Last-Event-ID")
	after := int64(0)
	if len(values) > 1 {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	if len(values) == 1 {
		value := values[0]
		if len(value) < 1 || len(value) > 19 || value != "0" && (value[0] < '1' || value[0] > '9') {
			writeProblem(w, r, invalidRequestProblem)
			return
		}
		for _, digit := range value {
			if digit < '0' || digit > '9' {
				writeProblem(w, r, invalidRequestProblem)
				return
			}
		}
		var err error
		after, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			writeProblem(w, r, invalidRequestProblem)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), timers.life)
	defer cancel()
	initial, stop := context.WithTimeout(ctx, 3*time.Second)
	identity, ok := handler.product.authenticateIdentity(w, r.WithContext(initial))
	initialError := initial.Err()
	stop()
	if !ok {
		return
	}
	if initialError != nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	actor, err := assistantActor(identity)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	release, ok := handler.statusGate.acquire(identity.Principal.UserID)
	if !ok {
		writeProblem(w, r, rateLimitProblem)
		return
	}
	defer release()
	token, ok := readCookieValue(r, sessionCookieName)
	if !ok {
		writeProblem(w, r, unauthenticatedProblem)
		return
	}
	read := func() (assistantconversation.StatusPage, error) {
		work, done := context.WithTimeout(ctx, 3*time.Second)
		defer done()
		current, err := handler.product.auth.service.Authenticate(work, token)
		if work.Err() != nil {
			return assistantconversation.StatusPage{}, assistant.ErrUnavailable
		}
		if err != nil {
			if errors.Is(err, auth.ErrUnauthenticated) {
				return assistantconversation.StatusPage{}, assistant.ErrDenied
			}
			return assistantconversation.StatusPage{}, assistant.ErrUnavailable
		}
		currentActor, err := assistantActor(current)
		if err != nil {
			return assistantconversation.StatusPage{}, assistant.ErrUnavailable
		}
		if currentActor != actor {
			return assistantconversation.StatusPage{}, assistant.ErrDenied
		}
		page, err := handler.statuses.Status(work, actor, id, after)
		if work.Err() != nil {
			return assistantconversation.StatusPage{}, assistant.ErrUnavailable
		}
		return page, err
	}
	page, err := read()
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	batch, anchor, err := assistantStatusBatch(page, actor, id, after)
	if err != nil {
		writeAssistantProblem(w, r, err)
		return
	}
	if err := assistantStatusDeadline(w, ctx); err != nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	// The server writes its final chunk after this handler returns. An expired
	// subscription deadline must not truncate that terminator. Permit only this
	// bounded transport completion; no further metadata reads or frames occur.
	defer func() { _ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(time.Second)) }()
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Vary", "Cookie")
	w.WriteHeader(http.StatusOK)
	write := func(body []byte) error {
		if err := assistantStatusDeadline(w, ctx); err != nil {
			return err
		}
		if len(body) == 0 {
			body = []byte(": ready\n\n")
		}
		written, err := w.Write(body)
		if err != nil {
			return err
		}
		if written != len(body) {
			return io.ErrShortWrite
		}
		return http.NewResponseController(w).Flush()
	}
	advance := func() {
		if len(page.Events) > 0 {
			after = page.Events[len(page.Events)-1].Sequence
		}
	}
	if write(batch) != nil {
		return
	}
	advance()
	if page.Reset != "" || page.Run.State.Terminal() && after >= page.Run.Version {
		return
	}
	lastUpdated := page.Run.UpdatedAt
	poll := time.NewTicker(timers.poll)
	defer poll.Stop()
	keepalive := time.NewTicker(timers.keepalive)
	defer keepalive.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-keepalive.C:
			if write([]byte(": keepalive\n\n")) != nil {
				return
			}
		case <-poll.C:
			page, err = read()
			if err != nil {
				return
			}
			var metadata assistantRunJSON
			batch, metadata, err = assistantStatusBatch(page, actor, id, after)
			if err != nil || metadata.ID != anchor.ID || metadata.ConversationID != anchor.ConversationID || metadata.Goal != anchor.Goal || metadata.Scope != anchor.Scope || metadata.CreatedAt != anchor.CreatedAt || metadata.Deadline != anchor.Deadline || metadata.Version < anchor.Version || page.Run.UpdatedAt.Before(lastUpdated) || metadata.Version == anchor.Version && (metadata.State != anchor.State || metadata.UpdatedAt != anchor.UpdatedAt) {
				return
			}
			anchor = metadata
			lastUpdated = page.Run.UpdatedAt
			if len(batch) > 0 && write(batch) != nil {
				return
			}
			advance()
			if page.Reset != "" || page.Run.State.Terminal() && after >= page.Run.Version {
				return
			}
		}
	}
}

func assistantStatusDeadline(w http.ResponseWriter, ctx context.Context) error {
	if ctx.Err() != nil {
		return assistant.ErrUnavailable
	}
	deadline := time.Now().Add(5 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	return http.NewResponseController(w).SetWriteDeadline(deadline)
}

// Build and validate the whole bounded batch before the first status is written.
// No producer queue or content snapshot exists in this transport.
func assistantStatusBatch(page assistantconversation.StatusPage, actor assistant.Actor, id string, after int64) ([]byte, assistantRunJSON, error) {
	metadata, err := assistantMetadata(page.Run, actor.UserID, id)
	if err != nil {
		return nil, assistantRunJSON{}, assistant.ErrUnavailable
	}
	var output bytes.Buffer
	appendFrame := func(sequence int64, event string, body any) error {
		data, err := json.Marshal(body)
		if err != nil {
			return assistant.ErrUnavailable
		}
		var frame []byte
		if sequence > 0 {
			frame = fmt.Appendf(frame, "id: %d\n", sequence)
		}
		frame = fmt.Appendf(frame, "event: %s\ndata: %s\n\n", event, data)
		if len(frame) > 16*1024 || output.Len()+len(frame) > assistantStatusMaximumBatch {
			return assistant.ErrQuota
		}
		_, _ = output.Write(frame)
		return nil
	}
	if page.Reset != "" {
		if !page.Reset.Valid() || len(page.Events) != 0 {
			return nil, assistantRunJSON{}, assistant.ErrUnavailable
		}
		err = appendFrame(0, "assistant.reset.v1", struct {
			RunID  string                            `json:"run_id"`
			Reason assistantconversation.ResetReason `json:"reason"`
		}{id, page.Reset})
		return output.Bytes(), metadata, err
	}
	if len(page.Events) > assistant.MaximumStatusVersions || after > page.Run.Version || after == 0 && len(page.Events) != 1 || len(page.Events) == 0 && after < page.Run.Version {
		return nil, assistantRunJSON{}, assistant.ErrUnavailable
	}
	sequence, version := after, int64(0)
	var previous time.Time
	for _, event := range page.Events {
		if event.Sequence <= sequence || sequence != 0 && event.Sequence != sequence+1 || event.Sequence > assistant.MaximumStatusVersions || event.Sequence > page.Run.Version || event.RunVersion < 1 || event.RunVersion > page.Run.Version || event.RunVersion < version || event.State != assistant.Queued && event.State != assistant.Running && !event.State.Terminal() || !assistant.Timestamp(event.At) || event.At.Before(page.Run.CreatedAt) || event.At.After(page.Run.UpdatedAt) || !previous.IsZero() && event.At.Before(previous) {
			return nil, assistantRunJSON{}, assistant.ErrUnavailable
		}
		if event.Sequence == page.Run.Version && (event.RunVersion != page.Run.Version || event.State != page.Run.State || !event.At.Equal(page.Run.UpdatedAt)) {
			return nil, assistantRunJSON{}, assistant.ErrUnavailable
		}
		if after == 0 && event.Sequence != page.Run.Version {
			return nil, assistantRunJSON{}, assistant.ErrUnavailable
		}
		err = appendFrame(event.Sequence, "assistant.status.v1", struct {
			RunID    string          `json:"run_id"`
			Sequence int64           `json:"sequence"`
			Version  int64           `json:"version"`
			State    assistant.State `json:"state"`
			At       string          `json:"at"`
		}{id, event.Sequence, event.RunVersion, event.State, event.At.UTC().Format(time.RFC3339Nano)})
		if err != nil {
			return nil, assistantRunJSON{}, assistant.ErrUnavailable
		}
		sequence, version, previous = event.Sequence, event.RunVersion, event.At
	}
	return output.Bytes(), metadata, nil
}

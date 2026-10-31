package assistantconversation

import (
	"context"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

type StatusRepository interface {
	AssistantEvents(context.Context, assistant.Actor, string, int64, int, time.Time) ([]assistant.Event, error)
}

type ResetReason string

const (
	ResetReplayGap     ResetReason = "replay_gap"
	ResetCursorAhead   ResetReason = "cursor_ahead"
	ResetReplayExpired ResetReason = "replay_expired"
	ResetQueueOverflow ResetReason = "queue_overflow"
)

func (reason ResetReason) Valid() bool {
	return reason == ResetReplayGap || reason == ResetCursorAhead || reason == ResetReplayExpired || reason == ResetQueueOverflow
}

// StatusPage contains internal metadata, never a content snapshot. Only explicit
// whitelisted frame publication may serialize it at the transport boundary.
type StatusPage struct {
	Run    assistant.Run
	Events []assistant.Event
	Reset  ResetReason
}

func (StatusPage) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected status page]")
}
func (StatusPage) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

// Status performs no volatile-content, evidence, provider or processor read.
// Every page re-enters current durable owner/session/source/profile authority.
func (service *Service) Status(ctx context.Context, actor assistant.Actor, id string, after int64) (StatusPage, error) {
	if ctx == nil || ctx.Err() != nil || !actor.Valid() || len(id) != 67 || id[:3] != "ar_" || !ValidNonce(id[3:]) || after < 0 {
		return StatusPage{}, assistant.ErrInvalid
	}
	if !service.open() {
		return StatusPage{}, assistant.ErrUnavailable
	}
	repository, ok := service.repository.(StatusRepository)
	if !ok {
		return StatusPage{}, assistant.ErrUnavailable
	}
	work, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	at := service.now().UTC().Truncate(time.Microsecond)
	if !assistant.Timestamp(at) {
		return StatusPage{}, assistant.ErrUnavailable
	}
	run, err := service.repository.GetAssistant(work, actor, id, at)
	if work.Err() != nil {
		return StatusPage{}, assistant.ErrUnavailable
	}
	if err != nil {
		return StatusPage{}, ownedError(err)
	}
	if !statusRun(run, actor, id) {
		return StatusPage{}, assistant.ErrUnavailable
	}
	page := StatusPage{Run: run}
	if after == 0 {
		page.Events = []assistant.Event{{Sequence: run.Version, RunVersion: run.Version, State: run.State, At: run.UpdatedAt}}
	} else if after > run.Version {
		page.Reset = ResetCursorAhead
	} else {
		events, err := repository.AssistantEvents(work, actor, id, after, assistant.MaximumStatusVersions+1, at)
		if work.Err() != nil {
			return StatusPage{}, assistant.ErrUnavailable
		}
		if err == assistant.ErrConflict {
			page.Reset = ResetReplayGap
			if run.UpdatedAt.Before(at.Add(-assistant.ReplayRetention)) {
				page.Reset = ResetReplayExpired
			}
		} else if err != nil {
			return StatusPage{}, ownedError(err)
		} else if len(events) > assistant.MaximumStatusVersions {
			page.Reset = ResetQueueOverflow
		} else {
			// Progress can commit between the two reads. Recheck current authority
			// and the upper publication bound after obtaining its retained events.
			current, err := service.repository.GetAssistant(work, actor, id, service.now().UTC().Truncate(time.Microsecond))
			if work.Err() != nil {
				return StatusPage{}, assistant.ErrUnavailable
			}
			if err != nil {
				return StatusPage{}, ownedError(err)
			}
			if !statusRun(current, actor, id) || !sameStatusIdentity(current, run) || current.Version < run.Version || current.UpdatedAt.Before(run.UpdatedAt) {
				return StatusPage{}, assistant.ErrUnavailable
			}
			page.Run = current
			sequence, version := after, int64(0)
			var previous time.Time
			for _, event := range events {
				if event.Sequence <= sequence || event.Sequence > assistant.MaximumStatusVersions || event.RunVersion < 1 || event.RunVersion > current.Version || event.RunVersion < version || !statusState(event.State) || !assistant.Timestamp(event.At) || event.At.Before(current.CreatedAt) || event.At.After(current.UpdatedAt) || !previous.IsZero() && event.At.Before(previous) {
					return StatusPage{}, assistant.ErrUnavailable
				}
				if event.Sequence != sequence+1 {
					page.Reset = ResetReplayGap
					break
				}
				if event.Sequence == current.Version && (event.RunVersion != current.Version || event.State != current.State || !event.At.Equal(current.UpdatedAt)) {
					return StatusPage{}, assistant.ErrUnavailable
				}
				sequence, version, previous = event.Sequence, event.RunVersion, event.At
			}
			if sequence < run.Version {
				page.Reset = ResetReplayGap
			}
			if page.Reset == "" {
				page.Events = slices.Clone(events)
			}
		}
	}
	if !service.open() {
		return StatusPage{}, assistant.ErrUnavailable
	}
	return page, nil
}

func sameStatusIdentity(a, b assistant.Run) bool {
	return a.ID == b.ID && a.ConversationID == b.ConversationID && a.OwnerID == b.OwnerID && a.SourceID == b.SourceID && a.ResourceID == b.ResourceID && a.Goal == b.Goal && a.SourceVersion == b.SourceVersion && a.RestoreGeneration == b.RestoreGeneration && a.PolicyVersion == b.PolicyVersion && a.ProfileID == b.ProfileID && a.ProfileVersion == b.ProfileVersion && a.ConfigurationVersion == b.ConfigurationVersion && a.Volatile == b.Volatile && a.Paid == b.Paid && a.CreatedAt.Equal(b.CreatedAt) && a.From.Equal(b.From) && a.Until.Equal(b.Until) && a.Deadline.Equal(b.Deadline)
}

func statusState(state assistant.State) bool {
	return state == assistant.Queued || state == assistant.Running || state.Terminal()
}
func statusRun(run assistant.Run, actor assistant.Actor, id string) bool {
	return run.ID == id && run.OwnerID == actor.UserID && run.Version >= 1 && run.Version <= assistant.MaximumStatusVersions && statusState(run.State) && assistant.Timestamp(run.CreatedAt) && assistant.Timestamp(run.UpdatedAt) && !run.UpdatedAt.Before(run.CreatedAt)
}

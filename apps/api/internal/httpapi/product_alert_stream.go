package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/realtime"
)

const (
	streamReadTimeout       = 10 * time.Second
	alertStreamPageSize     = 100
	alertStreamQueueSize    = 16
	alertStreamMaximumFrame = 256 * 1024
)

type alertChangeResponse struct {
	ID         string                 `json:"id"`
	SourceID   domain.SourceID        `json:"source_id"`
	RuleID     domain.AlertRuleID     `json:"rule_id"`
	AlertID    *domain.AlertID        `json:"alert_id,omitempty"`
	Kind       domain.AlertChangeKind `json:"kind"`
	OccurredAt string                 `json:"occurred_at"`
}

// Alert IDs come from committed durable changes; process restarts preserve replay.
func (handler productHTTP) durableAlertStream(w http.ResponseWriter, r *http.Request, source domain.SourceID, reader domain.AlertChangeReader, after int64) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	initial, next, err := handler.initialAlertFrames(ctx, source, reader, after)
	if err != nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Vary", "Cookie")
	w.WriteHeader(http.StatusOK)
	for _, frame := range initial {
		if writeSSE(w, frame.ID, frame.Event, frame.Data) != nil {
			return
		}
	}
	if http.NewResponseController(w).Flush() != nil {
		return
	}
	queue := make(chan realtime.Frame, alertStreamQueueSize)
	done := make(chan struct{})
	terminal := make(chan string, 1)
	go func() {
		defer close(done)
		defer close(queue)
		terminal <- handler.produceAlertChanges(ctx, source, reader, next, queue, streamPollInterval)
	}()
	defer func() { cancel(); <-done }()
	keepalive := time.NewTicker(streamKeepalive)
	authCheck := time.NewTicker(streamAuthCheck)
	lifetime := time.NewTimer(streamMaximumLife)
	defer keepalive.Stop()
	defer authCheck.Stop()
	defer lifetime.Stop()
	sessionToken, _ := readCookieValue(r, sessionCookieName)
	for {
		select {
		case <-ctx.Done():
			return
		case <-lifetime.C:
			_ = writeSSE(w, 0, "stream.reset.v1", []byte(`{"reason":"lifetime"}`))
			return
		case <-authCheck.C:
			if handler.auth.guestDemo != nil {
				continue
			}
			readContext, cancelRead := context.WithTimeout(ctx, streamReadTimeout)
			_, err := handler.auth.service.Authenticate(readContext, sessionToken)
			cancelRead()
			if err != nil {
				return
			}
		case <-keepalive.C:
			if writeKeepalive(w) != nil {
				return
			}
		case frame, open := <-queue:
			if !open {
				reason := <-terminal
				if reason != "" {
					data, _ := json.Marshal(struct {
						Reason string `json:"reason"`
					}{Reason: reason})
					_ = writeSSE(w, 0, "stream.reset.v1", data)
				}
				return
			}
			if writeSSE(w, frame.ID, frame.Event, frame.Data) != nil {
				return
			}
		}
	}
}

func (handler productHTTP) initialAlertFrames(ctx context.Context, source domain.SourceID, reader domain.AlertChangeReader, after int64) ([]realtime.Frame, int64, error) {
	readContext, cancel := context.WithTimeout(ctx, streamReadTimeout)
	defer cancel()
	if after > 0 {
		page, err := reader.ListAlertChanges(readContext, source, after, alertStreamPageSize)
		if err != nil {
			return nil, 0, err
		}
		if !page.Reset {
			frame, err := alertChangesFrame(source, after, page)
			if err != nil {
				return nil, 0, err
			}
			if len(page.Changes) == 0 {
				return nil, after, nil
			}
			return []realtime.Frame{frame}, page.NextID, nil
		}
	}
	// Read the cursor before the snapshot: concurrent commits may be repeated but
	// are never skipped. Snapshot pages remain explicitly bounded and pageable.
	position, err := reader.AlertChangePosition(readContext, source)
	if err != nil || position < 0 {
		return nil, 0, domain.ErrRepositoryUnavailable
	}
	page, err := handler.alertReader.ListAlerts(readContext, source, domain.PageRequest{Limit: alertStreamPageSize})
	if err != nil || page.ValidateFor(source, domain.PageRequest{Limit: alertStreamPageSize}) != nil {
		return nil, 0, domain.ErrRepositoryUnavailable
	}
	data, err := json.Marshal(streamEnvelope{Version: 1, Topic: streamAlerts, ObservedAt: authTimestamp(time.Now().UTC()), Data: alertPageJSON(page.Alerts, page.Page)})
	if err != nil || len(data) > alertStreamMaximumFrame {
		return nil, 0, domain.ErrRepositoryUnavailable
	}
	frames := make([]realtime.Frame, 0, 2)
	if after > 0 {
		frames = append(frames, realtime.Frame{Event: "stream.reset.v1", Data: []byte(`{"reason":"replay_gap"}`)})
	}
	frames = append(frames, realtime.Frame{ID: uint64(position), Event: "alerts.snapshot.v1", Data: data})
	return frames, position, nil
}

func (handler productHTTP) produceAlertChanges(ctx context.Context, source domain.SourceID, reader domain.AlertChangeReader, after int64, queue chan<- realtime.Frame, interval time.Duration) string {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		readContext, cancel := context.WithTimeout(ctx, streamReadTimeout)
		page, err := reader.ListAlertChanges(readContext, source, after, alertStreamPageSize)
		cancel()
		if err != nil {
			return "source_unavailable"
		}
		if page.Reset {
			frames, next, err := handler.initialAlertFrames(ctx, source, reader, after)
			if err != nil {
				return "source_unavailable"
			}
			for _, frame := range frames {
				if !enqueueAlertFrame(queue, frame) {
					return "backpressure"
				}
			}
			after = next
		} else if len(page.Changes) > 0 {
			frame, err := alertChangesFrame(source, after, page)
			if err != nil {
				return "source_unavailable"
			}
			if !enqueueAlertFrame(queue, frame) {
				return "backpressure"
			}
			after = page.NextID
			// Drain a bounded page each turn. Queue pressure ends this subscription.
			if len(page.Changes) == alertStreamPageSize {
				continue
			}
		}
		select {
		case <-ctx.Done():
			return ""
		case <-ticker.C:
		}
	}
}

func enqueueAlertFrame(queue chan<- realtime.Frame, frame realtime.Frame) bool {
	select {
	case queue <- frame:
		return true
	default:
		return false
	}
}

func alertChangesFrame(source domain.SourceID, after int64, page domain.AlertChangePage) (realtime.Frame, error) {
	if page.Reset || len(page.Changes) > alertStreamPageSize || page.NextID < after {
		return realtime.Frame{}, errors.New("invalid alert changes")
	}
	items := make([]alertChangeResponse, len(page.Changes))
	previous := after
	now := time.Now().UTC()
	for index, change := range page.Changes {
		if change.ID <= previous || change.SourceID != source || change.SourceID.Validate() != nil || change.RuleID.Validate() != nil || change.OccurredAt.IsZero() ||
			change.OccurredAt.Location() != time.UTC || change.OccurredAt.Nanosecond()%int(time.Microsecond) != 0 || change.OccurredAt.After(now) ||
			(change.AlertID != nil && change.AlertID.Validate() != nil) {
			return realtime.Frame{}, errors.New("invalid alert change")
		}
		switch change.Kind {
		case domain.AlertChangeAlert, domain.AlertChangeRuleCreated, domain.AlertChangeRuleChanged, domain.AlertChangeRuleDeleted:
		default:
			return realtime.Frame{}, errors.New("invalid alert change kind")
		}
		previous = change.ID
		if change.Kind == domain.AlertChangeAlert && change.AlertID == nil {
			return realtime.Frame{}, errors.New("missing alert change target")
		}
		items[index] = alertChangeResponse{ID: strconv.FormatInt(change.ID, 10), SourceID: source, RuleID: change.RuleID, AlertID: change.AlertID, Kind: change.Kind, OccurredAt: authTimestamp(change.OccurredAt)}
	}
	if len(items) > 0 && previous != page.NextID {
		return realtime.Frame{}, errors.New("invalid alert position")
	}
	data, err := json.Marshal(streamEnvelope{Version: 1, Topic: streamAlerts, ObservedAt: authTimestamp(time.Now().UTC()), Data: items})
	if err != nil || len(data) > alertStreamMaximumFrame {
		return realtime.Frame{}, errors.New("invalid alert frame size")
	}
	return realtime.Frame{ID: uint64(page.NextID), Event: "alerts.changed.v1", Data: data}, nil
}

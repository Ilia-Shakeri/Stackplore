package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/realtime"
)

const (
	streamPollInterval = 2 * time.Second
	streamKeepalive    = 15 * time.Second
	streamAuthCheck    = 30 * time.Second
	streamMaximumLife  = 30 * time.Minute
	streamWriteTimeout = 5 * time.Second
)

type streamTopic string

const (
	streamHealth  streamTopic = "health"
	streamMetrics streamTopic = "metrics"
	streamEvents  streamTopic = "events"
	streamLogs    streamTopic = "logs"
	streamAlerts  streamTopic = "alerts"
)

type streamEnvelope struct {
	Version    int         `json:"version"`
	Topic      streamTopic `json:"topic"`
	ObservedAt string      `json:"observed_at"`
	Data       any         `json:"data"`
}

type streamGate struct {
	mu      sync.Mutex
	maximum int
	perUser int
	active  int
	byUser  map[auth.UserID]int
}

func newStreamGate(maximum, perUser int) *streamGate {
	return &streamGate{maximum: maximum, perUser: perUser, byUser: make(map[auth.UserID]int)}
}

func (gate *streamGate) acquire(userID auth.UserID) (func(), bool) {
	if gate == nil || userID.Validate() != nil {
		return nil, false
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.active >= gate.maximum || gate.byUser[userID] >= gate.perUser {
		return nil, false
	}
	gate.active++
	gate.byUser[userID]++
	var once sync.Once
	return func() {
		once.Do(func() {
			gate.mu.Lock()
			gate.active--
			gate.byUser[userID]--
			if gate.byUser[userID] == 0 {
				delete(gate.byUser, userID)
			}
			gate.mu.Unlock()
		})
	}, true
}

func (handler productHTTP) streamHealth(w http.ResponseWriter, r *http.Request) {
	handler.stream(w, r, streamHealth)
}

func (handler productHTTP) streamMetrics(w http.ResponseWriter, r *http.Request) {
	handler.stream(w, r, streamMetrics)
}

func (handler productHTTP) streamEvents(w http.ResponseWriter, r *http.Request) {
	handler.stream(w, r, streamEvents)
}

func (handler productHTTP) streamLogs(w http.ResponseWriter, r *http.Request) {
	handler.stream(w, r, streamLogs)
}

func (handler productHTTP) streamAlerts(w http.ResponseWriter, r *http.Request) {
	handler.stream(w, r, streamAlerts)
}

func (handler productHTTP) stream(w http.ResponseWriter, r *http.Request, topic streamTopic) {
	if !requestHasNoBody(r) || !requestHasOnlySourceQuery(r) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	identity, ok := handler.authenticateIdentity(w, r)
	if !ok {
		return
	}
	release, ok := handler.streamGate.acquire(identity.User.ID)
	if !ok {
		writeProblem(w, r, rateLimitProblem)
		return
	}
	defer release()
	provider, err := handler.providerFor(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	lastID, err := parseLastEventID(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	initial, err := buildStreamSnapshot(r.Context(), topic, provider)
	if err != nil {
		var providerError *domain.ProviderError
		if errors.As(err, &providerError) {
			writeProviderProblem(w, r, err)
		} else {
			writeProblem(w, r, unavailableProblem)
		}
		return
	}
	producer := streamProducer(topic, provider, initial)
	key := string(topic) + "/" + string(provider.Source().ID)
	subscription, err := handler.streams.Subscribe(key, lastID, producer)
	if err != nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	defer subscription.Close()
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Vary", "Cookie")
	w.WriteHeader(http.StatusOK)
	if subscription.Reset {
		if err := writeSSE(w, 0, "stream.reset.v1", []byte(`{"reason":"replay_gap"}`)); err != nil {
			return
		}
	}
	keepalive := time.NewTicker(streamKeepalive)
	authCheck := time.NewTicker(streamAuthCheck)
	lifetime := time.NewTimer(streamMaximumLife)
	defer keepalive.Stop()
	defer authCheck.Stop()
	defer lifetime.Stop()
	sessionToken, _ := readCookieValue(r, sessionCookieName)
	for {
		select {
		case <-r.Context().Done():
			return
		case <-lifetime.C:
			_ = writeSSE(w, 0, "stream.reset.v1", []byte(`{"reason":"lifetime"}`))
			return
		case <-authCheck.C:
			if _, err := handler.auth.service.Authenticate(r.Context(), sessionToken); err != nil {
				return
			}
		case <-keepalive.C:
			if err := writeKeepalive(w); err != nil {
				return
			}
		case frame, open := <-subscription.C:
			if !open || writeSSE(w, frame.ID, frame.Event, frame.Data) != nil {
				return
			}
		}
	}
}

func streamProducer(topic streamTopic, provider ProductProvider, initial []byte) realtime.Producer {
	return func(ctx context.Context, publish func(string, []byte) error) {
		last := bytes.Clone(initial)
		sourceUnavailable := false
		if publish(string(topic)+".snapshot.v1", last) != nil {
			return
		}
		ticker := time.NewTicker(streamPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				data, err := buildStreamSnapshot(ctx, topic, provider)
				if err != nil {
					if !sourceUnavailable {
						_ = publish("stream.reset.v1", []byte(`{"reason":"source_unavailable"}`))
						sourceUnavailable = true
					}
					continue
				}
				sourceUnavailable = false
				if !bytes.Equal(data, last) {
					if publish(string(topic)+".snapshot.v1", data) != nil {
						return
					}
					last = bytes.Clone(data)
				}
			}
		}
	}
}

func buildStreamSnapshot(ctx context.Context, topic streamTopic, provider ProductProvider) ([]byte, error) {
	var refs []domain.ResourceRef
	var resources map[domain.ResourceID]bool
	if topic != streamAlerts {
		limit := domain.MaxHealthTargets
		switch topic {
		case streamLogs:
			limit = domain.MaxLogTargets
		case streamMetrics:
			limit = domain.MaxMetricTargets
		case streamEvents:
			limit = domain.MaxEventTargets
		}
		var err error
		refs, resources, err = streamResources(ctx, provider, limit)
		if err != nil {
			return nil, err
		}
	}
	observedAt := observationTime(provider)
	var data any
	switch topic {
	case streamHealth:
		result, err := provider.GetHealth(ctx, domain.HealthQuery{Source: providerRef(provider), Resources: refs})
		if err != nil {
			return nil, err
		}
		items := make([]healthResponse, len(result.Snapshots))
		for index, health := range result.Snapshots {
			reasons := make([]string, len(health.Evidence))
			for evidenceIndex := range health.Evidence {
				reasons[evidenceIndex] = string(health.Evidence[evidenceIndex].Code)
			}
			items[index] = healthResponse{
				ResourceID: health.ResourceID, State: string(health.State), ObservedAt: authTimestamp(health.Status.ObservedAt),
				Synthetic: resources[health.ResourceID], Reasons: reasons,
			}
		}
		observedAt, data = result.Status.ObservedAt, items
	case streamMetrics:
		metricProvider, ok := provider.(domain.CurrentMetricProvider)
		if !ok {
			return nil, errors.New("metric stream unsupported")
		}
		names := []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes, domain.MetricRestartCount}
		result, err := metricProvider.GetCurrentMetrics(ctx, domain.CurrentMetricQuery{Source: providerRef(provider), Resources: refs, Names: names})
		if err != nil {
			return nil, err
		}
		items := make([]metricResponse, 0, len(refs)*len(names))
		for _, set := range result.Sets {
			for _, sample := range set.Samples {
				items = append(items, metricResponse{
					ID: sample.ID, ResourceID: sample.ResourceID, Name: string(sample.Name), Unit: string(sample.Unit), Value: sample.Value,
					SampledAt: authTimestamp(sample.SampledAt), IntervalSeconds: int64(sample.Interval / time.Second), Synthetic: sample.Synthetic,
				})
			}
		}
		observedAt, data = result.Status.ObservedAt, items
	case streamEvents:
		eventProvider, ok := provider.(domain.EventHistoryProvider)
		if !ok {
			return nil, errors.New("event stream unsupported")
		}
		end := observationTime(provider).Add(time.Nanosecond)
		result, err := eventProvider.ListEvents(ctx, domain.EventQuery{
			Source: providerRef(provider), Resources: refs, Range: domain.TimeRange{Start: end.Add(-24 * time.Hour), End: end},
			Page: domain.PageRequest{Limit: 100},
		})
		if err != nil {
			return nil, err
		}
		items := make([]eventResponse, len(result.Events))
		for index, event := range result.Events {
			items[index] = eventResponse{
				ID: event.ID, ResourceID: event.ResourceID, EnvironmentID: event.EnvironmentID, Kind: string(event.Kind),
				Severity: string(event.Severity), ReasonCode: string(event.ReasonCode), Summary: event.Summary,
				OccurredAt: authTimestamp(event.OccurredAt), ObservedAt: authTimestamp(event.ObservedAt), Synthetic: event.Synthetic,
			}
		}
		observedAt, data = result.Status.ObservedAt, items
	case streamLogs:
		logProvider, ok := provider.(domain.LogReadProvider)
		if !ok {
			return nil, errors.New("log stream unsupported")
		}
		targets := make([]domain.LogTarget, len(refs))
		for index, ref := range refs {
			targets[index] = domain.LogTarget{Resource: ref}
		}
		end := observationTime(provider).Add(time.Nanosecond)
		result, err := logProvider.ReadLogs(ctx, domain.LogReadQuery{
			Source: providerRef(provider), Targets: targets, Range: domain.TimeRange{Start: end.Add(-24 * time.Hour), End: end},
			LineLimit: 100, ByteLimit: domain.MaxLogBytes,
		})
		if err != nil {
			return nil, err
		}
		items := make([]logResponse, len(result.Records))
		for index, record := range result.Records {
			items[index] = logResponse{
				ID: record.ID, ResourceID: record.ResourceID, Stream: string(record.Stream), Sequence: record.Sequence,
				Severity: string(record.Severity), Timestamp: authTimestamp(record.Timestamp), Message: record.Message,
				Redacted: record.Redacted, Truncated: record.Truncated, Synthetic: record.Synthetic,
			}
		}
		observedAt, data = result.Status.ObservedAt, items
	case streamAlerts:
		alertProvider, ok := provider.(interface{ Alerts() []domain.Alert })
		if !ok {
			return nil, errors.New("alert stream unsupported")
		}
		alerts := alertProvider.Alerts()
		items := make([]alertResponse, len(alerts))
		for index, alert := range alerts {
			items[index] = alertResponse{
				ID: alert.ID, RuleID: alert.RuleID, ResourceID: alert.ResourceID, EnvironmentID: alert.EnvironmentID,
				State: string(alert.State), Severity: string(alert.Severity), SummaryCode: string(alert.SummaryCode),
				StartedAt: authTimestamp(alert.StartedAt), UpdatedAt: authTimestamp(alert.UpdatedAt), Synthetic: alert.Synthetic,
			}
		}
		data = items
	default:
		return nil, errors.New("stream topic unsupported")
	}
	return json.Marshal(streamEnvelope{Version: 1, Topic: topic, ObservedAt: authTimestamp(observedAt), Data: data})
}

func streamResources(ctx context.Context, provider ProductProvider, limit int) ([]domain.ResourceRef, map[domain.ResourceID]bool, error) {
	page, err := provider.ListResources(ctx, domain.InventoryQuery{
		Source: providerRef(provider), SortField: domain.ResourceSortName, SortDirection: domain.SortAscending,
		Page: domain.PageRequest{Limit: limit},
	})
	if err != nil {
		return nil, nil, err
	}
	refs := make([]domain.ResourceRef, len(page.Resources))
	synthetic := make(map[domain.ResourceID]bool, len(page.Resources))
	for index, item := range page.Resources {
		refs[index] = item.Resource.Ref
		synthetic[item.Resource.Ref.ID] = item.Resource.Synthetic
	}
	return refs, synthetic, nil
}

func parseLastEventID(r *http.Request) (uint64, error) {
	values := r.Header.Values("Last-Event-ID")
	if len(values) == 0 {
		return 0, nil
	}
	if len(values) != 1 || values[0] == "" || len(values[0]) > 20 || len(values[0]) > 1 && values[0][0] == '0' {
		return 0, errors.New("invalid last event ID")
	}
	return strconv.ParseUint(values[0], 10, 64)
}

func writeSSE(w http.ResponseWriter, id uint64, event string, data []byte) error {
	if !json.Valid(data) || event == "" || bytes.ContainsAny(data, "\r\n") {
		return errors.New("invalid SSE frame")
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(streamWriteTimeout))
	if id > 0 {
		if _, err := fmt.Fprintf(w, "id: %d\n", id); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	return http.NewResponseController(w).Flush()
}

func writeKeepalive(w http.ResponseWriter) error {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(streamWriteTimeout))
	if _, err := w.Write([]byte(": keepalive\n\n")); err != nil {
		return err
	}
	return http.NewResponseController(w).Flush()
}

package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type streamAuthorityProvider struct {
	*demo.Provider
	mu              sync.Mutex
	denied          bool
	inventoryDenied bool
	typedDenial     bool
	generation      int
	state           domain.AvailabilityState
	partial         bool
}

func (provider *streamAuthorityProvider) set(denied, inventoryDenied, typedDenial bool, generation int) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.denied, provider.inventoryDenied, provider.typedDenial, provider.generation = denied, inventoryDenied, typedDenial, generation
}

func (provider *streamAuthorityProvider) status(inventory bool) (domain.ObservationStatus, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	status := explorationQueryStatus()
	status.ObservedAt = status.ObservedAt.Add(time.Duration(provider.generation) * time.Microsecond)
	if !inventory && provider.state != "" {
		status.State = provider.state
		status.ReasonCode = "provider.fixture_state"
		if provider.partial {
			status.Completeness = domain.CompletenessPartial
		}
	}
	if provider.denied && !inventory || provider.inventoryDenied && inventory {
		status.State, status.ReasonCode = domain.AvailabilityPermissionDenied, "provider.permission_denied"
		if provider.typedDenial {
			failure, _ := domain.NewProviderError("fixture.stream.read", domain.ProviderErrorPermissionDenied, false, errors.New("fixture permission denied"))
			return status, failure
		}
	}
	return status, nil
}

func (provider *streamAuthorityProvider) ListResources(ctx context.Context, query domain.InventoryQuery) (domain.ResourcePage, error) {
	status, err := provider.status(true)
	if err != nil || status.State == domain.AvailabilityPermissionDenied {
		result := domain.ResourcePage{Status: status, Page: domain.PageInfo{Limit: query.Page.Limit}}
		if err == nil {
			err = result.ValidateFor(query)
		}
		return result, err
	}
	return provider.Provider.ListResources(ctx, query)
}

func (provider *streamAuthorityProvider) GetHealth(ctx context.Context, query domain.HealthQuery) (domain.HealthResult, error) {
	status, err := provider.status(false)
	result := domain.HealthResult{Status: status}
	if err == nil && status.AllowsData() {
		result, err = provider.Provider.GetHealth(ctx, query)
		result.Status = status
	}
	if err == nil {
		err = result.ValidateFor(query)
	}
	return result, err
}

func (provider *streamAuthorityProvider) GetCurrentMetrics(ctx context.Context, query domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
	status, err := provider.status(false)
	result := domain.CurrentMetricResult{Status: status}
	if err == nil && status.AllowsData() {
		result, err = provider.Provider.GetCurrentMetrics(ctx, query)
		result.Status = status
	}
	if err == nil {
		err = result.ValidateFor(query)
	}
	return result, err
}

func (provider *streamAuthorityProvider) ListEvents(ctx context.Context, query domain.EventQuery) (domain.EventPage, error) {
	status, err := provider.status(false)
	result := domain.EventPage{Status: status, Page: domain.PageInfo{Limit: query.Page.Limit}}
	if err == nil && status.AllowsData() {
		result, err = provider.Provider.ListEvents(ctx, query)
		result.Status = status
	}
	if err == nil {
		err = result.ValidateFor(query)
	}
	return result, err
}

func (provider *streamAuthorityProvider) ReadLogs(ctx context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	status, err := provider.status(false)
	result := domain.LogPage{Status: status, Page: domain.PageInfo{Limit: query.LineLimit}}
	if err == nil && status.AllowsData() {
		result, err = provider.Provider.ReadLogs(ctx, query)
		result.Status = status
	}
	if err == nil {
		err = result.ValidateFor(query)
	}
	return result, err
}

type authorityHTTPStream struct {
	response *http.Response
	scanner  *bufio.Scanner
}

func startAuthorityHTTPStream(t *testing.T, server *httptest.Server, topic streamTopic, lastID string) *authorityHTTPStream {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/streams/"+string(topic), nil)
	if err != nil {
		t.Fatal("stream fixture request unavailable")
	}
	request.Host = "stackplore.test"
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: retainedEventMarker(t)})
	if lastID != "" {
		request.Header.Set("Last-Event-ID", lastID)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal("stream fixture HTTP request failed")
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 256*1024)
	return &authorityHTTPStream{response: response, scanner: scanner}
}

func (stream *authorityHTTPStream) next(t *testing.T) (uint64, string, []byte) {
	t.Helper()
	var id uint64
	var event string
	var data []byte
	for stream.scanner.Scan() {
		line := stream.scanner.Text()
		if line == "" && event != "" {
			return id, event, data
		}
		if value, ok := strings.CutPrefix(line, "id: "); ok {
			id, _ = strconv.ParseUint(value, 10, 64)
		} else if value, ok := strings.CutPrefix(line, "event: "); ok {
			event = value
		} else if value, ok := strings.CutPrefix(line, "data: "); ok {
			data = []byte(value)
		}
	}
	t.Fatal("expected stream frame unavailable")
	return 0, "", nil
}

func authorityStreamServer(t *testing.T, provider *streamAuthorityProvider) *httptest.Server {
	t.Helper()
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.ProductProviders = []ProductProvider{provider}
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{User: auth.User{ID: "usr_stream_authority"}}, nil
		}}
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func assertAuthoritySnapshot(t *testing.T, stream *authorityHTTPStream, topic streamTopic, expectedID uint64) {
	t.Helper()
	id, event, data := stream.next(t)
	var envelope struct {
		Data []json.RawMessage `json:"data"`
	}
	if stream.response.StatusCode != http.StatusOK || id != expectedID || event != string(topic)+".snapshot.v1" ||
		json.Unmarshal(data, &envelope) != nil || len(envelope.Data) == 0 {
		t.Fatal("authorized snapshot missing or wrong replay generation")
	}
}

func assertAuthorityResetAndClose(t *testing.T, stream *authorityHTTPStream) {
	t.Helper()
	_, event, data := stream.next(t)
	if event != "stream.reset.v1" || string(data) != `{"reason":"source_unavailable"}` {
		t.Fatal("permission loss did not reset active stream")
	}
	if stream.scanner.Scan() || stream.scanner.Err() != nil {
		t.Fatal("permission loss did not close active stream cleanly")
	}
}

func TestStreamsRejectDeniedReplayAndRecoverWithFreshFeed(t *testing.T) {
	for _, topic := range []streamTopic{streamHealth, streamMetrics, streamEvents, streamLogs} {
		t.Run(string(topic), func(t *testing.T) {
			t.Parallel()
			provider := &streamAuthorityProvider{Provider: demo.MustNew()}
			server := authorityStreamServer(t, provider)
			first := startAuthorityHTTPStream(t, server, topic, "")
			assertAuthoritySnapshot(t, first, topic, 1)
			provider.set(false, false, false, 1)
			assertAuthoritySnapshot(t, first, topic, 2)
			second := startAuthorityHTTPStream(t, server, topic, "1")
			assertAuthoritySnapshot(t, second, topic, 2)
			provider.set(true, false, false, 2)
			denied := startAuthorityHTTPStream(t, server, topic, "1")
			if denied.response.StatusCode != http.StatusForbidden || denied.response.Header.Get("Content-Type") != "application/problem+json" {
				t.Fatal("current permission denial allowed historical replay")
			}
			assertAuthorityResetAndClose(t, first)
			assertAuthorityResetAndClose(t, second)
			withoutID := startAuthorityHTTPStream(t, server, topic, "")
			if withoutID.response.StatusCode != http.StatusForbidden {
				t.Fatal("current permission denial allowed a fresh subscriber")
			}
			provider.set(false, false, false, 3)
			recovered := startAuthorityHTTPStream(t, server, topic, "1")
			_, event, data := recovered.next(t)
			if event != "stream.reset.v1" || string(data) != `{"reason":"replay_gap"}` {
				t.Fatal("permission recovery reused the previous replay ring")
			}
			assertAuthoritySnapshot(t, recovered, topic, 1)
		})
	}
}

func TestStreamSnapshotsKeepStalePartialAndOtherAvailability(t *testing.T) {
	for _, topic := range []streamTopic{streamHealth, streamMetrics, streamEvents, streamLogs} {
		for _, state := range []domain.AvailabilityState{domain.AvailabilityAvailable, domain.AvailabilityStale, domain.AvailabilityUnavailable, domain.AvailabilityUnsupported} {
			t.Run(string(topic)+"/"+string(state), func(t *testing.T) {
				provider := &streamAuthorityProvider{Provider: demo.MustNew(), state: state, partial: state == domain.AvailabilityAvailable || state == domain.AvailabilityStale}
				data, err := buildStreamSnapshot(context.Background(), topic, provider)
				var envelope struct {
					Data []json.RawMessage `json:"data"`
				}
				if err != nil || json.Unmarshal(data, &envelope) != nil || (len(envelope.Data) > 0) != (state == domain.AvailabilityAvailable || state == domain.AvailabilityStale) {
					t.Fatal("non-denied availability changed its existing snapshot behavior")
				}
			})
		}
	}
}

func TestStreamsPollDenialClosesActiveClientsAndAllowsFreshRecovery(t *testing.T) {
	for _, topic := range []streamTopic{streamHealth, streamMetrics, streamEvents, streamLogs} {
		t.Run(string(topic), func(t *testing.T) {
			t.Parallel()
			provider := &streamAuthorityProvider{Provider: demo.MustNew()}
			server := authorityStreamServer(t, provider)
			first := startAuthorityHTTPStream(t, server, topic, "")
			assertAuthoritySnapshot(t, first, topic, 1)
			provider.set(true, false, false, 1)
			assertAuthorityResetAndClose(t, first)
			provider.set(false, false, false, 2)
			recovered := startAuthorityHTTPStream(t, server, topic, "")
			assertAuthoritySnapshot(t, recovered, topic, 1)
		})
	}
}

func TestStreamsRejectInventoryAndTypedPermissionDenials(t *testing.T) {
	for _, topic := range []streamTopic{streamHealth, streamMetrics, streamEvents, streamLogs} {
		for _, inventory := range []bool{false, true} {
			for _, typed := range []bool{false, true} {
				name := string(topic) + "/inventory=" + strconv.FormatBool(inventory) + "/typed=" + strconv.FormatBool(typed)
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					provider := &streamAuthorityProvider{Provider: demo.MustNew()}
					provider.set(!inventory, inventory, typed, 0)
					response := startAuthorityHTTPStream(t, authorityStreamServer(t, provider), topic, "1").response
					if response.StatusCode != http.StatusForbidden {
						t.Fatal("current inventory or topic permission denial did not fail closed")
					}
				})
			}
		}
	}
}

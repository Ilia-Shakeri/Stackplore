package docker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestReadTransportRefusesAuthorityExpansionBeforeNetwork(t *testing.T) {
	base, _ := url.Parse("https://docker-proxy.example:8443")
	var calls atomic.Int32
	transport := &engineReadTransport{base: base, maximum: minimumEngineResponseBytes, next: rawRoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return engineResponse(http.StatusOK, `{}`), nil
	})}
	for _, test := range []struct{ method, path string }{
		{"POST", "/v1.56/containers/abc123/restart"}, {"DELETE", "/v1.56/containers/abc123"},
		{"GET", "/v1.56/containers/abc123/archive?path=/etc/passwd"}, {"GET", "/v1.56/containers/abc123/export"},
		{"GET", "/v1.56/containers/abc123/attach/ws"}, {"GET", "/v1.56/exec/abc123/json"},
		{"GET", "/v1.56/secrets"}, {"GET", "/v1.56/configs"}, {"GET", "/v1.56/images/get"},
		{"GET", "/v1.56/containers/abc123/logs?follow=1"}, {"GET", "/v1.56/containers/abc123/stats?stream=true"},
		{"GET", "/v1.56/containers/json?all=1&all=0"}, {"GET", "/v1.56/containers/json?all=1&filters=arbitrary"},
		{"GET", "/v1.56/containers/abc123%2f..%2fsecret/json"}, {"GET", "/v1.57/info"}, {"HEAD", "/v1.56/info"},
	} {
		t.Run(test.method+test.path, func(t *testing.T) {
			request, _ := http.NewRequest(test.method, base.String()+test.path, nil)
			if response, err := transport.RoundTrip(request); err == nil || response != nil {
				t.Fatal("unsafe request reached transport")
			}
		})
	}
	request, _ := http.NewRequest(http.MethodGet, "https://other.example/v1.56/info", nil)
	if _, err := transport.RoundTrip(request); err == nil {
		t.Fatal("different origin reached transport")
	}
	if calls.Load() != 0 {
		t.Fatal("rejected request made network call")
	}
}

type rawRoundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip rawRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func newTestCollector(t *testing.T, transport http.RoundTripper) *EngineCollector {
	t.Helper()
	collector, err := NewEngineCollector(EngineCollectorOptions{Client: &http.Client{Transport: transport}, BaseURL: "http://docker", MaximumItems: 100, MaximumResponse: minimumEngineResponseBytes, InspectConcurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = collector.Close() })
	return collector
}

func TestSDKNegotiatesAndIgnoresAmbientDockerTargets(t *testing.T) {
	t.Setenv("DOCKER_HOST", "tcp://other.example:2375")
	t.Setenv("DOCKER_API_VERSION", "1.99")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	var pings atomic.Int32
	collector := newTestCollector(t, rawRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "docker" {
			t.Fatal("ambient target used")
		}
		if request.URL.Path == "/_ping" && request.Method == http.MethodHead {
			pings.Add(1)
			response := engineResponse(http.StatusOK, "")
			response.Header.Set("Api-Version", "1.48")
			return response, nil
		}
		if request.URL.Path != "/v1.48/info" {
			t.Fatalf("wrong negotiated request: %s", request.URL.Path)
		}
		return engineResponse(http.StatusForbidden, `{"message":"untrusted upstream diagnostic"}`), nil
	}))
	if _, err := collector.Snapshot(context.Background()); !errors.Is(err, ErrPermissionDenied) || strings.Contains(err.Error(), "diagnostic") {
		t.Fatalf("unsafe error: %v", err)
	}
	if pings.Load() != 1 {
		t.Fatalf("negotiation count %d", pings.Load())
	}
}

func TestSDKRejectsOversizeTrailingAndRedirectResponses(t *testing.T) {
	for name, response := range map[string]*http.Response{
		"oversize": engineResponse(http.StatusOK, `{"Name":"`+strings.Repeat("x", minimumEngineResponseBytes)+`"}`),
		"trailing": engineResponse(http.StatusOK, `{"ID":"one"} {"ID":"two"}`),
		"redirect": engineResponse(http.StatusTemporaryRedirect, `{}`),
		"bad json": engineResponse(http.StatusOK, `{"ID":`),
	} {
		t.Run(name, func(t *testing.T) {
			collector := newTestCollector(t, roundTripFunc(func(*http.Request) (*http.Response, error) { return response, nil }))
			if _, err := collector.Snapshot(context.Background()); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}

func TestSnapshotKeepsAllowedPartitionsAndUnknownInspectionHealth(t *testing.T) {
	collector := newTestCollector(t, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/info":
			return engineResponse(http.StatusOK, `{"ID":"engine-one","Name":"Engine","ServerVersion":"29.0.0"}`), nil
		case "/containers/json":
			return engineResponse(http.StatusOK, `[{"Id":"abc123","Names":["/shop-api"],"State":"running","Labels":{"com.docker.compose.project":"shop","com.docker.compose.service":"api"}}]`), nil
		case "/containers/abc123/json":
			return engineResponse(http.StatusNotFound, `{"message":"gone"}`), nil
		case "/images/json":
			return engineResponse(http.StatusForbidden, `{}`), nil
		case "/networks":
			return engineResponse(http.StatusOK, `[]`), nil
		case "/volumes":
			return engineResponse(http.StatusOK, `{"Volumes":[]}`), nil
		default:
			t.Fatal("unexpected path")
			return nil, nil
		}
	}))
	provider, err := New("source_docker", "Docker", collector)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := provider.CaptureInventory(context.Background())
	if err != nil || capture.Complete || capture.Status.Completeness != domain.CompletenessPartial || len(capture.Coverage) != 7 {
		t.Fatalf("partial capture: %#v %v", capture, err)
	}
	found := false
	for _, partition := range capture.Coverage {
		if partition.Kind == "docker.image" && partition.Status.State != domain.AvailabilityPermissionDenied {
			t.Fatal("denial hidden")
		}
		if partition.Kind == "docker.network" && !partition.Complete() {
			t.Fatal("complete empty network not authoritative")
		}
	}
	for _, health := range capture.Health {
		for _, resource := range capture.Resources {
			if resource.Resource.Ref.ID == health.ResourceID && resource.Resource.Ref.Identity.Kind == "docker.container" {
				found = true
				if health.State != domain.HealthUnknown {
					t.Fatal("missing inspect became healthy")
				}
			}
		}
	}
	if !found {
		t.Fatal("allowed list item lost")
	}
}

func TestStatsMissingPriorSampleCannotBecomeZeroCPU(t *testing.T) {
	collector := newTestCollector(t, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/stats") {
			if request.URL.Query().Has("one-shot") || request.URL.Query().Get("stream") != "false" {
				t.Fatal("CPU previous sample disabled")
			}
			return engineResponse(http.StatusOK, `{"read":"`+time.Now().UTC().Format(time.RFC3339Nano)+`","cpu_stats":{"cpu_usage":{"total_usage":200},"system_cpu_usage":1000,"online_cpus":2},"memory_stats":{"usage":1000}}`), nil
		}
		return engineResponse(http.StatusOK, `{"RestartCount":0,"State":{"Status":"running"}}`), nil
	}))
	stats, err := collector.CurrentContainerStats(context.Background(), []string{"abc123"})
	if err != nil || len(stats) != 1 || !stats[0].CPUUnavailable {
		t.Fatalf("missing sample accepted: %#v %v", stats, err)
	}
	if _, _, supported := dockerMetricValue(stats[0], domain.MetricCPUUsageCores); supported {
		t.Fatal("zero CPU fabricated")
	}
	if value, _, supported := dockerMetricValue(stats[0], domain.MetricMemoryUsageBytes); !supported || value != 1000 {
		t.Fatal("valid independent memory lost")
	}
}

func TestReadBodyCapAndCancellationCloseUpstream(t *testing.T) {
	closed := false
	body := &closeProbe{Reader: strings.NewReader("123456"), close: func() { closed = true }}
	limited := &engineLimitedBody{ReadCloser: body, remaining: 3}
	if _, err := io.ReadAll(limited); !errors.Is(err, errEngineResponseLimit) || !closed {
		t.Fatal("body limit did not close source")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	collector := newTestCollector(t, rawRoundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("cancelled request reached network")
		return nil, nil
	}))
	if _, err := collector.Snapshot(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestObservationByteBudgetIsSharedAcrossConcurrentReads(t *testing.T) {
	budget := new(atomic.Int64)
	budget.Store(100)
	var consumed atomic.Int64
	done := make(chan struct{}, 8)
	for range 8 {
		go func() {
			body := &engineLimitedBody{ReadCloser: io.NopCloser(strings.NewReader(strings.Repeat("x", 100))), remaining: 100, budget: budget}
			data, _ := io.ReadAll(body)
			consumed.Add(int64(len(data)))
			done <- struct{}{}
		}()
	}
	for range 8 {
		<-done
	}
	if consumed.Load() > 100 || budget.Load() < 0 {
		t.Fatalf("shared budget exceeded: %d", consumed.Load())
	}
}

type closeProbe struct {
	io.Reader
	close func()
}

func (body *closeProbe) Close() error { body.close(); return nil }

func TestEventStreamFinishesOnEOFAndCancelsAtItemLimit(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	body := `{"Type":"container","Action":"start","Actor":{"ID":"abc123"},"timeNano":` + strconv.FormatInt(now.Add(-time.Second).UnixNano(), 10) + `}` + "\n"
	collector := newTestCollector(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return engineResponse(http.StatusOK, strings.Repeat(body, 101)), nil
	}))
	batch, err := collector.EngineEvents(context.Background(), now.Add(-time.Minute), now)
	if err != nil || !batch.Truncated || len(batch.Events) != 100 {
		t.Fatalf("event bound failed: %d %v", len(batch.Events), err)
	}
}

func TestEventCoverageUsesUnfilteredRetainedHistory(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	start := now.Add(-time.Minute)
	event := func(kind string, at time.Time) string {
		return `{"Type":"` + kind + `","Action":"start","Actor":{"ID":"abc123"},"timeNano":` + strconv.FormatInt(at.UnixNano(), 10) + `}` + "\n"
	}
	for name, test := range map[string]struct {
		body    string
		partial bool
		count   int
	}{
		"covered":              {event("plugin", start.Add(-time.Second)) + event("container", now.Add(-time.Second)), false, 1},
		"gap hidden by filter": {event("plugin", start.Add(time.Second)) + event("container", now.Add(-time.Second)), true, 1},
		"empty":                {"", true, 0}, "invalid time": {event("plugin", time.Unix(0, 0)) + event("container", now.Add(-time.Second)), true, 1},
	} {
		t.Run(name, func(t *testing.T) {
			collector := newTestCollector(t, roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.Query().Has("filters") || request.URL.Query().Get("since") != "0" {
					t.Fatal("retention coverage was filtered")
				}
				return engineResponse(http.StatusOK, test.body), nil
			}))
			batch, err := collector.EngineEvents(context.Background(), start, now)
			if err != nil || batch.Truncated != test.partial || len(batch.Events) != test.count {
				t.Fatalf("wrong event coverage: %#v %v", batch, err)
			}
		})
	}
}

func TestEventCancellationClosesBlockedResponse(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()
	closed := make(chan struct{})
	started := make(chan struct{})
	collector := newTestCollector(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		close(started)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: &closeProbe{Reader: reader, close: func() {
			_ = reader.Close()
			select {
			case <-closed:
			default:
				close(closed)
			}
		}}}, nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := collector.EngineEvents(ctx, time.Now().UTC().Add(-time.Minute), time.Now().UTC())
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("event stream did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation lost")
		}
	case <-time.After(time.Second):
		t.Fatal("blocked stream did not stop")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("cancelled event body stayed open")
	}
}

func TestEmptyMetricResponseIsUnavailableAndDoesNotInventZero(t *testing.T) {
	collector := newTestCollector(t, roundTripFunc(func(*http.Request) (*http.Response, error) { return engineResponse(http.StatusOK, `{}`), nil }))
	stats, err := collector.CurrentContainerStats(context.Background(), []string{"abc123"})
	if err != nil || len(stats) != 1 || stats[0].Status.State != domain.AvailabilityUnavailable {
		t.Fatalf("empty metric became available: %#v %v", stats, err)
	}
}

func TestLogFirstFrameBeyondByteCapIsExplicitlyPartial(t *testing.T) {
	frame := dockerLogFrame(1, []byte(time.Now().UTC().Format(time.RFC3339Nano)+" "+strings.Repeat("x", 1024)+"\n"))
	collector := newTestCollector(t, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/json") {
			return engineResponse(http.StatusOK, `{"Config":{"Tty":false}}`), nil
		}
		return engineResponse(http.StatusOK, string(frame)), nil
	}))
	batch, err := collector.ReadContainerLogBatch(context.Background(), ContainerLogRequest{ContainerIDs: []string{"abc123"}, Start: time.Now().UTC().Add(-time.Minute), End: time.Now().UTC().Add(time.Second), LineLimit: 100, ByteLimit: 64})
	if err != nil || !batch.Truncated || len(batch.Lines) != 0 {
		t.Fatalf("truncated empty frame claimed complete: %#v %v", batch, err)
	}
}

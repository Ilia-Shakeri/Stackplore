package docker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/moby/moby/api/types/container"
)

const writerTestContainerID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func writerTestInspect(start string) container.InspectResponse {
	return container.InspectResponse{ID: writerTestContainerID, Name: "/disposable", State: &container.State{Status: "running", StartedAt: start}}
}
func writerTestJSON(t *testing.T, start string) string {
	t.Helper()
	data, err := json.Marshal(writerTestInspect(start))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func newTestWriter(t *testing.T, transport http.RoundTripper) *SDKWriter {
	t.Helper()
	writer, err := NewSDKWriter(WriterOptions{Client: &http.Client{Transport: transport}, BaseURL: "http://docker", MaximumResponse: minimumEngineResponseBytes})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	return writer
}

func TestSDKWriterPinsVersionAndSendsOneFixedRestart(t *testing.T) {
	var posts atomic.Int32
	before := "2026-10-17T10:00:00Z"
	after := "2026-10-17T10:01:00Z"
	version := dockerContainerVersion(writerTestContainerID, writerTestInspect(before))
	writer := newTestWriter(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && r.URL.Path == "/containers/"+writerTestContainerID+"/json" {
			start := before
			if posts.Load() > 0 {
				start = after
			}
			return engineResponse(http.StatusOK, writerTestJSON(t, start)), nil
		}
		if r.Method != http.MethodPost || r.URL.Path != "/containers/"+writerTestContainerID+"/restart" || r.URL.RawQuery != "signal=SIGTERM&t=10" || r.Header.Get("Idempotency-Key") != "" || r.Body != nil && r.Body != http.NoBody {
			t.Fatalf("unexpected writer request: %s %s body=%t", r.Method, r.URL.RequestURI(), r.Body != nil && r.Body != http.NoBody)
		}
		posts.Add(1)
		return engineResponse(http.StatusNoContent, ""), nil
	}))
	if name, err := writer.PreviewContainer(context.Background(), writerTestContainerID, version); err != nil || name != "disposable" || posts.Load() != 0 {
		t.Fatalf("preview changed target: %v", err)
	}
	current, err := writer.RestartContainer(context.Background(), writerTestContainerID, version, "operation_durable_receipt")
	if err != nil || current == version || posts.Load() != 1 {
		t.Fatalf("restart: %v", err)
	}
	if _, err = writer.RestartContainer(context.Background(), writerTestContainerID, version, "operation_other"); !errors.Is(err, ErrVersionConflict) || posts.Load() != 1 {
		t.Fatal("stale target dispatched")
	}
	if _, err = writer.PreviewContainer(nil, writerTestContainerID, version); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestSDKWriterUnknownAcknowledgementIsUncertainAndNeverRetried(t *testing.T) {
	for _, status := range []int{0, http.StatusInternalServerError, http.StatusTemporaryRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var posts atomic.Int32
			start := "2026-10-17T10:00:00Z"
			writer := newTestWriter(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodGet {
					return engineResponse(http.StatusOK, writerTestJSON(t, start)), nil
				}
				posts.Add(1)
				if status == 0 {
					return nil, errors.New("private daemon diagnostic")
				}
				return engineResponse(status, `{"message":"private daemon diagnostic"}`), nil
			}))
			_, err := writer.RestartContainer(context.Background(), writerTestContainerID, dockerContainerVersion(writerTestContainerID, writerTestInspect(start)), "operation_lost_ack")
			if !errors.Is(err, domain.ErrOperationOutcomeUncertain) || posts.Load() != 1 || strings.Contains(err.Error(), "diagnostic") {
				t.Fatalf("unsafe uncertain result: %v", err)
			}
		})
	}
}

func TestWriterTransportRejectsEveryUnreviewedPathBeforeNetwork(t *testing.T) {
	base, _ := url.Parse("https://docker-writer.example:8443")
	var calls atomic.Int32
	transport := &engineWriteTransport{read: &engineReadTransport{base: base, maximum: minimumEngineResponseBytes, next: rawRoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return engineResponse(http.StatusNoContent, ""), nil
	})}}
	for _, test := range []struct{ method, path string }{{"POST", "/v1.56/containers/short/restart?signal=SIGTERM&t=10"}, {"POST", "/v1.56/containers/" + writerTestContainerID + "/restart?t=10"}, {"POST", "/v1.56/containers/" + writerTestContainerID + "/restart?signal=SIGKILL&t=10"}, {"POST", "/v1.56/containers/" + writerTestContainerID + "/restart?signal=SIGTERM&t=10&extra=1"}, {"POST", "/v1.56/containers/" + writerTestContainerID + "/stop?signal=SIGTERM&t=10"}, {"DELETE", "/v1.56/containers/" + writerTestContainerID}, {"GET", "/v1.56/containers/json?all=1"}, {"GET", "/v1.56/containers/" + writerTestContainerID + "/archive?path=/etc"}, {"POST", "/v1.57/containers/" + writerTestContainerID + "/restart?signal=SIGTERM&t=10"}, {"GET", "/v1.56/secrets"}, {"POST", "/v1.56/exec/one/start"}} {
		r, _ := http.NewRequest(test.method, base.String()+test.path, nil)
		if response, err := transport.RoundTrip(r); err == nil || response != nil {
			t.Fatal("unreviewed writer request reached network")
		}
	}
	r, _ := http.NewRequest(http.MethodPost, base.String()+"/v1.56/containers/"+writerTestContainerID+"/restart?signal=SIGTERM&t=10", nil)
	r.Header.Set("Idempotency-Key", "opaque")
	if _, err := transport.RoundTrip(r); err == nil {
		t.Fatal("retry enabling header accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("rejected writer request performed I/O")
	}
}

func TestReadOnlyDockerProviderDoesNotInheritWriterFromCollector(t *testing.T) {
	reader, err := New("source_docker", "Docker", &operationFixtureCollector{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := interface{}(reader).(domain.OperationProvider); ok {
		t.Fatal("read-only provider inherited mutation interface")
	}
}

package docker

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Path == "/_ping" {
		if request.Method != http.MethodHead {
			return engineResponse(http.StatusMethodNotAllowed, `{}`), nil
		}
		response := engineResponse(http.StatusOK, "")
		response.Header.Set("Api-Version", "1.56")
		return response, nil
	}
	match := engineVersionPath.FindStringSubmatch(request.URL.Path)
	if match == nil {
		return engineResponse(http.StatusBadRequest, `{}`), nil
	}
	copy := request.Clone(request.Context())
	copy.URL.Path = match[2]
	return roundTrip(copy)
}

func TestEngineCollectorUsesBoundedReviewedEndpoints(t *testing.T) {
	responses := map[string]string{
		"/info":                   `{"ID":"engine-one","Name":"Local engine"}`,
		"/containers/json?all=1":  `[{"ID":"abc123","Names":["/shop-api-1"],"ImageID":"sha256:image","Labels":{"com.docker.compose.project":"shop","com.docker.compose.service":"api"},"Mounts":[{"Type":"volume","Name":"shop_data"}],"NetworkSettings":{"Networks":{"default":{"NetworkID":"network-one"}}}}]`,
		"/containers/abc123/json": `{"State":{"Status":"running","Health":{"Status":"healthy"}}}`,
		"/images/json":            `[{"ID":"sha256:image","RepoTags":["shop/api:1.0"]}]`,
		"/networks":               `[{"ID":"network-one","Name":"shop_default"}]`,
		"/volumes":                `{"Volumes":[{"Name":"shop_data"}]}`,
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, exists := responses[request.URL.RequestURI()]
		if !exists || request.Method != http.MethodGet {
			t.Fatalf("unexpected Engine request: %s %s", request.Method, request.URL.RequestURI())
		}
		return engineResponse(http.StatusOK, body), nil
	})}
	now := time.Date(2026, time.August, 16, 8, 0, 0, 0, time.UTC)
	collector, err := NewEngineCollector(EngineCollectorOptions{
		Client: client, BaseURL: "http://docker", MaximumItems: 100,
		MaximumResponse: minimumEngineResponseBytes, InspectConcurrency: 2, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewEngineCollector() error = %v", err)
	}
	snapshot, err := collector.Snapshot(context.Background())
	if err != nil || snapshot.EngineID != "engine-one" || len(snapshot.Containers) != 1 ||
		snapshot.Containers[0].Health != "healthy" || snapshot.Containers[0].ComposeService != "api" ||
		len(snapshot.Networks) != 1 || len(snapshot.Volumes) != 1 {
		t.Fatalf("Snapshot() = %#v, error = %v", snapshot, err)
	}
}

func TestEngineCollectorRejectsUnsafeTargetAndMapsDenied(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return engineResponse(http.StatusForbidden, `{}`), nil
	})}
	if collector, err := NewEngineCollector(EngineCollectorOptions{
		Client: client, BaseURL: "http://127.0.0.1:2375", MaximumItems: 10,
		MaximumResponse: minimumEngineResponseBytes, InspectConcurrency: 1,
	}); err == nil || collector != nil {
		t.Fatal("unsafe Docker target was accepted")
	}
	collector, err := NewEngineCollector(EngineCollectorOptions{
		Client: client, BaseURL: "http://docker", MaximumItems: 10,
		MaximumResponse: minimumEngineResponseBytes, InspectConcurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Snapshot(context.Background()); err != ErrPermissionDenied {
		t.Fatalf("denied Snapshot() error = %v", err)
	}
}

func TestEngineCollectorReadsBoundedContainerStats(t *testing.T) {
	responses := map[string]string{
		"/containers/abc123/stats?stream=false": `{"read":"2026-08-16T08:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":200,"percpu_usage":[100,100]},"system_cpu_usage":1000,"online_cpus":2},"precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":500},"memory_stats":{"usage":1000,"stats":{"inactive_file":100}},"networks":{"eth0":{"rx_bytes":10,"tx_bytes":20}}}`,
		"/containers/abc123/json":               `{"RestartCount":3,"State":{"Status":"running"}}`,
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, exists := responses[request.URL.RequestURI()]
		if !exists || request.Method != http.MethodGet {
			t.Fatalf("unexpected Engine request: %s %s", request.Method, request.URL.RequestURI())
		}
		return engineResponse(http.StatusOK, body), nil
	})}
	now := time.Date(2026, time.August, 16, 8, 0, 0, 0, time.UTC)
	collector, err := NewEngineCollector(EngineCollectorOptions{
		Client: client, BaseURL: "http://docker", MaximumItems: 100,
		MaximumResponse: minimumEngineResponseBytes, InspectConcurrency: 2, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	stats, err := collector.CurrentContainerStats(context.Background(), []string{"abc123"})
	if err != nil || len(stats) != 1 || stats[0].CPUUsageCores != 0.4 || stats[0].MemoryUsageBytes != 900 ||
		stats[0].NetworkReadBytes != 10 || stats[0].NetworkWriteBytes != 20 || stats[0].RestartCount != 3 || stats[0].SampledAt != now {
		t.Fatalf("CurrentContainerStats() = %#v, error = %v", stats, err)
	}
}

func TestEngineCollectorReadsMultiplexedContainerLogs(t *testing.T) {
	now := time.Date(2026, time.August, 16, 8, 0, 0, 0, time.UTC)
	payload := dockerLogFrame(1, []byte("2026-08-16T07:59:30Z request done\n"))
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/containers/abc123/json":
			return engineResponse(http.StatusOK, `{"Config":{"Tty":false},"State":{"Status":"running"}}`), nil
		case "/containers/abc123/logs":
			if request.URL.Query().Get("timestamps") != "1" || request.URL.Query().Has("follow") {
				t.Fatalf("unsafe log query: %s", request.URL.RawQuery)
			}
			return engineResponse(http.StatusOK, string(payload)), nil
		default:
			t.Fatalf("unexpected Engine path: %s", request.URL.Path)
			return nil, nil
		}
	})}
	collector, err := NewEngineCollector(EngineCollectorOptions{
		Client: client, BaseURL: "http://docker", MaximumItems: 100,
		MaximumResponse: minimumEngineResponseBytes, InspectConcurrency: 2, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	lines, err := collector.ReadContainerLogs(context.Background(), ContainerLogRequest{
		ContainerIDs: []string{"abc123"}, Start: now.Add(-time.Hour), End: now.Add(time.Nanosecond),
		LineLimit: 10, ByteLimit: 1024,
	})
	if err != nil || len(lines) != 1 || lines[0].Stream != domain.LogStreamStdout || lines[0].Message != "request done" {
		t.Fatalf("ReadContainerLogs() = %#v, error = %v", lines, err)
	}
}

func TestEngineCollectorReadsBoundedFilteredEvents(t *testing.T) {
	now := time.Date(2026, time.August, 16, 8, 0, 0, 0, time.UTC)
	eventTime := now.Add(-30 * time.Second)
	body := `{"Type":"container","Action":"die","Actor":{"ID":"abc123"},"timeNano":` + strconv.FormatInt(eventTime.UnixNano(), 10) + `}` + "\n"
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/events" || request.URL.Query().Has("filters") ||
			request.URL.Query().Get("since") != "0" || request.URL.Query().Get("until") == "" {
			t.Fatalf("unsafe event query: %s", request.URL.RequestURI())
		}
		return engineResponse(http.StatusOK, body), nil
	})}
	collector, err := NewEngineCollector(EngineCollectorOptions{
		Client: client, BaseURL: "http://docker", MaximumItems: 100,
		MaximumResponse: minimumEngineResponseBytes, InspectConcurrency: 2, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := collector.EngineEvents(context.Background(), now.Add(-time.Hour), now.Add(time.Minute))
	if err != nil || !batch.Truncated || len(batch.Events) != 1 || batch.Events[0].ActorID != "abc123" || batch.Events[0].OccurredAt != eventTime {
		t.Fatalf("EngineEvents() = %#v, error = %v", batch, err)
	}
}

func dockerLogFrame(stream byte, payload []byte) []byte {
	result := bytes.NewBuffer(make([]byte, 0, len(payload)+8))
	result.Write([]byte{stream, 0, 0, 0})
	_ = binary.Write(result, binary.BigEndian, uint32(len(payload)))
	result.Write(payload)
	return result.Bytes()
}

func engineResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

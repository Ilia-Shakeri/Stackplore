package docker

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type recordingLogCollector struct {
	logFixtureCollector
	requests []ContainerLogRequest
}

func (collector *recordingLogCollector) ReadContainerLogs(_ context.Context, request ContainerLogRequest) ([]ContainerLogLine, error) {
	collector.requests = append(collector.requests, request)
	return collector.lines, nil
}

func TestDockerLogPagesUseOneFixedTailAndPreviousIsUnsupported(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Microsecond)
	collector := &recordingLogCollector{logFixtureCollector: logFixtureCollector{snapshot: Snapshot{ObservedAt: at, EngineID: "engine-one", EngineName: "Engine", Containers: []Container{{ID: "abc123", Name: "api", State: "running"}}}}}
	for index := range 3 {
		collector.lines = append(collector.lines, ContainerLogLine{ContainerID: "abc123", Stream: domain.LogStreamStdout, Timestamp: at.Add(time.Duration(index-3) * time.Second), Message: "fixture"})
	}
	provider, err := New("source_docker", "Docker", collector)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := provider.CaptureInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var resource domain.ResourceRef
	for _, item := range capture.Resources {
		if item.Resource.Ref.Identity.Kind == "docker.container" {
			resource = item.Resource.Ref
		}
	}
	query := domain.LogReadQuery{Source: capture.Source, Targets: []domain.LogTarget{{Resource: resource}}, Range: domain.TimeRange{Start: at.Add(-time.Minute), End: at}, LineLimit: 1, ByteLimit: 1024}
	first, err := provider.ReadLogs(context.Background(), query)
	if err != nil || len(first.Records) != 1 || !first.Page.HasMore {
		t.Fatalf("first page failed: %v", err)
	}
	query.Cursor = first.Page.NextCursor
	second, err := provider.ReadLogs(context.Background(), query)
	if err != nil || len(second.Records) != 1 || first.Records[0].ID == second.Records[0].ID {
		t.Fatalf("second page duplicated tail: %v", err)
	}
	if len(collector.requests) != 2 || collector.requests[0].LineLimit != domain.MaxLogLines || collector.requests[1].LineLimit != domain.MaxLogLines {
		t.Fatal("page offset changed daemon tail")
	}
	query.Cursor = ""
	query.Targets[0].Previous = true
	previous, err := provider.ReadLogs(context.Background(), query)
	if err != nil || previous.Status.State != domain.AvailabilityUnsupported || len(previous.Records) != 0 || len(collector.requests) != 2 {
		t.Fatal("previous target read current logs")
	}
}

package docker

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/moby/moby/api/types/container"
)

func TestDockerMetadataDropsHostPathsAddressesEnvironmentAndPrivateLabels(t *testing.T) {
	var inspection container.InspectResponse
	data := `{"Id":"abc123","RestartCount":2,"State":{"Status":"running","StartedAt":"2026-10-17T09:00:00.123456789Z","Error":"DO_NOT_EXPORT_ERROR"},"Config":{"Image":"shop/api:1.0","Env":["DO_NOT_EXPORT_ENV=value"],"Labels":{"private.label":"DO_NOT_EXPORT_LABEL"},"ExposedPorts":{"8080/tcp":{}}},"HostConfig":{"Memory":67108864,"NanoCpus":500000000},"NetworkSettings":{"Ports":{"8080/tcp":[{"HostIp":"192.0.2.10","HostPort":"18080"}]}},"Mounts":[{"Type":"bind","Source":"/DO_NOT_EXPORT_HOST","Destination":"/data","RW":false}]}`
	if json.Unmarshal([]byte(data), &inspection) != nil {
		t.Fatal("fixture invalid")
	}
	metadata, complete := containerMetadata(inspection, Container{State: "running", ComposeProject: "shop", ComposeService: "api", DependsOn: []string{"worker"}})
	if !complete || metadata == nil || metadata.Validate() != nil || metadata.StartedAt == nil || metadata.StartedAt.Nanosecond() != 123456000 || metadata.CPULimitCores == nil || *metadata.CPULimitCores != 0.5 || len(metadata.Ports) != 1 || !metadata.Ports[0].Published || len(metadata.Labels) != 3 {
		t.Fatal("safe normalized metadata incomplete")
	}
	encoded, err := json.Marshal(metadata)
	if err != nil || strings.Contains(string(encoded), "DO_NOT_EXPORT") || strings.Contains(string(encoded), "192.0.2.10") {
		t.Fatal("unreviewed vendor field crossed metadata boundary")
	}
}

func TestDockerLogMultiplexFramesPreserveSplitLines(t *testing.T) {
	first := dockerLogFrame(1, []byte("2026-10-17T09:00:00Z long "))
	second := dockerLogFrame(2, []byte("2026-10-17T09:00:01Z error\n"))
	third := dockerLogFrame(1, []byte("message\n"))
	lines := parseDockerLogPayload("abc123", append(append(first, second...), third...), false)
	if len(lines) != 2 || lines[0].Message != "long message" || lines[1].Message != "error" {
		t.Fatal("multiplex frame boundary lost line data")
	}
}

func TestDockerLogMalformedFramesRetainValidLinesAndMarkPartial(t *testing.T) {
	valid := dockerLogFrame(1, []byte("2026-10-17T09:00:00Z kept\n"))
	for _, suffix := range [][]byte{
		{1, 0, 0},
		dockerLogFrame(3, []byte("ignored")),
		{1, 0, 0, 0, 0, 0, 0, 8, 'x'},
		dockerLogFrame(2, []byte("invalid timestamp\n")),
	} {
		payload := append(append([]byte(nil), valid...), suffix...)
		lines, partial := parseDockerLogPayloadChecked("abc123", payload, false)
		if !partial || len(lines) != 1 || lines[0].Message != "kept" {
			t.Fatal("malformed logs lost valid records or claimed completeness")
		}
	}
	if _, partial := parseDockerLogPayloadChecked("abc123", []byte("invalid timestamp\n"), true); !partial {
		t.Fatal("malformed TTY timestamps claimed completeness")
	}
}

func TestDockerStatsSubtractsCgroupVersionSpecificInactiveMemory(t *testing.T) {
	stats := container.StatsResponse{MemoryStats: container.MemoryStats{Usage: 1000, Stats: map[string]uint64{"total_inactive_file": 300, "inactive_file": 100, "cache": 200}}}
	result, err := normalizeContainerStats("abc123", stats, 0, time.Now().UTC().Truncate(time.Microsecond))
	if err != nil || result.MemoryUsageBytes != 700 || !result.CPUUnavailable {
		t.Fatal("cgroup memory or missing CPU sample misreported")
	}
}

func TestDockerHealthDoesNotReuseAnOldHealthyProbeAfterStop(t *testing.T) {
	for _, test := range []struct {
		state, health string
		want          domain.HealthState
	}{
		{"exited", "healthy", domain.HealthUnhealthy}, {"dead", "healthy", domain.HealthUnhealthy},
		{"paused", "healthy", domain.HealthDegraded}, {"restarting", "healthy", domain.HealthDegraded},
		{"running", "starting", domain.HealthDegraded}, {"running", "healthy", domain.HealthHealthy},
		{"created", "healthy", domain.HealthUnknown},
	} {
		state, _ := containerHealth(Container{State: test.state, Health: test.health})
		if state != test.want {
			t.Fatalf("state %s reused misleading health", test.state)
		}
	}
}

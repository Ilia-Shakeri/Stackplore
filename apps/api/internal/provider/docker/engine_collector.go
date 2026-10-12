package docker

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const (
	minimumEngineResponseBytes = 64 * 1024
	maximumEngineResponseBytes = 32 * 1024 * 1024
	maximumEngineItems         = 10_000
)

type EngineCollectorOptions struct {
	Client             *http.Client
	BaseURL            string
	MaximumItems       int
	MaximumResponse    int64
	InspectConcurrency int
	Now                func() time.Time
}

type EngineCollector struct {
	client             *http.Client
	baseURL            string
	maximumItems       int
	maximumResponse    int64
	inspectConcurrency int
	now                func() time.Time
}

func NewEngineCollector(options EngineCollectorOptions) (*EngineCollector, error) {
	parsed, err := url.Parse(options.BaseURL)
	if err != nil || options.Client == nil || parsed.Scheme != "http" || parsed.Host != "docker" ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil ||
		options.MaximumItems < 1 || options.MaximumItems > maximumEngineItems ||
		options.MaximumResponse < minimumEngineResponseBytes || options.MaximumResponse > maximumEngineResponseBytes ||
		options.InspectConcurrency < 1 || options.InspectConcurrency > 32 {
		return nil, fmt.Errorf("Docker Engine collector options are invalid")
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &EngineCollector{
		client: options.Client, baseURL: strings.TrimSuffix(options.BaseURL, "/"),
		maximumItems: options.MaximumItems, maximumResponse: options.MaximumResponse,
		inspectConcurrency: options.InspectConcurrency, now: now,
	}, nil
}

func (collector *EngineCollector) Snapshot(ctx context.Context) (Snapshot, error) {
	if collector == nil || collector.client == nil || ctx == nil || ctx.Err() != nil {
		return Snapshot{}, errors.Join(ErrUnavailable, contextCause(ctx))
	}
	var info engineInfo
	if err := collector.getJSON(ctx, "/info", &info); err != nil {
		return Snapshot{}, err
	}
	var containers []engineContainer
	if err := collector.getJSON(ctx, "/containers/json?all=1", &containers); err != nil {
		return Snapshot{}, err
	}
	var images []engineImage
	if err := collector.getJSON(ctx, "/images/json?all=0", &images); err != nil {
		return Snapshot{}, err
	}
	var networks []engineNetwork
	if err := collector.getJSON(ctx, "/networks", &networks); err != nil {
		return Snapshot{}, err
	}
	var volumes engineVolumePage
	if err := collector.getJSON(ctx, "/volumes", &volumes); err != nil {
		return Snapshot{}, err
	}
	if len(containers) > collector.maximumItems || len(images) > collector.maximumItems ||
		len(networks) > collector.maximumItems || len(volumes.Volumes) > collector.maximumItems {
		return Snapshot{}, ErrUnavailable
	}
	inspections, err := collector.inspectContainers(ctx, containers)
	if err != nil {
		return Snapshot{}, err
	}
	return collector.normalize(info, containers, inspections, images, networks, volumes.Volumes), nil
}

func (collector *EngineCollector) inspectContainers(ctx context.Context, containers []engineContainer) ([]engineContainerInspect, error) {
	results := make([]engineContainerInspect, len(containers))
	jobs := make(chan int)
	workerContext, cancel := context.WithCancel(ctx)
	defer cancel()
	var firstErr error
	var errorOnce sync.Once
	var workers sync.WaitGroup
	workerCount := min(collector.inspectConcurrency, len(containers))
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				identifier := containers[index].ID
				if !safeDockerID(identifier) {
					errorOnce.Do(func() { firstErr = ErrUnavailable; cancel() })
					continue
				}
				if err := collector.getJSON(workerContext, "/containers/"+identifier+"/json", &results[index]); err != nil {
					errorOnce.Do(func() { firstErr = err; cancel() })
				}
			}
		}()
	}
	for index := range containers {
		select {
		case jobs <- index:
		case <-workerContext.Done():
			break
		}
		if workerContext.Err() != nil {
			break
		}
	}
	close(jobs)
	workers.Wait()
	return results, firstErr
}

func (collector *EngineCollector) CurrentContainerStats(ctx context.Context, containerIDs []string) ([]ContainerStats, error) {
	if collector == nil || collector.client == nil || ctx == nil || ctx.Err() != nil || len(containerIDs) > domain.MaxMetricTargets {
		return nil, errors.Join(ErrUnavailable, contextCause(ctx))
	}
	result := make([]ContainerStats, 0, len(containerIDs))
	seen := make(map[string]struct{}, len(containerIDs))
	for _, identifier := range containerIDs {
		if !safeDockerID(identifier) {
			return nil, ErrUnavailable
		}
		if _, exists := seen[identifier]; exists {
			return nil, ErrUnavailable
		}
		seen[identifier] = struct{}{}
		var stats engineContainerStats
		if err := collector.getJSON(ctx, "/containers/"+identifier+"/stats?stream=false&one-shot=true", &stats); err != nil {
			return nil, err
		}
		var inspection engineContainerInspect
		if err := collector.getJSON(ctx, "/containers/"+identifier+"/json", &inspection); err != nil {
			return nil, err
		}
		normalized, err := normalizeContainerStats(identifier, stats, inspection.RestartCount, collector.now().UTC().Truncate(time.Microsecond))
		if err != nil {
			return nil, err
		}
		result = append(result, normalized)
	}
	return result, nil
}

func (collector *EngineCollector) RestartContainer(
	ctx context.Context,
	identifier string,
	expectedVersion string,
	idempotencyKey string,
) (string, error) {
	if collector == nil || collector.client == nil || ctx == nil || ctx.Err() != nil || !safeDockerID(identifier) ||
		domain.ValidateOperationVersion(expectedVersion) != nil || idempotencyKey == "" {
		return "", errors.Join(ErrUnavailable, contextCause(ctx))
	}
	var before engineContainerInspect
	if err := collector.getJSON(ctx, "/containers/"+identifier+"/json", &before); err != nil {
		return "", err
	}
	if dockerContainerVersion(identifier, before) != expectedVersion {
		return "", ErrVersionConflict
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, collector.baseURL+"/containers/"+identifier+"/restart?t=10", nil)
	if err != nil {
		return "", ErrUnavailable
	}
	request.Header.Set("Accept", "application/json")
	response, err := collector.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return "", errors.Join(ErrUnavailable, ctx.Err())
		}
		return "", errors.Join(ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return "", ErrPermissionDenied
	}
	if response.StatusCode != http.StatusNoContent {
		return "", ErrUnavailable
	}
	var after engineContainerInspect
	if err := collector.getJSON(ctx, "/containers/"+identifier+"/json", &after); err != nil {
		return "", err
	}
	return dockerContainerVersion(identifier, after), nil
}

func (collector *EngineCollector) getJSON(ctx context.Context, path string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, collector.baseURL+path, nil)
	if err != nil {
		return ErrUnavailable
	}
	request.Header.Set("Accept", "application/json")
	response, err := collector.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return errors.Join(ErrUnavailable, ctx.Err())
		}
		return errors.Join(ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrPermissionDenied
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return ErrUnavailable
	}
	reader := io.LimitReader(response.Body, collector.maximumResponse+1)
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(target); err != nil {
		return ErrUnavailable
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrUnavailable
	}
	return nil
}

func (collector *EngineCollector) normalize(
	info engineInfo,
	containers []engineContainer,
	inspections []engineContainerInspect,
	images []engineImage,
	networks []engineNetwork,
	volumes []engineVolume,
) Snapshot {
	result := Snapshot{
		ObservedAt: collector.now().UTC().Truncate(time.Microsecond),
		EngineID:   info.ID, EngineName: info.Name,
		Containers: make([]Container, 0, len(containers)), Images: make([]Image, 0, len(images)),
		Networks: make([]Network, 0, len(networks)), Volumes: make([]Volume, 0, len(volumes)),
	}
	for index, item := range containers {
		inspection := inspections[index]
		health := ""
		if inspection.State.Health != nil {
			health = inspection.State.Health.Status
		}
		container := Container{
			ID: item.ID, Name: firstContainerName(item.Names), ImageID: item.ImageID,
			Version: dockerContainerVersion(item.ID, inspection),
			State:   inspection.State.Status, Health: health,
			ComposeProject: item.Labels["com.docker.compose.project"],
			ComposeService: item.Labels["com.docker.compose.service"],
		}
		for _, mount := range item.Mounts {
			if mount.Type == "volume" && mount.Name != "" {
				container.VolumeNames = append(container.VolumeNames, mount.Name)
			}
		}
		for _, network := range item.NetworkSettings.Networks {
			if network.NetworkID != "" {
				container.NetworkIDs = append(container.NetworkIDs, network.NetworkID)
			}
		}
		result.Containers = append(result.Containers, container)
	}
	for _, item := range images {
		name := item.ID
		if len(item.RepoTags) > 0 && item.RepoTags[0] != "<none>:<none>" {
			name = item.RepoTags[0]
		}
		result.Images = append(result.Images, Image{ID: item.ID, Name: name})
	}
	for _, item := range networks {
		result.Networks = append(result.Networks, Network{ID: item.ID, Name: item.Name})
	}
	for _, item := range volumes {
		result.Volumes = append(result.Volumes, Volume{Name: item.Name})
	}
	return result
}

type engineInfo struct{ ID, Name string }
type engineImage struct {
	ID       string
	RepoTags []string
}
type engineNetwork struct{ ID, Name string }
type engineVolume struct{ Name string }
type engineVolumePage struct{ Volumes []engineVolume }
type engineContainer struct {
	ID              string
	Names           []string
	ImageID         string
	Labels          map[string]string
	Mounts          []struct{ Type, Name string }
	NetworkSettings struct {
		Networks map[string]struct{ NetworkID string }
	}
}
type engineContainerInspect struct {
	RestartCount uint64
	Config       struct {
		TTY bool `json:"Tty"`
	}
	State struct {
		Status    string
		StartedAt string
		Health    *struct{ Status string }
	}
}

func dockerContainerVersion(identifier string, inspection engineContainerInspect) string {
	health := ""
	if inspection.State.Health != nil {
		health = inspection.State.Health.Status
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf(
		"stackplore-docker-version/v1\x00%s\x00%s\x00%s\x00%d\x00%s",
		identifier, inspection.State.Status, health, inspection.RestartCount, inspection.State.StartedAt,
	)))
	return "dv_" + base64.RawURLEncoding.EncodeToString(digest[:])
}

type engineCPUStats struct {
	CPUUsage struct {
		TotalUsage  uint64   `json:"total_usage"`
		PerCPUUsage []uint64 `json:"percpu_usage"`
	} `json:"cpu_usage"`
	SystemUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs  uint64 `json:"online_cpus"`
}

type engineContainerStats struct {
	CPUStats    engineCPUStats `json:"cpu_stats"`
	PreCPUStats engineCPUStats `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		ReadBytes  uint64 `json:"rx_bytes"`
		WriteBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

func normalizeContainerStats(identifier string, stats engineContainerStats, restartCount uint64, sampledAt time.Time) (ContainerStats, error) {
	cpuDelta := stats.CPUStats.CPUUsage.TotalUsage - min(stats.CPUStats.CPUUsage.TotalUsage, stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := stats.CPUStats.SystemUsage - min(stats.CPUStats.SystemUsage, stats.PreCPUStats.SystemUsage)
	cores := stats.CPUStats.OnlineCPUs
	if cores == 0 {
		cores = uint64(len(stats.CPUStats.CPUUsage.PerCPUUsage))
	}
	cpuUsage := 0.0
	if systemDelta > 0 && cores > 0 {
		cpuUsage = float64(cpuDelta) / float64(systemDelta) * float64(cores)
	}
	memoryUsage := stats.MemoryStats.Usage
	cache := stats.MemoryStats.Stats["cache"]
	if inactive := stats.MemoryStats.Stats["inactive_file"]; inactive > 0 {
		cache = inactive
	}
	memoryUsage -= min(memoryUsage, cache)
	var readBytes, writeBytes uint64
	for _, network := range stats.Networks {
		if network.ReadBytes > ^uint64(0)-readBytes || network.WriteBytes > ^uint64(0)-writeBytes {
			return ContainerStats{}, ErrUnavailable
		}
		readBytes += network.ReadBytes
		writeBytes += network.WriteBytes
	}
	const maximumSafeInteger = uint64(1<<53 - 1)
	if memoryUsage > maximumSafeInteger || readBytes > maximumSafeInteger || writeBytes > maximumSafeInteger || restartCount > maximumSafeInteger {
		return ContainerStats{}, ErrUnavailable
	}
	return ContainerStats{
		ContainerID: identifier, CPUUsageCores: cpuUsage, MemoryUsageBytes: memoryUsage,
		NetworkReadBytes: readBytes, NetworkWriteBytes: writeBytes, RestartCount: restartCount, SampledAt: sampledAt,
	}, nil
}

func safeDockerID(value string) bool {
	if len(value) < 1 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character != ':' && character != '-' && character != '_' &&
			(character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func firstContainerName(names []string) string {
	if len(names) == 0 {
		return "Unnamed container"
	}
	return strings.TrimPrefix(names[0], "/")
}

func contextCause(ctx context.Context) error {
	if ctx == nil {
		return errors.New("missing context")
	}
	return ctx.Err()
}

var _ Collector = (*EngineCollector)(nil)
var _ MetricCollector = (*EngineCollector)(nil)
var _ OperationCollector = (*EngineCollector)(nil)

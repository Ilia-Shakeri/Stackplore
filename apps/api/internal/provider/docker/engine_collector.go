package docker

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"

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
	sdk                *client.Client
	transport          *engineReadTransport
	maximumItems       int
	inspectConcurrency int
	now                func() time.Time
}

func engineClientBoundary(options EngineCollectorOptions) (*http.Client, *url.URL, error) {
	parsed, err := url.Parse(options.BaseURL)
	if err != nil || options.Client == nil || options.Client.Transport == nil ||
		!(parsed.Scheme == "http" && parsed.Host == "docker" || parsed.Scheme == "https" && parsed.Hostname() != "") ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.User != nil ||
		options.MaximumItems < 1 || options.MaximumItems > maximumEngineItems ||
		options.MaximumResponse < minimumEngineResponseBytes || options.MaximumResponse > maximumEngineResponseBytes ||
		options.InspectConcurrency < 1 || options.InspectConcurrency > 32 {
		return nil, nil, errors.New("Docker Engine collector options are invalid")
	}
	next := options.Client.Transport
	if supplied, ok := next.(*http.Transport); ok {
		bounded := supplied.Clone()
		bounded.Proxy = nil
		bounded.DisableCompression = true
		bounded.MaxIdleConns = 8
		bounded.MaxIdleConnsPerHost = 8
		bounded.MaxConnsPerHost = 8
		bounded.MaxResponseHeaderBytes = 64 << 10
		if bounded.ResponseHeaderTimeout <= 0 || bounded.ResponseHeaderTimeout > 10*time.Second {
			bounded.ResponseHeaderTimeout = 10 * time.Second
		}
		if bounded.IdleConnTimeout <= 0 || bounded.IdleConnTimeout > 30*time.Second {
			bounded.IdleConnTimeout = 30 * time.Second
		}
		if parsed.Scheme == "https" && bounded.TLSClientConfig != nil {
			if bounded.TLSClientConfig.InsecureSkipVerify || bounded.TLSClientConfig.ServerName != "" && bounded.TLSClientConfig.ServerName != parsed.Hostname() {
				return nil, nil, ErrUnavailable
			}
			bounded.TLSClientConfig = bounded.TLSClientConfig.Clone()
			bounded.TLSClientConfig.ServerName = parsed.Hostname()
			if bounded.TLSClientConfig.MinVersion < tls.VersionTLS12 {
				bounded.TLSClientConfig.MinVersion = tls.VersionTLS12
			}
		}
		next = bounded
	}
	httpClient := *options.Client
	httpClient.Transport = next
	httpClient.Jar = nil
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return ErrUnavailable }
	if httpClient.Timeout <= 0 || httpClient.Timeout > time.Minute {
		httpClient.Timeout = 20 * time.Second
	}
	return &httpClient, parsed, nil
}

func NewEngineCollector(options EngineCollectorOptions) (*EngineCollector, error) {
	httpClient, parsed, err := engineClientBoundary(options)
	if err != nil {
		return nil, err
	}
	transport := &engineReadTransport{base: parsed, next: httpClient.Transport, maximum: options.MaximumResponse}
	httpClient.Transport = transport
	// Never use ambient Docker environment variables. Host configuration must
	// precede the supplied transport, because WithHost configures its own dialer.
	sdk, err := client.New(client.WithHost(options.BaseURL), client.WithScheme(parsed.Scheme), client.WithHTTPClient(httpClient))
	if err != nil {
		return nil, ErrUnavailable
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &EngineCollector{
		sdk: sdk, transport: transport, maximumItems: options.MaximumItems,
		inspectConcurrency: options.InspectConcurrency, now: now,
	}, nil
}

func (collector *EngineCollector) Close() error {
	if collector == nil {
		return nil
	}
	collector.transport.CloseIdleConnections()
	return collector.sdk.Close()
}

func (collector *EngineCollector) Snapshot(ctx context.Context) (Snapshot, error) {
	if collector == nil || collector.sdk == nil || ctx == nil || ctx.Err() != nil {
		return Snapshot{}, errors.Join(ErrUnavailable, contextCause(ctx))
	}
	ctx = engineObservationContext(ctx)
	info, err := collector.sdk.Info(ctx, client.InfoOptions{})
	if err != nil {
		return Snapshot{}, engineError(ctx, err)
	}
	containers, containerErr := collector.sdk.ContainerList(ctx, client.ContainerListOptions{All: true})
	images, imageErr := collector.sdk.ImageList(ctx, client.ImageListOptions{})
	networks, networkErr := collector.sdk.NetworkList(ctx, client.NetworkListOptions{})
	volumes, volumeErr := collector.sdk.VolumeList(ctx, client.VolumeListOptions{})
	if ctx.Err() != nil {
		return Snapshot{}, engineError(ctx, ctx.Err())
	}
	if len(containers.Items) > collector.maximumItems || len(images.Items) > collector.maximumItems ||
		len(networks.Items) > collector.maximumItems || len(volumes.Items) > collector.maximumItems {
		return Snapshot{}, ErrUnavailable
	}
	inspections, inspectionErrors := collector.inspectContainers(ctx, containers.Items)
	if ctx.Err() != nil {
		return Snapshot{}, engineError(ctx, ctx.Err())
	}
	snapshot := collector.normalize(info.Info, containers.Items, inspections, images.Items, networks.Items, volumes.Items)
	snapshot.Coverage = map[domain.ResourceKind]domain.ObservationStatus{}
	for _, entry := range []struct {
		kind domain.ResourceKind
		err  error
	}{
		{"docker.engine", nil}, {"docker.container", containerErr}, {"docker.compose_project", containerErr},
		{"docker.compose_service", containerErr}, {"docker.image", imageErr}, {"docker.network", networkErr}, {"docker.volume", volumeErr},
	} {
		snapshot.Coverage[entry.kind] = collector.coverageStatus(ctx, entry.err, snapshot.ObservedAt)
	}
	if len(inspectionErrors) > 0 {
		status := snapshot.Coverage["docker.container"]
		status.Completeness, status.ReasonCode = domain.CompletenessPartial, "docker.inventory.inspect_incomplete"
		snapshot.Coverage["docker.container"] = status
		// Compose grouping comes from the complete list, independent of inspect.
	}
	return snapshot, nil
}

func (collector *EngineCollector) coverageStatus(ctx context.Context, err error, observedAt time.Time) domain.ObservationStatus {
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: observedAt}
	if err != nil {
		status.State, status.ReasonCode = domain.AvailabilityUnavailable, "docker.inventory.unavailable"
		if errors.Is(engineError(ctx, err), ErrPermissionDenied) {
			status.State, status.ReasonCode = domain.AvailabilityPermissionDenied, "docker.inventory.denied"
		}
	}
	return status
}

func (collector *EngineCollector) inspectContainers(ctx context.Context, containers []container.Summary) ([]container.InspectResponse, []error) {
	results := make([]container.InspectResponse, len(containers))
	failures := make([]error, len(containers))
	collector.parallel(ctx, len(containers), func(index int) {
		identifier := containers[index].ID
		if !safeDockerID(identifier) {
			failures[index] = ErrUnavailable
			return
		}
		result, err := collector.sdk.ContainerInspect(ctx, identifier, client.ContainerInspectOptions{})
		if err != nil || result.Container.State == nil {
			failures[index] = engineError(ctx, firstError(err, ErrUnavailable))
			return
		}
		results[index] = result.Container
	})
	return results, nonNilErrors(failures)
}

func (collector *EngineCollector) parallel(ctx context.Context, count int, run func(int)) {
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(collector.inspectConcurrency, count) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				if ctx.Err() == nil {
					run(index)
				}
			}
		}()
	}
	for index := range count {
		select {
		case jobs <- index:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return
		}
	}
	close(jobs)
	workers.Wait()
}

func (collector *EngineCollector) CurrentContainerStats(ctx context.Context, containerIDs []string) ([]ContainerStats, error) {
	if collector == nil || collector.sdk == nil || ctx == nil || ctx.Err() != nil || len(containerIDs) > domain.MaxMetricTargets {
		return nil, errors.Join(ErrUnavailable, contextCause(ctx))
	}
	ctx = engineObservationContext(ctx)
	seen := make(map[string]struct{}, len(containerIDs))
	for _, identifier := range containerIDs {
		if !safeDockerID(identifier) {
			return nil, ErrUnavailable
		}
		if _, duplicate := seen[identifier]; duplicate {
			return nil, ErrUnavailable
		}
		seen[identifier] = struct{}{}
	}
	result := make([]ContainerStats, len(containerIDs))
	collector.parallel(ctx, len(containerIDs), func(index int) {
		identifier := containerIDs[index]
		observedAt := collector.now().UTC().Truncate(time.Microsecond)
		failure := func(err error) {
			result[index] = ContainerStats{ContainerID: identifier, SampledAt: observedAt, Status: collector.coverageStatus(ctx, err, observedAt)}
		}
		response, err := collector.sdk.ContainerStats(ctx, identifier, client.ContainerStatsOptions{IncludePreviousSample: true})
		if err != nil {
			failure(err)
			return
		}
		var stats container.StatsResponse
		err = json.NewDecoder(response.Body).Decode(&stats)
		_ = response.Body.Close()
		observedAt = collector.now().UTC().Truncate(time.Microsecond)
		if err != nil || stats.Read.IsZero() || stats.Read.After(observedAt.Add(time.Minute)) || stats.ID != "" && stats.ID != identifier {
			failure(firstError(err, ErrUnavailable))
			return
		}
		inspection, err := collector.sdk.ContainerInspect(ctx, identifier, client.ContainerInspectOptions{})
		if err != nil || inspection.Container.RestartCount < 0 || inspection.Container.State == nil || inspection.Container.State.Status != "running" && inspection.Container.State.Status != "paused" {
			failure(firstError(err, ErrUnavailable))
			return
		}
		normalized, err := normalizeContainerStats(identifier, stats, uint64(inspection.Container.RestartCount), stats.Read.UTC().Truncate(time.Microsecond))
		if err != nil {
			failure(err)
			return
		}
		result[index] = normalized
	})
	if ctx.Err() != nil {
		return nil, engineError(ctx, ctx.Err())
	}
	return result, nil
}

func (collector *EngineCollector) normalize(info system.Info, containers []container.Summary, inspections []container.InspectResponse, images []image.Summary, networks []network.Summary, volumes []volume.Volume) Snapshot {
	result := Snapshot{
		ObservedAt: collector.now().UTC().Truncate(time.Microsecond), EngineID: info.ID, EngineName: info.Name,
		EngineVersion: info.ServerVersion,
		Containers:    make([]Container, 0, len(containers)), Images: make([]Image, 0, len(images)),
		Networks: make([]Network, 0, len(networks)), Volumes: make([]Volume, 0, len(volumes)),
	}
	for index, item := range containers {
		inspection := inspections[index]
		entry := Container{
			ID: item.ID, Name: firstContainerName(item.Names), ImageID: item.ImageID, State: string(item.State),
			ComposeProject: safeDockerLabel(item.Labels["com.docker.compose.project"]),
			ComposeService: safeDockerLabel(item.Labels["com.docker.compose.service"]),
			DependsOn:      declaredDependencies(item.Labels["stackplore.io/depends-on"]),
		}
		if inspection.State != nil {
			entry.Version, entry.State = dockerContainerVersion(item.ID, inspection), string(inspection.State.Status)
			if inspection.State.Health != nil {
				entry.Health = string(inspection.State.Health.Status)
			}
		} else {
			entry.InspectUnavailable = true
		}
		for _, mount := range item.Mounts {
			if mount.Type == "volume" && mount.Name != "" {
				entry.VolumeNames = append(entry.VolumeNames, mount.Name)
			}
		}
		if item.NetworkSettings != nil {
			for _, endpoint := range item.NetworkSettings.Networks {
				if endpoint != nil && endpoint.NetworkID != "" {
					entry.NetworkIDs = append(entry.NetworkIDs, endpoint.NetworkID)
				}
			}
		}
		sort.Strings(entry.NetworkIDs)
		sort.Strings(entry.VolumeNames)
		metadata, complete := containerMetadata(inspection, entry)
		entry.Metadata, entry.MetadataIncomplete = metadata, !complete
		result.Containers = append(result.Containers, entry)
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

func dockerContainerVersion(identifier string, inspection container.InspectResponse) string {
	if inspection.State == nil || inspection.RestartCount < 0 {
		return ""
	}
	health := ""
	if inspection.State.Health != nil {
		health = string(inspection.State.Health.Status)
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf(
		"stackplore-docker-version/v1\x00%s\x00%s\x00%s\x00%d\x00%s",
		identifier, inspection.State.Status, health, inspection.RestartCount, inspection.State.StartedAt,
	)))
	return "dv_" + base64.RawURLEncoding.EncodeToString(digest[:])
}

func normalizeContainerStats(identifier string, stats container.StatsResponse, restartCount uint64, sampledAt time.Time) (ContainerStats, error) {
	cores := uint64(stats.CPUStats.OnlineCPUs)
	if cores == 0 {
		cores = uint64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	cpuAvailable := stats.PreCPUStats.SystemUsage > 0 && stats.CPUStats.SystemUsage > stats.PreCPUStats.SystemUsage &&
		stats.CPUStats.CPUUsage.TotalUsage >= stats.PreCPUStats.CPUUsage.TotalUsage && cores > 0
	cpuUsage := 0.0
	if cpuAvailable {
		cpuUsage = float64(stats.CPUStats.CPUUsage.TotalUsage-stats.PreCPUStats.CPUUsage.TotalUsage) /
			float64(stats.CPUStats.SystemUsage-stats.PreCPUStats.SystemUsage) * float64(cores)
	}
	memoryUsage := stats.MemoryStats.Usage
	cache := stats.MemoryStats.Stats["cache"]
	if inactive, exists := stats.MemoryStats.Stats["total_inactive_file"]; exists {
		cache = inactive
	} else if inactive, exists := stats.MemoryStats.Stats["inactive_file"]; exists {
		cache = inactive
	}
	memoryUsage -= min(memoryUsage, cache)
	var readBytes, writeBytes uint64
	for _, network := range stats.Networks {
		if network.RxBytes > ^uint64(0)-readBytes || network.TxBytes > ^uint64(0)-writeBytes {
			return ContainerStats{}, ErrUnavailable
		}
		readBytes += network.RxBytes
		writeBytes += network.TxBytes
	}
	const maximumSafeInteger = uint64(1<<53 - 1)
	if memoryUsage > maximumSafeInteger || readBytes > maximumSafeInteger || writeBytes > maximumSafeInteger || restartCount > maximumSafeInteger {
		return ContainerStats{}, ErrUnavailable
	}
	return ContainerStats{
		ContainerID: identifier, CPUUsageCores: cpuUsage, CPUUnavailable: !cpuAvailable, MemoryUsageBytes: memoryUsage,
		NetworkReadBytes: readBytes, NetworkWriteBytes: writeBytes, RestartCount: restartCount, SampledAt: sampledAt,
	}, nil
}

func engineError(ctx context.Context, err error) error {
	if ctx != nil && ctx.Err() != nil {
		return errors.Join(ErrUnavailable, ctx.Err())
	}
	if errors.Is(err, ErrPermissionDenied) || errdefs.IsPermissionDenied(err) || errdefs.IsUnauthorized(err) {
		return ErrPermissionDenied
	}
	if errors.Is(err, errEngineResponseLimit) {
		return errEngineResponseLimit
	}
	return ErrUnavailable
}

func nonNilErrors(items []error) []error {
	result := make([]error, 0)
	for _, item := range items {
		if item != nil {
			result = append(result, item)
		}
	}
	return result
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

func safeDockerLabel(value string) string {
	if len(value) == 0 || len(value) > 128 {
		return ""
	}
	first := value[0]
	if !(first >= 'a' && first <= 'z' || first >= 'A' && first <= 'Z' || first >= '0' && first <= '9') {
		return ""
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character == '-' || character == '.') {
			return ""
		}
	}
	return value
}

func declaredDependencies(value string) []string {
	if len(value) > 4096 {
		return nil
	}
	seen := map[string]struct{}{}
	for _, item := range strings.Split(value, ",") {
		name := safeDockerLabel(strings.TrimSpace(item))
		if name != "" && len(seen) < 32 {
			seen[name] = struct{}{}
		}
	}
	return sortedKeys(seen)
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

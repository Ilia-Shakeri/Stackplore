package kubernetes

import (
	"context"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"strings"
	"time"
)

func (provider *Provider) GetCurrentMetrics(ctx context.Context, query domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
	if err := provider.validate(ctx, query.Source); err != nil {
		return domain.CurrentMetricResult{}, providerError("kubernetes.metric.current", err)
	}
	if err := query.Validate(); err != nil {
		return domain.CurrentMetricResult{}, providerError("kubernetes.metric.current", err)
	}
	collector, ok := provider.collector.(MetricCollector)
	observedAt := time.Now().UTC().Truncate(time.Microsecond)
	if !ok {
		result := domain.CurrentMetricResult{Status: observationStatus(observedAt, ErrUnsupported, false)}
		return result, result.ValidateFor(query)
	}
	state, snapshot, err := provider.observe(ctx)
	if err != nil {
		return domain.CurrentMetricResult{}, err
	}
	observedAt = state.status.ObservedAt
	observation, err := collector.CurrentPodMetrics(ctx)
	if err != nil {
		return domain.CurrentMetricResult{}, providerError("kubernetes.metric.current", err)
	}
	if observation.Status.State == "" {
		observation.Status = observationStatus(observedAt, nil, false)
	}
	// Per-metric authority is not persisted yet. A known usage denial fences the
	// entire metric port so cached usage cannot survive through configured facts.
	denied := observation.Status.State == domain.AvailabilityPermissionDenied
	for _, status := range observation.Namespaces {
		denied = denied || status.State == domain.AvailabilityPermissionDenied
	}
	if denied {
		result := domain.CurrentMetricResult{Status: observationStatus(observedAt, ErrPermissionDenied, false)}
		return result, result.ValidateFor(query)
	}
	byPodName := make(map[string]PodMetric, len(observation.Metrics))
	for _, metric := range observation.Metrics {
		key := metric.Namespace + "\x00" + metric.PodName
		if metric.Namespace == "" || metric.PodName == "" || metric.SampledAt.Location() != time.UTC || metric.SampledAt.Nanosecond()%int(time.Microsecond) != 0 || metric.SampledAt.After(observedAt.Add(maximumMetricClockSkew)) || metric.Window < time.Second || metric.Window > 24*time.Hour || metric.Window%time.Second != 0 {
			return domain.CurrentMetricResult{}, providerError("kubernetes.metric.current", ErrUnavailable)
		}
		if _, exists := byPodName[key]; exists {
			return domain.CurrentMetricResult{}, providerError("kubernetes.metric.current", ErrUnavailable)
		}
		byPodName[key] = metric
		if metric.SampledAt.After(observedAt) {
			observedAt = metric.SampledAt
		}
	}
	podsByUID := make(map[string]Pod, len(snapshot.Pods))
	for _, pod := range snapshot.Pods {
		podsByUID[pod.UID] = pod
	}
	available := observationStatus(observedAt, nil, false)
	sets := make([]domain.CurrentMetricSet, 0, len(query.Resources))
	statuses := make([]domain.ObservationStatus, 0, len(query.Resources))
	for _, resource := range query.Resources {
		set := domain.CurrentMetricSet{ResourceID: resource.ID, Status: available}
		podUID, containerName, supported := kubernetesMetricTarget(resource)
		pod, podExists := podsByUID[podUID]
		if !supported {
			set.Status = observationStatus(observedAt, ErrUnsupported, false)
		} else if !podExists {
			set.Status = observationStatus(observedAt, ErrUnavailable, false)
			for _, partition := range snapshot.Partitions {
				if partition.Kind == "kubernetes.pod" && partition.Scope == resource.Identity.Scope && !partition.Status.AllowsData() {
					set.Status = partition.Status
					break
				}
			}
		} else {
			metric, metricExists := byPodName[pod.Namespace+"\x00"+pod.Name]
			metricExists = metricExists && metric.PodUID == pod.UID &&
				metricWindowWithinPodLifetime(metric.SampledAt, metric.Window, pod.CreatedAt)
			missing := 0
			usageMissing := false
			missingStatus := observationStatus(observedAt, ErrUnsupported, false)
			for _, name := range query.Names {
				value, unit, known := kubernetesSpecMetricValue(pod, containerName, name)
				sampledAt, interval := snapshot.ObservedAt, time.Duration(0)
				if name == domain.MetricCPUUsageCores || name == domain.MetricMemoryUsageBytes {
					known = false
					if metricExists {
						value, unit, known = kubernetesMetricValue(pod, metric, containerName, name)
						sampledAt, interval = metric.SampledAt, metric.Window
					}
					if !known {
						usageMissing = true
						missingStatus = observation.Status
						if status, exists := observation.Namespaces[pod.Namespace]; exists {
							missingStatus = status
						}
						if missingStatus.AllowsData() {
							missingStatus = observationStatus(observedAt, ErrUnavailable, false)
						}
					}
				}
				if !known {
					missing++
					continue
				}
				set.Samples = append(set.Samples, domain.MetricSample{ID: domain.MetricSampleID("metric_kubernetes_" + shortDigest(string(resource.ID)+"\x00"+string(name)+"\x00"+sampledAt.Format(time.RFC3339Nano))), ResourceID: resource.ID, Name: name, Unit: unit, Value: value, SampledAt: sampledAt, Interval: interval})
			}
			if missing > 0 {
				if len(set.Samples) > 0 {
					set.Status = observationStatus(observedAt, ErrTruncated, true)
					set.Status.ReasonCode = "kubernetes.metric.partial"
					if usageMissing {
						set.Status.State = domain.AvailabilityStale
					}
				} else {
					set.Status = missingStatus
				}
			}
		}
		sets = append(sets, set)
		statuses = append(statuses, set.Status)
	}
	result := domain.CurrentMetricResult{Status: combineStatuses(observedAt, statuses), Sets: sets}
	if !result.Status.AllowsData() {
		result.Sets = nil
	}
	if err := result.ValidateFor(query); err != nil {
		return domain.CurrentMetricResult{}, providerError("kubernetes.metric.current", err)
	}
	return result, nil
}

func kubernetesSpecMetricValue(pod Pod, containerName string, name domain.MetricName) (float64, domain.MetricUnit, bool) {
	if name == domain.MetricRestartCount {
		var total uint64
		if containerName != "" {
			value, known := pod.ContainerRestarts[containerName]
			return float64(value), domain.MetricUnitCount, known
		}
		if len(pod.ContainerRestarts) == 0 {
			return 0, "", false
		}
		for _, value := range pod.ContainerRestarts {
			if value > 1<<53-1-total {
				return 0, "", false
			}
			total += value
		}
		return float64(total), domain.MetricUnitCount, true
	}
	// Pod scheduling requests include init/sidecar-specific maxima. Do not report
	// a simple container sum as the effective Pod scheduling request or limit.
	if containerName == "" {
		return 0, "", false
	}
	for _, container := range pod.Metadata.Containers {
		if container.Name != containerName {
			continue
		}
		switch name {
		case domain.MetricCPURequestCores:
			if container.CPURequestCores != nil {
				return *container.CPURequestCores, domain.MetricUnitCores, true
			}
		case domain.MetricCPULimitCores:
			if container.CPULimitCores != nil {
				return *container.CPULimitCores, domain.MetricUnitCores, true
			}
		case domain.MetricMemoryRequestBytes:
			if container.MemoryRequestBytes != nil {
				return float64(*container.MemoryRequestBytes), domain.MetricUnitBytes, true
			}
		case domain.MetricMemoryLimitBytes:
			if container.MemoryLimitBytes != nil {
				return float64(*container.MemoryLimitBytes), domain.MetricUnitBytes, true
			}
		}
	}
	return 0, "", false
}
func kubernetesMetricTarget(resource domain.ResourceRef) (string, string, bool) {
	nativeID := string(resource.Identity.NativeID())
	switch resource.Identity.Kind {
	case "kubernetes.pod":
		return nativeID, "", true
	case "kubernetes.container":
		podUID, container, found := strings.Cut(nativeID, "/container/")
		return podUID, container, found && podUID != "" && container != ""
	default:
		return "", "", false
	}
}

func kubernetesMetricValue(pod Pod, metric PodMetric, containerName string, name domain.MetricName) (float64, domain.MetricUnit, bool) {
	var cpu float64
	var memory, restarts uint64
	found := containerName == ""
	for _, container := range metric.Containers {
		if containerName != "" && container.Name != containerName {
			continue
		}
		found = true
		cpu += container.CPUCores
		if container.MemoryBytes > uint64(1<<53-1)-memory {
			return 0, "", false
		}
		memory += container.MemoryBytes
		if containerName != "" {
			break
		}
	}
	for name, count := range pod.ContainerRestarts {
		if containerName == "" || name == containerName {
			if count > uint64(1<<53-1)-restarts {
				return 0, "", false
			}
			restarts += count
		}
	}
	if !found {
		return 0, "", false
	}
	switch name {
	case domain.MetricCPUUsageCores:
		return cpu, domain.MetricUnitCores, true
	case domain.MetricMemoryUsageBytes:
		return float64(memory), domain.MetricUnitBytes, true
	case domain.MetricRestartCount:
		return float64(restarts), domain.MetricUnitCount, true
	default:
		return 0, "", false
	}
}

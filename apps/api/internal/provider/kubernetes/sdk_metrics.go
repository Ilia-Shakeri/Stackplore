package kubernetes

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func (collector *APICollector) CurrentPodMetrics(ctx context.Context) (MetricObservation, error) {
	if err := collector.valid(ctx); err != nil {
		return MetricObservation{}, err
	}
	ctx, cancel := collector.observationContext(ctx)
	defer cancel()
	at := collector.now().UTC().Truncate(time.Microsecond)
	result := MetricObservation{Namespaces: make(map[string]domain.ObservationStatus, len(collector.namespaces))}
	statuses := make([]domain.ObservationStatus, 0, len(collector.namespaces))
	budget := collector.maximumItems
	identityBudget := collector.maximumItems
	containerBudget := collector.maximumItems
	for _, namespace := range collector.namespaces {
		pods, identityErr := readSDKList(ctx, namespace, &identityBudget, func(ctx context.Context, options metav1.ListOptions) ([]corev1.Pod, string, error) {
			page, err := collector.core.Pods(namespace).List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return page.Items, page.Continue, nil
		})
		identities := make(map[string]observedPodIdentity, len(pods))
		for _, pod := range pods {
			identities[pod.Name] = observedPodIdentity{UID: string(pod.UID), CreatedAt: pod.CreationTimestamp.UTC().Truncate(time.Microsecond)}
		}
		items, err := readSDKList(ctx, namespace, &budget, func(ctx context.Context, options metav1.ListOptions) ([]metricsv1beta1.PodMetrics, string, error) {
			page, err := collector.metrics.PodMetricses(namespace).List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return page.Items, page.Continue, nil
		}, true)
		err = errors.Join(err, identityErr)
		mapped := make([]PodMetric, 0, len(items))
		for _, item := range items {
			window := item.Window.Duration
			identity, exists := identities[item.Name]
			if !exists || identity.UID == "" || item.UID != "" && string(item.UID) != identity.UID ||
				item.Timestamp.After(at.Add(maximumMetricClockSkew)) || !metricWindowWithinPodLifetime(item.Timestamp.Time, window, identity.CreatedAt) {
				err = errors.Join(err, ErrUnavailable)
				continue
			}
			metric := PodMetric{PodUID: identity.UID, Namespace: item.Namespace, PodName: item.Name, SampledAt: item.Timestamp.UTC().Truncate(time.Microsecond), Window: window.Round(time.Second)}
			if len(item.Containers) == 0 {
				err = errors.Join(err, ErrUnavailable)
				continue
			}
			valid := true
			seen := make(map[string]struct{}, len(item.Containers))
			for _, container := range item.Containers {
				containerBudget--
				if containerBudget < 0 {
					err = errors.Join(err, ErrTruncated)
					valid = false
					break
				}
				cpuValue, cpuKnown := container.Usage[corev1.ResourceCPU]
				memoryValue, memoryKnown := container.Usage[corev1.ResourceMemory]
				cpu, memory := safeCPUQuantity(cpuValue), safeMemoryQuantity(memoryValue)
				_, duplicate := seen[container.Name]
				seen[container.Name] = struct{}{}
				if container.Name == "" || duplicate || !cpuKnown || !memoryKnown || cpu == nil || memory == nil {
					err = errors.Join(err, ErrUnavailable)
					valid = false
					break
				}
				metric.Containers = append(metric.Containers, ContainerMetric{Name: container.Name, CPUCores: *cpu, MemoryBytes: *memory})
			}
			if valid && len(metric.Containers) > 0 {
				after, afterErr := collector.readPodIdentity(ctx, namespace, item.Name)
				if afterErr != nil || after.UID != identity.UID || !after.CreatedAt.Equal(identity.CreatedAt) {
					err = errors.Join(err, afterErr, ErrUnavailable)
					continue
				}
				mapped = append(mapped, metric)
			}
		}
		var status domain.ObservationStatus
		if errors.Is(err, ErrPermissionDenied) {
			mapped = nil
			status = observationStatus(at, ErrPermissionDenied, false)
		} else {
			status = observationStatus(at, err, len(mapped) > 0)
		}
		result.Namespaces[namespace] = status
		statuses = append(statuses, status)
		result.Metrics = append(result.Metrics, mapped...)
	}
	for _, status := range result.Namespaces {
		if status.State == domain.AvailabilityPermissionDenied {
			result.Status = observationStatus(at, ErrPermissionDenied, false)
			result.Metrics = nil
			return result, nil
		}
	}
	result.Status = combineStatuses(at, statuses)
	return result, nil
}

// Metrics API names are reusable and often have no UID. Accept the full sample
// window only inside the verified Pod lifetime, before rounding for transport.
func metricWindowWithinPodLifetime(sampledAt time.Time, window time.Duration, createdAt time.Time) bool {
	return !sampledAt.IsZero() && !createdAt.IsZero() && createdAt.UnixMicro() >= 0 &&
		window >= time.Second && window <= 24*time.Hour && !sampledAt.Add(-window).Before(createdAt)
}

package kubernetes

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type EventObservation struct {
	Status domain.ObservationStatus
	Events []ClusterEvent
}
type MetricObservation struct {
	Status     domain.ObservationStatus
	Metrics    []PodMetric
	Namespaces map[string]domain.ObservationStatus
}
type LogObservation struct {
	Status domain.ObservationStatus
	Lines  []PodLogLine
}

const maximumCollectedLogLines = 1000
const maximumMetricClockSkew = time.Minute

func (collector *APICollector) ClusterEvents(ctx context.Context) (EventObservation, error) {
	if err := collector.valid(ctx); err != nil {
		return EventObservation{}, err
	}
	ctx, cancel := collector.observationContext(ctx)
	defer cancel()
	at := collector.now().UTC().Truncate(time.Microsecond)
	result := EventObservation{}
	statuses := make([]domain.ObservationStatus, 0, len(collector.namespaces))
	budget := min(collector.maximumItems, 1000)
	for _, namespace := range collector.namespaces {
		items, err := readSDKList(ctx, namespace, &budget, func(ctx context.Context, options metav1.ListOptions) ([]corev1.Event, string, error) {
			page, err := collector.core.Events(namespace).List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return page.Items, page.Continue, nil
		})
		statuses = append(statuses, observationStatus(at, err, len(items) > 0))
		for _, item := range items {
			occurred := item.EventTime.Time
			if item.Series != nil && !item.Series.LastObservedTime.IsZero() {
				occurred = item.Series.LastObservedTime.Time
			} else if !item.LastTimestamp.IsZero() {
				occurred = item.LastTimestamp.Time
			} else if occurred.IsZero() {
				occurred = item.CreationTimestamp.Time
			}
			if occurred.IsZero() {
				continue
			}
			count := int64(item.Count)
			if item.Series != nil && item.Series.Count > 0 {
				count = int64(item.Series.Count)
			}
			result.Events = append(result.Events, ClusterEvent{UID: string(item.UID), InvolvedUID: string(item.InvolvedObject.UID), Reason: item.Reason, Message: item.Message, Type: item.Type, OccurredAt: occurred.UTC().Truncate(time.Microsecond), Occurrences: uint64(max(count, 1))})
		}
	}
	result.Status = combineStatuses(at, statuses)
	return result, nil
}

func (collector *APICollector) ReadPodLogs(ctx context.Context, request PodLogRequest) (LogObservation, error) {
	if err := collector.valid(ctx); err != nil {
		return LogObservation{}, err
	}
	ctx, cancel := collector.observationContext(ctx)
	defer cancel()
	if len(request.Targets) < 1 || len(request.Targets) > 64 || request.LineLimit < 1 || request.LineLimit > maximumCollectedLogLines ||
		request.ByteLimit < 1 || request.ByteLimit > domain.MaxLogBytes || request.Start.Location() != time.UTC {
		return LogObservation{}, ErrUnavailable
	}
	at := collector.now().UTC().Truncate(time.Microsecond)
	result := LogObservation{Lines: make([]PodLogLine, 0, request.LineLimit)}
	statuses := make([]domain.ObservationStatus, 0, len(request.Targets))
	seen := make(map[string]struct{}, len(request.Targets))
	for _, target := range request.Targets {
		if target.Key == "" || len(target.Key) > 512 || target.ExpectedUID == "" || len(target.ExpectedUID) > 512 || !collector.permits(target.Namespace) || !safeKubernetesPathSegment(target.PodName) || !safeKubernetesPathSegment(target.Container) {
			return LogObservation{}, ErrPermissionDenied
		}
		if _, duplicate := seen[target.Key]; duplicate {
			return LogObservation{}, ErrUnavailable
		}
		seen[target.Key] = struct{}{}
	}
	bytesUsed := 0
	for _, target := range request.Targets {
		remaining := request.ByteLimit - bytesUsed
		if remaining <= 0 || len(result.Lines) >= request.LineLimit {
			statuses = append(statuses, observationStatus(at, ErrTruncated, true))
			break
		}
		before, identityErr := collector.readPodIdentity(ctx, target.Namespace, target.PodName)
		if identityErr != nil || before.UID != target.ExpectedUID {
			if identityErr == nil {
				identityErr = ErrUnavailable
			}
			statuses = append(statuses, observationStatus(at, identityErr, false))
			continue
		}
		limitBytes := int64(remaining)
		since := metav1.NewTime(request.Start)
		stream, err := collector.core.Pods(target.Namespace).GetLogs(target.PodName, &corev1.PodLogOptions{Container: target.Container, Previous: target.Previous, Timestamps: true, SinceTime: &since, LimitBytes: &limitBytes, Follow: false}).Stream(ctx)
		if err != nil {
			statuses = append(statuses, observationStatus(at, mapAPIError(err), false))
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(stream, int64(remaining)+1))
		stream.Close()
		if readErr != nil {
			statuses = append(statuses, observationStatus(at, mapAPIError(readErr), false))
			continue
		}
		after, identityErr := collector.readPodIdentity(ctx, target.Namespace, target.PodName)
		if identityErr != nil || after.UID != target.ExpectedUID {
			if identityErr == nil {
				identityErr = ErrUnavailable
			}
			statuses = append(statuses, observationStatus(at, identityErr, false))
			continue
		}
		truncated := len(data) >= remaining
		if truncated {
			data = data[:remaining]
		}
		lines := parsePodLogLines(target.Key, data)
		if truncated && len(lines) > 0 {
			lines[len(lines)-1].Truncated = true
		}
		for _, line := range lines {
			if len(result.Lines) >= request.LineLimit || len(line.Message) > request.ByteLimit-bytesUsed {
				truncated = true
				break
			}
			bytesUsed += len(line.Message)
			result.Lines = append(result.Lines, line)
		}
		var targetErr error
		if truncated {
			targetErr = ErrTruncated
		}
		statuses = append(statuses, observationStatus(at, targetErr, len(lines) > 0))
	}
	result.Status = combineStatuses(at, statuses)
	return result, nil
}

func safeKubernetesPathSegment(value string) bool {
	if value == "" || len(value) > 253 {
		return false
	}
	for _, character := range value {
		if character != '.' && character != '-' && (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}
	return value != "." && value != ".."
}

func parsePodLogLines(targetKey string, data []byte) []PodLogLine {
	parts := strings.Split(string(data), "\n")
	result := make([]PodLogLine, 0, len(parts))
	for _, part := range parts {
		timestampText, message, found := strings.Cut(part, " ")
		if !found {
			continue
		}
		timestamp, err := time.Parse(time.RFC3339Nano, timestampText)
		if err != nil {
			continue
		}
		truncated := len(message) > domain.MaxLogLineBytes
		if truncated {
			message = message[:domain.MaxLogLineBytes]
		}
		result = append(result, PodLogLine{TargetKey: targetKey, Timestamp: timestamp.UTC().Truncate(time.Microsecond), Message: message, Truncated: truncated})
	}
	return result
}

// These existing guarded operations use the official request machinery. Their
// deployment, recent-auth and one-use confirmation gates remain above the adapter.
func (collector *APICollector) RestartWorkload(ctx context.Context, operation WorkloadOperation) (string, error) {
	if operation.RequestedReplicas != nil {
		return "", ErrUnavailable
	}
	body := map[string]any{"metadata": map[string]any{"resourceVersion": operation.ResourceVersion}, "spec": map[string]any{"template": map[string]any{"metadata": map[string]any{"annotations": map[string]string{"stackplore.io/restart-request": operation.IdempotencyKey}}}}}
	return collector.patchWorkload(ctx, operation, false, body)
}

func (collector *APICollector) ScaleWorkload(ctx context.Context, operation WorkloadOperation) (string, error) {
	if operation.RequestedReplicas == nil || *operation.RequestedReplicas < 0 || *operation.RequestedReplicas > domain.MaxOperationReplicas {
		return "", ErrUnavailable
	}
	body := map[string]any{"apiVersion": "autoscaling/v1", "kind": "Scale", "metadata": map[string]any{"name": operation.Name, "namespace": operation.Namespace, "resourceVersion": operation.ResourceVersion}, "spec": map[string]any{"replicas": *operation.RequestedReplicas}}
	return collector.patchWorkload(ctx, operation, true, body)
}

func (collector *APICollector) patchWorkload(ctx context.Context, operation WorkloadOperation, scale bool, body any) (string, error) {
	if err := collector.valid(ctx); err != nil {
		return "", err
	}
	if !collector.permits(operation.Namespace) || !safeKubernetesPathSegment(operation.Name) || operation.ResourceVersion == "" || len(operation.ResourceVersion) > 512 || operation.IdempotencyKey == "" || len(operation.IdempotencyKey) > 256 {
		return "", ErrUnavailable
	}
	resource := ""
	switch operation.Kind {
	case "Deployment":
		resource = "deployments"
	case "StatefulSet":
		resource = "statefulsets"
	case "DaemonSet":
		if !scale {
			resource = "daemonsets"
		}
	}
	if resource == "" {
		return "", ErrUnsupported
	}
	payload, err := json.Marshal(body)
	if err != nil || len(payload) > 16*1024 {
		return "", ErrUnavailable
	}
	request := collector.apps.RESTClient().Patch(types.MergePatchType).Namespace(operation.Namespace).Resource(resource).Name(operation.Name).Body(payload).MaxRetries(0)
	if scale {
		request = request.SubResource("scale")
	}
	data, err := request.Do(ctx).Raw()
	if err != nil {
		return "", mapAPIError(err)
	}
	var result struct {
		Metadata struct {
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
	}
	if json.Unmarshal(data, &result) != nil || result.Metadata.ResourceVersion == "" || len(result.Metadata.ResourceVersion) > 512 {
		return "", ErrUnavailable
	}
	return result.Metadata.ResourceVersion, nil
}

func contextCause(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

var _ EventCollector = (*APICollector)(nil)
var _ MetricCollector = (*APICollector)(nil)
var _ LogCollector = (*APICollector)(nil)
var _ OperationCollector = (*APICollector)(nil)

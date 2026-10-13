package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const (
	kubernetesAPIBase       = "https://kubernetes.default.svc"
	minimumAPIResponseBytes = 64 * 1024
	maximumAPIResponseBytes = 32 * 1024 * 1024
	maximumAPIItems         = 10_000
	maximumAPIPages         = 100
)

type APICollectorOptions struct {
	Client          *http.Client
	BearerToken     string
	MaximumItems    int
	MaximumResponse int64
	Now             func() time.Time
}

type APICollector struct {
	client          *http.Client
	bearerToken     string
	maximumItems    int
	maximumResponse int64
	now             func() time.Time
}

func NewAPICollector(options APICollectorOptions) (*APICollector, error) {
	if options.Client == nil || options.BearerToken == "" || strings.TrimSpace(options.BearerToken) != options.BearerToken ||
		strings.ContainsAny(options.BearerToken, "\r\n") || options.MaximumItems < 1 || options.MaximumItems > maximumAPIItems ||
		options.MaximumResponse < minimumAPIResponseBytes || options.MaximumResponse > maximumAPIResponseBytes {
		return nil, fmt.Errorf("Kubernetes API collector options are invalid")
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &APICollector{
		client: options.Client, bearerToken: options.BearerToken, maximumItems: options.MaximumItems,
		maximumResponse: options.MaximumResponse, now: now,
	}, nil
}

func (collector *APICollector) Snapshot(ctx context.Context) (Snapshot, error) {
	if collector == nil || collector.client == nil || ctx == nil || ctx.Err() != nil {
		return Snapshot{}, errors.Join(ErrUnavailable, contextCause(ctx))
	}
	budget := collector.maximumItems
	namespaces, err := getAll[namespaceObject](ctx, collector, "/api/v1/namespaces", &budget)
	if err != nil {
		return Snapshot{}, err
	}
	nodes, err := getAll[nodeObject](ctx, collector, "/api/v1/nodes", &budget)
	if err != nil {
		return Snapshot{}, err
	}
	pods, err := getAll[podObject](ctx, collector, "/api/v1/pods", &budget)
	if err != nil {
		return Snapshot{}, err
	}
	services, err := getAll[serviceObject](ctx, collector, "/api/v1/services", &budget)
	if err != nil {
		return Snapshot{}, err
	}
	claims, err := getAll[claimObject](ctx, collector, "/api/v1/persistentvolumeclaims", &budget)
	if err != nil {
		return Snapshot{}, err
	}
	deployments, err := getAll[scaleWorkloadObject](ctx, collector, "/apis/apps/v1/deployments", &budget)
	if err != nil {
		return Snapshot{}, err
	}
	statefulSets, err := getAll[scaleWorkloadObject](ctx, collector, "/apis/apps/v1/statefulsets", &budget)
	if err != nil {
		return Snapshot{}, err
	}
	daemonSets, err := getAll[daemonWorkloadObject](ctx, collector, "/apis/apps/v1/daemonsets", &budget)
	if err != nil {
		return Snapshot{}, err
	}
	replicaSets, err := getAll[scaleWorkloadObject](ctx, collector, "/apis/apps/v1/replicasets", &budget)
	if err != nil {
		return Snapshot{}, err
	}
	jobs, err := getAll[jobObject](ctx, collector, "/apis/batch/v1/jobs", &budget)
	if err != nil {
		return Snapshot{}, err
	}
	cronJobs, err := getAll[metadataOnlyObject](ctx, collector, "/apis/batch/v1/cronjobs", &budget)
	if err != nil {
		return Snapshot{}, err
	}
	ingresses, err := getAll[ingressObject](ctx, collector, "/apis/networking.k8s.io/v1/ingresses", &budget)
	if err != nil {
		return Snapshot{}, err
	}
	endpointSlices, err := getAll[endpointSliceObject](ctx, collector, "/apis/discovery.k8s.io/v1/endpointslices", &budget)
	if err != nil {
		return Snapshot{}, err
	}
	return normalizeAPISnapshot(collector.now().UTC().Truncate(time.Microsecond), namespaces, nodes, pods, services, claims, deployments, statefulSets, replicaSets, daemonSets, jobs, cronJobs, ingresses, endpointSlices), nil
}

func (collector *APICollector) ClusterEvents(ctx context.Context) ([]ClusterEvent, error) {
	if collector == nil || collector.client == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.Join(ErrUnavailable, contextCause(ctx))
	}
	budget := min(collector.maximumItems, 1000)
	items, err := getAll[eventObject](ctx, collector, "/api/v1/events", &budget)
	if err != nil {
		return nil, err
	}
	result := make([]ClusterEvent, 0, len(items))
	for _, item := range items {
		occurredAt := item.EventTime
		if !item.Series.LastObservedTime.IsZero() {
			occurredAt = item.Series.LastObservedTime
		} else if !item.LastTimestamp.IsZero() {
			occurredAt = item.LastTimestamp
		} else if occurredAt.IsZero() {
			occurredAt = item.Metadata.CreationTimestamp
		}
		if occurredAt.IsZero() {
			continue
		}
		result = append(result, ClusterEvent{
			UID: item.Metadata.UID, InvolvedUID: item.InvolvedObject.UID,
			Reason: item.Reason, Message: item.Message, Type: item.Type,
			OccurredAt:  occurredAt.UTC().Truncate(time.Microsecond),
			Occurrences: eventOccurrences(item),
		})
	}
	return result, nil
}

func (collector *APICollector) CurrentPodMetrics(ctx context.Context) ([]PodMetric, error) {
	if collector == nil || collector.client == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.Join(ErrUnavailable, contextCause(ctx))
	}
	budget := collector.maximumItems
	items, err := getAll[podMetricObject](ctx, collector, "/apis/metrics.k8s.io/v1beta1/pods", &budget)
	if err != nil {
		return nil, err
	}
	result := make([]PodMetric, 0, len(items))
	containerCount := 0
	for _, item := range items {
		window, err := time.ParseDuration(item.Window)
		if err != nil || window < time.Second || window > 24*time.Hour || window%time.Second != 0 || item.Timestamp.IsZero() {
			return nil, ErrUnavailable
		}
		metric := PodMetric{
			Namespace: item.Metadata.Namespace, PodName: item.Metadata.Name,
			SampledAt: item.Timestamp.UTC().Truncate(time.Microsecond), Window: window,
		}
		for _, container := range item.Containers {
			containerCount++
			if containerCount > collector.maximumItems || container.Name == "" {
				return nil, ErrUnavailable
			}
			cpu, err := parseCPUQuantity(container.Usage.CPU)
			if err != nil {
				return nil, ErrUnavailable
			}
			memory, err := parseMemoryQuantity(container.Usage.Memory)
			if err != nil {
				return nil, ErrUnavailable
			}
			metric.Containers = append(metric.Containers, ContainerMetric{Name: container.Name, CPUCores: cpu, MemoryBytes: memory})
		}
		result = append(result, metric)
	}
	return result, nil
}

func (collector *APICollector) RestartWorkload(ctx context.Context, operation WorkloadOperation) (string, error) {
	path, ok := workloadAPIPath(operation, false)
	if !ok || operation.RequestedReplicas != nil {
		return "", ErrUnavailable
	}
	body := map[string]any{
		"metadata": map[string]any{"resourceVersion": operation.ResourceVersion},
		"spec": map[string]any{"template": map[string]any{"metadata": map[string]any{
			"annotations": map[string]string{"stackplore.io/restart-request": operation.IdempotencyKey},
		}}},
	}
	return collector.patchWorkload(ctx, path, body)
}

func (collector *APICollector) ScaleWorkload(ctx context.Context, operation WorkloadOperation) (string, error) {
	path, ok := workloadAPIPath(operation, true)
	if !ok || operation.RequestedReplicas == nil || *operation.RequestedReplicas < 0 || *operation.RequestedReplicas > domain.MaxOperationReplicas {
		return "", ErrUnavailable
	}
	body := map[string]any{
		"apiVersion": "autoscaling/v1", "kind": "Scale",
		"metadata": map[string]any{
			"name": operation.Name, "namespace": operation.Namespace, "resourceVersion": operation.ResourceVersion,
		},
		"spec": map[string]any{"replicas": *operation.RequestedReplicas},
	}
	return collector.patchWorkload(ctx, path, body)
}

func workloadAPIPath(operation WorkloadOperation, scale bool) (string, bool) {
	if !safeKubernetesPathSegment(operation.Namespace) || !safeKubernetesPathSegment(operation.Name) ||
		operation.ResourceVersion == "" || operation.IdempotencyKey == "" {
		return "", false
	}
	plural := ""
	switch operation.Kind {
	case "Deployment":
		plural = "deployments"
	case "StatefulSet":
		plural = "statefulsets"
	case "DaemonSet":
		if scale {
			return "", false
		}
		plural = "daemonsets"
	default:
		return "", false
	}
	path := "/apis/apps/v1/namespaces/" + url.PathEscape(operation.Namespace) + "/" + plural + "/" + url.PathEscape(operation.Name)
	if scale {
		path += "/scale"
	}
	return path, true
}

func (collector *APICollector) patchWorkload(ctx context.Context, path string, body any) (string, error) {
	if collector == nil || collector.client == nil || ctx == nil || ctx.Err() != nil {
		return "", errors.Join(ErrUnavailable, contextCause(ctx))
	}
	payload, err := json.Marshal(body)
	if err != nil || len(payload) < 2 || len(payload) > 16*1024 {
		return "", ErrUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPatch, kubernetesAPIBase+path, bytes.NewReader(payload))
	if err != nil {
		return "", ErrUnavailable
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/merge-patch+json")
	request.Header.Set("Authorization", "Bearer "+collector.bearerToken)
	response, err := collector.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return "", ErrPermissionDenied
	}
	if response.StatusCode == http.StatusConflict {
		return "", ErrVersionConflict
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", ErrUnavailable
	}
	var result metadataOnlyObject
	reader := http.MaxBytesReader(nil, response.Body, collector.maximumResponse)
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&result); err != nil || result.Metadata.ResourceVersion == "" {
		return "", ErrUnavailable
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return "", ErrUnavailable
	}
	return result.Metadata.ResourceVersion, nil
}

func (collector *APICollector) ReadPodLogs(ctx context.Context, request PodLogRequest) ([]PodLogLine, error) {
	if collector == nil || collector.client == nil || ctx == nil || ctx.Err() != nil || len(request.Targets) > 64 ||
		request.LineLimit < 1 || request.LineLimit > domain.MaxLogLines || request.ByteLimit < 1 || request.ByteLimit > domain.MaxLogBytes ||
		request.Start.Location() != time.UTC {
		return nil, errors.Join(ErrUnavailable, contextCause(ctx))
	}
	result := make([]PodLogLine, 0, request.LineLimit)
	seen := make(map[string]struct{}, len(request.Targets))
	bytesUsed := 0
	for _, target := range request.Targets {
		if target.Key == "" || len(target.Key) > 512 || !safeKubernetesPathSegment(target.Namespace) ||
			!safeKubernetesPathSegment(target.PodName) || !safeKubernetesPathSegment(target.Container) {
			return nil, ErrUnavailable
		}
		if _, exists := seen[target.Key]; exists {
			return nil, ErrUnavailable
		}
		seen[target.Key] = struct{}{}
		remaining := request.ByteLimit - bytesUsed
		values := url.Values{
			"container": {target.Container}, "timestamps": {"true"},
			"sinceTime": {request.Start.Format(time.RFC3339Nano)}, "tailLines": {strconv.Itoa(request.LineLimit)},
			"limitBytes": {strconv.Itoa(remaining)}, "previous": {strconv.FormatBool(target.Previous)},
		}
		path := "/api/v1/namespaces/" + url.PathEscape(target.Namespace) + "/pods/" + url.PathEscape(target.PodName) + "/log?" + values.Encode()
		data, responseTruncated, err := collector.getRaw(ctx, path, remaining)
		if err != nil {
			return nil, err
		}
		lines := parsePodLogLines(target.Key, data)
		if responseTruncated && len(lines) > 0 {
			lines[len(lines)-1].Truncated = true
		}
		for _, line := range lines {
			lineBytes := len(line.Message)
			if len(result) >= request.LineLimit || lineBytes > request.ByteLimit-bytesUsed {
				return result, nil
			}
			bytesUsed += lineBytes
			result = append(result, line)
		}
		if len(result) >= request.LineLimit || bytesUsed >= request.ByteLimit {
			break
		}
	}
	return result, nil
}

func (collector *APICollector) getRaw(ctx context.Context, path string, maximum int) ([]byte, bool, error) {
	if maximum < 1 {
		return nil, false, ErrUnavailable
	}
	maximum = min(maximum, int(collector.maximumResponse))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, kubernetesAPIBase+path, nil)
	if err != nil {
		return nil, false, ErrUnavailable
	}
	request.Header.Set("Accept", "text/plain")
	request.Header.Set("Authorization", "Bearer "+collector.bearerToken)
	response, err := collector.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		return nil, false, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, false, ErrPermissionDenied
	}
	if response.StatusCode != http.StatusOK {
		return nil, false, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, int64(maximum)+1))
	if err != nil {
		return nil, false, ErrUnavailable
	}
	truncated := len(data) > maximum
	if truncated {
		data = data[:maximum]
	}
	return data, truncated, nil
}

func safeKubernetesPathSegment(value string) bool {
	if value == "" || len(value) > 253 {
		return false
	}
	for _, character := range value {
		if character != '.' && character != '-' && character != '_' &&
			(character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func parsePodLogLines(targetKey string, data []byte) []PodLogLine {
	parts := strings.Split(string(data), "\n")
	result := make([]PodLogLine, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
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
		result = append(result, PodLogLine{
			TargetKey: targetKey, Timestamp: timestamp.UTC().Truncate(time.Microsecond),
			Message: message, Truncated: truncated,
		})
	}
	return result
}

type listResponse[T any] struct {
	Metadata struct {
		Continue string `json:"continue"`
	} `json:"metadata"`
	Items []T `json:"items"`
}

func getAll[T any](ctx context.Context, collector *APICollector, path string, budget *int) ([]T, error) {
	result := make([]T, 0)
	continuation := ""
	for page := 0; page < maximumAPIPages; page++ {
		if *budget <= 0 {
			return nil, ErrUnavailable
		}
		values := url.Values{"limit": {strconvItoa(min(*budget, 500))}}
		if continuation != "" {
			values.Set("continue", continuation)
		}
		var response listResponse[T]
		if err := collector.getJSON(ctx, path+"?"+values.Encode(), &response); err != nil {
			return nil, err
		}
		if len(response.Items) > *budget {
			return nil, ErrUnavailable
		}
		result = append(result, response.Items...)
		*budget -= len(response.Items)
		if response.Metadata.Continue == "" {
			return result, nil
		}
		if response.Metadata.Continue == continuation || len(response.Metadata.Continue) > 4096 {
			return nil, ErrUnavailable
		}
		continuation = response.Metadata.Continue
	}
	return nil, ErrUnavailable
}

func (collector *APICollector) getJSON(ctx context.Context, path string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, kubernetesAPIBase+path, nil)
	if err != nil {
		return ErrUnavailable
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+collector.bearerToken)
	response, err := collector.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrPermissionDenied
	}
	if response.StatusCode != http.StatusOK {
		return ErrUnavailable
	}
	reader := http.MaxBytesReader(nil, response.Body, collector.maximumResponse)
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(target); err != nil {
		return ErrUnavailable
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return ErrUnavailable
	}
	return nil
}

type objectMetadata struct {
	UID               string            `json:"uid"`
	ResourceVersion   string            `json:"resourceVersion"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	CreationTimestamp time.Time         `json:"creationTimestamp"`
	Labels            map[string]string `json:"labels"`
	OwnerReferences   []ownerReference  `json:"ownerReferences"`
}

type eventObject struct {
	Metadata       objectMetadata `json:"metadata"`
	InvolvedObject struct {
		UID string `json:"uid"`
	} `json:"involvedObject"`
	Reason        string    `json:"reason"`
	Message       string    `json:"message"`
	Type          string    `json:"type"`
	EventTime     time.Time `json:"eventTime"`
	LastTimestamp time.Time `json:"lastTimestamp"`
	Count         uint64    `json:"count"`
	Series        struct {
		LastObservedTime time.Time `json:"lastObservedTime"`
		Count            uint64    `json:"count"`
	} `json:"series"`
}

func eventOccurrences(item eventObject) uint64 {
	if item.Series.Count > 0 {
		return item.Series.Count
	}
	return max(item.Count, 1)
}

type ownerReference struct {
	UID        string `json:"uid"`
	Kind       string `json:"kind"`
	Controller *bool  `json:"controller"`
}

type condition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type namespaceObject struct {
	Metadata objectMetadata `json:"metadata"`
	Status   struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type nodeObject struct {
	Metadata objectMetadata `json:"metadata"`
	Status   struct {
		Conditions []condition `json:"conditions"`
	} `json:"status"`
}

type scaleWorkloadObject struct {
	Metadata objectMetadata `json:"metadata"`
	Spec     struct {
		Replicas *int `json:"replicas"`
	} `json:"spec"`
	Status struct {
		ReadyReplicas int `json:"readyReplicas"`
	} `json:"status"`
}

type daemonWorkloadObject struct {
	Metadata objectMetadata `json:"metadata"`
	Status   struct {
		Desired int `json:"desiredNumberScheduled"`
		Ready   int `json:"numberReady"`
	} `json:"status"`
}

type jobObject struct {
	Metadata objectMetadata `json:"metadata"`
	Spec     struct {
		Completions *int `json:"completions"`
	} `json:"spec"`
	Status struct {
		Succeeded int `json:"succeeded"`
	} `json:"status"`
}

type metadataOnlyObject struct {
	Metadata objectMetadata `json:"metadata"`
}

type podObject struct {
	Metadata objectMetadata `json:"metadata"`
	Spec     struct {
		Containers []struct {
			Name    string `json:"name"`
			EnvFrom []struct {
				ConfigMapRef *namedReference `json:"configMapRef"`
				SecretRef    *namedReference `json:"secretRef"`
			} `json:"envFrom"`
			Env []struct {
				ValueFrom *struct {
					ConfigMapKeyRef *namedReference `json:"configMapKeyRef"`
					SecretKeyRef    *namedReference `json:"secretKeyRef"`
				} `json:"valueFrom"`
			} `json:"env"`
		} `json:"containers"`
		Volumes []struct {
			PersistentVolumeClaim *struct {
				ClaimName string `json:"claimName"`
			} `json:"persistentVolumeClaim"`
			ConfigMap *namedReference `json:"configMap"`
			Secret    *struct {
				SecretName string `json:"secretName"`
			} `json:"secret"`
		} `json:"volumes"`
	} `json:"spec"`
	Status struct {
		Phase             string      `json:"phase"`
		Conditions        []condition `json:"conditions"`
		ContainerStatuses []struct {
			Name         string `json:"name"`
			RestartCount uint64 `json:"restartCount"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

type podMetricObject struct {
	Metadata   objectMetadata `json:"metadata"`
	Timestamp  time.Time      `json:"timestamp"`
	Window     string         `json:"window"`
	Containers []struct {
		Name  string `json:"name"`
		Usage struct {
			CPU    string `json:"cpu"`
			Memory string `json:"memory"`
		} `json:"usage"`
	} `json:"containers"`
}

type namedReference struct {
	Name string `json:"name"`
}

type serviceObject struct {
	Metadata objectMetadata `json:"metadata"`
	Spec     struct {
		Selector map[string]string `json:"selector"`
	} `json:"spec"`
}

type claimObject struct {
	Metadata objectMetadata `json:"metadata"`
	Status   struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type ingressObject struct {
	Metadata objectMetadata `json:"metadata"`
	Spec     struct {
		DefaultBackend *ingressBackend `json:"defaultBackend"`
		Rules          []struct {
			HTTP *struct {
				Paths []struct {
					Backend ingressBackend `json:"backend"`
				} `json:"paths"`
			} `json:"http"`
		} `json:"rules"`
	} `json:"spec"`
}

type ingressBackend struct {
	Service *struct {
		Name string `json:"name"`
	} `json:"service"`
}

type endpointSliceObject struct {
	Metadata  objectMetadata `json:"metadata"`
	Endpoints []struct {
		TargetRef *struct {
			Kind string `json:"kind"`
			UID  string `json:"uid"`
		} `json:"targetRef"`
	} `json:"endpoints"`
}

func normalizeAPISnapshot(
	observedAt time.Time,
	namespaces []namespaceObject,
	nodes []nodeObject,
	pods []podObject,
	services []serviceObject,
	claims []claimObject,
	deployments, statefulSets, replicaSets []scaleWorkloadObject,
	daemonSets []daemonWorkloadObject,
	jobs []jobObject,
	cronJobs []metadataOnlyObject,
	ingresses []ingressObject,
	endpointSlices []endpointSliceObject,
) Snapshot {
	result := Snapshot{ObservedAt: observedAt, ClusterID: kubernetesAPIBase, ClusterName: "In-cluster Kubernetes", ContextName: "service-account"}
	for _, item := range namespaces {
		result.Namespaces = append(result.Namespaces, Namespace{Name: item.Metadata.Name, Phase: item.Status.Phase})
	}
	for _, item := range nodes {
		ready, known := readyCondition(item.Status.Conditions)
		result.Nodes = append(result.Nodes, Node{UID: item.Metadata.UID, Name: item.Metadata.Name, Ready: ready, ReadyKnown: known})
	}
	appendScaleWorkloads := func(kind string, items []scaleWorkloadObject) {
		for _, item := range items {
			desired, known := 0, item.Spec.Replicas != nil
			if known {
				desired = *item.Spec.Replicas
			}
			ownerKind, ownerUID := controllerOwner(item.Metadata.OwnerReferences)
			result.Workloads = append(result.Workloads, Workload{
				UID: item.Metadata.UID, Version: item.Metadata.ResourceVersion, Kind: kind, Namespace: item.Metadata.Namespace, Name: item.Metadata.Name,
				Desired: desired, Ready: item.Status.ReadyReplicas, ReadyKnown: known, OwnerKind: ownerKind, OwnerUID: ownerUID,
			})
		}
	}
	appendScaleWorkloads("Deployment", deployments)
	appendScaleWorkloads("StatefulSet", statefulSets)
	appendScaleWorkloads("ReplicaSet", replicaSets)
	for _, item := range daemonSets {
		ownerKind, ownerUID := controllerOwner(item.Metadata.OwnerReferences)
		result.Workloads = append(result.Workloads, Workload{
			UID: item.Metadata.UID, Version: item.Metadata.ResourceVersion, Kind: "DaemonSet", Namespace: item.Metadata.Namespace, Name: item.Metadata.Name,
			Desired: item.Status.Desired, Ready: item.Status.Ready, ReadyKnown: true, OwnerKind: ownerKind, OwnerUID: ownerUID,
		})
	}
	for _, item := range jobs {
		desired := 1
		if item.Spec.Completions != nil {
			desired = *item.Spec.Completions
		}
		ownerKind, ownerUID := controllerOwner(item.Metadata.OwnerReferences)
		result.Workloads = append(result.Workloads, Workload{
			UID: item.Metadata.UID, Kind: "Job", Namespace: item.Metadata.Namespace, Name: item.Metadata.Name,
			Desired: desired, Ready: item.Status.Succeeded, ReadyKnown: true, OwnerKind: ownerKind, OwnerUID: ownerUID,
		})
	}
	for _, item := range cronJobs {
		result.Workloads = append(result.Workloads, Workload{
			UID: item.Metadata.UID, Kind: "CronJob", Namespace: item.Metadata.Namespace, Name: item.Metadata.Name,
		})
	}
	configRefs := map[string]ObjectRef{}
	secretRefs := map[string]ObjectRef{}
	for _, item := range pods {
		ready, known := readyCondition(item.Status.Conditions)
		ownerKind, ownerUID := controllerOwner(item.Metadata.OwnerReferences)
		pod := Pod{
			UID: item.Metadata.UID, Namespace: item.Metadata.Namespace, Name: item.Metadata.Name, Phase: item.Status.Phase,
			Ready: ready, ReadyKnown: known, OwnerKind: ownerKind, OwnerUID: ownerUID, Labels: item.Metadata.Labels,
			ContainerRestarts: make(map[string]uint64, len(item.Status.ContainerStatuses)),
		}
		for _, status := range item.Status.ContainerStatuses {
			if status.Name != "" {
				pod.ContainerRestarts[status.Name] = status.RestartCount
			}
		}
		for _, container := range item.Spec.Containers {
			pod.Containers = appendUnique(pod.Containers, container.Name)
			for _, source := range container.EnvFrom {
				if source.ConfigMapRef != nil {
					pod.ConfigMapNames = appendUnique(pod.ConfigMapNames, source.ConfigMapRef.Name)
				}
				if source.SecretRef != nil {
					pod.SecretNames = appendUnique(pod.SecretNames, source.SecretRef.Name)
				}
			}
			for _, variable := range container.Env {
				if variable.ValueFrom == nil {
					continue
				}
				if variable.ValueFrom.ConfigMapKeyRef != nil {
					pod.ConfigMapNames = appendUnique(pod.ConfigMapNames, variable.ValueFrom.ConfigMapKeyRef.Name)
				}
				if variable.ValueFrom.SecretKeyRef != nil {
					pod.SecretNames = appendUnique(pod.SecretNames, variable.ValueFrom.SecretKeyRef.Name)
				}
			}
		}
		for _, volume := range item.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				pod.ClaimNames = appendUnique(pod.ClaimNames, volume.PersistentVolumeClaim.ClaimName)
			}
			if volume.ConfigMap != nil {
				pod.ConfigMapNames = appendUnique(pod.ConfigMapNames, volume.ConfigMap.Name)
			}
			if volume.Secret != nil {
				pod.SecretNames = appendUnique(pod.SecretNames, volume.Secret.SecretName)
			}
		}
		for _, name := range pod.ConfigMapNames {
			key := item.Metadata.Namespace + "/" + name
			configRefs[key] = ObjectRef{UID: "ref/configmap/" + key, Namespace: item.Metadata.Namespace, Name: name}
		}
		for _, name := range pod.SecretNames {
			key := item.Metadata.Namespace + "/" + name
			secretRefs[key] = ObjectRef{UID: "ref/secret/" + key, Namespace: item.Metadata.Namespace, Name: name}
		}
		result.Pods = append(result.Pods, pod)
	}
	for _, item := range services {
		result.Services = append(result.Services, Service{UID: item.Metadata.UID, Namespace: item.Metadata.Namespace, Name: item.Metadata.Name, Selector: item.Spec.Selector})
	}
	for _, item := range claims {
		result.Claims = append(result.Claims, PersistentVolumeClaim{UID: item.Metadata.UID, Namespace: item.Metadata.Namespace, Name: item.Metadata.Name, Phase: item.Status.Phase})
	}
	for _, item := range ingresses {
		entry := Ingress{UID: item.Metadata.UID, Namespace: item.Metadata.Namespace, Name: item.Metadata.Name}
		if item.Spec.DefaultBackend != nil && item.Spec.DefaultBackend.Service != nil {
			entry.ServiceNames = appendUnique(entry.ServiceNames, item.Spec.DefaultBackend.Service.Name)
		}
		for _, rule := range item.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil {
					entry.ServiceNames = appendUnique(entry.ServiceNames, path.Backend.Service.Name)
				}
			}
		}
		result.Ingresses = append(result.Ingresses, entry)
	}
	for _, item := range endpointSlices {
		entry := EndpointSlice{
			UID: item.Metadata.UID, Namespace: item.Metadata.Namespace, Name: item.Metadata.Name,
			ServiceName: item.Metadata.Labels["kubernetes.io/service-name"],
		}
		for _, endpoint := range item.Endpoints {
			if endpoint.TargetRef != nil && endpoint.TargetRef.Kind == "Pod" {
				entry.PodUIDs = appendUnique(entry.PodUIDs, endpoint.TargetRef.UID)
			}
		}
		result.EndpointSlices = append(result.EndpointSlices, entry)
	}
	result.ConfigMaps = sortedObjectRefs(configRefs)
	result.Secrets = sortedObjectRefs(secretRefs)
	return result
}

func readyCondition(conditions []condition) (bool, bool) {
	for _, item := range conditions {
		if item.Type == "Ready" {
			return item.Status == "True", true
		}
	}
	return false, false
}

func controllerOwner(owners []ownerReference) (string, string) {
	for _, owner := range owners {
		if owner.Controller != nil && *owner.Controller {
			return owner.Kind, owner.UID
		}
	}
	return "", ""
}

func appendUnique(items []string, value string) []string {
	if value == "" || slices.Contains(items, value) {
		return items
	}
	return append(items, value)
}

func sortedObjectRefs(values map[string]ObjectRef) []ObjectRef {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]ObjectRef, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

func contextCause(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

var _ EventCollector = (*APICollector)(nil)
var _ LogCollector = (*APICollector)(nil)
var _ OperationCollector = (*APICollector)(nil)
var _ MetricCollector = (*APICollector)(nil)

func parseCPUQuantity(value string) (float64, error) {
	multiplier := 1.0
	number := value
	for suffix, factor := range map[string]float64{"n": 1e-9, "u": 1e-6, "m": 1e-3} {
		if strings.HasSuffix(value, suffix) {
			number = strings.TrimSuffix(value, suffix)
			multiplier = factor
			break
		}
	}
	parsed, err := strconv.ParseFloat(number, 64)
	parsed *= multiplier
	if err != nil || parsed < 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed > 1<<53-1 {
		return 0, ErrUnavailable
	}
	return parsed, nil
}

func parseMemoryQuantity(value string) (uint64, error) {
	multiplier := float64(1)
	number := value
	for _, item := range []struct {
		suffix string
		factor float64
	}{
		{"Ki", 1 << 10}, {"Mi", 1 << 20}, {"Gi", 1 << 30}, {"Ti", 1 << 40},
		{"K", 1e3}, {"M", 1e6}, {"G", 1e9}, {"T", 1e12},
	} {
		if strings.HasSuffix(value, item.suffix) {
			number = strings.TrimSuffix(value, item.suffix)
			multiplier = item.factor
			break
		}
	}
	parsed, err := strconv.ParseFloat(number, 64)
	parsed *= multiplier
	if err != nil || parsed < 0 || parsed > 1<<53-1 || math.IsNaN(parsed) || math.IsInf(parsed, 0) || math.Trunc(parsed) != parsed {
		return 0, ErrUnavailable
	}
	return uint64(parsed), nil
}

func strconvItoa(value int) string { return fmt.Sprintf("%d", value) }

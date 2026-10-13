// Package kubernetes normalizes safe cluster observations into domain models.
package kubernetes

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/logsafe"
)

var (
	ErrPermissionDenied = errors.New("Kubernetes access denied")
	ErrUnavailable      = errors.New("Kubernetes API unavailable")
	ErrVersionConflict  = errors.New("Kubernetes resource version changed")
	ErrUnsupported      = errors.New("Kubernetes operation unsupported")
)

type Collector interface {
	Snapshot(context.Context) (Snapshot, error)
}

type EventCollector interface {
	ClusterEvents(context.Context) ([]ClusterEvent, error)
}

type MetricCollector interface {
	CurrentPodMetrics(context.Context) ([]PodMetric, error)
}

type LogCollector interface {
	ReadPodLogs(context.Context, PodLogRequest) ([]PodLogLine, error)
}

type WorkloadOperation struct {
	Kind              string
	Namespace         string
	Name              string
	ResourceVersion   string
	IdempotencyKey    string
	RequestedReplicas *int
}

type OperationCollector interface {
	RestartWorkload(context.Context, WorkloadOperation) (string, error)
	ScaleWorkload(context.Context, WorkloadOperation) (string, error)
}

type ClusterEvent struct {
	UID         string
	InvolvedUID string
	Reason      string
	Message     string
	Type        string
	OccurredAt  time.Time
	Occurrences uint64
}

type PodMetric struct {
	Namespace  string
	PodName    string
	SampledAt  time.Time
	Window     time.Duration
	Containers []ContainerMetric
}

type ContainerMetric struct {
	Name        string
	CPUCores    float64
	MemoryBytes uint64
}

type PodLogRequest struct {
	Targets   []PodLogTarget
	Start     time.Time
	LineLimit int
	ByteLimit int
}

type PodLogTarget struct {
	Key       string
	Namespace string
	PodName   string
	Container string
	Previous  bool
}

type PodLogLine struct {
	TargetKey string
	Timestamp time.Time
	Message   string
	Truncated bool
}

type Snapshot struct {
	ObservedAt     time.Time
	ClusterID      string
	ClusterName    string
	ContextName    string
	Namespaces     []Namespace
	Nodes          []Node
	Workloads      []Workload
	Pods           []Pod
	Services       []Service
	Ingresses      []Ingress
	EndpointSlices []EndpointSlice
	Claims         []PersistentVolumeClaim
	ConfigMaps     []ObjectRef
	Secrets        []ObjectRef
}

type Namespace struct {
	Name  string
	Phase string
}

type Node struct {
	UID        string
	Name       string
	Ready      bool
	ReadyKnown bool
}

type Workload struct {
	UID        string
	Version    string
	Kind       string
	Namespace  string
	Name       string
	Desired    int
	Ready      int
	ReadyKnown bool
	OwnerKind  string
	OwnerUID   string
}

type Pod struct {
	UID               string
	Namespace         string
	Name              string
	Phase             string
	Ready             bool
	ReadyKnown        bool
	OwnerKind         string
	OwnerUID          string
	Labels            map[string]string
	Containers        []string
	ContainerRestarts map[string]uint64
	ClaimNames        []string
	ConfigMapNames    []string
	SecretNames       []string
}

type Service struct {
	UID       string
	Namespace string
	Name      string
	Selector  map[string]string
}

type Ingress struct {
	UID          string
	Namespace    string
	Name         string
	ServiceNames []string
}

type EndpointSlice struct {
	UID         string
	Namespace   string
	Name        string
	ServiceName string
	PodUIDs     []string
}

type PersistentVolumeClaim struct {
	UID       string
	Namespace string
	Name      string
	Phase     string
}

type ObjectRef struct {
	UID       string
	Namespace string
	Name      string
}

type Provider struct {
	source      domain.InfrastructureSource
	environment domain.EnvironmentID
	collector   Collector
}

type normalized struct {
	status           domain.ObservationStatus
	resources        []domain.ResourceSnapshot
	health           map[domain.ResourceID]domain.HealthSnapshot
	workloadReplicas map[domain.ResourceID]domain.WorkloadReplicaSnapshot
	relationships    []domain.Relationship
}

func New(sourceID domain.SourceID, name string, collector Collector) (*Provider, error) {
	source := domain.InfrastructureSource{ID: sourceID, Provider: domain.ProviderKubernetes, Name: name}
	if collector == nil || source.Validate() != nil {
		return nil, fmt.Errorf("Kubernetes provider options are invalid")
	}
	digest := sha256.Sum256([]byte(sourceID))
	return &Provider{
		source: source, environment: domain.EnvironmentID("env_kubernetes_" + hex.EncodeToString(digest[:8])), collector: collector,
	}, nil
}

func (provider *Provider) Source() domain.InfrastructureSource { return provider.source }

func (provider *Provider) CaptureInventory(ctx context.Context) (domain.InventoryCapture, error) {
	if err := provider.validate(ctx, domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: provider.source.ID}); err != nil {
		return domain.InventoryCapture{}, providerError("kubernetes.inventory.capture", err)
	}
	state, snapshot, err := provider.observe(ctx)
	if err != nil {
		return domain.InventoryCapture{}, err
	}
	name := snapshot.ClusterName
	if snapshot.ContextName != "" {
		name += " (" + snapshot.ContextName + ")"
	}
	capture := domain.InventoryCapture{
		Source: domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: provider.source.ID}, Status: state.status,
		Environments: []domain.Environment{{ID: provider.environment, SourceID: provider.source.ID, Provider: domain.ProviderKubernetes, Name: name, Status: state.status}},
		Resources:    state.resources, Relationships: state.relationships, Complete: true,
		Health: make([]domain.HealthSnapshot, 0, len(state.health)),
	}
	for _, resource := range state.resources {
		capture.Health = append(capture.Health, state.health[resource.Resource.Ref.ID])
		if replicas, known := state.workloadReplicas[resource.Resource.Ref.ID]; known {
			capture.WorkloadReplicas = append(capture.WorkloadReplicas, replicas)
		}
	}
	if err := capture.ValidateFor(capture.Source); err != nil {
		return domain.InventoryCapture{}, providerError("kubernetes.inventory.capture", ErrUnavailable)
	}
	return capture, nil
}

func (provider *Provider) ListEnvironments(ctx context.Context, query domain.EnvironmentQuery) (domain.EnvironmentPage, error) {
	if err := provider.validate(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.EnvironmentPage{}, providerError("kubernetes.inventory.environments", firstError(err, query.Validate()))
	}
	state, snapshot, err := provider.observe(ctx)
	if err != nil {
		return domain.EnvironmentPage{}, err
	}
	fingerprint := pageFingerprint("environment", string(provider.source.ID))
	offset, err := decodeCursor(query.Page.Cursor, fingerprint)
	if err != nil || offset > 1 {
		return domain.EnvironmentPage{}, providerError("kubernetes.inventory.environments", firstError(err, errors.New("cursor is invalid")))
	}
	name := snapshot.ClusterName
	if snapshot.ContextName != "" {
		name += " (" + snapshot.ContextName + ")"
	}
	items, page := paginate([]domain.Environment{{
		ID: provider.environment, SourceID: provider.source.ID, Provider: domain.ProviderKubernetes, Name: name, Status: state.status,
	}}, offset, query.Page.Limit, fingerprint)
	result := domain.EnvironmentPage{Status: state.status, Environments: items, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.EnvironmentPage{}, providerError("kubernetes.inventory.environments", err)
	}
	return result, nil
}

func (provider *Provider) ListResources(ctx context.Context, query domain.InventoryQuery) (domain.ResourcePage, error) {
	if err := provider.validate(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.ResourcePage{}, providerError("kubernetes.inventory.resources", firstError(err, query.Validate()))
	}
	state, _, err := provider.observe(ctx)
	if err != nil {
		return domain.ResourcePage{}, err
	}
	items := make([]domain.ResourceSnapshot, 0, len(state.resources))
	for _, item := range state.resources {
		if query.Filter.Matches(item.Resource) && containsKind(query.Filter.Kinds, item.Resource.Ref.Identity.Kind) && containsScope(query.Filter.Scopes, item.Resource.Ref.Identity.Scope) {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(left, right int) bool { return compareResources(items[left], items[right], query) < 0 })
	fingerprint := inventoryFingerprint(query)
	offset, err := decodeCursor(query.Page.Cursor, fingerprint)
	if err != nil || offset > len(items) {
		return domain.ResourcePage{}, providerError("kubernetes.inventory.resources", firstError(err, errors.New("cursor is invalid")))
	}
	items, page := paginate(items, offset, query.Page.Limit, fingerprint)
	result := domain.ResourcePage{Status: state.status, Resources: items, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.ResourcePage{}, providerError("kubernetes.inventory.resources", err)
	}
	return result, nil
}

func (provider *Provider) GetResource(ctx context.Context, query domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	if err := provider.validate(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.ResourceSnapshot{}, providerError("kubernetes.inventory.detail", firstError(err, query.Validate()))
	}
	state, _, err := provider.observe(ctx)
	if err != nil {
		return domain.ResourceSnapshot{}, err
	}
	for _, item := range state.resources {
		if item.Resource.Ref.ID == query.ResourceID {
			return item, nil
		}
	}
	return domain.ResourceSnapshot{}, providerError("kubernetes.inventory.detail", errors.New("resource not found"))
}

func (provider *Provider) GetWorkloadReplicas(ctx context.Context, query domain.ResourceDetailQuery) (domain.WorkloadReplicaSnapshot, error) {
	if err := provider.validate(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.WorkloadReplicaSnapshot{}, providerError("kubernetes.workload.replicas", firstError(err, query.Validate()))
	}
	state, _, err := provider.observe(ctx)
	if err != nil {
		return domain.WorkloadReplicaSnapshot{}, err
	}
	result, exists := state.workloadReplicas[query.ResourceID]
	if !exists {
		return domain.WorkloadReplicaSnapshot{}, providerError("kubernetes.workload.replicas", ErrUnavailable)
	}
	if err := result.ValidateFor(query); err != nil {
		return domain.WorkloadReplicaSnapshot{}, providerError("kubernetes.workload.replicas", err)
	}
	return result, nil
}

func (provider *Provider) GetHealth(ctx context.Context, query domain.HealthQuery) (domain.HealthResult, error) {
	if err := provider.validate(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.HealthResult{}, providerError("kubernetes.health.read", firstError(err, query.Validate()))
	}
	state, _, err := provider.observe(ctx)
	if err != nil {
		return domain.HealthResult{}, err
	}
	items := make([]domain.HealthSnapshot, 0, len(query.Resources))
	for _, resource := range query.Resources {
		item, exists := state.health[resource.ID]
		if !exists {
			return domain.HealthResult{}, providerError("kubernetes.health.read", errors.New("resource not found"))
		}
		items = append(items, item)
	}
	result := domain.HealthResult{Status: state.status, Snapshots: items}
	if err := result.ValidateFor(query); err != nil {
		return domain.HealthResult{}, providerError("kubernetes.health.read", err)
	}
	return result, nil
}

func (provider *Provider) GetRelationships(ctx context.Context, query domain.TopologyQuery) (domain.RelationshipPage, error) {
	if err := provider.validate(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.RelationshipPage{}, providerError("kubernetes.topology.read", firstError(err, query.Validate()))
	}
	state, _, err := provider.observe(ctx)
	if err != nil {
		return domain.RelationshipPage{}, err
	}
	roots := make(map[domain.ResourceID]struct{}, len(query.Resources))
	for _, resource := range query.Resources {
		roots[resource.ID] = struct{}{}
	}
	items := make([]domain.Relationship, 0, len(state.relationships))
	for _, item := range state.relationships {
		_, from := roots[item.From]
		_, to := roots[item.To]
		if len(roots) == 0 || from || to {
			items = append(items, item)
		}
	}
	fingerprint := topologyFingerprint(query)
	offset, err := decodeCursor(query.Page.Cursor, fingerprint)
	if err != nil || offset > len(items) {
		return domain.RelationshipPage{}, providerError("kubernetes.topology.read", firstError(err, errors.New("cursor is invalid")))
	}
	items, page := paginate(items, offset, query.Page.Limit, fingerprint)
	result := domain.RelationshipPage{Status: state.status, Relationships: items, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.RelationshipPage{}, providerError("kubernetes.topology.read", err)
	}
	return result, nil
}

func (provider *Provider) GetCapabilities(ctx context.Context, source domain.ProviderRef) (domain.CapabilitySet, error) {
	if err := provider.validate(ctx, source); err != nil {
		return domain.CapabilitySet{}, providerError("kubernetes.capability.read", err)
	}
	observedAt := time.Now().UTC().Truncate(time.Microsecond)
	available := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: observedAt}
	unsupported := domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: observedAt, ReasonCode: "kubernetes.capability.not_implemented"}
	names := []domain.CapabilityName{
		domain.CapabilityInventoryList, domain.CapabilityInventoryDetail, domain.CapabilityHealth, domain.CapabilityTopology,
		domain.CapabilityLogRead, domain.CapabilityLogTail, domain.CapabilityMetricCurrent, domain.CapabilityMetricHistory,
		domain.CapabilityEventHistory, domain.CapabilityEventWatch,
		domain.CapabilityDockerRestart, domain.CapabilityWorkloadRestart, domain.CapabilityWorkloadScale,
	}
	items := make([]domain.CapabilityStatus, len(names))
	_, eventsAvailable := provider.collector.(EventCollector)
	_, metricsAvailable := provider.collector.(MetricCollector)
	_, logsAvailable := provider.collector.(LogCollector)
	_, operationsAvailable := provider.collector.(OperationCollector)
	for index, name := range names {
		status := unsupported
		if index < 4 || name == domain.CapabilityEventHistory && eventsAvailable || name == domain.CapabilityMetricCurrent && metricsAvailable || name == domain.CapabilityLogRead && logsAvailable ||
			(name == domain.CapabilityWorkloadRestart || name == domain.CapabilityWorkloadScale) && operationsAvailable {
			status = available
		}
		items[index] = domain.CapabilityStatus{Name: name, Status: status}
	}
	result := domain.CapabilitySet{Source: source, Capabilities: items}
	return result, result.ValidateFor(source)
}

func (provider *Provider) PreviewOperation(ctx context.Context, request domain.OperationRequest) (domain.OperationImpact, error) {
	if err := provider.validate(ctx, request.Source); err != nil || request.Validate() != nil {
		return domain.OperationImpact{}, providerError("kubernetes.operation.preview", firstError(err, request.Validate()))
	}
	if _, ok := provider.collector.(OperationCollector); !ok {
		return domain.OperationImpact{}, providerError("kubernetes.operation.preview", ErrUnsupported)
	}
	_, snapshot, err := provider.observe(ctx)
	if err != nil {
		return domain.OperationImpact{}, err
	}
	workload, ok := operationWorkload(snapshot, request.Resource)
	if !ok || !workloadSupports(request.Action, workload.Kind) {
		return domain.OperationImpact{}, providerError("kubernetes.operation.preview", ErrUnsupported)
	}
	if workload.Version == "" || kubernetesOperationVersion(workload.Version) != request.ExpectedVersion {
		return domain.OperationImpact{}, providerError("kubernetes.operation.preview", ErrVersionConflict)
	}
	impact := domain.OperationImpact{
		Request: request, DisplayName: workload.Name,
		EffectCode: "operation.kubernetes_restart", RiskCode: "operation.brief_interruption",
	}
	if request.Action == domain.OperationKubernetesScale {
		current := workload.Desired
		impact.CurrentReplicas = &current
		impact.EffectCode = "operation.kubernetes_scale"
		impact.RiskCode = "operation.capacity_change"
	}
	return impact, impact.Validate()
}

func (provider *Provider) ExecuteOperation(ctx context.Context, execution domain.OperationExecution) (domain.OperationProviderResult, error) {
	request := execution.Request
	if err := provider.validate(ctx, request.Source); err != nil || execution.Validate() != nil {
		return domain.OperationProviderResult{}, providerError("kubernetes.operation.execute", firstError(err, execution.Validate()))
	}
	collector, ok := provider.collector.(OperationCollector)
	if !ok {
		return domain.OperationProviderResult{}, providerError("kubernetes.operation.execute", ErrUnsupported)
	}
	_, snapshot, err := provider.observe(ctx)
	if err != nil {
		return domain.OperationProviderResult{}, err
	}
	workload, ok := operationWorkload(snapshot, request.Resource)
	if !ok || !workloadSupports(request.Action, workload.Kind) {
		return domain.OperationProviderResult{}, providerError("kubernetes.operation.execute", ErrUnsupported)
	}
	if workload.Version == "" || kubernetesOperationVersion(workload.Version) != request.ExpectedVersion {
		return domain.OperationProviderResult{}, providerError("kubernetes.operation.execute", ErrVersionConflict)
	}
	operation := WorkloadOperation{
		Kind: workload.Kind, Namespace: workload.Namespace, Name: workload.Name,
		ResourceVersion: workload.Version, IdempotencyKey: execution.IdempotencyKey,
		RequestedReplicas: request.Parameters.Replicas,
	}
	var version string
	if request.Action == domain.OperationKubernetesScale {
		version, err = collector.ScaleWorkload(ctx, operation)
	} else {
		version, err = collector.RestartWorkload(ctx, operation)
	}
	if err != nil {
		return domain.OperationProviderResult{}, providerError("kubernetes.operation.execute", err)
	}
	result := domain.OperationProviderResult{
		Action: request.Action, ResourceID: request.Resource.ID, PreviousVersion: request.ExpectedVersion,
		CurrentVersion: kubernetesOperationVersion(version), EffectCode: "operation.kubernetes_restart_started",
	}
	if request.Action == domain.OperationKubernetesScale {
		result.EffectCode = "operation.kubernetes_scale_accepted"
	}
	return result, result.ValidateFor(execution)
}

func operationWorkload(snapshot Snapshot, resource domain.ResourceRef) (Workload, bool) {
	for _, workload := range snapshot.Workloads {
		if workload.UID == string(resource.Identity.NativeID()) {
			return workload, true
		}
	}
	return Workload{}, false
}

func workloadSupports(action domain.OperationAction, kind string) bool {
	switch action {
	case domain.OperationKubernetesRestart:
		return kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet"
	case domain.OperationKubernetesScale:
		return kind == "Deployment" || kind == "StatefulSet"
	default:
		return false
	}
}

func kubernetesOperationVersion(resourceVersion string) string {
	digest := sha256.Sum256([]byte("stackplore-kubernetes-version/v1\x00" + resourceVersion))
	return "kv_" + base64.RawURLEncoding.EncodeToString(digest[:])
}

func (provider *Provider) ReadLogs(ctx context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	if err := provider.validate(ctx, query.Source); err != nil {
		return domain.LogPage{}, providerError("kubernetes.log.read", err)
	}
	if err := query.Validate(); err != nil {
		return domain.LogPage{}, providerError("kubernetes.log.read", err)
	}
	collector, ok := provider.collector.(LogCollector)
	observedAt := query.Range.End.Add(-time.Nanosecond).UTC().Truncate(time.Microsecond)
	if !ok {
		result := domain.LogPage{
			Status: domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: observedAt, ReasonCode: "kubernetes.log.not_implemented"},
			Page:   domain.PageInfo{Limit: query.LineLimit},
		}
		return result, result.ValidateFor(query)
	}
	_, snapshot, err := provider.observe(ctx)
	if err != nil {
		return domain.LogPage{}, err
	}
	pods := make(map[string]Pod, len(snapshot.Pods))
	for _, pod := range snapshot.Pods {
		pods[pod.UID] = pod
	}
	targets := make([]PodLogTarget, 0, len(query.Targets))
	requested := make(map[string]domain.LogTarget, len(query.Targets))
	partial := false
	const maximumPodLogTargets = 64
	for _, target := range query.Targets {
		podUID, container, supported := kubernetesLogTarget(target.Resource)
		pod, exists := pods[podUID]
		if !supported || !exists {
			partial = true
			continue
		}
		containers := []string{container}
		if container == "" {
			containers = pod.Containers
		}
		for _, name := range containers {
			if len(targets) >= maximumPodLogTargets {
				partial = true
				break
			}
			key := string(target.Resource.ID) + "\x00" + name
			targets = append(targets, PodLogTarget{Key: key, Namespace: pod.Namespace, PodName: pod.Name, Container: name, Previous: target.Previous})
			requested[key] = target
		}
	}
	fingerprint := kubernetesLogFingerprint(query)
	offset, err := decodeCursor(query.Cursor, fingerprint)
	if err != nil {
		return domain.LogPage{}, providerError("kubernetes.log.read", err)
	}
	lines, err := collector.ReadPodLogs(ctx, PodLogRequest{
		Targets: targets, Start: query.Range.Start,
		LineLimit: min(domain.MaxLogLines, offset+query.LineLimit+1), ByteLimit: query.ByteLimit,
	})
	if err != nil {
		return domain.LogPage{}, providerError("kubernetes.log.read", err)
	}
	records := make([]domain.LogRecord, 0, len(lines))
	for index, line := range lines {
		target, exists := requested[line.TargetKey]
		if !exists || line.Timestamp.Before(query.Range.Start) || !line.Timestamp.Before(query.Range.End) {
			continue
		}
		message, redacted, truncated := safeKubernetesLogMessage(line.Message, line.Truncated)
		records = append(records, domain.LogRecord{
			ID:         domain.LogEntryID("log_kubernetes_" + shortDigest(line.TargetKey+"\x00"+line.Timestamp.Format(time.RFC3339Nano)+"\x00"+strconv.Itoa(index))),
			ResourceID: target.Resource.ID, Container: target.Container, Stream: domain.LogStreamUnknown,
			Severity: kubernetesLogSeverity(message), Timestamp: line.Timestamp, Message: message,
			Redacted: redacted, Truncated: truncated,
		})
	}
	sort.Slice(records, func(left, right int) bool {
		if !records[left].Timestamp.Equal(records[right].Timestamp) {
			return records[left].Timestamp.Before(records[right].Timestamp)
		}
		return records[left].ID < records[right].ID
	})
	for index := range records {
		records[index].Sequence = uint64(index + 1)
	}
	if offset > len(records) {
		return domain.LogPage{}, providerError("kubernetes.log.read", errors.New("cursor is invalid"))
	}
	records, page := paginate(records, offset, query.LineLimit, fingerprint)
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: observedAt}
	if partial {
		status.Completeness = domain.CompletenessPartial
		status.ReasonCode = "kubernetes.log.resource_unsupported"
	}
	result := domain.LogPage{Status: status, Records: records, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.LogPage{}, providerError("kubernetes.log.read", err)
	}
	return result, nil
}

func kubernetesLogTarget(resource domain.ResourceRef) (string, string, bool) {
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

func kubernetesLogFingerprint(query domain.LogReadQuery) string {
	targets := make([]string, len(query.Targets))
	for index, target := range query.Targets {
		targets[index] = string(target.Resource.ID) + "\x00" + target.Container
	}
	return pageFingerprint("logs", strings.Join(targets, ","), query.Range.Start.Format(time.RFC3339Nano), query.Range.End.Format(time.RFC3339Nano))
}

func safeKubernetesLogMessage(value string, truncated bool) (string, bool, bool) {
	return logsafe.Message(value, truncated)
}

func kubernetesLogSeverity(message string) domain.LogSeverity {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "fatal"), strings.Contains(lower, "critical"):
		return domain.LogSeverityCritical
	case strings.Contains(lower, "error"), strings.Contains(lower, "failed"):
		return domain.LogSeverityError
	case strings.Contains(lower, "warn"):
		return domain.LogSeverityWarning
	case strings.Contains(lower, "debug"):
		return domain.LogSeverityDebug
	default:
		return domain.LogSeverityInfo
	}
}

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
		result := domain.CurrentMetricResult{Status: domain.ObservationStatus{
			State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete,
			ObservedAt: observedAt, ReasonCode: "kubernetes.metric.not_implemented",
		}}
		return result, result.ValidateFor(query)
	}
	state, snapshot, err := provider.observe(ctx)
	if err != nil {
		return domain.CurrentMetricResult{}, err
	}
	observedAt = state.status.ObservedAt
	metrics, err := collector.CurrentPodMetrics(ctx)
	if err != nil {
		return domain.CurrentMetricResult{}, providerError("kubernetes.metric.current", err)
	}
	byPodName := make(map[string]PodMetric, len(metrics))
	for _, metric := range metrics {
		key := metric.Namespace + "\x00" + metric.PodName
		if metric.Namespace == "" || metric.PodName == "" || metric.SampledAt.Location() != time.UTC || metric.SampledAt.Nanosecond()%int(time.Microsecond) != 0 ||
			metric.Window < time.Second || metric.Window > 24*time.Hour || metric.Window%time.Second != 0 {
			return domain.CurrentMetricResult{}, providerError("kubernetes.metric.current", errors.New("invalid collector metric"))
		}
		if _, exists := byPodName[key]; exists {
			return domain.CurrentMetricResult{}, providerError("kubernetes.metric.current", errors.New("duplicate collector metric"))
		}
		byPodName[key] = metric
		if metric.SampledAt.After(observedAt) {
			observedAt = metric.SampledAt
		}
	}
	podsByUID := make(map[string]Pod, len(snapshot.Pods))
	metricsByUID := make(map[string]PodMetric, len(snapshot.Pods))
	for _, pod := range snapshot.Pods {
		podsByUID[pod.UID] = pod
		if metric, exists := byPodName[pod.Namespace+"\x00"+pod.Name]; exists {
			metricsByUID[pod.UID] = metric
		}
	}
	available := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: observedAt}
	sets := make([]domain.CurrentMetricSet, 0, len(query.Resources))
	for _, resource := range query.Resources {
		set := domain.CurrentMetricSet{ResourceID: resource.ID, Status: available}
		podUID, containerName, supported := kubernetesMetricTarget(resource)
		pod, podExists := podsByUID[podUID]
		metric, metricExists := metricsByUID[podUID]
		if !supported {
			set.Status = domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: observedAt, ReasonCode: "kubernetes.metric.resource_unsupported"}
			sets = append(sets, set)
			continue
		}
		if !podExists || !metricExists {
			set.Status = domain.ObservationStatus{State: domain.AvailabilityUnavailable, Completeness: domain.CompletenessComplete, ObservedAt: observedAt, ReasonCode: "kubernetes.metric.pod_missing"}
			sets = append(sets, set)
			continue
		}
		set.Status.ObservedAt = metric.SampledAt
		for _, name := range query.Names {
			value, unit, exists := kubernetesMetricValue(pod, metric, containerName, name)
			if !exists {
				set.Status.Completeness = domain.CompletenessPartial
				set.Status.ReasonCode = "kubernetes.metric.name_unsupported"
				continue
			}
			set.Samples = append(set.Samples, domain.MetricSample{
				ID:         domain.MetricSampleID("metric_kubernetes_" + shortDigest(string(resource.ID)+"\x00"+string(name)+"\x00"+metric.SampledAt.Format(time.RFC3339Nano))),
				ResourceID: resource.ID, Name: name, Unit: unit, Value: value,
				SampledAt: metric.SampledAt, Interval: metric.Window,
			})
		}
		sets = append(sets, set)
	}
	result := domain.CurrentMetricResult{Status: available, Sets: sets}
	if err := result.ValidateFor(query); err != nil {
		return domain.CurrentMetricResult{}, providerError("kubernetes.metric.current", err)
	}
	return result, nil
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

func (provider *Provider) ListEvents(ctx context.Context, query domain.EventQuery) (domain.EventPage, error) {
	if err := provider.validate(ctx, query.Source); err != nil {
		return domain.EventPage{}, providerError("kubernetes.event.history", err)
	}
	if err := query.Validate(); err != nil {
		return domain.EventPage{}, providerError("kubernetes.event.history", err)
	}
	collector, ok := provider.collector.(EventCollector)
	observedAt := time.Now().UTC().Truncate(time.Microsecond)
	if !ok {
		result := domain.EventPage{
			Status: domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: observedAt, ReasonCode: "kubernetes.event.not_implemented"},
			Page:   domain.PageInfo{Limit: query.Page.Limit},
		}
		return result, result.ValidateFor(query)
	}
	state, _, err := provider.observe(ctx)
	if err != nil {
		return domain.EventPage{}, err
	}
	observedAt = state.status.ObservedAt
	rawEvents, err := collector.ClusterEvents(ctx)
	if err != nil {
		return domain.EventPage{}, providerError("kubernetes.event.history", err)
	}
	byNative := make(map[string]domain.ResourceID, len(state.resources))
	for _, resource := range state.resources {
		byNative[string(resource.Resource.Ref.Identity.NativeID())] = resource.Resource.Ref.ID
	}
	requested := make(map[domain.ResourceID]struct{}, len(query.Resources))
	for _, resource := range query.Resources {
		requested[resource.ID] = struct{}{}
	}
	severities := make(map[domain.Severity]struct{}, len(query.Severities))
	for _, severity := range query.Severities {
		severities[severity] = struct{}{}
	}
	items := make([]domain.InfrastructureEvent, 0, len(rawEvents))
	for _, raw := range rawEvents {
		if raw.UID == "" || raw.OccurredAt.Location() != time.UTC || raw.OccurredAt.Nanosecond()%int(time.Microsecond) != 0 ||
			raw.OccurredAt.Before(query.Range.Start) || !raw.OccurredAt.Before(query.Range.End) || raw.OccurredAt.After(observedAt) {
			continue
		}
		resourceID, scoped := byNative[raw.InvolvedUID]
		if len(requested) > 0 {
			if !scoped {
				continue
			}
			if _, exists := requested[resourceID]; !exists {
				continue
			}
		}
		severity := kubernetesEventSeverity(raw.Type)
		if len(severities) > 0 {
			if _, exists := severities[severity]; !exists {
				continue
			}
		}
		summary := safeEventSummary(raw.Message)
		if summary == "" {
			summary = safeEventSummary(raw.Reason)
		}
		if summary == "" {
			summary = "Kubernetes event"
		}
		event := domain.InfrastructureEvent{
			ID: domain.EventID("event_kubernetes_" + shortDigest(string(provider.source.ID)+"\x00"+raw.UID+"\x00"+raw.OccurredAt.Format(time.RFC3339Nano)+"\x00"+strconv.FormatUint(raw.Occurrences, 10)+"\x00"+string(severity)+"\x00"+string(kubernetesEventReason(raw.Reason))+"\x00"+summary+"\x00"+string(resourceID))), Provider: domain.ProviderKubernetes,
			SourceID: provider.source.ID, EnvironmentID: &provider.environment,
			Kind: domain.EventKind("kubernetes.event." + string(severity)), Severity: severity,
			ReasonCode: kubernetesEventReason(raw.Reason), Summary: summary,
			OccurredAt: raw.OccurredAt, ObservedAt: observedAt,
		}
		if scoped {
			event.ResourceID = &resourceID
		}
		items = append(items, event)
	}
	sort.Slice(items, func(left, right int) bool {
		if !items[left].OccurredAt.Equal(items[right].OccurredAt) {
			return items[left].OccurredAt.After(items[right].OccurredAt)
		}
		return items[left].ID < items[right].ID
	})
	fingerprint := eventFingerprint(query)
	offset, err := decodeCursor(query.Page.Cursor, fingerprint)
	if err != nil || offset > len(items) {
		return domain.EventPage{}, providerError("kubernetes.event.history", firstError(err, errors.New("cursor is invalid")))
	}
	items, page := paginate(items, offset, query.Page.Limit, fingerprint)
	result := domain.EventPage{Status: state.status, Events: items, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.EventPage{}, providerError("kubernetes.event.history", err)
	}
	return result, nil
}

func kubernetesEventSeverity(value string) domain.Severity {
	if strings.EqualFold(value, "warning") {
		return domain.SeverityWarning
	}
	return domain.SeverityInfo
}

func kubernetesEventReason(value string) domain.ReasonCode {
	var result strings.Builder
	for _, character := range strings.ToLower(value) {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' || character == '_' {
			result.WriteRune(character)
		} else if result.Len() > 0 {
			result.WriteByte('_')
		}
		if result.Len() >= 80 {
			break
		}
	}
	suffix := strings.Trim(result.String(), "_-")
	if suffix == "" || suffix[0] < 'a' || suffix[0] > 'z' {
		suffix = "unknown"
	}
	return domain.ReasonCode("kubernetes.event." + suffix)
}

func safeEventSummary(value string) string {
	message, _, _ := logsafe.Message(strings.TrimSpace(value), false)
	if len(message) > domain.MaxEventSummaryLength {
		end := 0
		for index := range message {
			if index > domain.MaxEventSummaryLength {
				break
			}
			end = index
		}
		message = message[:end]
	}
	return strings.TrimSpace(message)
}
func eventFingerprint(query domain.EventQuery) string {
	resources := make([]string, len(query.Resources))
	for index, resource := range query.Resources {
		resources[index] = string(resource.ID)
	}
	severities := make([]string, len(query.Severities))
	for index, severity := range query.Severities {
		severities[index] = string(severity)
	}
	return pageFingerprint("events", strings.Join(resources, ","), strings.Join(severities, ","), query.Range.Start.Format(time.RFC3339Nano), query.Range.End.Format(time.RFC3339Nano))
}

func (provider *Provider) observe(ctx context.Context) (normalized, Snapshot, error) {
	snapshot, err := provider.collector.Snapshot(ctx)
	if err != nil {
		return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.read", err)
	}
	if snapshot.ObservedAt.Location() != time.UTC || snapshot.ObservedAt.Nanosecond()%int(time.Microsecond) != 0 || snapshot.ClusterID == "" || snapshot.ClusterName == "" {
		return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.read", errors.New("invalid collector snapshot"))
	}
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: snapshot.ObservedAt}
	state := normalized{
		status: status, health: make(map[domain.ResourceID]domain.HealthSnapshot),
		workloadReplicas: make(map[domain.ResourceID]domain.WorkloadReplicaSnapshot),
	}
	refs := make(map[string]domain.ResourceRef)
	names := make(map[string]string)
	add := func(kind domain.ResourceKind, scope domain.Scope, nativeID, name string, health domain.HealthState, reason domain.ReasonCode) error {
		if len(state.resources) >= domain.MaxInventoryCaptureItems {
			return ErrUnavailable
		}
		identity, err := domain.NewResourceIdentity(domain.ProviderKubernetes, provider.source.ID, provider.environment, kind, scope, domain.NativeResourceID(nativeID))
		if err != nil {
			return err
		}
		ref, err := domain.NewResourceRef(identity)
		if err != nil {
			return err
		}
		key := resourceKey(string(kind), nativeID)
		if _, exists := refs[key]; exists {
			return fmt.Errorf("duplicate resource %s", key)
		}
		refs[key] = ref
		// Container names are unique within a pod, not within a namespace.
		// Container edges and reads use the immutable pod-derived resource key.
		if kind != "kubernetes.container" {
			lookupKey := nameKey(string(kind), scope, name)
			if existing, exists := names[lookupKey]; exists && existing != key {
				return fmt.Errorf("duplicate resource name %s", lookupKey)
			}
			names[lookupKey] = key
		}
		state.resources = append(state.resources, domain.ResourceSnapshot{Resource: domain.Resource{Ref: ref, DisplayName: name}, Status: status})
		healthSnapshot := domain.HealthSnapshot{ResourceID: ref.ID, State: health, Status: status}
		if health != domain.HealthUnknown {
			evidenceState := domain.EvidenceNegative
			if health == domain.HealthHealthy {
				evidenceState = domain.EvidencePositive
			}
			healthSnapshot.Evidence = []domain.HealthEvidence{{
				ID: domain.EvidenceID("evidence_kubernetes_" + shortDigest(string(ref.ID))), Code: reason, SourceCode: reason,
				Status: evidenceState, ObservedAt: snapshot.ObservedAt,
			}}
		}
		state.health[ref.ID] = healthSnapshot
		return nil
	}
	global := domain.Scope{Kind: domain.ScopeGlobal}
	if err := add("kubernetes.cluster", global, snapshot.ClusterID, snapshot.ClusterName, domain.HealthHealthy, "kubernetes.cluster.reachable"); err != nil {
		return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
	}
	for _, item := range snapshot.Namespaces {
		health, reason := namespaceHealth(item)
		if err := add("kubernetes.namespace", namespaceScope(item.Name), item.Name, item.Name, health, reason); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.Nodes {
		health, reason := conditionHealth(item.ReadyKnown, item.Ready, "node")
		if err := add("kubernetes.node", global, item.UID, item.Name, health, reason); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.Workloads {
		kind, err := workloadResourceKind(item.Kind)
		if err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
		health, reason := workloadHealth(item)
		if err := add(kind, namespaceScope(item.Namespace), item.UID, item.Name, health, reason); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
		if item.Version != "" {
			state.resources[len(state.resources)-1].Resource.Version = kubernetesOperationVersion(item.Version)
		}
		if item.ReadyKnown && item.Desired >= 0 && item.Ready >= 0 && item.Ready <= item.Desired {
			ref := refs[resourceKey(string(kind), item.UID)]
			state.workloadReplicas[ref.ID] = domain.WorkloadReplicaSnapshot{
				Resource: ref, Desired: uint64(item.Desired), Ready: uint64(item.Ready), Status: status,
			}
		}
	}
	for _, item := range snapshot.Pods {
		health, reason := podHealth(item)
		if err := add("kubernetes.pod", namespaceScope(item.Namespace), item.UID, item.Name, health, reason); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
		for _, name := range item.Containers {
			if err := add("kubernetes.container", namespaceScope(item.Namespace), item.UID+"/container/"+name, name, health, reason); err != nil {
				return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
			}
		}
	}
	for _, item := range snapshot.Services {
		if err := add("kubernetes.service", namespaceScope(item.Namespace), item.UID, item.Name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.Ingresses {
		if err := add("kubernetes.ingress", namespaceScope(item.Namespace), item.UID, item.Name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.EndpointSlices {
		if err := add("kubernetes.endpoint_slice", namespaceScope(item.Namespace), item.UID, item.Name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.Claims {
		health, reason := claimHealth(item)
		if err := add("kubernetes.persistent_volume_claim", namespaceScope(item.Namespace), item.UID, item.Name, health, reason); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.ConfigMaps {
		if err := add("kubernetes.config_map", namespaceScope(item.Namespace), item.UID, item.Name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	for _, item := range snapshot.Secrets {
		if err := add("kubernetes.secret", namespaceScope(item.Namespace), item.UID, item.Name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
		}
	}
	if err := provider.buildRelationships(&state, refs, names, snapshot); err != nil {
		return normalized{}, Snapshot{}, providerError("kubernetes.snapshot.normalize", err)
	}
	sort.Slice(state.resources, func(left, right int) bool {
		return state.resources[left].Resource.Ref.ID < state.resources[right].Resource.Ref.ID
	})
	sort.Slice(state.relationships, func(left, right int) bool { return state.relationships[left].ID < state.relationships[right].ID })
	return state, snapshot, nil
}

func (provider *Provider) buildRelationships(state *normalized, refs map[string]domain.ResourceRef, names map[string]string, snapshot Snapshot) error {
	counter := 0
	link := func(fromKey, toKey string, kind domain.RelationshipKind, provenance domain.RelationshipProvenance, certainty domain.EvidenceCertainty, confidence float64) error {
		from, fromOK := refs[fromKey]
		to, toOK := refs[toKey]
		if !fromOK || !toOK {
			return nil
		}
		if len(state.relationships) >= domain.MaxInventoryCaptureItems {
			return ErrUnavailable
		}
		counter++
		evidence := domain.RelationshipEvidence{
			ID: domain.EvidenceID(fmt.Sprintf("evidence_kubernetes_relation_%d", counter)), Provider: domain.ProviderKubernetes,
			SourceID: provider.source.ID, Provenance: provenance, Certainty: certainty, Confidence: confidence, ObservedAt: snapshot.ObservedAt,
		}
		relationship, err := domain.NewRelationship(from.ID, to.ID, kind, domain.RelationshipSourceToTarget, []domain.RelationshipEvidence{evidence})
		if err == nil {
			state.relationships = append(state.relationships, relationship)
		}
		return err
	}
	clusterKey := resourceKey("kubernetes.cluster", snapshot.ClusterID)
	for _, item := range snapshot.Namespaces {
		if err := link(clusterKey, resourceKey("kubernetes.namespace", item.Name), domain.RelationshipOwns, domain.ProvenanceOwnership, domain.EvidenceExplicit, 1); err != nil {
			return err
		}
	}
	for _, item := range snapshot.Workloads {
		kind, _ := workloadResourceKind(item.Kind)
		if err := link(resourceKey("kubernetes.namespace", item.Namespace), resourceKey(string(kind), item.UID), domain.RelationshipOwns, domain.ProvenanceOwnership, domain.EvidenceExplicit, 1); err != nil {
			return err
		}
		if item.OwnerUID != "" {
			ownerKind, err := workloadResourceKind(item.OwnerKind)
			if err != nil {
				return err
			}
			if err := link(resourceKey(string(ownerKind), item.OwnerUID), resourceKey(string(kind), item.UID), domain.RelationshipOwns, domain.ProvenanceOwnership, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
	}
	for _, item := range snapshot.Pods {
		podKey := resourceKey("kubernetes.pod", item.UID)
		if err := link(resourceKey("kubernetes.namespace", item.Namespace), podKey, domain.RelationshipOwns, domain.ProvenanceOwnership, domain.EvidenceExplicit, 1); err != nil {
			return err
		}
		if item.OwnerUID != "" {
			ownerKind, err := workloadResourceKind(item.OwnerKind)
			if err != nil {
				return err
			}
			if err := link(resourceKey(string(ownerKind), item.OwnerUID), podKey, domain.RelationshipOwns, domain.ProvenanceOwnership, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
		for _, name := range item.Containers {
			if err := link(podKey, resourceKey("kubernetes.container", item.UID+"/container/"+name), domain.RelationshipOwns, domain.ProvenanceOwnership, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
		for _, name := range item.ClaimNames {
			if err := link(podKey, findByName(names, "kubernetes.persistent_volume_claim", item.Namespace, name), domain.RelationshipMounts, domain.ProvenanceStorageReference, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
		for _, name := range item.ConfigMapNames {
			if err := link(podKey, findByName(names, "kubernetes.config_map", item.Namespace, name), domain.RelationshipReferences, domain.ProvenanceDeclaredDependency, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
		for _, name := range item.SecretNames {
			if err := link(podKey, findByName(names, "kubernetes.secret", item.Namespace, name), domain.RelationshipReferences, domain.ProvenanceDeclaredDependency, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
	}
	for _, service := range snapshot.Services {
		serviceKey := resourceKey("kubernetes.service", service.UID)
		for _, pod := range snapshot.Pods {
			if service.Namespace == pod.Namespace && selectorMatches(service.Selector, pod.Labels) {
				if err := link(serviceKey, resourceKey("kubernetes.pod", pod.UID), domain.RelationshipSelects, domain.ProvenanceSelector, domain.EvidenceExplicit, 1); err != nil {
					return err
				}
			}
		}
	}
	for _, ingress := range snapshot.Ingresses {
		for _, serviceName := range ingress.ServiceNames {
			if err := link(resourceKey("kubernetes.ingress", ingress.UID), findByName(names, "kubernetes.service", ingress.Namespace, serviceName), domain.RelationshipRoutesTo, domain.ProvenanceRoute, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
	}
	for _, endpoint := range snapshot.EndpointSlices {
		endpointKey := resourceKey("kubernetes.endpoint_slice", endpoint.UID)
		if err := link(findByName(names, "kubernetes.service", endpoint.Namespace, endpoint.ServiceName), endpointKey, domain.RelationshipRoutesTo, domain.ProvenanceEndpoint, domain.EvidenceExplicit, 1); err != nil {
			return err
		}
		for _, podUID := range endpoint.PodUIDs {
			if err := link(endpointKey, resourceKey("kubernetes.pod", podUID), domain.RelationshipRoutesTo, domain.ProvenanceEndpoint, domain.EvidenceExplicit, 1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (provider *Provider) validate(ctx context.Context, source domain.ProviderRef) error {
	if provider == nil || provider.collector == nil || ctx == nil {
		return errors.New("provider boundary is invalid")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if source != (domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: provider.source.ID}) {
		return errors.New("source is not available")
	}
	return nil
}

func providerError(operation domain.ProviderOperation, cause error) error {
	if cause == nil {
		cause = errors.New("invalid provider request")
	}
	code, retryable := domain.ProviderErrorInvalidRequest, false
	if errors.Is(cause, context.Canceled) {
		code = domain.ProviderErrorCanceled
	} else if errors.Is(cause, context.DeadlineExceeded) {
		code = domain.ProviderErrorDeadlineExceeded
	} else if errors.Is(cause, ErrPermissionDenied) {
		code = domain.ProviderErrorPermissionDenied
	} else if errors.Is(cause, ErrVersionConflict) {
		code = domain.ProviderErrorInvalidRequest
	} else if errors.Is(cause, ErrUnsupported) {
		code = domain.ProviderErrorUnsupported
	} else if errors.Is(cause, ErrUnavailable) {
		code, retryable = domain.ProviderErrorUnavailable, true
	}
	result, _ := domain.NewProviderError(operation, code, retryable, cause)
	return result
}

func firstError(first, second error) error {
	if first != nil {
		return first
	}
	return second
}

func namespaceScope(name string) domain.Scope {
	return domain.Scope{Kind: domain.ScopeNamespace, Name: name}
}

func namespaceHealth(item Namespace) (domain.HealthState, domain.ReasonCode) {
	switch strings.ToLower(item.Phase) {
	case "active":
		return domain.HealthHealthy, "kubernetes.namespace.active"
	case "terminating":
		return domain.HealthDegraded, "kubernetes.namespace.terminating"
	default:
		return domain.HealthUnknown, ""
	}
}

func conditionHealth(known, ready bool, kind string) (domain.HealthState, domain.ReasonCode) {
	if !known {
		return domain.HealthUnknown, ""
	}
	if ready {
		return domain.HealthHealthy, domain.ReasonCode("kubernetes." + kind + ".ready")
	}
	return domain.HealthUnhealthy, domain.ReasonCode("kubernetes." + kind + ".not_ready")
}

func workloadHealth(item Workload) (domain.HealthState, domain.ReasonCode) {
	if !item.ReadyKnown || item.Desired < 0 || item.Ready < 0 || item.Ready > item.Desired {
		return domain.HealthUnknown, ""
	}
	if item.Ready == item.Desired {
		return domain.HealthHealthy, "kubernetes.workload.ready"
	}
	if item.Ready > 0 {
		return domain.HealthDegraded, "kubernetes.workload.part_ready"
	}
	return domain.HealthUnhealthy, "kubernetes.workload.not_ready"
}

func podHealth(item Pod) (domain.HealthState, domain.ReasonCode) {
	switch strings.ToLower(item.Phase) {
	case "failed":
		return domain.HealthUnhealthy, "kubernetes.pod.failed"
	case "pending":
		return domain.HealthDegraded, "kubernetes.pod.pending"
	case "succeeded":
		return domain.HealthHealthy, "kubernetes.pod.succeeded"
	case "running":
		return conditionHealth(item.ReadyKnown, item.Ready, "pod")
	default:
		return domain.HealthUnknown, ""
	}
}

func claimHealth(item PersistentVolumeClaim) (domain.HealthState, domain.ReasonCode) {
	switch strings.ToLower(item.Phase) {
	case "bound":
		return domain.HealthHealthy, "kubernetes.claim.bound"
	case "pending":
		return domain.HealthDegraded, "kubernetes.claim.pending"
	case "lost":
		return domain.HealthUnhealthy, "kubernetes.claim.lost"
	default:
		return domain.HealthUnknown, ""
	}
}

func workloadResourceKind(kind string) (domain.ResourceKind, error) {
	value := strings.ToLower(kind)
	switch value {
	case "deployment", "statefulset", "daemonset", "replicaset", "job", "cronjob":
		return domain.ResourceKind("kubernetes." + value), nil
	default:
		return "", fmt.Errorf("unsupported workload kind %q", kind)
	}
}

func resourceKey(kind, nativeID string) string { return kind + "\x00" + nativeID }

func nameKey(kind string, scope domain.Scope, name string) string {
	return kind + "\x00" + string(scope.Kind) + "\x00" + scope.Name + "\x00" + name
}

func findByName(names map[string]string, kind, namespace, name string) string {
	return names[nameKey(kind, namespaceScope(namespace), name)]
}

func selectorMatches(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func containsKind(items []domain.ResourceKind, value domain.ResourceKind) bool {
	return len(items) == 0 || slices.Contains(items, value)
}

func containsScope(items []domain.Scope, value domain.Scope) bool {
	return len(items) == 0 || slices.Contains(items, value)
}

func compareResources(left, right domain.ResourceSnapshot, query domain.InventoryQuery) int {
	comparison := 0
	switch query.SortField {
	case domain.ResourceSortName:
		comparison = strings.Compare(left.Resource.DisplayName, right.Resource.DisplayName)
	case domain.ResourceSortKind:
		comparison = strings.Compare(string(left.Resource.Ref.Identity.Kind), string(right.Resource.Ref.Identity.Kind))
	case domain.ResourceSortObservedAt:
		comparison = left.Status.ObservedAt.Compare(right.Status.ObservedAt)
	}
	if comparison != 0 && query.SortDirection == domain.SortDescending {
		comparison = -comparison
	}
	if comparison == 0 {
		comparison = strings.Compare(string(left.Resource.Ref.ID), string(right.Resource.Ref.ID))
	}
	return comparison
}

func inventoryFingerprint(query domain.InventoryQuery) string {
	return pageFingerprint("inventory", string(query.Source.SourceID), query.Filter.Search, string(query.SortField), string(query.SortDirection), fmt.Sprint(query.Filter.Kinds), fmt.Sprint(query.Filter.Scopes))
}

func topologyFingerprint(query domain.TopologyQuery) string {
	ids := make([]string, len(query.Resources))
	for index, resource := range query.Resources {
		ids[index] = string(resource.ID)
	}
	return pageFingerprint("topology", strings.Join(ids, ","))
}

func pageFingerprint(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:8])
}

func shortDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:8])
}

func encodeCursor(fingerprint string, offset int) domain.PageCursor {
	return domain.PageCursor(base64.RawURLEncoding.EncodeToString([]byte(fingerprint + ":" + strconv.Itoa(offset))))
}

func decodeCursor(cursor domain.PageCursor, fingerprint string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(string(cursor))
	prefix, text, found := strings.Cut(string(decoded), ":")
	offset, numberErr := strconv.Atoi(text)
	if err != nil || !found || numberErr != nil || prefix != fingerprint || offset < 0 || base64.RawURLEncoding.EncodeToString(decoded) != string(cursor) {
		return 0, errors.New("cursor is invalid")
	}
	return offset, nil
}

func paginate[T any](items []T, offset, limit int, fingerprint string) ([]T, domain.PageInfo) {
	end := min(offset+limit, len(items))
	page := domain.PageInfo{Limit: limit, HasMore: end < len(items)}
	if page.HasMore {
		page.NextCursor = encodeCursor(fingerprint, end)
	}
	return slices.Clone(items[offset:end]), page
}

var _ domain.InventoryProvider = (*Provider)(nil)
var _ domain.HealthProvider = (*Provider)(nil)
var _ domain.TopologyProvider = (*Provider)(nil)
var _ domain.LogReadProvider = (*Provider)(nil)
var _ domain.CurrentMetricProvider = (*Provider)(nil)
var _ domain.EventHistoryProvider = (*Provider)(nil)
var _ domain.CapabilityProvider = (*Provider)(nil)

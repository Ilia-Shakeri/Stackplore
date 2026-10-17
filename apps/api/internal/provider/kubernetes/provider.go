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
	ClusterEvents(context.Context) (EventObservation, error)
}

type MetricCollector interface {
	CurrentPodMetrics(context.Context) (MetricObservation, error)
}

type LogCollector interface {
	ReadPodLogs(context.Context, PodLogRequest) (LogObservation, error)
}

type WorkloadOperation struct {
	Kind              string
	Namespace         string
	Name              string
	ResourceVersion   string
	ExpectedUID       string
	OperationID       string
	RequestedReplicas *int
}

type OperationCollector interface {
	PreviewWorkload(context.Context, WorkloadOperation) error
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
	PodUID     string
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
	ExpectedUID string
	Key         string
	Namespace   string
	PodName     string
	Container   string
	Previous    bool
}

type PodLogLine struct {
	TargetKey string
	Timestamp time.Time
	Message   string
	Truncated bool
}

type Snapshot struct {
	ObservedAt      time.Time
	Status          domain.ObservationStatus
	Partitions      []InventoryPartition
	ClusterObserved bool
	ClusterID       string
	ClusterName     string
	ContextName     string
	Namespaces      []Namespace
	Nodes           []Node
	Workloads       []Workload
	Pods            []Pod
	Services        []Service
	Ingresses       []Ingress
	EndpointSlices  []EndpointSlice
	Claims          []PersistentVolumeClaim
	ConfigMaps      []ObjectRef
	Secrets         []ObjectRef
}

type Namespace struct {
	UID   string
	Name  string
	Phase string
}

type Node struct {
	UID        string
	Name       string
	Ready      bool
	ReadyKnown bool
	Metadata   domain.KubernetesResourceMetadata
}

type Workload struct {
	UID            string
	Version        string
	Kind           string
	Namespace      string
	Name           string
	Desired        int
	Ready          int
	ReadyKnown     bool
	OwnerKind      string
	OwnerUID       string
	Metadata       domain.KubernetesResourceMetadata
	ClaimNames     []string
	ConfigMapNames []string
	SecretNames    []string
	DependsOn      []DependencyReference
}

type Pod struct {
	CreatedAt         time.Time
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
	Metadata          domain.KubernetesResourceMetadata
	DependsOn         []DependencyReference
}

type Service struct {
	UID       string
	Namespace string
	Name      string
	Selector  map[string]string
	DependsOn []DependencyReference
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

func (provider *Provider) Close() error {
	if provider != nil {
		if closer, ok := provider.collector.(interface{ Close() error }); ok {
			return closer.Close()
		}
	}
	return nil
}

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
		Resources:    state.resources, Relationships: state.relationships,
		Complete: state.status.State == domain.AvailabilityAvailable && state.status.Completeness == domain.CompletenessComplete,
		Health:   make([]domain.HealthSnapshot, 0, len(state.health)),
	}
	for _, partition := range snapshot.Partitions {
		capture.Coverage = append(capture.Coverage, domain.InventoryCoverage{Source: capture.Source, EnvironmentID: provider.environment,
			Kind: partition.Kind, ScopeMode: domain.InventoryExactScope, Scope: partition.Scope, Status: partition.Status})
	}
	if !state.status.AllowsData() {
		capture.Environments = nil
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
		item.Health = state.health[item.Resource.Ref.ID].InventoryState()
		if query.Filter.MatchesSnapshot(item) && containsKind(query.Filter.Kinds, item.Resource.Ref.Identity.Kind) && containsScope(query.Filter.Scopes, item.Resource.Ref.Identity.Scope) {
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

	for index, name := range names {
		status := unsupported
		if index < 4 || name == domain.CapabilityEventHistory && eventsAvailable || name == domain.CapabilityMetricCurrent && metricsAvailable || name == domain.CapabilityLogRead && logsAvailable {
			status = available
		}
		items[index] = domain.CapabilityStatus{Name: name, Status: status}
	}
	result := domain.CapabilitySet{Source: source, Capabilities: items}
	return result, result.ValidateFor(source)
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
	for _, condition := range item.Metadata.Conditions {
		if condition.Type == domain.KubernetesConditionFailed && condition.Status == domain.KubernetesConditionTrue {
			return domain.HealthUnhealthy, "kubernetes.workload.failed"
		}
	}
	if !item.ReadyKnown || item.Desired < 0 || item.Ready < 0 {
		return domain.HealthUnknown, ""
	}
	if item.Ready >= item.Desired {
		return domain.HealthHealthy, "kubernetes.workload.ready"
	}
	if item.Ready > 0 {
		return domain.HealthDegraded, "kubernetes.workload.part_ready"
	}
	return domain.HealthUnhealthy, "kubernetes.workload.not_ready"
}

func kubernetesContainerHealth(container domain.KubernetesContainerMetadata) (domain.HealthState, domain.ReasonCode) {
	switch container.State {
	case domain.KubernetesContainerRunning:
		if container.Ready {
			return domain.HealthHealthy, "kubernetes.container.ready"
		}
		return domain.HealthDegraded, "kubernetes.container.not_ready"
	case domain.KubernetesContainerWaiting:
		return domain.HealthDegraded, container.ReasonCode
	case domain.KubernetesContainerTerminated:
		if container.ReasonCode == "kubernetes.container.completed" {
			return domain.HealthHealthy, container.ReasonCode
		}
		return domain.HealthUnhealthy, container.ReasonCode
	default:
		return domain.HealthUnknown, ""
	}
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
	return pageFingerprint("inventory", string(query.Source.SourceID), fmt.Sprint(query.Filter), string(query.SortField), string(query.SortDirection), fmt.Sprint(query.Filter.Kinds), fmt.Sprint(query.Filter.Scopes))
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

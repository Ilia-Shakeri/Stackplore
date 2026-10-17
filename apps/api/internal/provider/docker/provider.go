// Package docker normalizes safe Docker Engine observations into domain models.
package docker

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
	ErrPermissionDenied = errors.New("Docker access denied")
	ErrUnavailable      = errors.New("Docker Engine unavailable")
	ErrVersionConflict  = errors.New("Docker resource version changed")
	ErrUnsupported      = errors.New("Docker operation unsupported")
)

type Collector interface {
	Snapshot(context.Context) (Snapshot, error)
}

type MetricCollector interface {
	CurrentContainerStats(context.Context, []string) ([]ContainerStats, error)
}

type LogCollector interface {
	ReadContainerLogs(context.Context, ContainerLogRequest) ([]ContainerLogLine, error)
}

type EventCollector interface {
	EngineEvents(context.Context, time.Time, time.Time) (DockerEventBatch, error)
}

type OperationCollector interface {
	RestartContainer(context.Context, string, string, string) (string, error)
}

type DockerEventBatch struct {
	Events    []DockerEvent
	Truncated bool
}

type DockerEvent struct {
	Type       string
	Action     string
	ActorID    string
	OccurredAt time.Time
}

type ContainerLogRequest struct {
	ContainerIDs   []string
	ContainerNames map[string]string
	Start          time.Time
	End            time.Time
	LineLimit      int
	ByteLimit      int
}

type ContainerLogLine struct {
	ContainerID string
	Stream      domain.LogStreamKind
	Timestamp   time.Time
	Message     string
	Truncated   bool
}

type ContainerStats struct {
	ContainerID       string
	CPUUsageCores     float64
	MemoryUsageBytes  uint64
	NetworkReadBytes  uint64
	NetworkWriteBytes uint64
	RestartCount      uint64
	SampledAt         time.Time
	CPUUnavailable    bool
	Status            domain.ObservationStatus
}

type Snapshot struct {
	ObservedAt    time.Time
	EngineID      string
	EngineName    string
	EngineVersion string
	Containers    []Container
	Images        []Image
	Networks      []Network
	Volumes       []Volume
	Coverage      map[domain.ResourceKind]domain.ObservationStatus
}

type Container struct {
	ID                 string
	Name               string
	Version            string
	ImageID            string
	State              string
	Health             string
	ComposeProject     string
	ComposeService     string
	NetworkIDs         []string
	VolumeNames        []string
	DependsOn          []string
	InspectUnavailable bool
	Metadata           *domain.DockerResourceMetadata
	MetadataIncomplete bool
}

type Image struct{ ID, Name string }
type Network struct{ ID, Name string }
type Volume struct{ Name string }

type Provider struct {
	source      domain.InfrastructureSource
	environment domain.EnvironmentID
	collector   Collector
}

type normalized struct {
	status        domain.ObservationStatus
	resources     []domain.ResourceSnapshot
	health        map[domain.ResourceID]domain.HealthSnapshot
	relationships []domain.Relationship
	coverage      []domain.InventoryCoverage
}

func New(sourceID domain.SourceID, name string, collector Collector) (*Provider, error) {
	source := domain.InfrastructureSource{ID: sourceID, Provider: domain.ProviderDocker, Name: name}
	if collector == nil || source.Validate() != nil {
		return nil, fmt.Errorf("Docker provider options are invalid")
	}
	digest := sha256.Sum256([]byte(sourceID))
	return &Provider{source: source, environment: domain.EnvironmentID("env_docker_" + hex.EncodeToString(digest[:8])), collector: collector}, nil
}

func (provider *Provider) Source() domain.InfrastructureSource { return provider.source }

func (provider *Provider) Close() error {
	if provider == nil {
		return nil
	}
	if closer, ok := provider.collector.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

func (provider *Provider) CaptureInventory(ctx context.Context) (domain.InventoryCapture, error) {
	if err := provider.validate(ctx, domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: provider.source.ID}); err != nil {
		return domain.InventoryCapture{}, providerError("docker.inventory.capture", err)
	}
	state, err := provider.observe(ctx)
	if err != nil {
		return domain.InventoryCapture{}, err
	}
	capture := domain.InventoryCapture{
		Source: domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: provider.source.ID}, Status: state.status,
		Environments: []domain.Environment{{ID: provider.environment, SourceID: provider.source.ID, Provider: domain.ProviderDocker, Name: "Docker Engine", Status: state.status}},
		Resources:    state.resources, Relationships: state.relationships,
		Complete: state.status.State == domain.AvailabilityAvailable && state.status.Completeness == domain.CompletenessComplete,
		Coverage: state.coverage,
		Health:   make([]domain.HealthSnapshot, 0, len(state.health)),
	}
	for _, resource := range state.resources {
		capture.Health = append(capture.Health, state.health[resource.Resource.Ref.ID])
	}
	if err := capture.ValidateFor(capture.Source); err != nil {
		return domain.InventoryCapture{}, providerError("docker.inventory.capture", ErrUnavailable)
	}
	return capture, nil
}

func (provider *Provider) ListEnvironments(ctx context.Context, query domain.EnvironmentQuery) (domain.EnvironmentPage, error) {
	if err := provider.validate(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.EnvironmentPage{}, providerError("docker.inventory.environments", err)
	}
	state, err := provider.observe(ctx)
	if err != nil {
		return domain.EnvironmentPage{}, err
	}
	offset, err := decodeCursor(query.Page.Cursor, fingerprint("environment", string(provider.source.ID)))
	if err != nil || offset > 1 {
		return domain.EnvironmentPage{}, providerError("docker.inventory.environments", err)
	}
	items, page := paginate([]domain.Environment{{
		ID: provider.environment, SourceID: provider.source.ID, Provider: domain.ProviderDocker,
		Name: "Docker Engine", Status: state.status,
	}}, offset, query.Page.Limit, fingerprint("environment", string(provider.source.ID)))
	result := domain.EnvironmentPage{Status: state.status, Environments: items, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.EnvironmentPage{}, providerError("docker.inventory.environments", err)
	}
	return result, nil
}

func (provider *Provider) ListResources(ctx context.Context, query domain.InventoryQuery) (domain.ResourcePage, error) {
	if err := provider.validate(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.ResourcePage{}, providerError("docker.inventory.resources", err)
	}
	state, err := provider.observe(ctx)
	if err != nil {
		return domain.ResourcePage{}, err
	}
	items := make([]domain.ResourceSnapshot, 0, len(state.resources))
	for _, item := range state.resources {
		item.Health = state.health[item.Resource.Ref.ID].InventoryState()
		if query.Filter.MatchesSnapshot(item) && containsKind(query.Filter.Kinds, item.Resource.Ref.Identity.Kind) &&
			containsScope(query.Filter.Scopes, item.Resource.Ref.Identity.Scope) {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(left, right int) bool { return compareResources(items[left], items[right], query) < 0 })
	fingerprint := inventoryFingerprint(query)
	offset, err := decodeCursor(query.Page.Cursor, fingerprint)
	if err != nil || offset > len(items) {
		return domain.ResourcePage{}, providerError("docker.inventory.resources", err)
	}
	items, page := paginate(items, offset, query.Page.Limit, fingerprint)
	result := domain.ResourcePage{Status: state.status, Resources: items, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.ResourcePage{}, providerError("docker.inventory.resources", err)
	}
	return result, nil
}

func (provider *Provider) GetResource(ctx context.Context, query domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	if err := provider.validate(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.ResourceSnapshot{}, providerError("docker.inventory.detail", err)
	}
	state, err := provider.observe(ctx)
	if err != nil {
		return domain.ResourceSnapshot{}, err
	}
	for _, item := range state.resources {
		if item.Resource.Ref.ID == query.ResourceID {
			return item, nil
		}
	}
	return domain.ResourceSnapshot{}, providerError("docker.inventory.detail", errors.New("resource not found"))
}

func (provider *Provider) GetHealth(ctx context.Context, query domain.HealthQuery) (domain.HealthResult, error) {
	if err := provider.validate(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.HealthResult{}, providerError("docker.health.read", err)
	}
	state, err := provider.observe(ctx)
	if err != nil {
		return domain.HealthResult{}, err
	}
	items := make([]domain.HealthSnapshot, 0, len(query.Resources))
	for _, resource := range query.Resources {
		item, exists := state.health[resource.ID]
		if !exists {
			return domain.HealthResult{}, providerError("docker.health.read", errors.New("resource not found"))
		}
		items = append(items, item)
	}
	result := domain.HealthResult{Status: state.status, Snapshots: items}
	if err := result.ValidateFor(query); err != nil {
		return domain.HealthResult{}, providerError("docker.health.read", err)
	}
	return result, nil
}

func (provider *Provider) GetRelationships(ctx context.Context, query domain.TopologyQuery) (domain.RelationshipPage, error) {
	if err := provider.validate(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.RelationshipPage{}, providerError("docker.topology.read", err)
	}
	state, err := provider.observe(ctx)
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
		return domain.RelationshipPage{}, providerError("docker.topology.read", err)
	}
	items, page := paginate(items, offset, query.Page.Limit, fingerprint)
	result := domain.RelationshipPage{Status: state.status, Relationships: items, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.RelationshipPage{}, providerError("docker.topology.read", err)
	}
	return result, nil
}

func (provider *Provider) GetCapabilities(ctx context.Context, source domain.ProviderRef) (domain.CapabilitySet, error) {
	if err := provider.validate(ctx, source); err != nil {
		return domain.CapabilitySet{}, providerError("docker.capability.read", err)
	}
	observedAt := time.Now().UTC().Truncate(time.Microsecond)
	available := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: observedAt}
	unsupported := domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: observedAt, ReasonCode: "docker.capability.not_implemented"}
	names := []domain.CapabilityName{
		domain.CapabilityInventoryList, domain.CapabilityInventoryDetail, domain.CapabilityHealth,
		domain.CapabilityTopology, domain.CapabilityLogRead, domain.CapabilityLogTail,
		domain.CapabilityMetricCurrent, domain.CapabilityMetricHistory,
		domain.CapabilityEventHistory, domain.CapabilityEventWatch,
		domain.CapabilityDockerRestart, domain.CapabilityWorkloadRestart, domain.CapabilityWorkloadScale,
	}
	items := make([]domain.CapabilityStatus, len(names))
	_, metricsAvailable := provider.collector.(MetricCollector)
	_, logsAvailable := provider.collector.(LogCollector)
	_, eventsAvailable := provider.collector.(EventCollector)
	for index, name := range names {
		status := unsupported
		if index < 4 || name == domain.CapabilityMetricCurrent && metricsAvailable || name == domain.CapabilityLogRead && logsAvailable || name == domain.CapabilityEventHistory && eventsAvailable {
			status = available
		}
		items[index] = domain.CapabilityStatus{Name: name, Status: status}
	}
	result := domain.CapabilitySet{Source: source, Capabilities: items}
	return result, result.ValidateFor(source)
}

func (provider *Provider) ReadLogs(ctx context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	if err := provider.validate(ctx, query.Source); err != nil {
		return domain.LogPage{}, providerError("docker.log.read", err)
	}
	if err := query.Validate(); err != nil {
		return domain.LogPage{}, providerError("docker.log.read", err)
	}
	collector, ok := provider.collector.(LogCollector)
	observedAt := time.Now().UTC().Truncate(time.Microsecond)
	if !ok {
		result := domain.LogPage{
			Status: domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: observedAt, ReasonCode: "docker.log.not_implemented"},
			Page:   domain.PageInfo{Limit: query.LineLimit},
		}
		return result, result.ValidateFor(query)
	}
	containerIDs := make([]string, 0, len(query.Targets))
	requested := make(map[string]domain.ResourceID, len(query.Targets))
	containerNames := make(map[string]string, len(query.Targets))
	unsupported := false
	for _, target := range query.Targets {
		if target.Resource.Identity.Kind != "docker.container" || target.Previous {
			unsupported = true
			continue
		}
		nativeID := string(target.Resource.Identity.NativeID())
		containerIDs = append(containerIDs, nativeID)
		requested[nativeID] = target.Resource.ID
		containerNames[nativeID] = target.Container
	}
	if len(containerIDs) == 0 && unsupported {
		result := domain.LogPage{Status: domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: observedAt, ReasonCode: "docker.log.target_unsupported"}, Page: domain.PageInfo{Limit: query.LineLimit}}
		return result, result.ValidateFor(query)
	}
	fingerprint := logFingerprint(query)
	offset, err := decodeCursor(query.Cursor, fingerprint)
	if err != nil {
		return domain.LogPage{}, providerError("docker.log.read", err)
	}
	request := ContainerLogRequest{
		ContainerIDs: containerIDs, ContainerNames: containerNames, Start: query.Range.Start, End: query.Range.End,
		LineLimit: domain.MaxLogLines, ByteLimit: query.ByteLimit,
	}
	var lines []ContainerLogLine
	truncatedBatch := false
	if bounded, ok := collector.(interface {
		ReadContainerLogBatch(context.Context, ContainerLogRequest) (ContainerLogBatch, error)
	}); ok {
		batch, readErr := bounded.ReadContainerLogBatch(ctx, request)
		lines, err, truncatedBatch = batch.Lines, readErr, batch.Truncated
	} else {
		lines, err = collector.ReadContainerLogs(ctx, request)
	}
	if err != nil {
		return domain.LogPage{}, providerError("docker.log.read", err)
	}
	records := make([]domain.LogRecord, 0, len(lines))
	for index, line := range lines {
		resourceID, exists := requested[line.ContainerID]
		if !exists || line.Timestamp.Before(query.Range.Start) || !line.Timestamp.Before(query.Range.End) {
			continue
		}
		message, redacted, truncated := safeLogMessage(line.Message, line.Truncated)
		records = append(records, domain.LogRecord{
			ID:         domain.LogEntryID("log_docker_" + shortDigest(line.ContainerID+"\x00"+line.Timestamp.Format(time.RFC3339Nano)+"\x00"+string(line.Stream)+"\x00"+strconv.Itoa(index))),
			ResourceID: resourceID, Stream: line.Stream, Severity: logSeverity(message),
			Timestamp: line.Timestamp, Message: message, Redacted: redacted, Truncated: truncated,
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
		return domain.LogPage{}, providerError("docker.log.read", errors.New("cursor is invalid"))
	}
	records, page := paginate(records, offset, query.LineLimit, fingerprint)
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: observedAt}
	if unsupported {
		status.Completeness = domain.CompletenessPartial
		status.ReasonCode = "docker.log.resource_unsupported"
	}
	if truncatedBatch {
		status.Completeness, status.ReasonCode = domain.CompletenessPartial, "docker.log.collector_bound"
	}
	result := domain.LogPage{Status: status, Records: records, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.LogPage{}, providerError("docker.log.read", err)
	}
	return result, nil
}

func logFingerprint(query domain.LogReadQuery) string {
	targets := make([]string, len(query.Targets))
	for index, target := range query.Targets {
		targets[index] = string(target.Resource.ID) + "\x00" + target.Container + "\x00" + strconv.FormatBool(target.Previous)
	}
	return fingerprint("logs", strings.Join(targets, ","), query.Range.Start.Format(time.RFC3339Nano), query.Range.End.Format(time.RFC3339Nano))
}

func safeLogMessage(value string, truncated bool) (string, bool, bool) {
	return logsafe.Message(value, truncated)
}

func logSeverity(message string) domain.LogSeverity {
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

func (provider *Provider) ListEvents(ctx context.Context, query domain.EventQuery) (domain.EventPage, error) {
	if err := provider.validate(ctx, query.Source); err != nil {
		return domain.EventPage{}, providerError("docker.event.history", err)
	}
	if err := query.Validate(); err != nil {
		return domain.EventPage{}, providerError("docker.event.history", err)
	}
	collector, ok := provider.collector.(EventCollector)
	observedAt := time.Now().UTC().Truncate(time.Microsecond)
	if !ok {
		result := domain.EventPage{
			Status: domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: observedAt, ReasonCode: "docker.event.not_implemented"},
			Page:   domain.PageInfo{Limit: query.Page.Limit},
		}
		return result, result.ValidateFor(query)
	}
	state, err := provider.observe(ctx)
	if err != nil {
		return domain.EventPage{}, err
	}
	observedAt = state.status.ObservedAt
	collectorEnd := query.Range.End
	if collectorEnd.After(observedAt.Add(time.Microsecond)) {
		collectorEnd = observedAt.Add(time.Microsecond)
	}
	batch, err := collector.EngineEvents(ctx, query.Range.Start, collectorEnd)
	if err != nil {
		return domain.EventPage{}, providerError("docker.event.history", err)
	}
	byNative := make(map[string]domain.ResourceID, len(state.resources))
	for _, resource := range state.resources {
		key := string(resource.Resource.Ref.Identity.Kind) + "\x00" + string(resource.Resource.Ref.Identity.NativeID())
		byNative[key] = resource.Resource.Ref.ID
	}
	requested := make(map[domain.ResourceID]struct{}, len(query.Resources))
	for _, resource := range query.Resources {
		requested[resource.ID] = struct{}{}
	}
	severities := make(map[domain.Severity]struct{}, len(query.Severities))
	for _, severity := range query.Severities {
		severities[severity] = struct{}{}
	}
	items := make([]domain.InfrastructureEvent, 0, len(batch.Events))
	for _, raw := range batch.Events {
		if raw.OccurredAt.Location() != time.UTC || raw.OccurredAt.Nanosecond()%int(time.Microsecond) != 0 ||
			raw.OccurredAt.Before(query.Range.Start) || !raw.OccurredAt.Before(query.Range.End) || raw.OccurredAt.After(observedAt) {
			continue
		}
		kind := dockerEventResourceKind(raw.Type)
		resourceID, scoped := byNative[string(kind)+"\x00"+raw.ActorID]
		if len(requested) > 0 {
			if !scoped {
				continue
			}
			if _, exists := requested[resourceID]; !exists {
				continue
			}
		}
		severity := dockerEventSeverity(raw.Action)
		if len(severities) > 0 {
			if _, exists := severities[severity]; !exists {
				continue
			}
		}
		typeCode := dockerEventCode(raw.Type)
		actionCode := dockerEventCode(raw.Action)
		event := domain.InfrastructureEvent{
			ID:       domain.EventID("event_docker_" + shortDigest(string(provider.source.ID)+"\x00"+raw.Type+"\x00"+raw.Action+"\x00"+raw.ActorID+"\x00"+raw.OccurredAt.Format(time.RFC3339Nano)+"\x00"+string(resourceID))),
			Provider: domain.ProviderDocker, SourceID: provider.source.ID, EnvironmentID: &provider.environment,
			Kind: domain.EventKind("docker." + typeCode + ".changed"), Severity: severity,
			ReasonCode: domain.ReasonCode("docker.event." + actionCode),
			Summary:    "Docker " + typeCode + " " + actionCode, OccurredAt: raw.OccurredAt, ObservedAt: observedAt,
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
	fingerprint := dockerEventFingerprint(query)
	offset, err := decodeCursor(query.Page.Cursor, fingerprint)
	if err != nil || offset > len(items) {
		if err == nil {
			err = errors.New("cursor is invalid")
		}
		return domain.EventPage{}, providerError("docker.event.history", err)
	}
	items, page := paginate(items, offset, query.Page.Limit, fingerprint)
	status := state.status
	if batch.Truncated {
		status.Completeness = domain.CompletenessPartial
		status.ReasonCode = "docker.event.collector_bound"
	}
	result := domain.EventPage{Status: status, Events: items, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.EventPage{}, providerError("docker.event.history", err)
	}
	return result, nil
}

func dockerEventResourceKind(value string) domain.ResourceKind {
	switch strings.ToLower(value) {
	case "container":
		return "docker.container"
	case "image":
		return "docker.image"
	case "network":
		return "docker.network"
	case "volume":
		return "docker.volume"
	case "daemon":
		return "docker.engine"
	default:
		return "docker.unknown"
	}
}

func dockerEventSeverity(action string) domain.Severity {
	action = strings.ToLower(action)
	switch {
	case strings.Contains(action, "oom"):
		return domain.SeverityCritical
	case strings.Contains(action, "die"), strings.Contains(action, "kill"):
		return domain.SeverityError
	case strings.Contains(action, "destroy"), strings.Contains(action, "remove"), strings.Contains(action, "stop"),
		strings.Contains(action, "disconnect"), strings.Contains(action, "unhealthy"):
		return domain.SeverityWarning
	default:
		return domain.SeverityInfo
	}
}

func dockerEventCode(value string) string {
	var result strings.Builder
	for _, character := range strings.ToLower(value) {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' || character == '_' {
			result.WriteRune(character)
		} else if result.Len() > 0 {
			result.WriteByte('_')
		}
		if result.Len() >= 72 {
			break
		}
	}
	code := strings.Trim(result.String(), "_-")
	if code == "" || code[0] < 'a' || code[0] > 'z' {
		return "unknown"
	}
	return code
}

func dockerEventFingerprint(query domain.EventQuery) string {
	resources := make([]string, len(query.Resources))
	for index, resource := range query.Resources {
		resources[index] = string(resource.ID)
	}
	severities := make([]string, len(query.Severities))
	for index, severity := range query.Severities {
		severities[index] = string(severity)
	}
	return fingerprint("events", strings.Join(resources, ","), strings.Join(severities, ","), query.Range.Start.Format(time.RFC3339Nano), query.Range.End.Format(time.RFC3339Nano))
}

func (provider *Provider) GetCurrentMetrics(ctx context.Context, query domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
	if err := provider.validate(ctx, query.Source); err != nil || query.Validate() != nil {
		return domain.CurrentMetricResult{}, providerError("docker.metric.current", err)
	}
	collector, ok := provider.collector.(MetricCollector)
	observedAt := time.Now().UTC().Truncate(time.Microsecond)
	if !ok {
		result := domain.CurrentMetricResult{Status: domain.ObservationStatus{
			State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete,
			ObservedAt: observedAt, ReasonCode: "docker.metric.not_implemented",
		}}
		return result, result.ValidateFor(query)
	}
	containerIDs := make([]string, 0, len(query.Resources))
	for _, resource := range query.Resources {
		if resource.Identity.Kind == "docker.container" {
			containerIDs = append(containerIDs, string(resource.Identity.NativeID()))
		}
	}
	stats, err := collector.CurrentContainerStats(ctx, containerIDs)
	if err != nil {
		return domain.CurrentMetricResult{}, providerError("docker.metric.current", err)
	}
	byContainer := make(map[string]ContainerStats, len(stats))
	for _, item := range stats {
		if item.ContainerID == "" || item.SampledAt.Location() != time.UTC || item.SampledAt.Nanosecond()%int(time.Microsecond) != 0 {
			return domain.CurrentMetricResult{}, providerError("docker.metric.current", errors.New("invalid collector metric"))
		}
		if _, exists := byContainer[item.ContainerID]; exists {
			return domain.CurrentMetricResult{}, providerError("docker.metric.current", errors.New("duplicate collector metric"))
		}
		byContainer[item.ContainerID] = item
		if item.SampledAt.After(observedAt) {
			observedAt = item.SampledAt
		}
	}
	available := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: observedAt}
	sets := make([]domain.CurrentMetricSet, 0, len(query.Resources))
	for _, resource := range query.Resources {
		set := domain.CurrentMetricSet{ResourceID: resource.ID, Status: available}
		stats, exists := byContainer[string(resource.Identity.NativeID())]
		if resource.Identity.Kind != "docker.container" {
			set.Status = domain.ObservationStatus{State: domain.AvailabilityUnsupported, Completeness: domain.CompletenessComplete, ObservedAt: observedAt, ReasonCode: "docker.metric.resource_unsupported"}
			sets = append(sets, set)
			continue
		}
		if !exists {
			set.Status = domain.ObservationStatus{State: domain.AvailabilityUnavailable, Completeness: domain.CompletenessComplete, ObservedAt: observedAt, ReasonCode: "docker.metric.container_missing"}
			sets = append(sets, set)
			continue
		}
		set.Status.ObservedAt = stats.SampledAt
		if stats.Status.State != "" && stats.Status.State != domain.AvailabilityAvailable {
			set.Status = stats.Status
			sets = append(sets, set)
			available.Completeness, available.ReasonCode = domain.CompletenessPartial, "docker.metric.partial"
			continue
		}
		for _, name := range query.Names {
			value, unit, supported := dockerMetricValue(stats, name)
			if !supported {
				set.Status.Completeness = domain.CompletenessPartial
				set.Status.ReasonCode = "docker.metric.name_unsupported"
				available.Completeness, available.ReasonCode = domain.CompletenessPartial, "docker.metric.partial"
				continue
			}
			set.Samples = append(set.Samples, domain.MetricSample{
				ID:         domain.MetricSampleID("metric_docker_" + shortDigest(string(resource.ID)+"\x00"+string(name)+"\x00"+stats.SampledAt.Format(time.RFC3339Nano))),
				ResourceID: resource.ID, Name: name, Unit: unit, Value: value, SampledAt: stats.SampledAt,
			})
		}
		sets = append(sets, set)
	}
	result := domain.CurrentMetricResult{Status: available, Sets: sets}
	if err := result.ValidateFor(query); err != nil {
		return domain.CurrentMetricResult{}, providerError("docker.metric.current", err)
	}
	return result, nil
}

func dockerMetricValue(stats ContainerStats, name domain.MetricName) (float64, domain.MetricUnit, bool) {
	switch name {
	case domain.MetricCPUUsageCores:
		return stats.CPUUsageCores, domain.MetricUnitCores, !stats.CPUUnavailable
	case domain.MetricMemoryUsageBytes:
		return float64(stats.MemoryUsageBytes), domain.MetricUnitBytes, true
	case domain.MetricNetworkReceiveBytesTotal:
		return float64(stats.NetworkReadBytes), domain.MetricUnitBytes, true
	case domain.MetricNetworkSendBytesTotal:
		return float64(stats.NetworkWriteBytes), domain.MetricUnitBytes, true
	case domain.MetricRestartCount:
		return float64(stats.RestartCount), domain.MetricUnitCount, true
	default:
		return 0, "", false
	}
}

func (provider *Provider) observe(ctx context.Context) (normalized, error) {
	snapshot, err := provider.collector.Snapshot(ctx)
	if err != nil {
		return normalized{}, providerError("docker.snapshot.read", err)
	}
	if snapshot.ObservedAt.Location() != time.UTC || snapshot.ObservedAt.Nanosecond()%int(time.Microsecond) != 0 || snapshot.EngineID == "" || snapshot.EngineName == "" {
		return normalized{}, providerError("docker.snapshot.read", errors.New("invalid collector snapshot"))
	}
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: snapshot.ObservedAt}
	state := normalized{status: status, health: make(map[domain.ResourceID]domain.HealthSnapshot)}
	for _, kind := range []domain.ResourceKind{"docker.engine", "docker.container", "docker.compose_project", "docker.compose_service", "docker.image", "docker.network", "docker.volume"} {
		partitionStatus, exists := snapshot.Coverage[kind]
		if !exists {
			partitionStatus = status
		}
		if partitionStatus.Validate() != nil || !partitionStatus.ObservedAt.Equal(snapshot.ObservedAt) {
			return normalized{}, providerError("docker.snapshot.read", ErrUnavailable)
		}
		state.coverage = append(state.coverage, domain.InventoryCoverage{
			Source: domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: provider.source.ID}, EnvironmentID: provider.environment,
			Kind: kind, ScopeMode: domain.InventoryAllScopes, Status: partitionStatus,
		})
		if partitionStatus.State != domain.AvailabilityAvailable || partitionStatus.Completeness != domain.CompletenessComplete {
			state.status.Completeness, state.status.ReasonCode = domain.CompletenessPartial, "docker.inventory.partial"
		}
	}
	byNative := make(map[string]domain.ResourceRef)
	add := func(kind domain.ResourceKind, scope domain.Scope, nativeID, name string, health domain.HealthState, reason domain.ReasonCode) error {
		if len(state.resources) >= domain.MaxInventoryCaptureItems {
			return ErrUnavailable
		}
		identity, err := domain.NewResourceIdentity(domain.ProviderDocker, provider.source.ID, provider.environment, kind, scope, domain.NativeResourceID(nativeID))
		if err != nil {
			return err
		}
		ref, err := domain.NewResourceRef(identity)
		if err != nil {
			return err
		}
		resource := domain.ResourceSnapshot{Resource: domain.Resource{Ref: ref, DisplayName: name}, Status: status}
		state.resources = append(state.resources, resource)
		byNative[string(kind)+"\x00"+nativeID] = ref
		healthSnapshot := domain.HealthSnapshot{ResourceID: ref.ID, State: health, Status: status}
		if reason != "" {
			evidenceState := domain.EvidenceUnknown
			if health == domain.HealthHealthy {
				evidenceState = domain.EvidencePositive
			} else if health == domain.HealthUnhealthy {
				evidenceState = domain.EvidenceNegative
			}
			healthSnapshot.Evidence = []domain.HealthEvidence{{
				ID: domain.EvidenceID("evidence_docker_" + shortDigest(string(ref.ID))), Code: reason,
				SourceCode: reason, Status: evidenceState, ObservedAt: snapshot.ObservedAt,
			}}
		}
		state.health[ref.ID] = healthSnapshot
		return nil
	}
	if err := add("docker.engine", domain.Scope{Kind: domain.ScopeGlobal}, snapshot.EngineID, snapshot.EngineName, domain.HealthHealthy, "docker.engine.reachable"); err != nil {
		return normalized{}, providerError("docker.snapshot.normalize", err)
	}
	engineMetadata := &domain.DockerResourceMetadata{EngineVersion: snapshot.EngineVersion, State: "unknown", Health: "unknown", Ports: []domain.DockerPortMetadata{}, Mounts: []domain.DockerMountMetadata{}, Labels: []domain.DockerLabelMetadata{}}
	if engineMetadata.Validate() == nil {
		state.resources[len(state.resources)-1].Metadata = &domain.ResourceMetadata{Docker: engineMetadata}
	}
	projects := map[string]struct{}{}
	services := map[string]struct{}{}
	for _, container := range snapshot.Containers {
		if container.ComposeProject != "" {
			projects[container.ComposeProject] = struct{}{}
		}
		if container.ComposeProject != "" && container.ComposeService != "" {
			services[container.ComposeProject+"/"+container.ComposeService] = struct{}{}
		}
	}
	for _, project := range sortedKeys(projects) {
		if err := add("docker.compose_project", domain.Scope{Kind: domain.ScopeProject, Name: project}, "project/"+project, project, domain.HealthUnknown, ""); err != nil {
			return normalized{}, providerError("docker.snapshot.normalize", err)
		}
	}
	for _, service := range sortedKeys(services) {
		project, name, _ := strings.Cut(service, "/")
		if err := add("docker.compose_service", domain.Scope{Kind: domain.ScopeProject, Name: project}, "service/"+service, name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, providerError("docker.snapshot.normalize", err)
		}
	}
	for _, image := range snapshot.Images {
		if err := add("docker.image", domain.Scope{Kind: domain.ScopeGlobal}, image.ID, image.Name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, providerError("docker.snapshot.normalize", err)
		}
	}
	for _, network := range snapshot.Networks {
		if err := add("docker.network", domain.Scope{Kind: domain.ScopeGlobal}, network.ID, network.Name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, providerError("docker.snapshot.normalize", err)
		}
	}
	for _, volume := range snapshot.Volumes {
		if err := add("docker.volume", domain.Scope{Kind: domain.ScopeGlobal}, volume.Name, volume.Name, domain.HealthUnknown, ""); err != nil {
			return normalized{}, providerError("docker.snapshot.normalize", err)
		}
	}
	for _, container := range snapshot.Containers {
		scope := domain.Scope{Kind: domain.ScopeGlobal}
		if container.ComposeProject != "" {
			scope = domain.Scope{Kind: domain.ScopeProject, Name: container.ComposeProject}
		}
		health, reason := containerHealth(container)
		if err := add("docker.container", scope, container.ID, container.Name, health, reason); err != nil {
			return normalized{}, providerError("docker.snapshot.normalize", err)
		}
		state.resources[len(state.resources)-1].Resource.Version = container.Version
		if container.Metadata != nil {
			state.resources[len(state.resources)-1].Metadata = &domain.ResourceMetadata{Docker: container.Metadata}
		}
		if container.MetadataIncomplete {
			state.resources[len(state.resources)-1].Status.Completeness = domain.CompletenessPartial
			state.resources[len(state.resources)-1].Status.ReasonCode = "docker.metadata.partial"
			state.status.Completeness, state.status.ReasonCode = domain.CompletenessPartial, "docker.inventory.partial"
		}
		if container.InspectUnavailable {
			resource := &state.resources[len(state.resources)-1]
			resource.Status.Completeness, resource.Status.ReasonCode = domain.CompletenessPartial, "docker.inventory.inspect_incomplete"
			state.health[resource.Resource.Ref.ID] = domain.HealthSnapshot{ResourceID: resource.Resource.Ref.ID, State: domain.HealthUnknown, Status: resource.Status}
		}
	}
	if err := provider.buildRelationships(&state, byNative, snapshot); err != nil {
		return normalized{}, providerError("docker.snapshot.normalize", err)
	}
	sort.Slice(state.resources, func(left, right int) bool {
		return state.resources[left].Resource.Ref.ID < state.resources[right].Resource.Ref.ID
	})
	sort.Slice(state.relationships, func(left, right int) bool { return state.relationships[left].ID < state.relationships[right].ID })
	return state, nil
}

func (provider *Provider) buildRelationships(state *normalized, refs map[string]domain.ResourceRef, snapshot Snapshot) error {
	add := func(from, to domain.ResourceRef, kind domain.RelationshipKind, provenance domain.RelationshipProvenance, evidenceID string) error {
		if len(state.relationships) >= domain.MaxInventoryCaptureItems {
			return ErrUnavailable
		}
		evidence := domain.RelationshipEvidence{ID: domain.EvidenceID(evidenceID), Provider: domain.ProviderDocker, SourceID: provider.source.ID, Provenance: provenance, Certainty: domain.EvidenceExplicit, Confidence: 1, ObservedAt: snapshot.ObservedAt}
		relationship, err := domain.NewRelationship(from.ID, to.ID, kind, domain.RelationshipSourceToTarget, []domain.RelationshipEvidence{evidence})
		if err == nil {
			state.relationships = append(state.relationships, relationship)
		}
		return err
	}
	seen := map[string]struct{}{}
	link := func(fromKey, toKey string, kind domain.RelationshipKind, provenance domain.RelationshipProvenance) error {
		from, fromOK := refs[fromKey]
		to, toOK := refs[toKey]
		if !fromOK || !toOK {
			return nil
		}
		key := fromKey + "\x00" + toKey + "\x00" + string(kind) + "\x00" + string(provenance)
		if _, duplicate := seen[key]; duplicate {
			return nil
		}
		seen[key] = struct{}{}
		return add(from, to, kind, provenance, "evidence_docker_relation_"+shortDigest(key))
	}
	for service := range mapFromServices(snapshot.Containers) {
		project, _, _ := strings.Cut(service, "/")
		if err := link("docker.compose_project\x00project/"+project, "docker.compose_service\x00service/"+service, domain.RelationshipOwns, domain.ProvenanceOwnership); err != nil {
			return err
		}
	}
	for _, container := range snapshot.Containers {
		containerKey := "docker.container\x00" + container.ID
		if container.ComposeProject != "" && container.ComposeService != "" {
			if err := link("docker.compose_service\x00service/"+container.ComposeProject+"/"+container.ComposeService, containerKey, domain.RelationshipOwns, domain.ProvenanceOwnership); err != nil {
				return err
			}
		}
		if err := link(containerKey, "docker.image\x00"+container.ImageID, domain.RelationshipReferences, domain.ProvenanceStorageReference); err != nil {
			return err
		}
		for _, network := range container.NetworkIDs {
			if err := link(containerKey, "docker.network\x00"+network, domain.RelationshipMemberOf, domain.ProvenanceNetworkMembership); err != nil {
				return err
			}
		}
		for _, volume := range container.VolumeNames {
			if err := link(containerKey, "docker.volume\x00"+volume, domain.RelationshipMounts, domain.ProvenanceStorageReference); err != nil {
				return err
			}
		}
		if container.ComposeProject != "" && container.ComposeService != "" {
			for _, dependency := range container.DependsOn {
				if dependency == container.ComposeService {
					continue
				}
				if err := link("docker.compose_service\x00service/"+container.ComposeProject+"/"+container.ComposeService,
					"docker.compose_service\x00service/"+container.ComposeProject+"/"+dependency,
					domain.RelationshipDependsOn, domain.ProvenanceDeclaredDependency); err != nil {
					return err
				}
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
	if source != (domain.ProviderRef{Provider: domain.ProviderDocker, SourceID: provider.source.ID}) {
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

func containerHealth(container Container) (domain.HealthState, domain.ReasonCode) {
	switch container.State {
	case "paused", "restarting":
		return domain.HealthDegraded, "docker.container.degraded"
	case "exited", "dead":
		return domain.HealthUnhealthy, "docker.container.stopped"
	case "running":
	default:
		return domain.HealthUnknown, ""
	}
	switch container.Health {
	case "healthy":
		return domain.HealthHealthy, "docker.container.healthy"
	case "unhealthy":
		return domain.HealthUnhealthy, "docker.container.unhealthy"
	case "starting":
		return domain.HealthDegraded, "docker.container.health_starting"
	}
	return domain.HealthHealthy, "docker.container.running"
}

func containsKind(items []domain.ResourceKind, value domain.ResourceKind) bool {
	return len(items) == 0 || slices.Contains(items, value)
}
func containsScope(items []domain.Scope, value domain.Scope) bool {
	return len(items) == 0 || slices.Contains(items, value)
}
func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func mapFromServices(containers []Container) map[string]struct{} {
	result := map[string]struct{}{}
	for _, item := range containers {
		if item.ComposeProject != "" && item.ComposeService != "" {
			result[item.ComposeProject+"/"+item.ComposeService] = struct{}{}
		}
	}
	return result
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
	return fingerprint("inventory", string(query.Source.SourceID), fmt.Sprint(query.Filter), string(query.SortField), string(query.SortDirection), fmt.Sprint(query.Filter.Kinds), fmt.Sprint(query.Filter.Scopes))
}
func topologyFingerprint(query domain.TopologyQuery) string {
	ids := make([]string, len(query.Resources))
	for index, resource := range query.Resources {
		ids[index] = string(resource.ID)
	}
	return fingerprint("topology", strings.Join(ids, ","))
}
func fingerprint(parts ...string) string {
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
var _ domain.CapabilityProvider = (*Provider)(nil)

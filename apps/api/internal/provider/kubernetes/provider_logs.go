package kubernetes

import (
	"context"
	"errors"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/logsafe"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

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
	unresolved := make([]domain.ObservationStatus, 0, len(query.Targets))
	partial := false
	const maximumPodLogTargets = 64
	for _, target := range query.Targets {
		podUID, container, supported := kubernetesLogTarget(target.Resource)
		pod, exists := pods[podUID]
		if !supported || !exists {
			partial = true
			status := observationStatus(observedAt, ErrUnsupported, false)
			if supported {
				status = observationStatus(observedAt, ErrUnavailable, false)
				for _, partition := range snapshot.Partitions {
					if partition.Kind == "kubernetes.pod" && partition.Scope == target.Resource.Identity.Scope && !partition.Status.AllowsData() {
						status = partition.Status
						break
					}
				}
			}
			unresolved = append(unresolved, status)
			continue
		}
		containers := []string{container}
		if target.Container != "" {
			if container != "" && container != target.Container || !slices.Contains(pod.Containers, target.Container) {
				partial = true
				unresolved = append(unresolved, observationStatus(observedAt, ErrUnsupported, false))
				continue
			}
			containers = []string{target.Container}
		} else if container == "" {
			containers = pod.Containers
		}
		for _, name := range containers {
			if len(targets) >= maximumPodLogTargets {
				partial = true
				break
			}
			key := string(target.Resource.ID) + "\x00" + name + "\x00" + strconv.FormatBool(target.Previous)
			targets = append(targets, PodLogTarget{Key: key, ExpectedUID: pod.UID, Namespace: pod.Namespace, PodName: pod.Name, Container: name, Previous: target.Previous})
			actualTarget := target
			actualTarget.Container = name
			requested[key] = actualTarget
		}
	}
	if len(targets) == 0 {
		status := observationStatus(observedAt, ErrUnsupported, false)
		if len(unresolved) > 0 {
			status = combineStatuses(observedAt, unresolved)
		}
		result := domain.LogPage{Status: status, Page: domain.PageInfo{Limit: query.LineLimit}}
		return result, result.ValidateFor(query)
	}
	observation, err := collector.ReadPodLogs(ctx, PodLogRequest{
		Targets: targets, Start: query.Range.Start,
		LineLimit: maximumCollectedLogLines, ByteLimit: query.ByteLimit,
	})
	if err != nil {
		return domain.LogPage{}, providerError("kubernetes.log.read", err)
	}
	records := make([]domain.LogRecord, 0, len(observation.Lines))
	bytesUsed := 0
	for index, line := range observation.Lines {
		target, exists := requested[line.TargetKey]
		if !exists || line.Timestamp.Before(query.Range.Start) || !line.Timestamp.Before(query.Range.End) {
			continue
		}
		message, redacted, truncated := safeKubernetesLogMessage(line.Message, line.Truncated)
		remaining := query.ByteLimit - bytesUsed
		if remaining <= 0 {
			partial = true
			break
		}
		if len(message) > remaining {
			end := remaining
			for end > 0 && !utf8.ValidString(message[:end]) {
				end--
			}
			message = message[:end]
			truncated = true
			partial = true
		}
		bytesUsed += len(message)
		records = append(records, domain.LogRecord{
			ID:         domain.LogEntryID("log_kubernetes_" + shortDigest(line.TargetKey+"\x00"+line.Timestamp.Format(time.RFC3339Nano)+"\x00"+strconv.Itoa(index)+"\x00"+message)),
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
	identities := make([]string, len(records))
	for index, record := range records {
		identities[index] = string(record.ID)
	}
	fingerprint := pageFingerprint(kubernetesLogFingerprint(query), strings.Join(identities, ","))
	offset, err := decodeCursor(query.Cursor, fingerprint)
	if err != nil {
		return domain.LogPage{}, providerError("kubernetes.log.read", err)
	}
	if offset > len(records) {
		return domain.LogPage{}, providerError("kubernetes.log.read", errors.New("cursor is invalid"))
	}
	records, page := paginate(records, offset, query.LineLimit, fingerprint)
	status := observation.Status
	if status.State == "" {
		status = observationStatus(observedAt, nil, false)
	}
	if partial && status.AllowsData() {
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
		targets[index] = string(target.Resource.ID) + "\x00" + target.Container + "\x00" + strconv.FormatBool(target.Previous)
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

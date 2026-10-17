package kubernetes

import (
	"context"
	"errors"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/logsafe"
	"sort"
	"strconv"
	"strings"
	"time"
)

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
	observation, err := collector.ClusterEvents(ctx)
	if err != nil {
		return domain.EventPage{}, providerError("kubernetes.event.history", err)
	}
	rawEvents := observation.Events
	status := observation.Status
	if status.State == "" {
		status = state.status
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
		if query.MatchesTextAndEnvironment(event) {
			items = append(items, event)
		}
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
	result := domain.EventPage{Status: status, Events: items, Page: page}
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
	switch strings.ToLower(value) {
	case "scheduled", "failedscheduling", "pulling", "pulled", "created", "started", "killing", "backoff", "failed", "unhealthy",
		"failedmount", "failedattachvolume", "successfulattachvolume", "successfulcreate", "successfuldelete", "failedcreate", "faileddelete",
		"scalingreplicaset", "nodenotready", "nodeready", "evicted", "oomkilling", "preempting", "imagepullbackoff", "errimagepull":
		return domain.ReasonCode("kubernetes.event." + strings.ToLower(value))
	default:
		return "kubernetes.event.unknown"
	}
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
	return pageFingerprint("events", strings.Join(resources, ","), strings.Join(severities, ","), string(query.EnvironmentID), query.Search, query.Range.Start.Format(time.RFC3339Nano), query.Range.End.Format(time.RFC3339Nano))
}

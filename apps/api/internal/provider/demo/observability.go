package demo

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func seedObservability(provider *Provider) error {
	edges := []struct {
		from, to int
		kind     domain.RelationshipKind
	}{
		{0, 3, domain.RelationshipDependsOn},
		{0, 4, domain.RelationshipDependsOn},
		{1, 2, domain.RelationshipRoutesTo},
		{2, 0, domain.RelationshipRoutesTo},
		{5, 3, domain.RelationshipDependsOn},
	}
	for index, edge := range edges {
		evidence := domain.RelationshipEvidence{
			ID:       domain.EvidenceID(fmt.Sprintf("evidence_topology_%d", index+1)),
			Provider: domain.ProviderDemo, SourceID: SourceID,
			Provenance: domain.ProvenanceDeclaredDependency, Certainty: domain.EvidenceExplicit,
			Confidence: 1, ObservedAt: observedAt, Synthetic: true,
		}
		relationship, err := domain.NewRelationship(
			provider.resources[edge.from].Resource.Ref.ID,
			provider.resources[edge.to].Resource.Ref.ID,
			edge.kind,
			domain.RelationshipSourceToTarget,
			[]domain.RelationshipEvidence{evidence},
		)
		if err != nil {
			return err
		}
		provider.relationships = append(provider.relationships, relationship)
	}
	sort.Slice(provider.relationships, func(left, right int) bool {
		return provider.relationships[left].ID < provider.relationships[right].ID
	})

	messages := []string{
		"request completed status=200 duration_ms=34",
		"checkout failed reason=payment_declined",
		"gateway latency above demo threshold",
		"database checkpoint completed",
		"cache hit ratio stable",
		"job completed queue=notifications",
	}
	severities := []domain.LogSeverity{
		domain.LogSeverityInfo, domain.LogSeverityError, domain.LogSeverityWarning,
		domain.LogSeverityInfo, domain.LogSeverityInfo, domain.LogSeverityInfo,
	}
	eventSeverities := []domain.Severity{
		domain.SeverityInfo, domain.SeverityError, domain.SeverityWarning,
		domain.SeverityInfo, domain.SeverityInfo, domain.SeverityInfo,
	}
	for index, resource := range provider.resources {
		resourceID := resource.Resource.Ref.ID
		timestamp := observedAt.Add(-time.Duration(len(provider.resources)-index) * time.Minute)
		provider.logs = append(provider.logs, domain.LogRecord{
			ID: domain.LogEntryID(fmt.Sprintf("log_demo_%d", index+1)), ResourceID: resourceID,
			Stream: domain.LogStreamStdout, Sequence: uint64(index + 1), Severity: severities[index],
			Timestamp: timestamp, Message: messages[index], Synthetic: true,
		})
		environmentID := EnvironmentID
		provider.events = append(provider.events, domain.InfrastructureEvent{
			ID:       domain.EventID(fmt.Sprintf("event_demo_%d", index+1)),
			Provider: domain.ProviderDemo, SourceID: SourceID, EnvironmentID: &environmentID, ResourceID: &resourceID,
			Kind: "demo.health.changed", Severity: eventSeverities[index], ReasonCode: "demo.health.observed",
			Summary: messages[index], OccurredAt: timestamp, ObservedAt: observedAt, Synthetic: true,
		})
	}
	sort.Slice(provider.events, func(left, right int) bool {
		if provider.events[left].OccurredAt.Equal(provider.events[right].OccurredAt) {
			return provider.events[left].ID < provider.events[right].ID
		}
		return provider.events[left].OccurredAt.After(provider.events[right].OccurredAt)
	})
	environmentID := EnvironmentID
	resourceID := provider.resources[1].Resource.Ref.ID
	provider.alerts = []domain.Alert{{
		ID: "alert_demo_checkout", RuleID: "rule_demo_checkout_health", SourceID: SourceID,
		EnvironmentID: &environmentID, ResourceID: &resourceID,
		State: domain.AlertFiring, Severity: domain.SeverityError,
		EvidenceRange: domain.TimeRange{Start: observedAt.Add(-5 * time.Minute), End: observedAt.Add(-time.Minute)},
		StartedAt:     observedAt.Add(-5 * time.Minute), UpdatedAt: observedAt,
		SummaryCode: "demo.alert.checkout_unhealthy", Synthetic: true,
	}}
	return nil
}

func (provider *Provider) GetRelationships(ctx context.Context, query domain.TopologyQuery) (domain.RelationshipPage, error) {
	if err := provider.validateContext(ctx, query.Source); err != nil {
		return domain.RelationshipPage{}, invalid("demo.topology.read", err)
	}
	if err := query.Validate(); err != nil {
		return domain.RelationshipPage{}, invalid("demo.topology.read", err)
	}
	requested := make(map[domain.ResourceID]struct{}, len(query.Resources))
	for _, resource := range query.Resources {
		requested[resource.ID] = struct{}{}
	}
	items := make([]domain.Relationship, 0, len(provider.relationships))
	for _, relationship := range provider.relationships {
		_, fromRequested := requested[relationship.From]
		_, toRequested := requested[relationship.To]
		if len(requested) == 0 || fromRequested || toRequested {
			items = append(items, relationship)
		}
	}
	fingerprint := topologyFingerprint(query)
	offset, err := decodeCursor(query.Page.Cursor, fingerprint)
	if err != nil || offset > len(items) {
		return domain.RelationshipPage{}, invalid("demo.topology.read", err)
	}
	pageItems, page := paginate(items, offset, query.Page.Limit, fingerprint)
	result := domain.RelationshipPage{Status: availableStatus(), Relationships: pageItems, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.RelationshipPage{}, invalid("demo.topology.read", err)
	}
	return result, nil
}

func (provider *Provider) ReadLogs(ctx context.Context, query domain.LogReadQuery) (domain.LogPage, error) {
	if err := provider.validateContext(ctx, query.Source); err != nil {
		return domain.LogPage{}, invalid("demo.log.read", err)
	}
	if err := query.Validate(); err != nil {
		return domain.LogPage{}, invalid("demo.log.read", err)
	}
	targets := make(map[string]struct{}, len(query.Targets))
	for _, target := range query.Targets {
		targets[string(target.Resource.ID)+"\x00"+target.Container] = struct{}{}
	}
	items := make([]domain.LogRecord, 0, len(provider.logs))
	for _, record := range provider.logs {
		if _, exists := targets[string(record.ResourceID)+"\x00"+record.Container]; !exists {
			continue
		}
		if record.Timestamp.Before(query.Range.Start) || !record.Timestamp.Before(query.Range.End) {
			continue
		}
		items = append(items, record)
	}
	fingerprint := logFingerprint(query)
	offset, err := decodeCursor(query.Cursor, fingerprint)
	if err != nil || offset > len(items) {
		return domain.LogPage{}, invalid("demo.log.read", err)
	}
	end := min(offset+query.LineLimit, len(items))
	pageItems := slices.Clone(items[offset:end])
	remaining := query.ByteLimit
	for index := range pageItems {
		if len(pageItems[index].Message) > remaining {
			pageItems[index].Message = pageItems[index].Message[:remaining]
			pageItems[index].Truncated = true
		}
		remaining -= len(pageItems[index].Message)
		if remaining == 0 {
			end = offset + index + 1
			pageItems = pageItems[:index+1]
			break
		}
	}
	page := domain.PageInfo{Limit: query.LineLimit, HasMore: end < len(items)}
	if page.HasMore {
		page.NextCursor = encodeCursor(fingerprint, end)
	}
	result := domain.LogPage{Status: availableStatus(), Records: pageItems, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.LogPage{}, invalid("demo.log.read", err)
	}
	return result, nil
}

func (provider *Provider) GetCurrentMetrics(ctx context.Context, query domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
	if err := provider.validateContext(ctx, query.Source); err != nil {
		return domain.CurrentMetricResult{}, invalid("demo.metric.current", err)
	}
	if err := query.Validate(); err != nil {
		return domain.CurrentMetricResult{}, invalid("demo.metric.current", err)
	}
	result := domain.CurrentMetricResult{Status: availableStatus(), Sets: make([]domain.CurrentMetricSet, 0, len(query.Resources))}
	for resourceIndex, resource := range query.Resources {
		set := domain.CurrentMetricSet{ResourceID: resource.ID, Status: availableStatus(), Samples: make([]domain.MetricSample, 0, len(query.Names))}
		for metricIndex, name := range query.Names {
			set.Samples = append(set.Samples, domain.MetricSample{
				ID:         domain.MetricSampleID(fmt.Sprintf("metric_demo_%d_%d", resourceIndex+1, metricIndex+1)),
				ResourceID: resource.ID, Name: name, Unit: metricUnit(name),
				Value: metricValue(name, resourceIndex), SampledAt: observedAt, Interval: 30 * time.Second, Synthetic: true,
			})
		}
		result.Sets = append(result.Sets, set)
	}
	if err := result.ValidateFor(query); err != nil {
		return domain.CurrentMetricResult{}, invalid("demo.metric.current", err)
	}
	return result, nil
}

func (provider *Provider) GetMetricHistory(ctx context.Context, query domain.MetricHistoryQuery) (domain.MetricHistoryResult, error) {
	if err := provider.validateContext(ctx, query.Source); err != nil {
		return domain.MetricHistoryResult{}, invalid("demo.metric.history", err)
	}
	if err := query.Validate(); err != nil {
		return domain.MetricHistoryResult{}, invalid("demo.metric.history", err)
	}
	resources := slices.Clone(query.Resources)
	sort.Slice(resources, func(left, right int) bool { return resources[left].ID < resources[right].ID })
	names := slices.Clone(query.Names)
	sort.Slice(names, func(left, right int) bool { return names[left] < names[right] })
	samples := make([]domain.MetricSample, 0, 12*len(resources)*len(names))
	for point := 0; point < 12; point++ {
		sampledAt := observedAt.Add(time.Duration(point-11) * 5 * time.Minute)
		if sampledAt.Before(query.Range.Start) || !sampledAt.Before(query.Range.End) {
			continue
		}
		for resourceIndex, resource := range resources {
			for metricIndex, name := range names {
				value := metricValue(name, resourceIndex)
				if name == domain.MetricCPUUsageCores {
					value += float64((point+resourceIndex)%5) / 20
				}
				if name == domain.MetricMemoryUsageBytes {
					value += float64((point+resourceIndex)%4) * 8 * 1024 * 1024
				}
				samples = append(samples, domain.MetricSample{
					ID:         domain.MetricSampleID(fmt.Sprintf("metric_history_%02d_%02d_%02d", point, resourceIndex, metricIndex)),
					ResourceID: resource.ID, Name: name, Unit: metricUnit(name), Value: value,
					SampledAt: sampledAt, Interval: 5 * time.Minute, Synthetic: true,
				})
			}
		}
	}
	status := availableStatus()
	if len(samples) > query.PointLimit {
		samples = downsampleMetricHistory(samples, query.PointLimit)
		status.Completeness = domain.CompletenessPartial
		status.ReasonCode = "demo.metric.downsampled"
	}
	result := domain.MetricHistoryResult{Status: status, Samples: samples}
	if err := result.ValidateFor(query); err != nil {
		return domain.MetricHistoryResult{}, invalid("demo.metric.history", err)
	}
	return result, nil
}

func downsampleMetricHistory(samples []domain.MetricSample, limit int) []domain.MetricSample {
	if limit >= len(samples) {
		return slices.Clone(samples)
	}
	if limit == 1 {
		return []domain.MetricSample{samples[len(samples)-1]}
	}
	result := make([]domain.MetricSample, limit)
	for index := range result {
		result[index] = samples[index*(len(samples)-1)/(limit-1)]
	}
	return result
}

func (provider *Provider) ListEvents(ctx context.Context, query domain.EventQuery) (domain.EventPage, error) {
	if err := provider.validateContext(ctx, query.Source); err != nil {
		return domain.EventPage{}, invalid("demo.event.history", err)
	}
	if err := query.Validate(); err != nil {
		return domain.EventPage{}, invalid("demo.event.history", err)
	}
	resources := make(map[domain.ResourceID]struct{}, len(query.Resources))
	for _, resource := range query.Resources {
		resources[resource.ID] = struct{}{}
	}
	severities := make(map[domain.Severity]struct{}, len(query.Severities))
	for _, severity := range query.Severities {
		severities[severity] = struct{}{}
	}
	items := make([]domain.InfrastructureEvent, 0, len(provider.events))
	for _, event := range provider.events {
		if event.OccurredAt.Before(query.Range.Start) || !event.OccurredAt.Before(query.Range.End) {
			continue
		}
		if len(resources) > 0 {
			if event.ResourceID == nil {
				continue
			}
			if _, exists := resources[*event.ResourceID]; !exists {
				continue
			}
		}
		if len(severities) > 0 {
			if _, exists := severities[event.Severity]; !exists {
				continue
			}
		}
		items = append(items, event)
	}
	fingerprint := eventFingerprint(query)
	offset, err := decodeCursor(query.Page.Cursor, fingerprint)
	if err != nil || offset > len(items) {
		return domain.EventPage{}, invalid("demo.event.history", err)
	}
	pageItems, page := paginate(items, offset, query.Page.Limit, fingerprint)
	result := domain.EventPage{Status: availableStatus(), Events: pageItems, Page: page}
	if err := result.ValidateFor(query); err != nil {
		return domain.EventPage{}, invalid("demo.event.history", err)
	}
	return result, nil
}

func topologyFingerprint(query domain.TopologyQuery) string {
	ids := make([]string, len(query.Resources))
	for index, resource := range query.Resources {
		ids[index] = string(resource.ID)
	}
	return fingerprint("topology", strings.Join(ids, ","))
}

func logFingerprint(query domain.LogReadQuery) string {
	targets := make([]string, len(query.Targets))
	for index, target := range query.Targets {
		targets[index] = string(target.Resource.ID) + ":" + target.Container
	}
	return fingerprint(
		"logs", strings.Join(targets, ","), query.Range.Start.Format(time.RFC3339Nano),
		query.Range.End.Format(time.RFC3339Nano), fmt.Sprint(query.LineLimit), fmt.Sprint(query.ByteLimit),
	)
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
	return fingerprint(
		"events", strings.Join(resources, ","), strings.Join(severities, ","),
		query.Range.Start.Format(time.RFC3339Nano), query.Range.End.Format(time.RFC3339Nano),
	)
}

func metricUnit(name domain.MetricName) domain.MetricUnit {
	switch name {
	case domain.MetricCPUUsageCores, domain.MetricCPURequestCores, domain.MetricCPULimitCores:
		return domain.MetricUnitCores
	case domain.MetricRestartCount:
		return domain.MetricUnitCount
	default:
		return domain.MetricUnitBytes
	}
}

func metricValue(name domain.MetricName, index int) float64 {
	switch metricUnit(name) {
	case domain.MetricUnitCores:
		return 0.25 + float64(index)/10
	case domain.MetricUnitCount:
		return float64(index % 3)
	default:
		return float64((index + 1) * 64 * 1024 * 1024)
	}
}

var (
	_ domain.TopologyProvider      = (*Provider)(nil)
	_ domain.LogReadProvider       = (*Provider)(nil)
	_ domain.CurrentMetricProvider = (*Provider)(nil)
	_ domain.MetricHistoryProvider = (*Provider)(nil)
	_ domain.EventHistoryProvider  = (*Provider)(nil)
)

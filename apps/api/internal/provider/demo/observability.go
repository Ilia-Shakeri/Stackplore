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

func (provider *Provider) GetRelationships(ctx context.Context, query domain.TopologyQuery) (domain.RelationshipPage, error) {
	if err := provider.validateContext(ctx, query.Source); err != nil {
		return domain.RelationshipPage{}, invalid("demo.topology.read", err)
	}
	if err := query.Validate(); err != nil {
		return domain.RelationshipPage{}, invalid("demo.topology.read", err)
	}
	requested := make(map[domain.ResourceID]struct{}, len(query.Resources))
	for _, resource := range query.Resources {
		if !provider.knownResource(resource) {
			return domain.RelationshipPage{}, notFound("demo.topology.read")
		}
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
	pageItems = cloneRelationships(pageItems)
	result := domain.RelationshipPage{Status: provider.availableStatus(), Relationships: pageItems, Page: page}
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
		if !provider.knownResource(target.Resource) {
			return domain.LogPage{}, notFound("demo.log.read")
		}
		if target.Previous || target.Container != "" {
			status := provider.unsupportedStatus()
			status.ReasonCode = "demo.log.container_unsupported"
			return domain.LogPage{Status: status, Page: domain.PageInfo{Limit: query.LineLimit}}, nil
		}
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
	result := domain.LogPage{Status: provider.availableStatus(), Records: pageItems, Page: page}
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
	result := domain.CurrentMetricResult{Status: provider.availableStatus(), Sets: make([]domain.CurrentMetricSet, 0, len(query.Resources))}
	for _, resource := range query.Resources {
		if !provider.knownResource(resource) {
			return domain.CurrentMetricResult{}, notFound("demo.metric.current")
		}
		set := domain.CurrentMetricSet{ResourceID: resource.ID, Status: provider.availableStatus(), Samples: make([]domain.MetricSample, 0, len(query.Names))}
		for _, name := range query.Names {
			set.Samples = append(set.Samples, provider.metricSample(resource, name, provider.clock))
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
	for _, resource := range resources {
		if !provider.knownResource(resource) {
			return domain.MetricHistoryResult{}, notFound("demo.metric.history")
		}
	}
	for point := -11; point <= provider.step; point++ {
		sampledAt := observedAt.Add(time.Duration(point) * scenarioTick)
		if sampledAt.Before(query.Range.Start) || !sampledAt.Before(query.Range.End) {
			continue
		}
		for _, resource := range resources {
			for _, name := range names {
				samples = append(samples, provider.metricSample(resource, name, sampledAt))
			}
		}
	}
	status := provider.availableStatus()
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
		if !provider.knownResource(resource) {
			return domain.EventPage{}, notFound("demo.event.history")
		}
		resources[resource.ID] = struct{}{}
	}
	severities := make(map[domain.Severity]struct{}, len(query.Severities))
	for _, severity := range query.Severities {
		severities[severity] = struct{}{}
	}
	items := make([]domain.InfrastructureEvent, 0, len(provider.events))
	for _, event := range provider.events {
		if !query.MatchesTextAndEnvironment(event) {
			continue
		}
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
		items = append(items, cloneEvent(event))
	}
	fingerprint := eventFingerprint(query)
	offset, err := decodeCursor(query.Page.Cursor, fingerprint)
	if err != nil || offset > len(items) {
		return domain.EventPage{}, invalid("demo.event.history", err)
	}
	pageItems, page := paginate(items, offset, query.Page.Limit, fingerprint)
	result := domain.EventPage{Status: provider.availableStatus(), Events: pageItems, Page: page}
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
		string(query.EnvironmentID), query.Search,
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

var (
	_ domain.TopologyProvider      = (*Provider)(nil)
	_ domain.LogReadProvider       = (*Provider)(nil)
	_ domain.CurrentMetricProvider = (*Provider)(nil)
	_ domain.MetricHistoryProvider = (*Provider)(nil)
	_ domain.EventHistoryProvider  = (*Provider)(nil)
)

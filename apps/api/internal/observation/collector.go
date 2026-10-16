package observation

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type collection struct {
	inventory    domain.InventoryCapture
	capabilities domain.CapabilitySet
	metrics      []domain.MetricSample
	events       []domain.InfrastructureEvent
}

func (worker *Worker) collect(ctx context.Context, configuration domain.SourceConfiguration) (collection, error) {
	ref := domain.ProviderRef{Provider: configuration.Source.Provider, SourceID: configuration.Source.ID}
	live, err := worker.resolver.ResolveProvider(ctx, configuration)
	if err != nil || live == nil || live.Source().ID != ref.SourceID || live.Source().Provider != ref.Provider {
		return collection{}, errors.Join(ErrUnavailable, err)
	}
	capabilities, err := live.GetCapabilities(ctx, ref)
	if err != nil || capabilities.ValidateFor(ref) != nil {
		return collection{}, errors.Join(ErrUnavailable, err)
	}
	capabilities.Capabilities = slices.Clone(capabilities.Capabilities)
	var inventory domain.InventoryCapture
	if provider, supported := live.(domain.InventoryCaptureProvider); supported {
		inventory, err = provider.CaptureInventory(ctx)
		if err == nil {
			err = inventory.ValidateFor(ref)
		}
	} else {
		inventory, err = worker.collectPages(ctx, live, ref, &capabilities)
	}
	if err != nil {
		return collection{}, errors.Join(ErrUnavailable, err)
	}
	relationshipCount := len(inventory.Relationships)
	inventory = worker.boundInventory(inventory)
	if inventory.ValidateFor(ref) != nil {
		return collection{}, ErrUnavailable
	}
	if !inventory.Status.AllowsData() {
		return collection{inventory: inventory, capabilities: unavailableCapabilities(ref, inventory.Status)}, nil
	}
	setCapability(&capabilities, domain.CapabilityInventoryList, inventory.Status)
	setCapability(&capabilities, domain.CapabilityInventoryDetail, inventory.Status)
	if !inventory.Complete || relationshipCount > len(inventory.Relationships) {
		setCapability(&capabilities, domain.CapabilityTopology, partialStatus(inventory.Status, "observation.inventory_bound"))
	}
	result := collection{inventory: inventory, capabilities: capabilities}
	worker.collectMetrics(ctx, live, &result)
	worker.collectEvents(ctx, live, &result)
	return result, nil
}

func (worker *Worker) collectPages(ctx context.Context, provider LiveProvider, ref domain.ProviderRef, capabilities *domain.CapabilitySet) (domain.InventoryCapture, error) {
	capture := domain.InventoryCapture{Source: ref, Complete: true}
	page := domain.PageRequest{Limit: min(domain.MaxPageSize, worker.maximumItems)}
	seen := make(map[domain.PageCursor]struct{})
	for count := 0; count < worker.maximumPages; count++ {
		query := domain.EnvironmentQuery{Source: ref, Page: page}
		result, err := provider.ListEnvironments(ctx, query)
		if err != nil || result.ValidateFor(query) != nil {
			return domain.InventoryCapture{}, errors.Join(ErrUnavailable, err)
		}
		capture.Status = mergeStatus(capture.Status, result.Status)
		if !result.Status.AllowsData() {
			capture.Status, capture.Complete = result.Status, false
			return capture, nil
		}
		remaining := worker.maximumItems - len(capture.Environments)
		capture.Environments = append(capture.Environments, result.Environments[:min(remaining, len(result.Environments))]...)
		if len(result.Environments) > remaining || result.Page.HasMore && len(capture.Environments) >= worker.maximumItems || count+1 == worker.maximumPages && result.Page.HasMore {
			capture.Complete = false
			capture.Status = partialStatus(capture.Status, "observation.inventory_bound")
			break
		}
		if !result.Page.HasMore {
			break
		}
		if _, repeated := seen[result.Page.NextCursor]; repeated {
			return domain.InventoryCapture{}, ErrUnavailable
		}
		seen[result.Page.NextCursor] = struct{}{}
		page.Cursor = result.Page.NextCursor
	}
	page.Cursor = ""
	clear(seen)
	for count := 0; count < worker.maximumPages; count++ {
		query := domain.InventoryQuery{Source: ref, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: page}
		result, err := provider.ListResources(ctx, query)
		if err != nil || result.ValidateFor(query) != nil {
			return domain.InventoryCapture{}, errors.Join(ErrUnavailable, err)
		}
		capture.Status = mergeStatus(capture.Status, result.Status)
		if !result.Status.AllowsData() {
			return domain.InventoryCapture{Source: ref, Status: result.Status}, nil
		}
		remaining := worker.maximumItems - len(capture.Resources)
		capture.Resources = append(capture.Resources, result.Resources[:min(remaining, len(result.Resources))]...)
		if len(result.Resources) > remaining || result.Page.HasMore && len(capture.Resources) >= worker.maximumItems || count+1 == worker.maximumPages && result.Page.HasMore {
			capture.Complete = false
			capture.Status = partialStatus(capture.Status, "observation.inventory_bound")
			break
		}
		if !result.Page.HasMore {
			break
		}
		if _, repeated := seen[result.Page.NextCursor]; repeated {
			return domain.InventoryCapture{}, ErrUnavailable
		}
		seen[result.Page.NextCursor] = struct{}{}
		page.Cursor = result.Page.NextCursor
	}
	if capture.Status.State != domain.AvailabilityAvailable || capture.Status.Completeness != domain.CompletenessComplete {
		capture.Complete = false
	}
	healthStatus := availableStatus(capture.Status.ObservedAt)
	for offset := 0; offset < len(capture.Resources); offset += domain.MaxHealthTargets {
		refs := resourceRefs(capture.Resources[offset:min(offset+domain.MaxHealthTargets, len(capture.Resources))])
		query := domain.HealthQuery{Source: ref, Resources: refs}
		result, err := provider.GetHealth(ctx, query)
		if err != nil || result.ValidateFor(query) != nil {
			healthStatus = errorStatus(err, worker.time())
			continue
		}
		healthStatus = mergeStatus(healthStatus, result.Status)
		capture.Health = append(capture.Health, result.Snapshots...)
		capture.Status.ObservedAt = latest(capture.Status.ObservedAt, result.Status.ObservedAt)
	}
	if len(capture.Health) > 0 && len(capture.Health) < len(capture.Resources) {
		healthStatus = partialStatus(availableStatus(capture.Status.ObservedAt), "observation.partial_health")
	}
	setCapability(capabilities, domain.CapabilityHealth, healthStatus)
	page.Cursor = ""
	clear(seen)
	topologyStatus := availableStatus(capture.Status.ObservedAt)
	for count := 0; count < worker.maximumPages; count++ {
		query := domain.TopologyQuery{Source: ref, Page: page}
		result, err := provider.GetRelationships(ctx, query)
		if err != nil || result.ValidateFor(query) != nil || !result.Status.AllowsData() {
			topologyStatus = errorStatus(err, worker.time())
			if err == nil && result.Status.Validate() == nil && !result.Status.AllowsData() {
				topologyStatus = result.Status
			}
			break
		}
		topologyStatus = mergeStatus(topologyStatus, result.Status)
		remaining := worker.maximumItems - len(capture.Relationships)
		capture.Relationships = append(capture.Relationships, result.Relationships[:min(remaining, len(result.Relationships))]...)
		capture.Status.ObservedAt = latest(capture.Status.ObservedAt, result.Status.ObservedAt)
		if len(result.Relationships) > remaining || result.Page.HasMore && len(capture.Relationships) >= worker.maximumItems || count+1 == worker.maximumPages && result.Page.HasMore {
			topologyStatus = partialStatus(topologyStatus, "observation.topology_bound")
			break
		}
		if !result.Page.HasMore {
			break
		}
		if _, repeated := seen[result.Page.NextCursor]; repeated {
			return domain.InventoryCapture{}, ErrUnavailable
		}
		seen[result.Page.NextCursor] = struct{}{}
		page.Cursor = result.Page.NextCursor
	}
	if len(capture.Relationships) > 0 && !topologyStatus.AllowsData() {
		topologyStatus = partialStatus(availableStatus(capture.Status.ObservedAt), "observation.partial_topology")
	}
	setCapability(capabilities, domain.CapabilityTopology, topologyStatus)
	return capture, nil
}

func (worker *Worker) boundInventory(capture domain.InventoryCapture) domain.InventoryCapture {
	if len(capture.Environments) > worker.maximumItems || len(capture.Resources) > worker.maximumItems {
		capture.Complete = false
		capture.Status = partialStatus(capture.Status, "observation.inventory_bound")
	}
	capture.Environments = capture.Environments[:min(len(capture.Environments), worker.maximumItems)]
	environments := make(map[domain.EnvironmentID]struct{}, len(capture.Environments))
	for _, environment := range capture.Environments {
		environments[environment.ID] = struct{}{}
	}
	resources := make([]domain.ResourceSnapshot, 0, min(len(capture.Resources), worker.maximumItems))
	ids := make(map[domain.ResourceID]struct{}, cap(resources))
	for _, resource := range capture.Resources {
		if _, known := environments[resource.Resource.Ref.Identity.EnvironmentID]; !known || len(resources) >= worker.maximumItems {
			capture.Complete = false
			capture.Status = partialStatus(capture.Status, "observation.inventory_bound")
			continue
		}
		resources = append(resources, resource)
		ids[resource.Resource.Ref.ID] = struct{}{}
	}
	capture.Resources = resources
	health := make([]domain.HealthSnapshot, 0, min(len(capture.Health), worker.maximumItems))
	for _, snapshot := range capture.Health {
		if _, kept := ids[snapshot.ResourceID]; kept {
			health = append(health, snapshot)
		}
	}
	capture.Health = health
	replicas := make([]domain.WorkloadReplicaSnapshot, 0, min(len(capture.WorkloadReplicas), worker.maximumItems))
	for _, snapshot := range capture.WorkloadReplicas {
		if _, kept := ids[snapshot.Resource.ID]; kept {
			replicas = append(replicas, snapshot)
		}
	}
	capture.WorkloadReplicas = replicas
	relationships := make([]domain.Relationship, 0, min(len(capture.Relationships), worker.maximumItems))
	for _, relationship := range capture.Relationships {
		_, from := ids[relationship.From]
		_, to := ids[relationship.To]
		if from && to && len(relationships) < worker.maximumItems {
			relationships = append(relationships, relationship)
		}
	}
	capture.Relationships = relationships
	return capture
}

func (worker *Worker) collectMetrics(ctx context.Context, provider LiveProvider, collected *collection) {
	metrics, supported := provider.(domain.CurrentMetricProvider)
	if !supported {
		status := unavailableStatus(domain.AvailabilityUnsupported, worker.time(), "observation.metrics_unsupported")
		setCapability(&collected.capabilities, domain.CapabilityMetricCurrent, status)
		setCapability(&collected.capabilities, domain.CapabilityMetricHistory, status)
		return
	}
	names := []domain.MetricName{domain.MetricCPUUsageCores, domain.MetricMemoryUsageBytes, domain.MetricNetworkReceiveBytesTotal,
		domain.MetricNetworkSendBytesTotal, domain.MetricRestartCount, domain.MetricCPURequestCores, domain.MetricCPULimitCores,
		domain.MetricMemoryRequestBytes, domain.MetricMemoryLimitBytes}
	status := availableStatus(collected.inventory.Status.ObservedAt)
	setCount := 0
	missingSets := false
	states := make(map[domain.AvailabilityState]int)
	for offset := 0; offset < len(collected.inventory.Resources); offset += domain.MaxMetricTargets {
		query := domain.CurrentMetricQuery{Source: collected.inventory.Source,
			Resources: resourceRefs(collected.inventory.Resources[offset:min(offset+domain.MaxMetricTargets, len(collected.inventory.Resources))]), Names: names}
		result, err := metrics.GetCurrentMetrics(ctx, query)
		if err != nil || result.ValidateFor(query) != nil {
			status = errorStatus(err, worker.time())
			break
		}
		status = mergeStatus(status, result.Status)
		missingSets = missingSets || len(result.Sets) < len(query.Resources)
		for _, set := range result.Sets {
			setCount++
			states[set.Status.State]++
			if !set.Status.AllowsData() || set.Status.Completeness != domain.CompletenessComplete {
				status = partialStatus(status, "observation.partial_metrics")
			}
			remaining := worker.maximumMetricSamples - len(collected.metrics)
			collected.metrics = append(collected.metrics, set.Samples[:min(remaining, len(set.Samples))]...)
			if len(set.Samples) > remaining {
				status = partialStatus(status, "observation.metric_bound")
			}
		}
		if len(collected.metrics) >= worker.maximumMetricSamples {
			if offset+domain.MaxMetricTargets < len(collected.inventory.Resources) {
				status = partialStatus(status, "observation.metric_bound")
			}
			break
		}
	}
	if status.State == domain.AvailabilityPermissionDenied || states[domain.AvailabilityPermissionDenied] > 0 {
		status = unavailableStatus(domain.AvailabilityPermissionDenied, status.ObservedAt, "observation.permission_denied")
		collected.metrics = nil
	} else if len(collected.metrics) > 0 && (!status.AllowsData() || status.State == domain.AvailabilityStale || missingSets ||
		states[domain.AvailabilityStale]+states[domain.AvailabilityUnavailable]+states[domain.AvailabilityUnsupported] > 0) {
		// Per-resource failure state is not yet persisted. Apply the conservative
		// port-wide state so an older sample never looks fresh after a read gap.
		status = partialStatus(unavailableStatus(domain.AvailabilityStale, status.ObservedAt, "observation.partial_metrics"), "observation.partial_metrics")
	} else if len(collected.metrics) == 0 && setCount > 0 {
		if states[domain.AvailabilityUnsupported] == setCount {
			status = unavailableStatus(domain.AvailabilityUnsupported, status.ObservedAt, "observation.metrics_unsupported")
		} else if states[domain.AvailabilityUnavailable] > 0 {
			status = unavailableStatus(domain.AvailabilityUnavailable, status.ObservedAt, "observation.provider_unavailable")
		}
	}
	setCapability(&collected.capabilities, domain.CapabilityMetricCurrent, status)
	// Durable history inherits collection gaps and denials. The reader may expose
	// older samples as stale, but a fresh available label would hide that gap.
	setCapability(&collected.capabilities, domain.CapabilityMetricHistory, status)
}

func (worker *Worker) collectEvents(ctx context.Context, provider LiveProvider, collected *collection) {
	events, supported := provider.(domain.EventHistoryProvider)
	if !supported {
		setCapability(&collected.capabilities, domain.CapabilityEventHistory, unavailableStatus(domain.AvailabilityUnsupported, worker.time(), "observation.events_unsupported"))
		return
	}
	end := worker.time()
	if clock, ok := provider.(interface{ ObservationTime() time.Time }); ok {
		end = clock.ObservationTime()
	}
	end = end.UTC().Truncate(time.Microsecond)
	page := domain.PageRequest{Limit: min(domain.MaxPageSize, worker.maximumEvents)}
	seen := make(map[domain.PageCursor]struct{})
	status := availableStatus(end)
	resourceIDs := make(map[domain.ResourceID]struct{}, len(collected.inventory.Resources))
	environmentIDs := make(map[domain.EnvironmentID]struct{}, len(collected.inventory.Environments))
	for _, resource := range collected.inventory.Resources {
		resourceIDs[resource.Resource.Ref.ID] = struct{}{}
	}
	for _, environment := range collected.inventory.Environments {
		environmentIDs[environment.ID] = struct{}{}
	}
	for count := 0; count < worker.maximumPages; count++ {
		query := domain.EventQuery{Source: collected.inventory.Source,
			Range: domain.TimeRange{Start: end.Add(-worker.eventLookback), End: end}, Page: page}
		result, err := events.ListEvents(ctx, query)
		if err != nil || result.ValidateFor(query) != nil {
			status = errorStatus(err, worker.time())
			break
		}
		status = mergeStatus(status, result.Status)
		bounded := false
		for _, event := range result.Events {
			if event.ResourceID != nil {
				if _, known := resourceIDs[*event.ResourceID]; !known {
					status = partialStatus(status, "observation.events_scope_bound")
					continue
				}
			}
			if event.EnvironmentID != nil {
				if _, known := environmentIDs[*event.EnvironmentID]; !known {
					status = partialStatus(status, "observation.events_scope_bound")
					continue
				}
			}
			if len(collected.events) >= worker.maximumEvents {
				bounded = true
				break
			}
			collected.events = append(collected.events, event)
		}
		if bounded || result.Page.HasMore && len(collected.events) >= worker.maximumEvents || count+1 == worker.maximumPages && result.Page.HasMore {
			status = partialStatus(status, "observation.event_bound")
			break
		}
		if !result.Page.HasMore {
			break
		}
		if _, repeated := seen[result.Page.NextCursor]; repeated {
			status = unavailableStatus(domain.AvailabilityUnavailable, worker.time(), "observation.event_cursor_reused")
			break
		}
		seen[result.Page.NextCursor] = struct{}{}
		page.Cursor = result.Page.NextCursor
	}
	if len(collected.events) > 0 && !status.AllowsData() {
		status = partialStatus(availableStatus(status.ObservedAt), "observation.partial_events")
	}
	setCapability(&collected.capabilities, domain.CapabilityEventHistory, status)
}

func resourceRefs(resources []domain.ResourceSnapshot) []domain.ResourceRef {
	refs := make([]domain.ResourceRef, len(resources))
	for index := range resources {
		refs[index] = resources[index].Resource.Ref
	}
	return refs
}

func availableStatus(at time.Time) domain.ObservationStatus {
	return domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: at}
}

func unavailableStatus(state domain.AvailabilityState, at time.Time, reason domain.ReasonCode) domain.ObservationStatus {
	return domain.ObservationStatus{State: state, Completeness: domain.CompletenessComplete, ObservedAt: at, ReasonCode: reason}
}

func partialStatus(status domain.ObservationStatus, reason domain.ReasonCode) domain.ObservationStatus {
	if status.AllowsData() {
		status.Completeness, status.ReasonCode = domain.CompletenessPartial, reason
	}
	return status
}

func mergeStatus(left, right domain.ObservationStatus) domain.ObservationStatus {
	if left.ObservedAt.IsZero() {
		return right
	}
	if !right.AllowsData() {
		return right
	}
	if !left.AllowsData() {
		return left
	}
	left.ObservedAt = latest(left.ObservedAt, right.ObservedAt)
	if right.State == domain.AvailabilityStale {
		left.State, left.ReasonCode = domain.AvailabilityStale, right.ReasonCode
	}
	if right.Completeness == domain.CompletenessPartial {
		left.Completeness, left.ReasonCode = domain.CompletenessPartial, right.ReasonCode
	}
	return left
}

func latest(left, right time.Time) time.Time {
	if right.After(left) {
		return right
	}
	return left
}

func setCapability(set *domain.CapabilitySet, name domain.CapabilityName, status domain.ObservationStatus) {
	for index := range set.Capabilities {
		if set.Capabilities[index].Name == name {
			set.Capabilities[index].Status = status
			return
		}
	}
}

func unavailableCapabilities(source domain.ProviderRef, status domain.ObservationStatus) domain.CapabilitySet {
	names := domain.CapabilityNames()
	set := domain.CapabilitySet{Source: source, Capabilities: make([]domain.CapabilityStatus, len(names))}
	for index, name := range names {
		set.Capabilities[index] = domain.CapabilityStatus{Name: name, Status: status}
	}
	return set
}

func errorStatus(err error, at time.Time) domain.ObservationStatus {
	state, reason := domain.AvailabilityUnavailable, domain.ReasonCode("observation.provider_unavailable")
	if errors.Is(err, ErrProfileUnavailable) {
		return unavailableStatus(domain.AvailabilityUnavailable, at, "observation.profile_unavailable")
	}
	var providerError *domain.ProviderError
	if errors.As(err, &providerError) {
		switch providerError.Code {
		case domain.ProviderErrorPermissionDenied:
			state, reason = domain.AvailabilityPermissionDenied, "observation.permission_denied"
		case domain.ProviderErrorUnsupported:
			state, reason = domain.AvailabilityUnsupported, "observation.unsupported"
		case domain.ProviderErrorDeadlineExceeded:
			reason = "observation.deadline_exceeded"
		}
	} else if errors.Is(err, context.DeadlineExceeded) {
		reason = "observation.deadline_exceeded"
	}
	return unavailableStatus(state, at, reason)
}

package domain

import "context"

const MaxInventoryCaptureItems = 10_000

// InventoryCapture is one normalized provider observation. Each slice is bounded
// before validation allocates identity indexes. Optional observations are separate.
type InventoryCapture struct {
	Source           ProviderRef
	Status           ObservationStatus
	Environments     []Environment
	Resources        []ResourceSnapshot
	Health           []HealthSnapshot
	Relationships    []Relationship
	WorkloadReplicas []WorkloadReplicaSnapshot
	Complete         bool
}

type InventoryCaptureProvider interface {
	CaptureInventory(context.Context) (InventoryCapture, error)
}

func (capture InventoryCapture) ValidateFor(source ProviderRef) error {
	if source.Validate() != nil || capture.Source != source || capture.Status.Validate() != nil ||
		!repositoryTimeSupported(capture.Status.ObservedAt) || len(capture.Environments) > MaxInventoryCaptureItems ||
		len(capture.Resources) > MaxInventoryCaptureItems || len(capture.Health) > MaxInventoryCaptureItems ||
		len(capture.Relationships) > MaxInventoryCaptureItems || len(capture.WorkloadReplicas) > MaxInventoryCaptureItems {
		return ErrRepositoryInvalidInput
	}
	if capture.Complete && (capture.Status.State != AvailabilityAvailable || capture.Status.Completeness != CompletenessComplete) {
		return ErrRepositoryInvalidInput
	}
	if !capture.Status.AllowsData() && (len(capture.Environments)+len(capture.Resources)+len(capture.Health)+len(capture.Relationships)+len(capture.WorkloadReplicas) != 0 || capture.Complete) {
		return ErrRepositoryInvalidInput
	}
	synthetic := source.Provider == ProviderDemo
	environments := make(map[EnvironmentID]struct{}, len(capture.Environments))
	for _, environment := range capture.Environments {
		if environment.Validate() != nil || environment.SourceID != source.SourceID || environment.Provider != source.Provider ||
			environment.Synthetic != synthetic || environment.Status.ObservedAt.After(capture.Status.ObservedAt) ||
			!repositoryTimeSupported(environment.Status.ObservedAt) {
			return ErrRepositoryInvalidInput
		}
		if _, duplicate := environments[environment.ID]; duplicate {
			return ErrRepositoryInvalidInput
		}
		environments[environment.ID] = struct{}{}
	}
	resources := make(map[ResourceID]struct{}, len(capture.Resources))
	for _, resource := range capture.Resources {
		if resource.Validate() != nil || !source.Matches(resource.Resource.Ref) || resource.Resource.Synthetic != synthetic ||
			resource.Status.ObservedAt.After(capture.Status.ObservedAt) || !repositoryTimeSupported(resource.Status.ObservedAt) {
			return ErrRepositoryInvalidInput
		}
		if _, known := environments[resource.Resource.Ref.Identity.EnvironmentID]; !known {
			return ErrRepositoryInvalidInput
		}
		if _, duplicate := resources[resource.Resource.Ref.ID]; duplicate {
			return ErrRepositoryInvalidInput
		}
		resources[resource.Resource.Ref.ID] = struct{}{}
	}
	healthIDs := make(map[ResourceID]struct{}, len(capture.Health))
	for _, health := range capture.Health {
		if health.Validate() != nil || health.Status.ObservedAt.After(capture.Status.ObservedAt) || !repositoryTimeSupported(health.Status.ObservedAt) {
			return ErrRepositoryInvalidInput
		}
		if _, known := resources[health.ResourceID]; !known {
			return ErrRepositoryInvalidInput
		}
		if _, duplicate := healthIDs[health.ResourceID]; duplicate {
			return ErrRepositoryInvalidInput
		}
		for _, evidence := range health.Evidence {
			if evidence.Synthetic != synthetic || !repositoryTimeSupported(evidence.ObservedAt) {
				return ErrRepositoryInvalidInput
			}
		}
		healthIDs[health.ResourceID] = struct{}{}
	}
	relationshipIDs := make(map[RelationshipID]struct{}, len(capture.Relationships))
	for _, relationship := range capture.Relationships {
		if relationship.Validate() != nil || relationship.ObservedAt.After(capture.Status.ObservedAt) || !repositoryTimeSupported(relationship.ObservedAt) {
			return ErrRepositoryInvalidInput
		}
		if _, known := resources[relationship.From]; !known {
			return ErrRepositoryInvalidInput
		}
		if _, known := resources[relationship.To]; !known {
			return ErrRepositoryInvalidInput
		}
		if _, duplicate := relationshipIDs[relationship.ID]; duplicate {
			return ErrRepositoryInvalidInput
		}
		for _, evidence := range relationship.Evidence {
			if evidence.Provider != source.Provider || evidence.SourceID != source.SourceID || evidence.Synthetic != synthetic || !repositoryTimeSupported(evidence.ObservedAt) {
				return ErrRepositoryInvalidInput
			}
		}
		relationshipIDs[relationship.ID] = struct{}{}
	}
	replicas := make(map[ResourceID]struct{}, len(capture.WorkloadReplicas))
	for _, snapshot := range capture.WorkloadReplicas {
		if snapshot.ValidateFor(ResourceDetailQuery{Source: source, ResourceID: snapshot.Resource.ID}) != nil ||
			snapshot.Status.ObservedAt.After(capture.Status.ObservedAt) || !repositoryTimeSupported(snapshot.Status.ObservedAt) {
			return ErrRepositoryInvalidInput
		}
		if _, known := resources[snapshot.Resource.ID]; !known {
			return ErrRepositoryInvalidInput
		}
		if _, duplicate := replicas[snapshot.Resource.ID]; duplicate {
			return ErrRepositoryInvalidInput
		}
		replicas[snapshot.Resource.ID] = struct{}{}
	}
	return nil
}

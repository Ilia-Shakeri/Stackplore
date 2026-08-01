package domain

import (
	"fmt"
	"strings"
)

const MaxHealthTargets = 100

type HealthQuery struct {
	Source    ProviderRef
	Resources []ResourceRef
}

func (query HealthQuery) Validate() error {
	if err := query.Source.Validate(); err != nil {
		return err
	}
	return validateResourceRefs(query.Source, query.Resources, MaxHealthTargets, false)
}

type HealthResult struct {
	Status    ObservationStatus
	Snapshots []HealthSnapshot
}

func (result HealthResult) ValidateFor(query HealthQuery) error {
	if err := query.Validate(); err != nil {
		return err
	}
	if err := result.Status.Validate(); err != nil {
		return err
	}
	if len(result.Snapshots) > len(query.Resources) {
		return fmt.Errorf("health result exceeds requested resources")
	}
	if !result.Status.AllowsData() && len(result.Snapshots) != 0 {
		return fmt.Errorf("unavailable health result must not contain data")
	}
	if result.Status.AllowsData() && result.Status.Completeness == CompletenessComplete && len(result.Snapshots) != len(query.Resources) {
		return fmt.Errorf("complete health result must contain every requested resource")
	}
	requested := make(map[ResourceID]struct{}, len(query.Resources))
	for _, resource := range query.Resources {
		requested[resource.ID] = struct{}{}
	}
	seen := make(map[ResourceID]struct{}, len(result.Snapshots))
	for _, snapshot := range result.Snapshots {
		if err := snapshot.Validate(); err != nil {
			return err
		}
		if snapshot.Status.ObservedAt.After(result.Status.ObservedAt) {
			return fmt.Errorf("health snapshot cannot be newer than its result")
		}
		if _, exists := requested[snapshot.ResourceID]; !exists {
			return fmt.Errorf("health result contains an unrequested resource")
		}
		if _, exists := seen[snapshot.ResourceID]; exists {
			return fmt.Errorf("health result contains a duplicate resource")
		}
		seen[snapshot.ResourceID] = struct{}{}
	}
	return nil
}

type TopologyQuery struct {
	Source    ProviderRef
	Resources []ResourceRef
	Page      PageRequest
}

func (query TopologyQuery) Validate() error {
	if err := query.Source.Validate(); err != nil {
		return err
	}
	if err := validateResourceRefs(query.Source, query.Resources, MaxFilterValues, true); err != nil {
		return err
	}
	return query.Page.Validate()
}

type RelationshipPage struct {
	Status        ObservationStatus
	Relationships []Relationship
	Page          PageInfo
}

func (page RelationshipPage) ValidateFor(query TopologyQuery) error {
	if err := query.Validate(); err != nil {
		return err
	}
	if err := page.Status.Validate(); err != nil {
		return err
	}
	if err := page.Page.ValidateFor(query.Page); err != nil {
		return err
	}
	if len(page.Relationships) > query.Page.Limit {
		return fmt.Errorf("relationship page exceeds requested limit")
	}
	if err := validatePageAvailability(page.Status, page.Page, len(page.Relationships)); err != nil {
		return err
	}
	roots := make(map[ResourceID]struct{}, len(query.Resources))
	for _, resource := range query.Resources {
		roots[resource.ID] = struct{}{}
	}
	seen := make(map[RelationshipID]struct{}, len(page.Relationships))
	for index, relationship := range page.Relationships {
		if err := relationship.Validate(); err != nil {
			return err
		}
		if relationship.ObservedAt.After(page.Status.ObservedAt) {
			return fmt.Errorf("relationship cannot be newer than its page")
		}
		if len(roots) > 0 {
			_, fromRequested := roots[relationship.From]
			_, toRequested := roots[relationship.To]
			if !fromRequested && !toRequested {
				return fmt.Errorf("relationship page does not touch a requested resource")
			}
		}
		for _, evidence := range relationship.Evidence {
			if evidence.Provider != query.Source.Provider || evidence.SourceID != query.Source.SourceID {
				return fmt.Errorf("relationship evidence belongs to another source")
			}
		}
		if _, exists := seen[relationship.ID]; exists {
			return fmt.Errorf("relationship page contains a duplicate relationship")
		}
		if index > 0 && strings.Compare(string(page.Relationships[index-1].ID), string(relationship.ID)) > 0 {
			return fmt.Errorf("relationship page must use ascending ID order")
		}
		seen[relationship.ID] = struct{}{}
	}
	return nil
}

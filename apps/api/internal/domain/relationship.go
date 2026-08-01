package domain

import (
	"fmt"
	"math"
	"sort"
	"time"
)

const MaxRelationshipEvidence = 32

type RelationshipDirection string

const (
	RelationshipSourceToTarget RelationshipDirection = "source_to_target"
	RelationshipBidirectional  RelationshipDirection = "bidirectional"
	RelationshipUndirected     RelationshipDirection = "undirected"
)

func (direction RelationshipDirection) Validate() error {
	switch direction {
	case RelationshipSourceToTarget, RelationshipBidirectional, RelationshipUndirected:
		return nil
	default:
		return fmt.Errorf("unknown relationship direction %q", direction)
	}
}

type RelationshipKind string

const (
	RelationshipOwns       RelationshipKind = "owns"
	RelationshipSelects    RelationshipKind = "selects"
	RelationshipRoutesTo   RelationshipKind = "routes_to"
	RelationshipDependsOn  RelationshipKind = "depends_on"
	RelationshipMounts     RelationshipKind = "mounts"
	RelationshipReferences RelationshipKind = "references"
	RelationshipMemberOf   RelationshipKind = "member_of"
)

func (kind RelationshipKind) Validate() error {
	switch kind {
	case RelationshipOwns, RelationshipSelects, RelationshipRoutesTo, RelationshipDependsOn, RelationshipMounts, RelationshipReferences, RelationshipMemberOf:
		return nil
	default:
		return fmt.Errorf("unknown relationship kind %q", kind)
	}
}

type RelationshipProvenance string

const (
	ProvenanceOwnership          RelationshipProvenance = "ownership"
	ProvenanceSelector           RelationshipProvenance = "selector"
	ProvenanceRoute              RelationshipProvenance = "route"
	ProvenanceAnnotation         RelationshipProvenance = "annotation"
	ProvenanceDeclaredDependency RelationshipProvenance = "declared_dependency"
	ProvenanceStorageReference   RelationshipProvenance = "storage_reference"
	ProvenanceNetworkMembership  RelationshipProvenance = "network_membership"
	ProvenanceEndpoint           RelationshipProvenance = "endpoint"
)

func (provenance RelationshipProvenance) Validate() error {
	switch provenance {
	case ProvenanceOwnership, ProvenanceSelector, ProvenanceRoute, ProvenanceAnnotation, ProvenanceDeclaredDependency, ProvenanceStorageReference, ProvenanceNetworkMembership, ProvenanceEndpoint:
		return nil
	default:
		return fmt.Errorf("unknown relationship provenance %q", provenance)
	}
}

type EvidenceCertainty string

const (
	EvidenceExplicit EvidenceCertainty = "explicit"
	EvidenceInferred EvidenceCertainty = "inferred"
)

func (certainty EvidenceCertainty) Validate() error {
	switch certainty {
	case EvidenceExplicit, EvidenceInferred:
		return nil
	default:
		return fmt.Errorf("unknown evidence certainty %q", certainty)
	}
}

type RelationshipEvidence struct {
	ID         EvidenceID
	Provider   ProviderKind
	SourceID   SourceID
	Provenance RelationshipProvenance
	Certainty  EvidenceCertainty
	Confidence float64
	ObservedAt time.Time
	Synthetic  bool
}

func (evidence RelationshipEvidence) Validate() error {
	if err := evidence.ID.Validate(); err != nil {
		return err
	}
	if err := evidence.Provider.Validate(); err != nil {
		return err
	}
	if err := evidence.SourceID.Validate(); err != nil {
		return err
	}
	if err := evidence.Provenance.Validate(); err != nil {
		return err
	}
	if err := evidence.Certainty.Validate(); err != nil {
		return err
	}
	if math.IsNaN(evidence.Confidence) || math.IsInf(evidence.Confidence, 0) || evidence.Confidence < 0 || evidence.Confidence > 1 {
		return fmt.Errorf("relationship confidence must be finite and between 0 and 1")
	}
	return validateUTCTime("relationship evidence time", evidence.ObservedAt)
}

type Relationship struct {
	ID         RelationshipID
	From       ResourceID
	To         ResourceID
	Kind       RelationshipKind
	Direction  RelationshipDirection
	Evidence   []RelationshipEvidence
	ObservedAt time.Time
}

func NewRelationship(from, to ResourceID, kind RelationshipKind, direction RelationshipDirection, evidence []RelationshipEvidence) (Relationship, error) {
	id, err := NewRelationshipID(RelationshipIdentity{From: from, To: to, Kind: kind, Direction: direction})
	if err != nil {
		return Relationship{}, err
	}
	coalesced, err := CoalesceRelationshipEvidence(evidence)
	if err != nil {
		return Relationship{}, err
	}
	var observedAt time.Time
	for _, item := range coalesced {
		if item.ObservedAt.After(observedAt) {
			observedAt = item.ObservedAt
		}
	}
	from, to = canonicalRelationshipEndpoints(from, to, direction)
	relationship := Relationship{ID: id, From: from, To: to, Kind: kind, Direction: direction, Evidence: coalesced, ObservedAt: observedAt}
	if err := relationship.Validate(); err != nil {
		return Relationship{}, err
	}
	return relationship, nil
}

func (relationship Relationship) Validate() error {
	if err := relationship.ID.Validate(); err != nil {
		return err
	}
	expected, err := NewRelationshipID(RelationshipIdentity{
		From: relationship.From, To: relationship.To, Kind: relationship.Kind, Direction: relationship.Direction,
	})
	if err != nil {
		return err
	}
	if relationship.ID != expected {
		return fmt.Errorf("relationship ID does not match identity")
	}
	canonicalFrom, canonicalTo := canonicalRelationshipEndpoints(relationship.From, relationship.To, relationship.Direction)
	if relationship.From != canonicalFrom || relationship.To != canonicalTo {
		return fmt.Errorf("symmetric relationship endpoints are not canonical")
	}
	if len(relationship.Evidence) == 0 {
		return fmt.Errorf("relationship requires evidence")
	}
	if len(relationship.Evidence) > MaxRelationshipEvidence {
		return fmt.Errorf("relationship evidence exceeds %d items", MaxRelationshipEvidence)
	}
	if err := validateUTCTime("relationship observation time", relationship.ObservedAt); err != nil {
		return err
	}
	seen := make(map[EvidenceID]struct{}, len(relationship.Evidence))
	latest := time.Time{}
	for _, evidence := range relationship.Evidence {
		if err := evidence.Validate(); err != nil {
			return err
		}
		if _, exists := seen[evidence.ID]; exists {
			return fmt.Errorf("duplicate relationship evidence ID %q", evidence.ID)
		}
		seen[evidence.ID] = struct{}{}
		if evidence.ObservedAt.After(latest) {
			latest = evidence.ObservedAt
		}
	}
	if !relationship.ObservedAt.Equal(latest) {
		return fmt.Errorf("relationship observation time must match newest evidence")
	}
	return nil
}

func CoalesceRelationshipEvidence(items []RelationshipEvidence) ([]RelationshipEvidence, error) {
	if len(items) > MaxRelationshipEvidence {
		return nil, fmt.Errorf("relationship evidence exceeds %d items", MaxRelationshipEvidence)
	}
	byID := make(map[EvidenceID]RelationshipEvidence, len(items))
	for _, item := range items {
		if err := item.Validate(); err != nil {
			return nil, err
		}
		existing, exists := byID[item.ID]
		if !exists {
			byID[item.ID] = item
			continue
		}
		if existing.Provider != item.Provider || existing.SourceID != item.SourceID || existing.Provenance != item.Provenance || existing.Certainty != item.Certainty || existing.Synthetic != item.Synthetic {
			return nil, fmt.Errorf("relationship evidence ID %q has conflicting provenance", item.ID)
		}
		if item.ObservedAt.After(existing.ObservedAt) {
			existing = item
		} else if item.ObservedAt.Equal(existing.ObservedAt) && item.Confidence != existing.Confidence {
			return nil, fmt.Errorf("relationship evidence ID %q has conflicting values at one observation time", item.ID)
		}
		byID[item.ID] = existing
	}
	result := make([]RelationshipEvidence, 0, len(byID))
	for _, item := range byID {
		result = append(result, item)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result, nil
}

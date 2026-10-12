package domain

import "fmt"

type InfrastructureSource struct {
	ID        SourceID
	Provider  ProviderKind
	Name      string
	Synthetic bool
}

type ProviderRef struct {
	Provider ProviderKind
	SourceID SourceID
}

func (ref ProviderRef) Validate() error {
	if err := ref.Provider.Validate(); err != nil {
		return err
	}
	return ref.SourceID.Validate()
}

func (ref ProviderRef) Matches(resource ResourceRef) bool {
	return resource.Identity.Provider == ref.Provider && resource.Identity.SourceID == ref.SourceID
}

func (source InfrastructureSource) Validate() error {
	if err := source.ID.Validate(); err != nil {
		return err
	}
	if err := source.Provider.Validate(); err != nil {
		return err
	}
	return validateDisplayText("source name", source.Name, maxDisplayNameLength, false)
}

type Environment struct {
	ID        EnvironmentID
	SourceID  SourceID
	Provider  ProviderKind
	Name      string
	Status    ObservationStatus
	Synthetic bool
}

func (environment Environment) Validate() error {
	if err := environment.ID.Validate(); err != nil {
		return err
	}
	if err := environment.SourceID.Validate(); err != nil {
		return err
	}
	if err := environment.Provider.Validate(); err != nil {
		return err
	}
	if err := environment.Status.Validate(); err != nil {
		return err
	}
	return validateDisplayText("environment name", environment.Name, maxDisplayNameLength, false)
}

type ResourceRef struct {
	ID       ResourceID
	Identity ResourceIdentity
}

func NewResourceRef(identity ResourceIdentity) (ResourceRef, error) {
	id, err := NewResourceID(identity)
	if err != nil {
		return ResourceRef{}, err
	}
	return ResourceRef{ID: id, Identity: identity}, nil
}

func (ref ResourceRef) Validate() error {
	if err := ref.ID.Validate(); err != nil {
		return err
	}
	expected, err := NewResourceID(ref.Identity)
	if err != nil {
		return err
	}
	if ref.ID != expected {
		return fmt.Errorf("resource ID does not match identity")
	}
	return nil
}

type Resource struct {
	Ref         ResourceRef
	DisplayName string
	Version     string
	Synthetic   bool
}

func (resource Resource) Validate() error {
	if err := resource.Ref.Validate(); err != nil {
		return err
	}
	if resource.Version != "" {
		if err := ValidateOperationVersion(resource.Version); err != nil {
			return err
		}
	}
	return validateDisplayText("resource display name", resource.DisplayName, maxDisplayNameLength, false)
}

type ResourceSnapshot struct {
	Resource Resource
	Status   ObservationStatus
}

func (snapshot ResourceSnapshot) Validate() error {
	if err := snapshot.Resource.Validate(); err != nil {
		return err
	}
	return snapshot.Status.Validate()
}

func (snapshot ResourceSnapshot) ValidateFor(query ResourceDetailQuery) error {
	if err := query.Validate(); err != nil {
		return err
	}
	if err := snapshot.Validate(); err != nil {
		return err
	}
	if !query.Source.Matches(snapshot.Resource.Ref) {
		return fmt.Errorf("resource detail belongs to another source")
	}
	if snapshot.Resource.Ref.ID != query.ResourceID {
		return fmt.Errorf("resource detail does not match requested resource")
	}
	return nil
}

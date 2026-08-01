package domain

import (
	"fmt"
	"strings"
)

type ResourceSortField string

const (
	ResourceSortName       ResourceSortField = "name"
	ResourceSortKind       ResourceSortField = "kind"
	ResourceSortObservedAt ResourceSortField = "observed_at"
)

func (field ResourceSortField) Validate() error {
	switch field {
	case ResourceSortName, ResourceSortKind, ResourceSortObservedAt:
		return nil
	default:
		return fmt.Errorf("unknown resource sort field %q", field)
	}
}

type ResourceFilter struct {
	Search string
	Kinds  []ResourceKind
	Scopes []Scope
}

// Matches applies the fixed inventory search law to safe public resource fields.
func (filter ResourceFilter) Matches(resource Resource) bool {
	if filter.Search == "" {
		return true
	}
	needle := strings.ToLower(filter.Search)
	candidates := [...]string{
		resource.DisplayName,
		string(resource.Ref.ID),
		string(resource.Ref.Identity.EnvironmentID),
		string(resource.Ref.Identity.Kind),
		string(resource.Ref.Identity.Scope.Kind),
		resource.Ref.Identity.Scope.Name,
	}
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate), needle) {
			return true
		}
	}
	return false
}

func (filter ResourceFilter) Validate() error {
	if err := validateDisplayText("resource search", filter.Search, MaxSearchLength, true); err != nil {
		return err
	}
	if len(filter.Kinds) > MaxFilterValues {
		return fmt.Errorf("resource kind filter exceeds %d values", MaxFilterValues)
	}
	seenKinds := make(map[ResourceKind]struct{}, len(filter.Kinds))
	for _, kind := range filter.Kinds {
		if err := kind.Validate(); err != nil {
			return err
		}
		if _, exists := seenKinds[kind]; exists {
			return fmt.Errorf("resource kind filter contains a duplicate")
		}
		seenKinds[kind] = struct{}{}
	}
	if len(filter.Scopes) > MaxFilterValues {
		return fmt.Errorf("scope filter exceeds %d values", MaxFilterValues)
	}
	seenScopes := make(map[Scope]struct{}, len(filter.Scopes))
	for _, scope := range filter.Scopes {
		if err := scope.Validate(); err != nil {
			return err
		}
		if _, exists := seenScopes[scope]; exists {
			return fmt.Errorf("scope filter contains a duplicate")
		}
		seenScopes[scope] = struct{}{}
	}
	return nil
}

type InventoryQuery struct {
	Source        ProviderRef
	Filter        ResourceFilter
	SortField     ResourceSortField
	SortDirection SortDirection
	Page          PageRequest
}

type ResourceDetailQuery struct {
	Source     ProviderRef
	ResourceID ResourceID
}

func (query ResourceDetailQuery) Validate() error {
	if err := query.Source.Validate(); err != nil {
		return err
	}
	if err := query.ResourceID.Validate(); err != nil {
		return err
	}
	return nil
}

type EnvironmentQuery struct {
	Source ProviderRef
	Page   PageRequest
}

func (query EnvironmentQuery) Validate() error {
	if err := query.Source.Validate(); err != nil {
		return err
	}
	return query.Page.Validate()
}

type EnvironmentPage struct {
	Status       ObservationStatus
	Environments []Environment
	Page         PageInfo
}

func (page EnvironmentPage) ValidateFor(query EnvironmentQuery) error {
	if err := query.Validate(); err != nil {
		return err
	}
	if err := page.Status.Validate(); err != nil {
		return err
	}
	if err := page.Page.ValidateFor(query.Page); err != nil {
		return err
	}
	if len(page.Environments) > query.Page.Limit {
		return fmt.Errorf("environment page exceeds requested limit")
	}
	if err := validatePageAvailability(page.Status, page.Page, len(page.Environments)); err != nil {
		return err
	}
	seen := make(map[EnvironmentID]struct{}, len(page.Environments))
	for index, environment := range page.Environments {
		if err := environment.Validate(); err != nil {
			return err
		}
		if environment.Status.ObservedAt.After(page.Status.ObservedAt) {
			return fmt.Errorf("environment cannot be newer than its page")
		}
		if environment.SourceID != query.Source.SourceID || environment.Provider != query.Source.Provider {
			return fmt.Errorf("environment page contains another source")
		}
		if _, exists := seen[environment.ID]; exists {
			return fmt.Errorf("environment page contains a duplicate")
		}
		if index > 0 && page.Environments[index-1].ID > environment.ID {
			return fmt.Errorf("environment page must use ascending ID order")
		}
		seen[environment.ID] = struct{}{}
	}
	return nil
}

func (query InventoryQuery) Validate() error {
	if err := query.Source.Validate(); err != nil {
		return err
	}
	if err := query.Filter.Validate(); err != nil {
		return err
	}
	if err := query.SortField.Validate(); err != nil {
		return err
	}
	if err := query.SortDirection.Validate(); err != nil {
		return err
	}
	return query.Page.Validate()
}

type ResourcePage struct {
	Status    ObservationStatus
	Resources []ResourceSnapshot
	Page      PageInfo
}

func (page ResourcePage) ValidateFor(query InventoryQuery) error {
	if err := query.Validate(); err != nil {
		return err
	}
	if err := page.Status.Validate(); err != nil {
		return err
	}
	if err := page.Page.ValidateFor(query.Page); err != nil {
		return err
	}
	if len(page.Resources) > query.Page.Limit {
		return fmt.Errorf("resource page exceeds requested limit")
	}
	if err := validatePageAvailability(page.Status, page.Page, len(page.Resources)); err != nil {
		return err
	}
	kinds := make(map[ResourceKind]struct{}, len(query.Filter.Kinds))
	for _, kind := range query.Filter.Kinds {
		kinds[kind] = struct{}{}
	}
	scopes := make(map[Scope]struct{}, len(query.Filter.Scopes))
	for _, scope := range query.Filter.Scopes {
		scopes[scope] = struct{}{}
	}
	seen := make(map[ResourceID]struct{}, len(page.Resources))
	for index, snapshot := range page.Resources {
		if err := snapshot.Validate(); err != nil {
			return err
		}
		if snapshot.Status.ObservedAt.After(page.Status.ObservedAt) {
			return fmt.Errorf("resource cannot be newer than its page")
		}
		if !query.Source.Matches(snapshot.Resource.Ref) {
			return fmt.Errorf("resource page contains a resource from another source")
		}
		if !query.Filter.Matches(snapshot.Resource) {
			return fmt.Errorf("resource page contains a resource outside the requested search")
		}
		if len(kinds) > 0 {
			if _, exists := kinds[snapshot.Resource.Ref.Identity.Kind]; !exists {
				return fmt.Errorf("resource page contains an unrequested kind")
			}
		}
		if len(scopes) > 0 {
			if _, exists := scopes[snapshot.Resource.Ref.Identity.Scope]; !exists {
				return fmt.Errorf("resource page contains an unrequested scope")
			}
		}
		if _, exists := seen[snapshot.Resource.Ref.ID]; exists {
			return fmt.Errorf("resource page contains a duplicate resource")
		}
		if index > 0 && compareResourceSnapshots(page.Resources[index-1], snapshot, query.SortField, query.SortDirection) > 0 {
			return fmt.Errorf("resource page does not match requested sort")
		}
		seen[snapshot.Resource.Ref.ID] = struct{}{}
	}
	return nil
}

func compareResourceSnapshots(left ResourceSnapshot, right ResourceSnapshot, field ResourceSortField, direction SortDirection) int {
	comparison := 0
	switch field {
	case ResourceSortName:
		comparison = strings.Compare(left.Resource.DisplayName, right.Resource.DisplayName)
	case ResourceSortKind:
		comparison = strings.Compare(string(left.Resource.Ref.Identity.Kind), string(right.Resource.Ref.Identity.Kind))
	case ResourceSortObservedAt:
		comparison = left.Status.ObservedAt.Compare(right.Status.ObservedAt)
	}
	if comparison != 0 {
		if direction == SortDescending {
			return -comparison
		}
		return comparison
	}
	return strings.Compare(string(left.Resource.Ref.ID), string(right.Resource.Ref.ID))
}

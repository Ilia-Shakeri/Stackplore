package domain

import "fmt"

const MaxInventoryCoverageItems = 2048

type InventoryScopeMode string

const (
	InventoryAllScopes  InventoryScopeMode = "all_scopes"
	InventoryExactScope InventoryScopeMode = "exact_scope"
)

// InventoryCoverage describes one trusted collector partition, including empty
// lists. Only an available, complete partition can retire unseen identities.
type InventoryCoverage struct {
	Source        ProviderRef
	EnvironmentID EnvironmentID
	Kind          ResourceKind
	ScopeMode     InventoryScopeMode
	Scope         Scope
	Status        ObservationStatus
}

func (coverage InventoryCoverage) ValidateFor(source ProviderRef) error {
	if source.Validate() != nil || coverage.Source != source || coverage.EnvironmentID.Validate() != nil ||
		coverage.Kind.Validate() != nil || coverage.Status.Validate() != nil || !repositoryTimeSupported(coverage.Status.ObservedAt) {
		return ErrRepositoryInvalidInput
	}
	if coverage.ScopeMode == InventoryAllScopes && coverage.Scope == (Scope{}) ||
		coverage.ScopeMode == InventoryExactScope && coverage.Scope.Validate() == nil {
		return nil
	}
	return ErrRepositoryInvalidInput
}

func (coverage InventoryCoverage) Matches(resource ResourceRef) bool {
	return coverage.Source.Matches(resource) && coverage.EnvironmentID == resource.Identity.EnvironmentID &&
		coverage.Kind == resource.Identity.Kind && (coverage.ScopeMode == InventoryAllScopes || coverage.ScopeMode == InventoryExactScope && coverage.Scope == resource.Identity.Scope)
}

func (coverage InventoryCoverage) Complete() bool {
	return coverage.Status.State == AvailabilityAvailable && coverage.Status.Completeness == CompletenessComplete
}

func ValidateInventoryCoverage(source ProviderRef, coverage []InventoryCoverage) error {
	if len(coverage) > MaxInventoryCoverageItems {
		return ErrRepositoryInvalidInput
	}
	type group struct {
		environment EnvironmentID
		kind        ResourceKind
	}
	type scopes struct {
		all   bool
		exact map[Scope]struct{}
	}
	groups := make(map[group]*scopes, len(coverage))
	for _, item := range coverage {
		if item.ValidateFor(source) != nil {
			return ErrRepositoryInvalidInput
		}
		key := group{item.EnvironmentID, item.Kind}
		entry := groups[key]
		if entry == nil {
			entry = &scopes{exact: make(map[Scope]struct{})}
			groups[key] = entry
		}
		if entry.all || item.ScopeMode == InventoryAllScopes && len(entry.exact) > 0 {
			return fmt.Errorf("inventory coverage overlaps")
		}
		if item.ScopeMode == InventoryAllScopes {
			entry.all = true
		} else {
			if _, duplicate := entry.exact[item.Scope]; duplicate {
				return fmt.Errorf("inventory coverage repeats")
			}
			entry.exact[item.Scope] = struct{}{}
		}
	}
	return nil
}

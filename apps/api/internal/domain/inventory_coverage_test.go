package domain

import (
	"testing"
)

func TestInventoryCoverageRejectsOverlapAndDistinguishesEmptyCompletePartition(t *testing.T) {
	source := fixtureProvider()
	item := InventoryCoverage{Source: source, EnvironmentID: "environment_alpha", Kind: "demo.service", ScopeMode: InventoryExactScope,
		Scope: Scope{Kind: ScopeNamespace, Name: "allowed"}, Status: fixtureStatus()}
	if err := ValidateInventoryCoverage(source, []InventoryCoverage{item}); err != nil || !item.Complete() {
		t.Fatalf("empty complete partition must carry deletion proof: %v", err)
	}
	for _, mode := range []string{"duplicate", "all_then_exact", "exact_then_all", "foreign_source", "scope_on_all", "invalid_status"} {
		t.Run(mode, func(t *testing.T) {
			items := []InventoryCoverage{item, item}
			switch mode {
			case "all_then_exact":
				items[0].ScopeMode, items[0].Scope = InventoryAllScopes, Scope{}
			case "exact_then_all":
				items[1].ScopeMode, items[1].Scope = InventoryAllScopes, Scope{}
			case "foreign_source":
				items = items[:1]
				items[0].Source.SourceID = "source_other"
			case "scope_on_all":
				items = items[:1]
				items[0].ScopeMode = InventoryAllScopes
			case "invalid_status":
				items = items[:1]
				items[0].Status.Completeness = "invented"
			}
			if ValidateInventoryCoverage(source, items) == nil {
				t.Fatal("unsafe coverage accepted")
			}
		})
	}
	second := item
	second.Scope.Name = "denied"
	second.Status.State, second.Status.ReasonCode = AvailabilityPermissionDenied, "source.permission_denied"
	if ValidateInventoryCoverage(source, []InventoryCoverage{item, second}) != nil || second.Complete() {
		t.Fatal("independent denied scope must not complete or block valid scope")
	}
}

package domain

import (
	"math"
	"testing"
	"time"
)

func TestInventorySummaryRejectsInconsistentAndUnsafeCounters(t *testing.T) {
	status := ObservationStatus{State: AvailabilityAvailable, Completeness: CompletenessComplete, ObservedAt: time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)}
	valid := InventorySummary{Status: status, Resources: 223, Healthy: 3, Unknown: 220}
	if valid.Validate() != nil {
		t.Fatal("complete active resource count rejected")
	}
	for _, invalid := range []InventorySummary{
		{Status: status, Resources: 223, Healthy: 3, Unknown: 219},
		{Status: status, Resources: 223, Healthy: math.MaxInt64, Degraded: math.MaxInt64, Unhealthy: 225},
		{Status: status, Resources: 9007199254740992, Unknown: 9007199254740992},
		{Status: status, Resources: 1, Healthy: 2, Unknown: -1},
	} {
		if invalid.Validate() == nil {
			t.Fatal("unsafe resource summary accepted")
		}
	}
	valid.Status.State = AvailabilityPermissionDenied
	valid.Status.ReasonCode = "provider.permission_denied"
	if valid.Validate() == nil {
		t.Fatal("denied inventory exposed cached resource counters")
	}
}

func TestInventoryHealthClassificationKeepsIncompleteEvidenceUnknown(t *testing.T) {
	status := ObservationStatus{State: AvailabilityAvailable, Completeness: CompletenessComplete, ObservedAt: time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)}
	value := HealthSnapshot{State: HealthHealthy, Status: status}
	if value.InventoryState() != HealthHealthy {
		t.Fatal("complete health evidence discarded")
	}
	value.Status.Completeness = CompletenessPartial
	value.Status.ReasonCode = "health.partial"
	if value.InventoryState() != HealthUnknown {
		t.Fatal("partial health claimed healthy")
	}
	value.Status.Completeness = CompletenessComplete
	value.Status.State = AvailabilityPermissionDenied
	value.Status.ReasonCode = "health.permission_denied"
	if value.InventoryState() != HealthUnknown {
		t.Fatal("denied health claimed healthy")
	}
	if (HealthSnapshot{}).InventoryState() != HealthUnknown {
		t.Fatal("missing health claimed known")
	}
}

package domain

import (
	"testing"
	"time"
)

func fixtureTime() time.Time {
	return time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
}

func fixtureProvider() ProviderRef {
	return ProviderRef{Provider: ProviderDemo, SourceID: "source_demo"}
}

func fixtureIdentity(nativeID string) ResourceIdentity {
	return ResourceIdentity{
		Provider:      ProviderDemo,
		SourceID:      "source_demo",
		EnvironmentID: "env_primary",
		Kind:          "demo.service",
		Scope:         Scope{Kind: ScopeGlobal},
		nativeID:      NativeResourceID(nativeID),
	}
}

func fixtureResourceRef(t *testing.T, nativeID string) ResourceRef {
	t.Helper()
	ref, err := NewResourceRef(fixtureIdentity(nativeID))
	if err != nil {
		t.Fatalf("NewResourceRef() error = %v", err)
	}
	return ref
}

func fixtureStatus() ObservationStatus {
	return ObservationStatus{
		State:        AvailabilityAvailable,
		Completeness: CompletenessComplete,
		ObservedAt:   fixtureTime(),
	}
}

func fixturePage(limit int) PageInfo {
	return PageInfo{Limit: limit}
}

func requireInvalid(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

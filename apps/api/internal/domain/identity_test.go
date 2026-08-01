package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResourceIdentityIsDeterministicAndVersioned(t *testing.T) {
	identity := fixtureIdentity("service|alpha")
	first, err := NewResourceID(identity)
	if err != nil {
		t.Fatalf("NewResourceID() error = %v", err)
	}
	second, err := NewResourceID(identity)
	if err != nil {
		t.Fatalf("NewResourceID() second error = %v", err)
	}
	if first != second {
		t.Fatalf("resource IDs differ: %q != %q", first, second)
	}
	const expected = ResourceID("r1_MMKUw4Iq3iNF2nBIFD2VSt-ZsNxuz26VJQi2XKEXwew")
	if first != expected {
		t.Fatalf("resource ID = %q, want %q", first, expected)
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("ResourceID.Validate() error = %v", err)
	}
	requireInvalid(t, ResourceID(string(first)+"\n").Validate())
}

func TestResourceIdentityUsesEveryStablePart(t *testing.T) {
	base := fixtureIdentity("service-alpha")
	baseID, err := NewResourceID(base)
	if err != nil {
		t.Fatalf("NewResourceID() error = %v", err)
	}
	tests := map[string]func(ResourceIdentity) ResourceIdentity{
		"provider": func(value ResourceIdentity) ResourceIdentity {
			value.Provider = ProviderDocker
			return value
		},
		"source": func(value ResourceIdentity) ResourceIdentity {
			value.SourceID = "source_other"
			return value
		},
		"environment": func(value ResourceIdentity) ResourceIdentity {
			value.EnvironmentID = "env_other"
			return value
		},
		"kind": func(value ResourceIdentity) ResourceIdentity {
			value.Kind = "demo.worker"
			return value
		},
		"scope": func(value ResourceIdentity) ResourceIdentity {
			value.Scope = Scope{Kind: ScopeProject, Name: "project-one"}
			return value
		},
		"native ID": func(value ResourceIdentity) ResourceIdentity {
			value.nativeID = "service-beta"
			return value
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			changed, err := NewResourceID(mutate(base))
			if err != nil {
				t.Fatalf("NewResourceID() error = %v", err)
			}
			if changed == baseID {
				t.Fatal("stable identity change kept the same ID")
			}
		})
	}
}

func TestResourceDisplayNameDoesNotChangeIdentity(t *testing.T) {
	ref := fixtureResourceRef(t, "service-alpha")
	first := Resource{Ref: ref, DisplayName: "First name"}
	second := Resource{Ref: ref, DisplayName: "Renamed service"}
	if err := first.Validate(); err != nil {
		t.Fatalf("first Resource.Validate() error = %v", err)
	}
	if err := second.Validate(); err != nil {
		t.Fatalf("second Resource.Validate() error = %v", err)
	}
	if first.Ref.ID != second.Ref.ID {
		t.Fatal("display rename changed identity")
	}
	second.DisplayName = "split\u2028name"
	requireInvalid(t, second.Validate())
	second.DisplayName = "spoof\u202Ename"
	requireInvalid(t, second.Validate())
	second.DisplayName = "نیم\u200cفاصله"
	if err := second.Validate(); err != nil {
		t.Fatalf("Persian Resource.Validate() error = %v", err)
	}
	second.DisplayName = "\u200c\u200d"
	requireInvalid(t, second.Validate())
}

func TestResourceRefRejectsMismatchedIdentity(t *testing.T) {
	ref := fixtureResourceRef(t, "service-alpha")
	ref.Identity.nativeID = "service-beta"
	requireInvalid(t, ref.Validate())
}

func TestNativeIdentityDoesNotMarshal(t *testing.T) {
	identity := fixtureIdentity("private-native-value")
	payload, err := json.Marshal(identity)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(payload), "private-native-value") || strings.Contains(string(payload), "nativeID") {
		t.Fatalf("marshaled identity exposed native value: %s", payload)
	}
}

func TestRelationshipIdentityHonorsDirection(t *testing.T) {
	first := fixtureResourceRef(t, "first").ID
	second := fixtureResourceRef(t, "second").ID
	for _, direction := range []RelationshipDirection{RelationshipBidirectional, RelationshipUndirected} {
		forward, err := NewRelationshipID(RelationshipIdentity{From: first, To: second, Kind: RelationshipDependsOn, Direction: direction})
		if err != nil {
			t.Fatalf("NewRelationshipID() error = %v", err)
		}
		reverse, err := NewRelationshipID(RelationshipIdentity{From: second, To: first, Kind: RelationshipDependsOn, Direction: direction})
		if err != nil {
			t.Fatalf("NewRelationshipID() reverse error = %v", err)
		}
		if forward != reverse {
			t.Fatalf("%s relationship changed when endpoints reversed", direction)
		}
	}
	forward, err := NewRelationshipID(RelationshipIdentity{From: first, To: second, Kind: RelationshipDependsOn, Direction: RelationshipSourceToTarget})
	if err != nil {
		t.Fatalf("NewRelationshipID() error = %v", err)
	}
	reverse, err := NewRelationshipID(RelationshipIdentity{From: second, To: first, Kind: RelationshipDependsOn, Direction: RelationshipSourceToTarget})
	if err != nil {
		t.Fatalf("NewRelationshipID() reverse error = %v", err)
	}
	if forward == reverse {
		t.Fatal("source-to-target relationship lost direction")
	}
}

func FuzzResourceIdentityLengthFraming(f *testing.F) {
	f.Add("a|b", "c")
	f.Add("الف", "بتا")
	f.Fuzz(func(t *testing.T, scopeName, nativeID string) {
		left := fixtureIdentity(nativeID + "|tail")
		left.Scope = Scope{Kind: ScopeProject, Name: scopeName}
		right := fixtureIdentity(nativeID)
		right.Scope = Scope{Kind: ScopeProject, Name: scopeName + "|tail"}
		leftID, leftErr := NewResourceID(left)
		rightID, rightErr := NewResourceID(right)
		if leftErr != nil || rightErr != nil {
			return
		}
		if left != right && leftID == rightID {
			t.Fatal("different framed identities produced the same ID")
		}
	})
}

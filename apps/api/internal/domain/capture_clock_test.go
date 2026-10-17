package domain

import (
	"testing"
	"time"
)

func TestCaptureCompletionSeparatesReceiptAndObservationClocks(t *testing.T) {
	at := fixtureTime()
	source := fixtureProvider()
	capabilities := CapabilitySet{Source: source}
	for _, name := range CapabilityNames() {
		capabilities.Capabilities = append(capabilities.Capabilities, CapabilityStatus{Name: name, Status: fixtureStatus()})
	}
	completion := CaptureCompletion{Token: CaptureToken{Source: source, ConfigurationVersion: 1, Generation: 1, ObservedAt: at}, Status: fixtureStatus(), Capabilities: capabilities, Complete: true}
	if err := completion.Validate(); err != nil {
		t.Fatal("legacy observation-clock fallback rejected")
	}
	completion.CollectedAt = at.Add(time.Hour)
	if err := completion.Validate(); err != nil {
		t.Fatal("later operational receipt rejected")
	}
	for _, invalid := range []time.Time{at.Add(-time.Microsecond), at.Add(time.Nanosecond), at.In(time.FixedZone("offset", 3600))} {
		completion.CollectedAt = invalid
		if completion.Validate() == nil {
			t.Fatal("invalid collection receipt accepted")
		}
	}
}

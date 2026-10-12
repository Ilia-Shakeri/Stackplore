package notification

import (
	"context"
	"testing"
)

func TestRuntimeSettingsExposeNoDestinationSecret(t *testing.T) {
	settings, err := NewRuntimeSettings(true, "webhook_primary")
	if err != nil {
		t.Fatal(err)
	}
	status, err := settings.GetNotificationSettings(context.Background())
	if err != nil || !status.Enabled || status.DestinationReferenceID != "webhook_primary" {
		t.Fatalf("GetNotificationSettings() = (%#v, %v)", status, err)
	}
	if _, err := NewRuntimeSettings(false, "webhook_primary"); err == nil {
		t.Fatal("disabled settings accepted a destination reference")
	}
}

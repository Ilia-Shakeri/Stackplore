package config

import (
	"errors"
	"testing"
)

func TestAssistantProfileGlobalRegistrationAndDefaultOff(t *testing.T) {
	settings, err := Load(getter(nil))
	if err != nil || settings.Assistant.Enabled() {
		t.Fatal("default assistant configuration enabled or invalid")
	}
	settings, err = Load(getter(assistantProfileEnvironment()))
	if err != nil || !settings.Assistant.Enabled() || settings.Assistant.Validate() != nil {
		t.Fatal("complete trusted profile was not composed")
	}
	settings.GuestDemo = true
	if !errors.Is(Validate(settings), ErrInvalid) {
		t.Fatal("guest composition accepted local processor capability")
	}
	values := assistantProfileEnvironment()
	values[assistantEnvironmentPrefix+"ENDPOINT"] = "https://invalid.example"
	if _, err := Load(getter(values)); !errors.Is(err, ErrInvalid) {
		t.Fatal("unknown profile input accepted by global configuration")
	}
}

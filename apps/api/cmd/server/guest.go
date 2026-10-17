package main

import (
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

func guestHTTPOptions(settings config.Config) *httpapi.GuestDemoOptions {
	if !settings.GuestDemo || !settings.DemoDeployment {
		return nil
	}
	return &httpapi.GuestDemoOptions{FixtureVersion: demo.FixtureVersion, ScenarioStep: 0, ObservedAt: demo.ObservationTime()}
}

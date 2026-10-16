package main

import (
	"context"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

func TestDemoSeedUsesRuleEvidenceAndLifecycle(t *testing.T) {
	seed, err := buildDemoAlertSeed(context.Background(), demo.MustNew())
	if err != nil {
		t.Fatal(err)
	}
	if seed.Validate() != nil || seed.FixtureVersion != demo.FixtureVersion || seed.Mutation.Alert.State != domain.AlertFiring ||
		!seed.Mutation.Alert.Synthetic || seed.Mutation.Transition == nil || seed.Mutation.Notification == nil ||
		!seed.Mutation.Notification.Synthetic || seed.Mutation.Alert.SummaryCode != "alert.resource_unhealthy.matched" {
		t.Fatal("demo evidence did not produce a valid synthetic firing seed")
	}
}

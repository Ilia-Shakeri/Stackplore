package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

func savePackagedDemo(t *testing.T, store *Store) domain.DemoAlertSeed {
	t.Helper()
	ctx := context.Background()
	provider := demo.MustNew()
	at := time.Now().UTC().Truncate(time.Microsecond)
	configuration := domain.SourceConfiguration{Source: provider.Source(), ProfileID: domain.SourceProfileDemoFixture, Enabled: true, Version: 1, CreatedAt: at, UpdatedAt: at}
	if err := store.EnsureSourceConfigurations(ctx, []domain.SourceConfiguration{configuration}); err != nil {
		t.Fatal(err)
	}
	configuration, err := store.GetSourceConfiguration(ctx, provider.Source().ID)
	if err != nil {
		t.Fatal(err)
	}
	token, err := store.BeginCapture(ctx, configuration.Source.ID, configuration.Version, at)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := provider.CaptureInventory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCapture(ctx, domain.CaptureBatch{Token: token, Environments: initial.Environments, Resources: initial.Resources, Health: initial.Health, Relationships: initial.Relationships}); err != nil {
		t.Fatal(err)
	}
	capabilities, err := provider.GetCapabilities(ctx, initial.Source)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishCapture(ctx, domain.CaptureCompletion{Token: token, Status: initial.Status, Capabilities: capabilities, Complete: true}); err != nil {
		t.Fatal(err)
	}
	alert := provider.Alerts()[0]
	seed := domain.DemoAlertSeed{FixtureVersion: demo.FixtureVersion, Rule: domain.AlertRule{ID: alert.RuleID, SourceID: alert.SourceID, EnvironmentID: alert.EnvironmentID, ResourceID: alert.ResourceID, DisplayName: "Demo checkout health", Kind: domain.AlertRuleResourceUnhealthy, Enabled: true, Severity: alert.Severity, EvaluationInterval: 30 * time.Second}, Mutation: domain.AlertMutation{Alert: alert, DedupeKey: string(alert.RuleID), Transition: &domain.AlertTransition{ID: "transition_guest_fixture", AlertID: alert.ID, To: domain.AlertFiring, OccurredAt: alert.UpdatedAt, Actor: domain.AuditActor{Kind: domain.AuditActorSystem}, ReasonCode: alert.SummaryCode, EvidenceRange: alert.EvidenceRange}}}
	if err := seed.Validate(); err != nil {
		t.Fatal(err)
	}
	return seed
}

func TestDemoDeploymentRejectsNormalDatabaseAndForeignHistory(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		fixture := newObservationFixture(t)
		profile := DeploymentProfile{DataRoot: "/fixture/normal", FixtureVersion: demo.FixtureVersion}
		if err := fixture.store.ConfigureDeployment(fixture.ctx, profile); err != nil {
			t.Fatal(err)
		}
		profile.Demo, profile.Guest, profile.DataRoot = true, true, "/fixture/demo"
		if err := fixture.store.ConfigureDeployment(fixture.ctx, profile); !errors.Is(err, domain.ErrRepositoryConflict) {
			t.Fatalf("normal deployment exposed as guest: %v", err)
		}
	})
	t.Run("unclassified_used", func(t *testing.T) {
		store, _, _ := openIntegrationStore(t)
		ctx := context.Background()
		at := time.Now().UTC().Truncate(time.Microsecond)
		configuration := domain.SourceConfiguration{Source: domain.InfrastructureSource{ID: "source_other", Provider: domain.ProviderDemo, Name: "Other fixture", Synthetic: true}, ProfileID: domain.SourceProfileDemoFixture, Enabled: true, Version: 1, CreatedAt: at, UpdatedAt: at}
		if err := store.EnsureSourceConfigurations(ctx, []domain.SourceConfiguration{configuration}); err != nil {
			t.Fatal(err)
		}
		profile := DeploymentProfile{Demo: true, Guest: true, DataRoot: "/fixture/demo", FixtureVersion: demo.FixtureVersion}
		if err := store.ConfigureDeployment(ctx, profile); !errors.Is(err, domain.ErrRepositoryConflict) {
			t.Fatalf("unclassified used database exposed: %v", err)
		}
	})
}

func TestGuestDemoResetKeepsProtectedAuditAndUsesNoInventedAccount(t *testing.T) {
	store, _, _ := openIntegrationStore(t)
	ctx := context.Background()
	profile := DeploymentProfile{Demo: true, Guest: true, DataRoot: "/fixture/demo-only", FixtureVersion: demo.FixtureVersion}
	if err := store.ConfigureDeployment(ctx, profile); err != nil {
		t.Fatal(err)
	}
	seed := savePackagedDemo(t, store)
	if err := store.EnsureDemoAlerts(ctx, seed); err != nil {
		t.Fatalf("guest seed without account: %v", err)
	}
	rule, err := store.GetAlertRule(ctx, seed.Rule.ID)
	if err != nil || !rule.SystemOwned || rule.CreatedByUserID != "" || rule.UpdatedByUserID != "" {
		t.Fatal("guest seed invented account ownership")
	}
	readProtectedEvidence := func(wantSeeds int) map[domain.AuditEntryID]domain.AuditEntry {
		t.Helper()
		var users, sessions, grants, deliveries int
		if err := store.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM sessions),(SELECT count(*) FROM recent_auth_grants),(SELECT count(*) FROM notification_deliveries)`).Scan(&users, &sessions, &grants, &deliveries); err != nil || users+sessions+grants+deliveries != 0 {
			t.Fatal("guest created authentication or outbound work")
		}
		page, err := store.ListAuditEntries(ctx, domain.PageRequest{Limit: 10})
		if err != nil || page.Page.HasMore || len(page.Entries) != wantSeeds+1 {
			t.Fatal("guest lost or invented protected evidence")
		}
		entries := make(map[domain.AuditEntryID]domain.AuditEntry, len(page.Entries))
		var creations, seeds int
		for _, entry := range page.Entries {
			if entry.Actor != (domain.AuditActor{Kind: domain.AuditActorSystem}) || entry.Target != (domain.AuditTarget{Kind: "source.configuration", ID: string(seed.Rule.SourceID)}) || entry.Outcome != domain.AuditSuccess {
				t.Fatal("guest evidence has invented ownership, target or outcome")
			}
			switch {
			case entry.Action == "source.create" && entry.ReasonCode == "source.created":
				creations++
			case entry.Action == "alert.demo_seed" && entry.ReasonCode == "alert.demo_seeded":
				seeds++
			default:
				t.Fatal("guest evidence has unexpected action or reason")
			}
			entries[entry.ID] = entry
		}
		if creations != 1 || seeds != wantSeeds || len(entries) != len(page.Entries) {
			t.Fatal("guest source audit duplicated or seed evidence lost")
		}
		return entries
	}
	protected := readProtectedEvidence(1)
	if err := store.ConfigureDeployment(ctx, profile); err != nil {
		t.Fatalf("repeat process reset: %v", err)
	}
	page, err := store.ListAlerts(ctx, seed.Rule.SourceID, domain.PageRequest{Limit: 10})
	if err != nil || len(page.Alerts) != 0 {
		t.Fatal("guest reset kept old derived alert state")
	}
	for id, entry := range readProtectedEvidence(1) {
		if original, exists := protected[id]; !exists || original != entry {
			t.Fatal("guest reset changed protected audit evidence")
		}
	}
	seed = savePackagedDemo(t, store)
	if err := store.EnsureDemoAlerts(ctx, seed); err != nil {
		t.Fatalf("reseed after reset: %v", err)
	}
	reseeded := readProtectedEvidence(2)
	for id, entry := range protected {
		if retained, exists := reseeded[id]; !exists || retained != entry {
			t.Fatal("guest reseed replaced protected audit history")
		}
	}
	profile.DataRoot = "/fixture/other-demo"
	if err := store.ConfigureDeployment(ctx, profile); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatalf("different guest data root accepted: %v", err)
	}
}

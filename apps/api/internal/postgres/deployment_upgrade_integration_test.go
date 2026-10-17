package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

func TestNormalFixtureUpgradePreservesAdministratorRulesAndDisabledSource(t *testing.T) {
	store, _, _ := openIntegrationStore(t)
	ctx := context.Background()
	profile := DeploymentProfile{DataRoot: "/fixture/normal", FixtureVersion: "demo-v1"}
	if err := store.ConfigureDeployment(ctx, profile); err != nil {
		t.Fatal(err)
	}
	seed := savePackagedDemo(t, store)
	at := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := store.pool.Exec(ctx, `INSERT INTO users(id,login_name,canonical_login_name,display_name,role,enabled,created_at_us,updated_at_us) VALUES('user_upgrade','upgrade','upgrade','Upgrade administrator','administrator',1,$1,$1)`, at.UnixMicro()); err != nil {
		t.Fatal(err)
	}
	seed.FixtureVersion = "demo-v1"
	if err := store.EnsureDemoAlerts(ctx, seed); err != nil {
		t.Fatal(err)
	}
	custom := domain.AlertRuleRecord{Rule: domain.AlertRule{ID: "rule_custom_demo", SourceID: demo.SourceID, DisplayName: "Operator connection rule", Kind: domain.AlertRuleProviderConnectionLost, Enabled: false, Severity: domain.SeverityWarning, EvaluationInterval: time.Minute}, CreatedByUserID: "user_upgrade", UpdatedByUserID: "user_upgrade", CreatedAt: at, UpdatedAt: at, Version: 1}
	command := domain.AlertRuleCreate{Record: custom, Audit: domain.AuditEntry{ID: "audit_upgrade_custom_rule", OccurredAt: at, Actor: domain.AuditActor{Kind: domain.AuditActorUser, ID: "user_upgrade"}, Action: "alert.rule_create", Target: domain.AuditTarget{Kind: "alert.rule", ID: string(custom.Rule.ID)}, Outcome: domain.AuditSuccess, RequestID: "request_upgrade_custom_rule", ReasonCode: "alert.rule_created"}}
	if err := store.CreateAlertRule(ctx, command); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE infrastructure_sources SET enabled=0,row_version=row_version+1 WHERE id=$1`, string(demo.SourceID)); err != nil {
		t.Fatal(err)
	}
	var auditBefore int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries`).Scan(&auditBefore); err != nil {
		t.Fatal(err)
	}
	profile.FixtureVersion = demo.FixtureVersion
	if err := store.ConfigureDeployment(ctx, profile); err != nil {
		t.Fatal(err)
	}
	actual, err := store.GetAlertRule(ctx, custom.Rule.ID)
	if err != nil || actual.Rule != custom.Rule || actual.Version != custom.Version || actual.CreatedByUserID != custom.CreatedByUserID || actual.UpdatedByUserID != custom.UpdatedByUserID {
		t.Fatalf("normal upgrade changed administrator configuration: %#v, %v", actual, err)
	}
	configuration, err := store.GetSourceConfiguration(ctx, demo.SourceID)
	if err != nil || configuration.Enabled {
		t.Fatalf("normal upgrade reenabled disabled source: %#v, %v", configuration, err)
	}
	var rules, alerts, users, audit int
	if err := store.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM alert_rules WHERE id=$1),(SELECT count(*) FROM alerts WHERE source_id=$2),(SELECT count(*) FROM users),(SELECT count(*) FROM audit_entries)`, string(domain.DemoCheckoutHealthRuleID), string(demo.SourceID)).Scan(&rules, &alerts, &users, &audit); err != nil || rules != 0 || alerts != 0 || users != 1 || audit != auditBefore {
		t.Fatalf("upgrade reset wrong state: packaged rules=%d alerts=%d users=%d audits=%d/%d err=%v", rules, alerts, users, audit, auditBefore, err)
	}
	if err := store.ConfigureDeployment(ctx, profile); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetAlertRule(ctx, custom.Rule.ID); err != nil {
		t.Fatal("same-version restart deleted administrator rule")
	}
	// A prior process can stop after its pending marker commits but before reset completes.
	if _, err := store.pool.Exec(ctx, `UPDATE deployment_profile SET reset_pending=true WHERE singleton=true`); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfigureDeployment(ctx, profile); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetAlertRule(ctx, custom.Rule.ID); err != nil {
		t.Fatal("interrupted reset lost administrator rule")
	}
	var pending, enabled bool
	var epoch int
	if err := store.pool.QueryRow(ctx, `SELECT profile.reset_pending,profile.reset_epoch,source.enabled=1 FROM deployment_profile AS profile CROSS JOIN infrastructure_sources AS source WHERE profile.singleton=true AND source.id=$1`, string(demo.SourceID)).Scan(&pending, &epoch, &enabled); err != nil || pending || enabled || epoch != 2 {
		t.Fatalf("interrupted reset did not finish safely: pending=%t enabled=%t epoch=%d err=%v", pending, enabled, epoch, err)
	}
}

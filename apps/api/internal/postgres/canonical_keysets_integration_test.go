package postgres

import (
	"slices"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func useNonByteIDCollation(t *testing.T, fixture observationFixture, table string) {
	t.Helper()
	var available bool
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT EXISTS(SELECT 1 FROM pg_collation WHERE collname='en-US-x-icu')`).Scan(&available); err != nil {
		t.Fatal("inspect non-byte collation support")
	}
	if !available {
		t.Skip("non-byte ICU collation is not installed on this PostgreSQL server")
	}
	if table != "notifications" && table != "alert_rules" {
		t.Fatal("unexpected keyset fixture table")
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `ALTER TABLE `+table+` ALTER COLUMN id TYPE TEXT COLLATE "en-US-x-icu"`); err != nil {
		t.Fatal("set owned fixture ID collation")
	}
}

func TestNotificationKeysetUsesByteOrderUnderNonByteCollation(t *testing.T) {
	fixture := newObservationFixture(t)
	useNonByteIDCollation(t, fixture, "notifications")
	wanted := []string{"notification_equal_A", "notification_equal_Z", "notification_equal_a", "notification_equal_z"}
	for _, id := range wanted {
		if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO notifications(id,notification_kind,severity,summary_code,created_at_us,synthetic) VALUES($1,'notification.alert_firing','warning','alert.condition_matched',$2,0)`, id, fixture.at.UnixMicro()); err != nil {
			t.Fatal("seed equal-time notifications")
		}
	}
	if page, err := fixture.store.ListNotifications(fixture.ctx, domain.PageRequest{Limit: 10}); err != nil || len(page.Notifications) != len(wanted) {
		t.Fatalf("equal-time notification page failed: %v", err)
	}
	request := domain.PageRequest{Limit: 2}
	seen := make([]string, 0, len(wanted))
	for range 4 {
		page, err := fixture.store.ListNotifications(fixture.ctx, request)
		if err != nil {
			t.Fatalf("equal-time notification keyset failed: %v", err)
		}
		for _, item := range page.Notifications {
			seen = append(seen, string(item.ID))
		}
		if !page.Page.HasMore {
			break
		}
		request.Cursor = page.Page.NextCursor
	}
	if !slices.Equal(seen, wanted) {
		t.Fatal("notification keyset skipped, repeated, or reordered opaque IDs")
	}
}

func TestAlertRuleKeysetUsesByteOrderUnderNonByteCollation(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("seed equal-time rule owner")
	}
	useNonByteIDCollation(t, fixture, "alert_rules")
	wanted := []string{"rule_equal_A", "rule_equal_Z", "rule_equal_a", "rule_equal_z"}
	at := fixture.at.Add(time.Second)
	for _, id := range wanted {
		rule := seed.Rule
		rule.ID = domain.AlertRuleID(id)
		command := domain.AlertRuleCreate{
			Record: domain.AlertRuleRecord{Rule: rule, CreatedByUserID: "user_alert_seed", UpdatedByUserID: "user_alert_seed", CreatedAt: at, UpdatedAt: at, Version: 1},
			Audit:  alertCommandAudit("audit_"+id, at, "alert.rule_create", "alert.rule", id, "alert.rule_created"),
		}
		if err := fixture.store.CreateAlertRule(fixture.ctx, command); err != nil {
			t.Fatal("seed equal-time alert rule")
		}
	}
	if page, err := fixture.store.ListAlertRules(fixture.ctx, domain.PageRequest{Limit: 10}); err != nil || len(page.Rules) != len(wanted)+1 {
		t.Fatalf("equal-time rule page failed: %v", err)
	}
	request := domain.PageRequest{Limit: 2}
	seen := make([]string, 0, len(wanted))
	for range 4 {
		page, err := fixture.store.ListAlertRules(fixture.ctx, request)
		if err != nil {
			t.Fatalf("equal-time rule keyset failed: %v", err)
		}
		for _, item := range page.Rules {
			if item.Rule.ID != seed.Rule.ID {
				seen = append(seen, string(item.Rule.ID))
			}
		}
		if !page.Page.HasMore {
			break
		}
		request.Cursor = page.Page.NextCursor
	}
	if !slices.Equal(seen, wanted) {
		t.Fatal("alert rule keyset skipped, repeated, or reordered opaque IDs")
	}
}

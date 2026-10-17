package postgres

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestAlertKeysetKeepsCanonicalMixedCaseIDsAtEqualTimestamps(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("seed equal-time alert rule")
	}
	at := fixture.at.Add(-time.Second).UnixMicro()
	wanted := []string{"alert_equal_A", "alert_equal_Z", "alert_equal_a", "alert_equal_z"}
	for _, id := range wanted {
		if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO alerts(id,rule_id,source_id,resource_id,dedupe_key,state,severity,evidence_start_at_us,evidence_end_at_us,started_at_us,updated_at_us,resolved_at_us,summary_code,synthetic)
VALUES($1,$2,$3,$4,$1,'resolved','error',$5::bigint-60000000,$5,$5::bigint-60000000,$5,$5,'alert.resource_unhealthy.clear',1)`, id, string(seed.Rule.ID), string(seed.Rule.SourceID), string(*seed.Rule.ResourceID), at); err != nil {
			t.Fatal("seed equal-time opaque alert IDs")
		}
	}
	request := domain.PageRequest{Limit: 2}
	seen := make([]string, 0, len(wanted))
	for range 4 {
		page, err := fixture.store.ListAlerts(fixture.ctx, seed.Rule.SourceID, request)
		if err != nil {
			t.Fatalf("equal-time alert page failed: %v", err)
		}
		for _, alert := range page.Alerts {
			if alert.ID != seed.Mutation.Alert.ID {
				seen = append(seen, string(alert.ID))
			}
		}
		if !page.Page.HasMore {
			break
		}
		request.Cursor = page.Page.NextCursor
	}
	if !slices.Equal(seen, wanted) {
		t.Fatal("equal-time alert pagination skipped, repeated, or reordered opaque IDs")
	}
	var indexDefinition string
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT pg_get_indexdef(indexrelid) FROM pg_index WHERE indexrelid='alerts_source_order_idx'::regclass`).Scan(&indexDefinition); err != nil || !strings.Contains(indexDefinition, `COLLATE "C"`) {
		t.Fatal("alert ordering index lacks canonical byte collation")
	}
}

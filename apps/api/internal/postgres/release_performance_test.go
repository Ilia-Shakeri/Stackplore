package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

type releasePlanNode struct {
	NodeType            string            `json:"Node Type"`
	IndexName           string            `json:"Index Name"`
	ActualRows          float64           `json:"Actual Rows"`
	ActualLoops         float64           `json:"Actual Loops"`
	SharedHitBlocks     int64             `json:"Shared Hit Blocks"`
	SharedReadBlocks    int64             `json:"Shared Read Blocks"`
	TempReadBlocks      int64             `json:"Temp Read Blocks"`
	TempWrittenBlocks   int64             `json:"Temp Written Blocks"`
	RowsRemovedByFilter float64           `json:"Rows Removed by Filter"`
	Plans               []releasePlanNode `json:"Plans"`
}

type releasePlanDocument struct {
	Plan          releasePlanNode `json:"Plan"`
	PlanningTime  float64         `json:"Planning Time"`
	ExecutionTime float64         `json:"Execution Time"`
}

// TestReleaseRepresentativeKeysetPlansOnRealPostgreSQL records measured work,
// not a hardware-specific latency promise. Each fixture owns a disposable schema.
func TestReleaseRepresentativeKeysetPlansOnRealPostgreSQL(t *testing.T) {
	fixture := newObservationFixture(t)
	ctx, cancel := context.WithTimeout(fixture.ctx, 90*time.Second)
	defer cancel()
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(ctx, seed); err != nil {
		t.Fatal("seed measured alert rule")
	}
	var version string
	if err := fixture.store.pool.QueryRow(ctx, `SHOW server_version`).Scan(&version); err != nil {
		t.Fatal("read PostgreSQL version")
	}
	t.Logf("performance_environment postgresql=%s page_limit=50 samples=5 warmup=1", version)
	ids := make(map[int]string, 20000)
	for _, size := range []int{1000, 20000} {
		start := 1
		if size == 20000 {
			start = 1001
		}
		resources, snapshots := make([][]any, 0, size-start+1), make([][]any, 0, size-start+1)
		for point := start; point <= size; point++ {
			identity, err := domain.NewResourceIdentity(domain.ProviderDemo, fixture.source.Source.ID, "environment_capture", "demo.service", domain.Scope{Kind: domain.ScopeGlobal}, domain.NativeResourceID(fmt.Sprintf("scale_%06d", point)))
			if err != nil {
				t.Fatal("create measured resource identity")
			}
			ref, err := domain.NewResourceRef(identity)
			if err != nil {
				t.Fatal("create measured resource reference")
			}
			ids[point] = string(ref.ID)
			at := fixture.completion.Status.ObservedAt.Add(-time.Second).UnixMicro()
			resources = append(resources, []any{string(ref.ID), string(fixture.source.Source.ID), "demo", "environment_capture", "demo.service", "global", "", string(identity.NativeID()), int64(1), at, at, int64(1)})
			snapshots = append(snapshots, []any{string(ref.ID), fmt.Sprintf("Scale resource %06d", point), "available", "complete", at})
		}
		for _, batch := range []struct {
			table   string
			columns []string
			rows    [][]any
		}{
			{"resources", []string{"id", "source_id", "provider", "environment_id", "kind", "scope_kind", "scope_name", "native_id", "synthetic", "first_seen_at_us", "last_seen_at_us", "capture_generation"}, resources},
			{"resource_snapshots", []string{"resource_id", "display_name", "availability", "completeness", "observed_at_us"}, snapshots},
		} {
			if _, err := fixture.store.pool.CopyFrom(ctx, pgx.Identifier{batch.table}, batch.columns, pgx.CopyFromRows(batch.rows)); err != nil {
				t.Fatalf("seed measured %s", batch.table)
			}
		}
		if _, err := fixture.store.pool.Exec(ctx, `INSERT INTO infrastructure_events(id,origin_kind,provider,source_id,kind,severity,reason_code,summary,occurred_at_us,observed_at_us,synthetic)
SELECT 'event_scale_'||lpad(point::text,6,'0'),'provider','demo',$1,'demo.health.changed','warning','demo.health.changed','Synthetic measured event',$2::bigint-point::bigint*1000000,$2,1 FROM generate_series($3::integer,$4::integer) AS point`, string(fixture.source.Source.ID), fixture.completion.Status.ObservedAt.UnixMicro(), start, size); err != nil {
			t.Fatal("seed measured events")
		}
		if _, err := fixture.store.pool.Exec(ctx, `INSERT INTO alerts(id,rule_id,source_id,resource_id,dedupe_key,state,severity,evidence_start_at_us,evidence_end_at_us,started_at_us,updated_at_us,resolved_at_us,summary_code,synthetic)
SELECT 'alert_scale_'||lpad(point::text,6,'0'),$1,$2,$3,'scale_'||point,'resolved','error',$4::bigint-point::bigint*1000000-60000000,$4::bigint-point::bigint*1000000,$4::bigint-point::bigint*1000000-60000000,$4::bigint-point::bigint*1000000,$4::bigint-point::bigint*1000000,'alert.resource_unhealthy.clear',1 FROM generate_series($5::integer,$6::integer) AS point`, string(seed.Rule.ID), string(seed.Rule.SourceID), string(*seed.Rule.ResourceID), fixture.completion.Status.ObservedAt.UnixMicro(), start, size); err != nil {
			t.Fatal("seed measured alerts")
		}
		if _, err := fixture.store.pool.Exec(ctx, `ANALYZE resources; ANALYZE resource_snapshots; ANALYZE infrastructure_events; ANALYZE alerts`); err != nil {
			t.Fatal("analyze measured fixture")
		}
		for _, depth := range []int{0, size / 2, size - 100} {
			boundName, boundID := "", ""
			if depth > 0 {
				boundName, boundID = fmt.Sprintf("Scale resource %06d", depth), ids[depth]
			}
			inventorySQL := `SELECT ` + observedResourceColumns + activeResourceJoin + ` AND snapshot.display_name COLLATE "C">=$3 COLLATE "C" AND (snapshot.display_name COLLATE "C">$3 COLLATE "C" OR (snapshot.display_name COLLATE "C"=$3 COLLATE "C" AND resource.id COLLATE "C">$4 COLLATE "C")) ORDER BY snapshot.display_name COLLATE "C",resource.id COLLATE "C" LIMIT 51`
			recordReleasePlan(t, ctx, fixture, "inventory_name", size, depth, inventorySQL, string(fixture.source.Source.ID), "demo", boundName, boundID)
			at := fixture.completion.Status.ObservedAt.Add(-time.Duration(depth) * time.Second).UnixMicro()
			eventID := fmt.Sprintf("event_scale_%06d", depth)
			eventSQL := `SELECT id,provider,source_id,environment_id,resource_id,kind,severity,reason_code,summary,occurred_at_us,observed_at_us,synthetic
FROM infrastructure_events WHERE source_id=$1 AND provider=$2 AND occurred_at_us>=$3 AND occurred_at_us<$4
AND EXISTS(SELECT 1 FROM infrastructure_sources AS source WHERE source.id=infrastructure_events.source_id
AND (source.partition_generation=0 OR (infrastructure_events.origin_kind='system' AND infrastructure_events.resource_id IS NULL)
OR (infrastructure_events.resource_id IS NOT NULL AND EXISTS(SELECT 1 FROM resources AS resource WHERE resource.id=infrastructure_events.resource_id AND ` + authorizedResourcePredicate + `))))
AND (cardinality($5::text[])=0 OR resource_id=ANY($5::text[])) AND (cardinality($6::text[])=0 OR severity=ANY($6::text[]))
AND occurred_at_us<=$7 AND (occurred_at_us<$7 OR (occurred_at_us=$7 AND id COLLATE "C">$8 COLLATE "C"))
ORDER BY occurred_at_us DESC,id COLLATE "C" LIMIT 51`
			recordReleasePlan(t, ctx, fixture, "events", size, depth, eventSQL, string(fixture.source.Source.ID), "demo", fixture.completion.Status.ObservedAt.Add(-24*time.Hour).UnixMicro(), fixture.completion.Status.ObservedAt.Add(time.Second).UnixMicro(), []string{}, []string{}, at, eventID)
			alertID := fmt.Sprintf("alert_scale_%06d", depth)
			alertSQL := `SELECT id,rule_id,source_id,environment_id,resource_id,state,severity,evidence_start_at_us,evidence_end_at_us,started_at_us,updated_at_us,acknowledged_at_us,resolved_at_us,summary_code,synthetic FROM alerts WHERE source_id=$1 AND updated_at_us<=$2 AND (updated_at_us<$2 OR (updated_at_us=$2 AND id COLLATE "C">$3 COLLATE "C")) ORDER BY updated_at_us DESC,id COLLATE "C" ASC LIMIT 51`
			recordReleasePlan(t, ctx, fixture, "alerts", size, depth, alertSQL, string(seed.Rule.SourceID), at, alertID)
			verifyReleaseRepositoryPages(t, ctx, fixture, seed.Rule.SourceID, size, depth, boundName, boundID, at, eventID, alertID)
		}
	}
}

func recordReleasePlan(t *testing.T, ctx context.Context, fixture observationFixture, name string, size, depth int, statement string, arguments ...any) {
	t.Helper()
	measurements := make([]float64, 0, 5)
	var last releasePlanDocument
	for sample := range 6 {
		var output []byte
		if err := fixture.store.pool.QueryRow(ctx, `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) `+statement, arguments...).Scan(&output); err != nil {
			t.Fatalf("explain %s: %v", name, err)
		}
		var document []releasePlanDocument
		if json.Unmarshal(output, &document) != nil || len(document) != 1 {
			t.Fatal("invalid measured plan")
		}
		last = document[0]
		if last.Plan.NodeType != "Limit" || last.Plan.ActualRows > 51 || last.Plan.ActualRows < 50 {
			t.Fatalf("%s lost bounded lookahead page", name)
		}
		if sample > 0 {
			measurements = append(measurements, last.ExecutionTime)
		}
	}
	sort.Float64s(measurements)
	indexes, sorts, filtered := map[string]bool{}, 0, float64(0)
	var visit func(releasePlanNode)
	visit = func(node releasePlanNode) {
		if node.IndexName != "" {
			indexes[node.IndexName] = true
		}
		if node.NodeType == "Sort" || node.NodeType == "Incremental Sort" {
			sorts++
		}
		filtered += node.RowsRemovedByFilter * node.ActualLoops
		for _, child := range node.Plans {
			visit(child)
		}
	}
	visit(last.Plan)
	if size == 20000 {
		requiredIndex := map[string]string{"inventory_name": "resource_snapshots_name_idx", "alerts": "alerts_source_order_idx"}[name]
		if requiredIndex != "" && (!indexes[requiredIndex] || filtered > 51) {
			t.Fatalf("%s regressed to scanning prior keyset rows: index present=%v filtered=%.0f", name, indexes[requiredIndex], filtered)
		}
	}
	indexNames := make([]string, 0, len(indexes))
	for index := range indexes {
		indexNames = append(indexNames, index)
	}
	sort.Strings(indexNames)
	t.Logf("performance_plan name=%s rows=%d depth=%d returned=%.0f median_ms=%.3f min_ms=%.3f max_ms=%.3f shared_hit=%d shared_read=%d temp_read=%d temp_written=%d sorts=%d filtered=%.0f indexes=%v", name, size, depth, last.Plan.ActualRows, measurements[2], measurements[0], measurements[4], last.Plan.SharedHitBlocks, last.Plan.SharedReadBlocks, last.Plan.TempReadBlocks, last.Plan.TempWrittenBlocks, sorts, filtered, indexNames)
}

func verifyReleaseRepositoryPages(t *testing.T, ctx context.Context, fixture observationFixture, sourceID domain.SourceID, size, depth int, boundName, boundID string, at int64, eventID, alertID string) {
	t.Helper()
	inventory := domain.InventoryQuery{Source: fixture.token.Source, SortField: domain.ResourceSortName, SortDirection: domain.SortAscending, Page: domain.PageRequest{Limit: 50}}
	events := domain.EventQuery{Source: fixture.token.Source, Range: domain.TimeRange{Start: fixture.completion.Status.ObservedAt.Add(-24 * time.Hour), End: fixture.completion.Status.ObservedAt.Add(time.Second)}, Page: domain.PageRequest{Limit: 50}}
	alertPage := domain.PageRequest{Limit: 50}
	if depth > 0 {
		binding := inventory
		binding.Page = domain.PageRequest{}
		var err error
		inventory.Page.Cursor, err = fixture.store.encodeObservationCursor("resources", observationQueryHash(binding), boundName, boundID)
		if err != nil {
			t.Fatal("encode measured inventory anchor")
		}
		eventBinding := events
		eventBinding.Page = domain.PageRequest{}
		events.Page.Cursor, err = fixture.store.encodeObservationCursor("events", observationQueryHash(eventBinding), strconv.FormatInt(at, 10), eventID)
		if err != nil {
			t.Fatal("encode measured event anchor")
		}
		alertPage.Cursor, err = fixture.store.encodeAlertCursor(sourceID, domain.Alert{ID: domain.AlertID(alertID), UpdatedAt: time.UnixMicro(at).UTC()})
		if err != nil {
			t.Fatal("encode measured alert anchor")
		}
	}
	resourcePage, err := fixture.store.ListResources(ctx, inventory)
	if err != nil || len(resourcePage.Resources) != 50 || !resourcePage.Page.HasMore {
		t.Fatalf("inventory keyset page size=%d depth=%d failed: %v", size, depth, err)
	}
	eventPage, err := fixture.store.ListEvents(ctx, events)
	if err != nil || len(eventPage.Events) != 50 || !eventPage.Page.HasMore {
		t.Fatalf("event keyset page size=%d depth=%d failed: %v", size, depth, err)
	}
	alerts, err := fixture.store.ListAlerts(ctx, sourceID, alertPage)
	if err != nil || len(alerts.Alerts) != 50 || !alerts.Page.HasMore {
		t.Fatalf("alert keyset page size=%d depth=%d failed: %v", size, depth, err)
	}
	if depth > 0 && (resourcePage.Resources[0].Resource.DisplayName <= boundName || eventPage.Events[0].OccurredAt.After(time.UnixMicro(at)) || !alerts.Alerts[0].UpdatedAt.Before(time.UnixMicro(at))) {
		t.Fatal("keyset page crossed its immutable anchor")
	}
}

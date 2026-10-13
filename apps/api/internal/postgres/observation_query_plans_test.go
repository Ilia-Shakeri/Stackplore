package postgres

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

func TestObservationHotPathQueryPlansOnRealPostgreSQL(t *testing.T) {
	fixture := newObservationFixture(t)
	resources := make([][]any, 0, 2000)
	snapshots := make([][]any, 0, 2000)
	metrics := make([][]any, 0, 20000)
	events := make([][]any, 0, 10000)
	audits := make([][]any, 0, 10000)
	ids := make([]string, 0, 100)
	for index := range 2000 {
		identity, err := domain.NewResourceIdentity(domain.ProviderDemo, fixture.source.Source.ID, "environment_capture", "demo.service", domain.Scope{Kind: domain.ScopeGlobal}, domain.NativeResourceID(fmt.Sprintf("plan_%d", index)))
		if err != nil {
			t.Fatal("make query-plan resource")
		}
		ref, err := domain.NewResourceRef(identity)
		if err != nil {
			t.Fatal("make query-plan resource reference")
		}
		if len(ids) < 100 {
			ids = append(ids, string(ref.ID))
		}
		at := fixture.at.Add(-time.Second).UnixMicro()
		resources = append(resources, []any{string(ref.ID), string(fixture.source.Source.ID), "demo", "environment_capture", "demo.service", "global", "", string(identity.NativeID()), int64(1), at, at, int64(1)})
		snapshots = append(snapshots, []any{string(ref.ID), fmt.Sprintf("Plan resource %04d", index), "available", "complete", at})
		for point := range 10 {
			sampledAt := fixture.at.Add(-time.Duration(point+1) * 30 * time.Second).UnixMicro()
			metrics = append(metrics, []any{fmt.Sprintf("metric_plan_%d_%d", index, point), string(ref.ID), "cpu.usage.cores", "cores", float64(point) / 10, sampledAt, int64(30), int64(1)})
		}
	}
	for index := range 10000 {
		occurredAt := fixture.at.Add(-time.Duration(index%1000+1) * time.Second).UnixMicro()
		events = append(events, []any{fmt.Sprintf("event_plan_%d", index), "provider", "demo", string(fixture.source.Source.ID), "demo.health.changed", "warning", "demo.health.changed", "Health changed", occurredAt, fixture.at.UnixMicro(), int64(1)})
		audits = append(audits, []any{fmt.Sprintf("audit_plan_%d", index), occurredAt, "system", "collector.persist", "source.observation", "success", fmt.Sprintf("request_plan_%d", index), "collector.persisted"})
	}
	nativeConfiguration := domain.SourceConfiguration{Source: domain.InfrastructureSource{ID: "source_plan_native", Provider: domain.ProviderDocker, Name: "Native plan fixture"},
		ProfileID: domain.SourceProfileDockerSocketDevelopment, Enabled: true, Version: 1, CreatedAt: fixture.at.Add(-time.Hour), UpdatedAt: fixture.at.Add(-time.Hour)}
	if err := fixture.store.EnsureSourceConfigurations(fixture.ctx, []domain.SourceConfiguration{nativeConfiguration}); err != nil {
		t.Fatal("seed native query-plan source")
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO environments
(id,source_id,provider,display_name,availability,completeness,observed_at_us,synthetic,first_seen_at_us,last_seen_at_us)
VALUES('environment_plan_native',$1,'docker','Native plan fixture','available','complete',$2,0,$2,$2)`, string(nativeConfiguration.Source.ID), fixture.at.UnixMicro()); err != nil {
		t.Fatal("seed native query-plan environment")
	}
	for index := range 100 {
		identity, err := domain.NewResourceIdentity(domain.ProviderDocker, nativeConfiguration.Source.ID, "environment_plan_native", "docker.container", domain.Scope{Kind: domain.ScopeGlobal}, domain.NativeResourceID(fmt.Sprintf("plan_native_%d", index)))
		if err != nil {
			t.Fatal("make native query-plan resource")
		}
		ref, err := domain.NewResourceRef(identity)
		if err != nil {
			t.Fatal("make native query-plan reference")
		}
		at := fixture.at.Add(-time.Second).UnixMicro()
		resources = append(resources, []any{string(ref.ID), string(nativeConfiguration.Source.ID), "docker", "environment_plan_native", "docker.container", "global", "", string(identity.NativeID()), int64(0), at, at, int64(0)})
		for point := range 10 {
			sampledAt := fixture.at.Add(-time.Duration(point+1) * 30 * time.Second).UnixMicro()
			metrics = append(metrics, []any{fmt.Sprintf("metric_plan_native_%d_%d", index, point), string(ref.ID), "cpu.usage.cores", "cores", float64(point) / 10, sampledAt, int64(30), int64(0)})
		}
	}
	for _, copy := range []struct {
		table   string
		columns []string
		rows    [][]any
	}{
		{"resources", []string{"id", "source_id", "provider", "environment_id", "kind", "scope_kind", "scope_name", "native_id", "synthetic", "first_seen_at_us", "last_seen_at_us", "capture_generation"}, resources},
		{"resource_snapshots", []string{"resource_id", "display_name", "availability", "completeness", "observed_at_us"}, snapshots},
		{"metric_samples", []string{"id", "resource_id", "metric_name", "unit", "value", "sampled_at_us", "interval_seconds", "synthetic"}, metrics},
		{"infrastructure_events", []string{"id", "origin_kind", "provider", "source_id", "kind", "severity", "reason_code", "summary", "occurred_at_us", "observed_at_us", "synthetic"}, events},
		{"audit_entries", []string{"id", "occurred_at_us", "actor_kind", "action", "target_kind", "outcome", "request_id", "reason_code"}, audits},
	} {
		if _, err := fixture.store.pool.CopyFrom(fixture.ctx, pgx.Identifier{copy.table}, copy.columns, pgx.CopyFromRows(copy.rows)); err != nil {
			t.Fatalf("seed query-plan %s: %v", copy.table, err)
		}
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `ANALYZE resources; ANALYZE resource_snapshots; ANALYZE metric_samples; ANALYZE infrastructure_events; ANALYZE audit_entries`); err != nil {
		t.Fatal("analyze query-plan fixtures")
	}
	plans := []struct {
		name, statement string
		arguments       []any
	}{
		{"inventory_name", `SELECT resource.id,snapshot.display_name` + activeResourceJoin + ` ORDER BY snapshot.display_name COLLATE "C",resource.id COLLATE "C" LIMIT 51`, []any{string(fixture.source.Source.ID), "demo"}},
		{"event_keyset", `SELECT id,occurred_at_us FROM infrastructure_events WHERE source_id=$1 AND provider='demo'
AND occurred_at_us>=$2 AND occurred_at_us<$3 AND occurred_at_us<=$4 AND (occurred_at_us<$4 OR (occurred_at_us=$4 AND id COLLATE "C">$5 COLLATE "C"))
ORDER BY occurred_at_us DESC,id COLLATE "C" LIMIT 51`, []any{string(fixture.source.Source.ID), fixture.at.Add(-time.Hour).UnixMicro(), fixture.at.UnixMicro(), fixture.at.Add(-time.Minute).UnixMicro(), "event_plan_1"}},
		{"current_metrics", `SELECT resource.id,sample.value FROM resources AS resource CROSS JOIN unnest($1::text[]) AS name(metric_name)
JOIN LATERAL(SELECT value FROM metric_samples WHERE resource_id=resource.id AND metric_name=name.metric_name ORDER BY sampled_at_us DESC,id COLLATE "C" LIMIT 1) AS sample ON TRUE
WHERE resource.source_id=$2 AND resource.id=ANY($3::text[]) LIMIT 100`, []any{[]string{"cpu.usage.cores"}, string(fixture.source.Source.ID), ids}},
		{"metric_retention", `SELECT item.id FROM metric_samples AS item WHERE item.sampled_at_us<$1 AND item.synthetic=0 AND ` + historicalMetricPredicate + ` ORDER BY item.sampled_at_us,item.id LIMIT 250 FOR UPDATE OF item SKIP LOCKED`, []any{fixture.at.UnixMicro()}},
		{"audit_keyset", `SELECT id,occurred_at_us FROM audit_entries
WHERE occurred_at_us<=$1 AND (occurred_at_us<$1 OR (occurred_at_us=$1 AND id COLLATE "C">$2 COLLATE "C"))
ORDER BY occurred_at_us DESC,id COLLATE "C" LIMIT 51`, []any{fixture.at.Add(-time.Minute).UnixMicro(), "audit_plan_1"}},
	}
	for _, query := range plans {
		var output []byte
		if err := fixture.store.pool.QueryRow(fixture.ctx, `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) `+query.statement, query.arguments...).Scan(&output); err != nil {
			t.Fatalf("query plan %s: %v", query.name, err)
		}
		var document []map[string]any
		if json.Unmarshal(output, &document) != nil || len(document) != 1 {
			t.Fatal("query plan output rejected")
		}
		t.Logf("query_plan %s %s", query.name, string(output))
	}
}

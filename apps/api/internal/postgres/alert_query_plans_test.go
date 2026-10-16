package postgres

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestAlertHistoryHotPathQueryPlansOnRealPostgreSQL(t *testing.T) {
	fixture := newObservationFixture(t)
	seed := durableDemoSeed(t, fixture)
	if err := fixture.store.EnsureDemoAlerts(fixture.ctx, seed); err != nil {
		t.Fatal("seed alert query-plan fixture")
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO alert_changes(source_id,rule_id,change_kind,occurred_at_us) SELECT $1,$2,'rule.changed',$3::bigint+point FROM generate_series(1,10000) AS point`, string(seed.Rule.SourceID), string(seed.Rule.ID), fixture.at.Add(-time.Hour).UnixMicro()); err != nil {
		t.Fatalf("seed alert change query-plan history: %v", err)
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO source_collection_outcomes(source_id,generation,configuration_version,collected_at_us,availability,completeness,observed_at_us,metric_availability,metric_completeness,metric_observed_at_us,event_availability,event_completeness,event_observed_at_us) SELECT $1,point+1,1,$2::bigint+point::bigint*30000000,'available','complete',$2::bigint+point::bigint*30000000,'available','complete',$2::bigint+point::bigint*30000000,'available','complete',$2::bigint+point::bigint*30000000 FROM generate_series(1,10000) AS point`, string(seed.Rule.SourceID), fixture.at.Add(-4*24*time.Hour).UnixMicro()); err != nil {
		t.Fatalf("seed collection query-plan history: %v", err)
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO workload_replica_history(resource_id,capture_generation,sampled_at_us,desired_replicas,ready_replicas,availability,completeness) SELECT $1,point+1,$2::bigint+point::bigint*30000000,3,2,'available','complete' FROM generate_series(1,10000) AS point`, string(fixture.batch.Resources[0].Resource.Ref.ID), fixture.at.Add(-4*24*time.Hour).UnixMicro()); err != nil {
		t.Fatalf("seed replica query-plan history: %v", err)
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO metric_samples(id,resource_id,metric_name,unit,value,sampled_at_us,interval_seconds,synthetic) SELECT 'metric_alert_plan_'||point,$1,$3,'cores',0.5,$2::bigint+point::bigint*30000000,30,1 FROM generate_series(1,10000) AS point`, string(fixture.batch.Resources[0].Resource.Ref.ID), fixture.at.Add(-4*24*time.Hour).UnixMicro(), string(domain.MetricCPUUsageCores)); err != nil {
		t.Fatalf("seed metric query-plan history: %v", err)
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `ANALYZE alert_changes; ANALYZE source_collection_outcomes; ANALYZE workload_replica_history; ANALYZE metric_samples`); err != nil {
		t.Fatal("analyze alert history")
	}
	queries := []struct {
		name, statement string
		arguments       []any
	}{
		{"alert_replay", `SELECT id,rule_id,change_kind FROM alert_changes WHERE source_id=$1 AND id>$2 ORDER BY id LIMIT 100`, []any{string(seed.Rule.SourceID), int64(5000)}},
		{"collection_window", `SELECT collected_at_us,availability,completeness FROM source_collection_outcomes WHERE source_id=$1 AND configuration_version=(SELECT row_version FROM infrastructure_sources WHERE id=$1) AND collected_at_us>=$2 AND collected_at_us<$3 ORDER BY collected_at_us,generation LIMIT 4097`, []any{string(seed.Rule.SourceID), fixture.at.Add(-24 * time.Hour).UnixMicro(), fixture.at.UnixMicro()}},
		{"collection_current", `SELECT collected_at_us,availability,completeness FROM source_collection_outcomes WHERE source_id=$1 AND configuration_version=1 ORDER BY generation DESC LIMIT 1`, []any{string(seed.Rule.SourceID)}},
		{"replica_window", `SELECT sampled_at_us,desired_replicas,ready_replicas FROM workload_replica_history WHERE resource_id=$1 AND sampled_at_us>=$2 AND sampled_at_us<$3 ORDER BY sampled_at_us LIMIT 4097`, []any{string(fixture.batch.Resources[0].Resource.Ref.ID), fixture.at.Add(-24 * time.Hour).UnixMicro(), fixture.at.UnixMicro()}},
		{"metric_window", `SELECT sample.id,sample.resource_id,sample.metric_name,sample.unit,sample.value,sample.sampled_at_us,sample.interval_seconds,sample.synthetic FROM metric_samples AS sample JOIN resources AS resource ON resource.id=sample.resource_id WHERE resource.source_id=$1 AND resource.provider=$2 AND sample.resource_id=$3 AND sample.metric_name=$4 AND sample.sampled_at_us>=$5 AND sample.sampled_at_us<$6 ORDER BY sample.sampled_at_us,sample.id COLLATE "C" LIMIT 4097`, []any{string(seed.Rule.SourceID), "demo", string(fixture.batch.Resources[0].Resource.Ref.ID), string(domain.MetricCPUUsageCores), fixture.at.Add(-24 * time.Hour).UnixMicro(), fixture.at.UnixMicro()}},
		{"alert_replay_prune", `SELECT item.id FROM alert_changes AS item WHERE occurred_at_us<$1 AND item.id<>(SELECT MAX(newest.id) FROM alert_changes AS newest WHERE newest.source_id=item.source_id) ORDER BY occurred_at_us,id LIMIT 33 FOR UPDATE SKIP LOCKED`, []any{fixture.at.UnixMicro()}},
		{"collection_prune", `SELECT item.source_id,item.generation FROM source_collection_outcomes AS item WHERE collected_at_us<$1 AND item.generation<>(SELECT MAX(newest.generation) FROM source_collection_outcomes AS newest WHERE newest.source_id=item.source_id) ORDER BY collected_at_us,source_id,generation LIMIT 33 FOR UPDATE SKIP LOCKED`, []any{fixture.at.UnixMicro()}},
	}
	for _, query := range queries {
		var output []byte
		if err := fixture.store.pool.QueryRow(fixture.ctx, `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) `+query.statement, query.arguments...).Scan(&output); err != nil {
			t.Fatalf("alert query-plan %s: %v", query.name, err)
		}
		var document []map[string]any
		if json.Unmarshal(output, &document) != nil || len(document) != 1 {
			t.Fatal("alert plan output rejected")
		}
		t.Logf("query_plan %s %s", query.name, string(output))
	}
}

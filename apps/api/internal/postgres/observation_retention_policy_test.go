package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/observation"
)

func seedRetentionPolicyHistory(t *testing.T, store *Store, at time.Time, age time.Duration) {
	t.Helper()
	ctx := context.Background()
	configuration := domain.SourceConfiguration{Source: domain.InfrastructureSource{ID: "source_policy", Provider: domain.ProviderDocker, Name: "Retention policy fixture"},
		ProfileID: domain.SourceProfileDockerSocketDevelopment, Enabled: true, Version: 1, CreatedAt: at.Add(-age - time.Hour), UpdatedAt: at.Add(-age - time.Hour)}
	if err := store.EnsureSourceConfigurations(ctx, []domain.SourceConfiguration{configuration}); err != nil {
		t.Fatal("seed retention policy source")
	}
	token, err := store.BeginCapture(ctx, configuration.Source.ID, 1, at)
	if err != nil {
		t.Fatal("begin retention policy capture")
	}
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: at.Add(-age)}
	environment := domain.Environment{ID: "environment_policy", SourceID: configuration.Source.ID, Provider: domain.ProviderDocker, Name: "Policy fixture", Status: status}
	identity, err := domain.NewResourceIdentity(domain.ProviderDocker, configuration.Source.ID, environment.ID, "docker.container", domain.Scope{Kind: domain.ScopeGlobal}, "policy_container")
	if err != nil {
		t.Fatal("make retention policy identity")
	}
	ref, err := domain.NewResourceRef(identity)
	if err != nil {
		t.Fatal("make retention policy reference")
	}
	batch := domain.CaptureBatch{Token: token, Environments: []domain.Environment{environment},
		Resources: []domain.ResourceSnapshot{{Resource: domain.Resource{Ref: ref, DisplayName: "Policy fixture"}, Status: status}},
		Health: []domain.HealthSnapshot{{ResourceID: ref.ID, State: domain.HealthHealthy, Status: status,
			Evidence: []domain.HealthEvidence{{ID: "evidence_policy", Code: "health.ready", SourceCode: "docker.state", Status: domain.EvidencePositive, ObservedAt: status.ObservedAt}}}},
		Metrics: []domain.MetricSample{
			{ID: "metric_policy_old", ResourceID: ref.ID, Name: domain.MetricCPUUsageCores, Unit: domain.MetricUnitCores, Value: .5, SampledAt: at.Add(-age), Interval: time.Second},
			{ID: "metric_policy_current", ResourceID: ref.ID, Name: domain.MetricCPUUsageCores, Unit: domain.MetricUnitCores, Value: .5, SampledAt: at.Add(-time.Minute), Interval: time.Second}},
		Events: []domain.InfrastructureEvent{{ID: "event_policy_old", Provider: domain.ProviderDocker, SourceID: configuration.Source.ID,
			Kind: "docker.lifecycle", Severity: domain.SeverityInfo, ReasonCode: "docker.lifecycle", Summary: "Policy fixture event", OccurredAt: at.Add(-age), ObservedAt: at.Add(-age)}}}
	if err := store.SaveCapture(ctx, batch); err != nil {
		t.Fatal("save retention policy capture")
	}
	seedObservationTerminalDelivery(t, observationFixture{store: store, ctx: ctx, at: at.Add(-age + time.Hour)}, "delivery_policy_old", "")
}

func retentionPolicyCounts(t *testing.T, store *Store) [4]int {
	t.Helper()
	var counts [4]int
	if err := store.pool.QueryRow(context.Background(), `SELECT
(SELECT count(*) FROM metric_samples WHERE id='metric_policy_old'),
(SELECT count(*) FROM health_history WHERE synthetic=0),
(SELECT count(*) FROM infrastructure_events WHERE id='event_policy_old'),
(SELECT count(*) FROM notification_deliveries WHERE id='delivery_policy_old')`).Scan(&counts[0], &counts[1], &counts[2], &counts[3]); err != nil {
		t.Fatal("read retention policy history")
	}
	var current int
	if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM metric_samples WHERE id='metric_policy_current'`).Scan(&current); err != nil || current != 1 {
		t.Fatal("retention removed newest metric")
	}
	return counts
}

type retentionPolicyChangeBetweenBatches struct {
	store *Store
	calls int
}

func (repository *retentionPolicyChangeBetweenBatches) PruneObservationHistory(ctx context.Context, at time.Time, limit int) (int64, error) {
	removed, err := repository.store.PruneObservationHistory(ctx, at, limit)
	repository.calls++
	if err == nil && repository.calls == 1 {
		_, err = repository.store.pool.Exec(ctx, `UPDATE application_settings SET metric_retention_hours=8760 WHERE singleton=1`)
	}
	return removed, err
}

func TestScheduledRetentionReloadsEffectivePolicyBetweenBoundedBatches(t *testing.T) {
	store, _, _ := openIntegrationStore(t)
	at := time.Now().UTC().Truncate(time.Microsecond)
	seedRetentionPolicyHistory(t, store, at, 8*24*time.Hour)
	if _, err := store.pool.Exec(context.Background(), `INSERT INTO metric_samples
(id,resource_id,metric_name,unit,value,sampled_at_us,interval_seconds,synthetic)
SELECT 'metric_policy_second',resource_id,metric_name,unit,value,sampled_at_us+1000000,interval_seconds,synthetic FROM metric_samples WHERE id='metric_policy_old'`); err != nil {
		t.Fatal("seed second eligible metric")
	}
	repository := &retentionPolicyChangeBetweenBatches{store: store}
	worker, err := observation.NewRetentionWorker(observation.RetentionOptions{Repository: repository, Now: func() time.Time { return at },
		PollInterval: time.Minute, Timeout: 10 * time.Second, BatchLimit: 4, MaximumBatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	removed, err := worker.RunOnce(context.Background())
	if err != nil || removed != 1 || repository.calls != 2 {
		t.Fatal("scheduled retention did not reload policy for its next bounded batch")
	}
	var remaining int
	if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM metric_samples WHERE id='metric_policy_second'`).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatal("scheduled retention deleted history protected by the newer policy")
	}
}

func TestScheduledRetentionUsesEffectiveApplicationPolicyForEveryClass(t *testing.T) {
	for _, check := range []struct {
		name  string
		age   time.Duration
		hours [4]int
		want  [4]int
	}{
		{"default_policy_control", 8 * 24 * time.Hour, [4]int{168, 720, 720, 720}, [4]int{0, 1, 1, 1}},
		{"increased_policy_preserves_eight_day_metric", 8 * 24 * time.Hour, [4]int{8760, 8760, 8760, 8760}, [4]int{1, 1, 1, 1}},
		{"increased_policy_preserves_all_classes", 31 * 24 * time.Hour, [4]int{8760, 8760, 8760, 8760}, [4]int{1, 1, 1, 1}},
		{"decreased_policy_expires_all_classes", 2 * time.Hour, [4]int{1, 1, 1, 1}, [4]int{}},
		{"independent_class_policies", 8 * 24 * time.Hour, [4]int{8760, 1, 8760, 1}, [4]int{1, 0, 1, 0}},
	} {
		t.Run(check.name, func(t *testing.T) {
			store, _, _ := openIntegrationStore(t)
			at := time.Now().UTC().Truncate(time.Microsecond)
			seedRetentionPolicyHistory(t, store, at, check.age)
			if _, err := store.pool.Exec(context.Background(), `UPDATE application_settings SET metric_retention_hours=$1,health_retention_hours=$2,event_retention_hours=$3,delivery_retention_hours=$4 WHERE singleton=1`, check.hours[0], check.hours[1], check.hours[2], check.hours[3]); err != nil {
				t.Fatal("set effective retention policy")
			}
			worker, err := observation.NewRetentionWorker(observation.RetentionOptions{Repository: store, Now: func() time.Time { return at },
				PollInterval: time.Minute, Timeout: 10 * time.Second, BatchLimit: 4, MaximumBatches: 10})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := worker.RunOnce(context.Background()); err != nil {
				t.Fatal("run scheduled effective retention")
			}
			if got := retentionPolicyCounts(t, store); got != check.want {
				t.Fatalf("effective retention mismatch: got=%v want=%v", got, check.want)
			}
		})
	}
}

func waitRetentionBlockedBy(t *testing.T, store *Store, blocker uint32) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		if err := store.pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1::INTEGER=ANY(pg_blocking_pids(pid)))`, int32(blocker)).Scan(&waiting); err != nil {
			t.Fatal("inspect owned retention transaction wait")
		}
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("retention did not wait for owned transaction")
}

func TestRetentionSerializesConcurrentPolicyIncreaseBeforeDeletion(t *testing.T) {
	store, _, _ := openIntegrationStore(t)
	at := time.Now().UTC().Truncate(time.Microsecond)
	seedRetentionPolicyHistory(t, store, at, 8*24*time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	update, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatal("begin owned policy update")
	}
	defer update.Rollback(context.Background())
	if _, err := update.Exec(ctx, `UPDATE application_settings SET metric_retention_hours=8760 WHERE singleton=1`); err != nil {
		t.Fatal("hold concurrent policy increase")
	}
	result := make(chan error, 1)
	go func() {
		_, err := store.PruneObservationHistory(ctx, at, 4)
		result <- err
	}()
	waitRetentionBlockedBy(t, store, update.Conn().PgConn().PID())
	if err := update.Commit(ctx); err != nil {
		t.Fatal("commit concurrent policy increase")
	}
	if err := <-result; err != nil {
		t.Fatal("finish serialized retention")
	}
	if got := retentionPolicyCounts(t, store); got != ([4]int{1, 1, 1, 1}) {
		t.Fatal("retention used policy preceding the committed increase")
	}
}

func TestRetentionHoldsSharedPolicyLockThroughDeletion(t *testing.T) {
	store, _, _ := openIntegrationStore(t)
	at := time.Now().UTC().Truncate(time.Microsecond)
	seedRetentionPolicyHistory(t, store, at, 8*24*time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	blocker, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatal("begin owned deletion blocker")
	}
	defer blocker.Rollback(context.Background())
	if _, err := blocker.Exec(ctx, `LOCK TABLE metric_samples IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal("hold owned metric table")
	}
	result := make(chan error, 1)
	go func() {
		_, err := store.PruneObservationHistory(ctx, at, 4)
		result <- err
	}()
	waitRetentionBlockedBy(t, store, blocker.Conn().PgConn().PID())
	updateContext, updateCancel := context.WithTimeout(ctx, 150*time.Millisecond)
	_, updateErr := store.pool.Exec(updateContext, `UPDATE application_settings SET metric_retention_hours=8760 WHERE singleton=1`)
	updateCancel()
	if updateErr == nil || updateContext.Err() != context.DeadlineExceeded {
		t.Fatal("policy increase passed a live retention batch")
	}
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal("release owned deletion blocker")
	}
	if err := <-result; err != nil {
		t.Fatal("finish retention under shared policy lock")
	}
	if got := retentionPolicyCounts(t, store); got != ([4]int{0, 1, 1, 1}) {
		t.Fatal("batch changed policy while deleting")
	}
}

package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

type DeploymentProfile struct {
	Demo           bool
	Guest          bool
	DataRoot       string
	FixtureVersion string
	Reset          bool
}

var fixtureVersionPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)

// ConfigureDeployment runs under process ownership before any listener or job.
// A failed or interrupted reset stays pending; the next startup finishes it.
func (store *Store) ConfigureDeployment(ctx context.Context, profile DeploymentProfile) error {
	if ctx == nil || profile.DataRoot == "" || !fixtureVersionPattern.MatchString(profile.FixtureVersion) || (profile.Reset || profile.Guest) && !profile.Demo {
		return domain.ErrRepositoryInvalidInput
	}
	mode := "normal"
	if profile.Demo {
		mode = "demo"
	}
	digest := sha256.Sum256([]byte(profile.DataRoot))
	rootDigest := hex.EncodeToString(digest[:])
	reset := false
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		var previousMode, previousRoot, previousVersion string
		var pending bool
		err := transaction.transaction.QueryRow(ctx, `SELECT mode,data_root_digest,fixture_version,reset_pending FROM deployment_profile WHERE singleton=true FOR UPDATE`).Scan(&previousMode, &previousRoot, &previousVersion, &pending)
		fresh := errors.Is(err, pgx.ErrNoRows)
		if err != nil && !fresh {
			return err
		}
		if !fresh && (previousMode != mode || profile.Demo && previousRoot != rootDigest) {
			return domain.ErrRepositoryConflict
		}
		if profile.Demo {
			var contaminated bool
			if err := transaction.transaction.QueryRow(ctx, `SELECT
EXISTS(SELECT 1 FROM infrastructure_sources WHERE provider<>'demo' OR synthetic<>1 OR id<>'source_demo')
OR EXISTS(SELECT 1 FROM resources WHERE synthetic<>1 OR provider<>'demo')
OR EXISTS(SELECT 1 FROM environments WHERE synthetic<>1 OR provider<>'demo')
OR EXISTS(SELECT 1 FROM infrastructure_events WHERE synthetic<>1 OR provider<>'demo')
OR EXISTS(SELECT 1 FROM metric_samples WHERE synthetic<>1)
OR EXISTS(SELECT 1 FROM notifications WHERE synthetic<>1)
OR EXISTS(SELECT 1 FROM alerts WHERE synthetic<>1)
OR EXISTS(SELECT 1 FROM source_connection_references)`).Scan(&contaminated); err != nil {
				return err
			}
			if contaminated {
				return domain.ErrRepositoryConflict
			}
			if fresh {
				var previouslyUsed bool
				if err := transaction.transaction.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users) OR EXISTS(SELECT 1 FROM infrastructure_sources) OR EXISTS(SELECT 1 FROM audit_entries)`).Scan(&previouslyUsed); err != nil {
					return err
				}
				if previouslyUsed {
					return domain.ErrRepositoryConflict
				}
			}
		}
		var legacyFixture bool
		if fresh {
			if err := transaction.transaction.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM infrastructure_sources WHERE id='source_demo' AND provider='demo' AND synthetic=1)`).Scan(&legacyFixture); err != nil {
				return err
			}
		}
		reset = pending || profile.Reset || profile.Demo || !fresh && previousVersion != profile.FixtureVersion || legacyFixture
		_, err = transaction.transaction.Exec(ctx, `INSERT INTO deployment_profile(singleton,mode,data_root_digest,fixture_version,reset_pending,guest_demo)
VALUES(true,$1,$2,$3,$4,$5) ON CONFLICT(singleton) DO UPDATE SET reset_pending=EXCLUDED.reset_pending,data_root_digest=EXCLUDED.data_root_digest,guest_demo=EXCLUDED.guest_demo`, mode, rootDigest, profile.FixtureVersion, reset, profile.Guest)
		return err
	})
	if err != nil {
		return mapRepositoryWriteError(ctx, err)
	}
	if reset {
		if err := store.resetDemoRuntime(ctx, profile.Demo); err != nil {
			return err
		}
	}
	return mapRepositoryWriteError(ctx, store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		_, err := transaction.transaction.Exec(ctx, `UPDATE deployment_profile SET fixture_version=$1,reset_pending=false,reset_epoch=reset_epoch+CASE WHEN reset_pending THEN 1 ELSE 0 END WHERE singleton=true`, profile.FixtureVersion)
		return err
	}))
}

func (store *Store) resetDemoRuntime(ctx context.Context, isolatedDemo bool) error {
	// Keep identities, users, authentication records and protected audit history.
	// Every mutable statement is scoped to the packaged synthetic source.
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		_, err := transaction.transaction.Exec(ctx, `INSERT INTO alert_change_retention_positions(source_id,pruned_through_id)
SELECT source.id,COALESCE((SELECT max(id) FROM alert_changes WHERE source_id=source.id),0)
FROM infrastructure_sources AS source WHERE source.id='source_demo' AND source.provider='demo' AND source.synthetic=1
ON CONFLICT(source_id) DO UPDATE SET pruned_through_id=GREATEST(alert_change_retention_positions.pruned_through_id,EXCLUDED.pruned_through_id)`)
		return err
	})
	if err != nil {
		return mapRepositoryWriteError(ctx, err)
	}
	resource := `EXISTS(SELECT 1 FROM resources AS resource WHERE resource.id=item.resource_id AND resource.source_id=$1 AND resource.provider='demo' AND resource.synthetic=1)`
	rules := `item.source_id=$1 AND (item.system_owned OR item.id=$2)`
	if isolatedDemo {
		rules = `item.source_id=$1`
	}
	for _, target := range []struct{ table, guard string }{
		{"notification_deliveries", `EXISTS(SELECT 1 FROM notifications AS notice WHERE notice.id=item.notification_id AND notice.source_id=$1 AND notice.synthetic=1)`},
		{"notifications", `item.source_id=$1 AND item.synthetic=1`},
		{"alert_changes", `item.source_id=$1`},
		{"alert_transitions", `EXISTS(SELECT 1 FROM alerts AS alert WHERE alert.id=item.alert_id AND alert.source_id=$1 AND alert.synthetic=1)`},
		{"alerts", `item.source_id=$1 AND item.synthetic=1`},
		{"alert_rules", rules},
		{"demo_alert_seeds", `item.source_id=$1`},
		{"source_collection_outcomes", `item.source_id=$1`},
		{"workload_replica_history", resource},
		{"health_evidence", resource},
		{"resource_health_current", resource},
		{"health_history", resource + ` AND item.synthetic=1`},
		{"metric_samples", resource + ` AND item.synthetic=1`},
		{"infrastructure_events", `item.source_id=$1 AND item.provider='demo' AND item.synthetic=1`},
		{"relationship_evidence", `EXISTS(SELECT 1 FROM relationships AS relation WHERE relation.id=item.relationship_id AND relation.source_id=$1)`},
		{"relationships", `item.source_id=$1`},
		{"resource_snapshots", resource},
		{"inventory_partitions", `item.source_id=$1`},
		{"source_status", `item.source_id=$1`},
		{"source_capabilities", `item.source_id=$1`},
	} {
		arguments := []any{"source_demo"}
		if target.table == "alert_rules" && !isolatedDemo {
			arguments = append(arguments, string(domain.DemoCheckoutHealthRuleID))
		}
		finished := false
		for range 100 {
			var removed int64
			err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
				result, err := transaction.transaction.Exec(ctx, `WITH selected AS (SELECT item.ctid FROM `+target.table+` AS item
WHERE `+target.guard+` LIMIT 1000 FOR UPDATE OF item)
DELETE FROM `+target.table+` AS item USING selected WHERE item.ctid=selected.ctid`, arguments...)
				removed = result.RowsAffected()
				return err
			})
			if err != nil {
				return mapRepositoryWriteError(ctx, err)
			}
			if removed < 1000 {
				finished = true
				break
			}
		}
		if !finished {
			return domain.ErrRepositoryUnavailable
		}
	}
	for _, table := range []string{"resources", "environments"} {
		finished := false
		for range 100 {
			var changed int64
			err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
				result, err := transaction.transaction.Exec(ctx, `WITH selected AS (SELECT id FROM `+table+` WHERE source_id=$1 AND synthetic=1 AND retired_at_us IS NULL LIMIT 1000 FOR UPDATE)
UPDATE `+table+` AS item SET retired_at_us=item.last_seen_at_us,row_version=item.row_version+1 FROM selected WHERE item.id=selected.id`, "source_demo")
				changed = result.RowsAffected()
				return err
			})
			if err != nil {
				return mapRepositoryWriteError(ctx, err)
			}
			if changed < 1000 {
				finished = true
				break
			}
		}
		if !finished {
			return domain.ErrRepositoryUnavailable
		}
	}
	return mapRepositoryWriteError(ctx, store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		_, err := transaction.transaction.Exec(ctx, `UPDATE infrastructure_sources SET enabled=CASE WHEN $2 THEN 1 ELSE enabled END,row_version=row_version+1,partition_generation=0,capture_started_at_us=NULL,capture_finished_at_us=NULL
WHERE id=$1 AND provider='demo' AND synthetic=1`, "source_demo", isolatedDemo)
		return err
	}))
}

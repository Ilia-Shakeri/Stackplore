package postgres

import (
	"context"
	"strings"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Aliases in these fixed SQL fragments are repository-owned, never input.
const matchingPartition = `partition.source_id=resource.source_id AND partition.environment_id=resource.environment_id
AND partition.kind=resource.kind AND partition.coverage_generation=source.partition_generation
AND (partition.scope_mode='all_scopes' OR (partition.scope_kind=resource.scope_kind AND partition.scope_name=resource.scope_name))`

const activeResourcePredicate = `NOT source.operator_policy_pending AND resource.retired_at_us IS NULL AND (
(source.partition_generation=0 AND resource.capture_generation>=source.completed_generation)
OR (source.partition_generation>0 AND EXISTS(SELECT 1 FROM inventory_partitions AS partition WHERE ` + matchingPartition + `
AND partition.availability NOT IN ('permission_denied','unsupported') AND resource.capture_generation>=partition.completed_generation)))`

const activeEnvironmentPredicate = `NOT source.operator_policy_pending AND environment.retired_at_us IS NULL AND (
(source.partition_generation=0 AND environment.capture_generation>=source.completed_generation)
OR (source.partition_generation>0 AND EXISTS(SELECT 1 FROM inventory_partitions AS partition
WHERE partition.source_id=environment.source_id AND partition.environment_id=environment.id
AND partition.coverage_generation=source.partition_generation
AND partition.availability NOT IN ('permission_denied','unsupported'))))`

const partitionStale = `source.partition_generation>0 AND EXISTS(SELECT 1 FROM inventory_partitions AS partition WHERE ` + matchingPartition + ` AND partition.availability IN ('unavailable','stale'))`
const partitionPartial = `source.partition_generation>0 AND EXISTS(SELECT 1 FROM inventory_partitions AS partition WHERE ` + matchingPartition + ` AND partition.completeness='partial')`

const authorizedResourcePredicate = `(source.partition_generation=0 OR EXISTS(SELECT 1 FROM inventory_partitions AS partition WHERE ` + matchingPartition + ` AND partition.availability NOT IN ('permission_denied','unsupported')))`

const retiredPartitionResourcePredicate = `source.partition_generation>0 AND EXISTS(SELECT 1 FROM inventory_partitions AS partition WHERE ` + matchingPartition + ` AND partition.completed_generation=source.partition_generation AND resource.capture_generation<partition.completed_generation)`

func activeIdentityPredicate(alias string) string {
	return strings.ReplaceAll(activeResourcePredicate, "resource.", alias+".")
}

func saveInventoryPartitions(ctx context.Context, tx pgx.Tx, completion domain.CaptureCompletion) error {
	if len(completion.Coverage) == 0 {
		return nil
	}
	count := len(completion.Coverage)
	environments, kinds, modes, scopeKinds, scopeNames := make([]string, count), make([]string, count), make([]string, count), make([]string, count), make([]string, count)
	states, completeness, reasons := make([]string, count), make([]string, count), make([]string, count)
	observed := make([]int64, count)
	for index, item := range completion.Coverage {
		environments[index], kinds[index], modes[index] = string(item.EnvironmentID), string(item.Kind), string(item.ScopeMode)
		scopeKinds[index], scopeNames[index] = string(item.Scope.Kind), item.Scope.Name
		states[index], completeness[index], reasons[index] = string(item.Status.State), string(item.Status.Completeness), string(item.Status.ReasonCode)
		observed[index] = item.Status.ObservedAt.UnixMicro()
	}
	result, err := tx.Exec(ctx, `INSERT INTO inventory_partitions
(source_id,environment_id,kind,scope_mode,scope_kind,scope_name,coverage_generation,completed_generation,availability,completeness,observed_at_us,reason_code)
SELECT $1,entry.environment_id,entry.kind,entry.scope_mode,entry.scope_kind,entry.scope_name,$2,
CASE WHEN entry.availability='available' AND entry.completeness='complete' THEN $2 ELSE source.completed_generation END,
entry.availability,entry.completeness,entry.observed_at_us,NULLIF(entry.reason_code,'')
FROM unnest($3::text[],$4::text[],$5::text[],$6::text[],$7::text[],$8::text[],$9::text[],$10::bigint[],$11::text[])
AS entry(environment_id,kind,scope_mode,scope_kind,scope_name,availability,completeness,observed_at_us,reason_code)
JOIN environments AS environment ON environment.id=entry.environment_id AND environment.source_id=$1
JOIN infrastructure_sources AS source ON source.id=environment.source_id
ON CONFLICT(source_id,environment_id,kind,scope_mode,scope_kind,scope_name) DO UPDATE SET
coverage_generation=EXCLUDED.coverage_generation,
completed_generation=CASE WHEN EXCLUDED.availability='available' AND EXCLUDED.completeness='complete'
THEN EXCLUDED.coverage_generation ELSE inventory_partitions.completed_generation END,
availability=EXCLUDED.availability,completeness=EXCLUDED.completeness,observed_at_us=EXCLUDED.observed_at_us,reason_code=EXCLUDED.reason_code`,
		string(completion.Token.Source.SourceID), completion.Token.Generation, environments, kinds, modes, scopeKinds, scopeNames, states, completeness, observed, reasons)
	if err == nil && result.RowsAffected() != int64(count) {
		return domain.ErrRepositoryConflict
	}
	return err
}

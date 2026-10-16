package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (store *Store) GetWorkloadReplicaHistory(ctx context.Context, query domain.WorkloadReplicaHistoryQuery) (domain.WorkloadReplicaHistoryResult, error) {
	if ctx == nil || query.Validate() != nil {
		return domain.WorkloadReplicaHistoryResult{}, domain.ErrRepositoryInvalidInput
	}
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.WorkloadReplicaHistoryResult, error) {
		status, err := reader.GetObservationStatus(ctx, query.Source)
		if err != nil {
			return domain.WorkloadReplicaHistoryResult{}, err
		}
		result := domain.WorkloadReplicaHistoryResult{Status: status, Samples: []domain.WorkloadReplicaSnapshot{}}
		if !status.AllowsData() {
			return result, nil
		}
		current, err := reader.GetWorkloadReplicas(ctx, domain.ResourceDetailQuery{Source: query.Source, ResourceID: query.Resource.ID})
		if err != nil {
			return domain.WorkloadReplicaHistoryResult{}, err
		}
		if !current.Status.AllowsData() || current.Status.Completeness != domain.CompletenessComplete {
			result.Status = current.Status
			return result, nil
		}
		rows, err := reader.queries.Query(ctx, `SELECT sample.sampled_at_us,sample.desired_replicas,sample.ready_replicas,sample.availability,sample.completeness,sample.reason_code FROM workload_replica_history AS sample JOIN resources AS resource ON resource.id=sample.resource_id WHERE resource.source_id=$1 AND resource.provider=$2 AND resource.id=$3 AND sample.sampled_at_us>=$4 AND sample.sampled_at_us<$5 ORDER BY sample.sampled_at_us LIMIT $6`, string(query.Source.SourceID), string(query.Source.Provider), string(query.Resource.ID), query.Range.Start.UnixMicro(), query.Range.End.UnixMicro(), query.Limit+1)
		if err != nil {
			return domain.WorkloadReplicaHistoryResult{}, mapRepositoryReadError(ctx, err)
		}
		defer rows.Close()
		for rows.Next() {
			var at, desired, ready int64
			var state, complete string
			var reason sql.NullString
			if err := rows.Scan(&at, &desired, &ready, &state, &complete, &reason); err != nil {
				return domain.WorkloadReplicaHistoryResult{}, domain.ErrRepositoryUnavailable
			}
			sample := domain.WorkloadReplicaSnapshot{Resource: query.Resource, Desired: uint64(desired), Ready: uint64(ready), Status: domain.ObservationStatus{State: domain.AvailabilityState(state), Completeness: domain.Completeness(complete), ObservedAt: time.UnixMicro(at).UTC(), ReasonCode: domain.ReasonCode(reason.String)}}
			if sample.ValidateFor(domain.ResourceDetailQuery{Source: query.Source, ResourceID: query.Resource.ID}) != nil {
				return domain.WorkloadReplicaHistoryResult{}, domain.ErrRepositoryUnavailable
			}
			result.Samples = append(result.Samples, sample)
		}
		if err := rows.Err(); err != nil {
			return domain.WorkloadReplicaHistoryResult{}, mapRepositoryReadError(ctx, err)
		}
		if len(result.Samples) > query.Limit {
			result.Samples = result.Samples[:query.Limit]
			result.Status.Completeness = domain.CompletenessPartial
			result.Status.ReasonCode = "source.workload_history_limited"
		}
		return result, nil
	})
}

var _ domain.WorkloadReplicaHistoryProvider = (*Store)(nil)

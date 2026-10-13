package postgres

import (
	"context"
	"strconv"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

type observationRead struct {
	*Store
	queries pgx.Tx
}

// One logical result uses one bounded database snapshot, including its source
// status and evidence. Pagination across separate requests remains a keyset view.
func readObservation[T any](ctx context.Context, store *Store, read func(context.Context, *observationRead) (T, error)) (result T, err error) {
	if ctx == nil || read == nil {
		return result, domain.ErrRepositoryInvalidInput
	}
	if !store.available() {
		return result, domain.ErrRepositoryUnavailable
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, mapRepositoryReadError(ctx, err)
	}
	defer func() {
		rollbackContext, rollbackCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer rollbackCancel()
		_ = tx.Rollback(rollbackContext)
	}()
	if _, err := tx.Exec(ctx, `SELECT set_config('statement_timeout',$1,true)`, strconv.FormatInt(store.queryTimeout.Milliseconds(), 10)+"ms"); err != nil {
		return result, mapRepositoryReadError(ctx, err)
	}
	result, err = read(ctx, &observationRead{Store: store, queries: tx})
	if err != nil {
		var empty T
		return empty, err
	}
	if err := tx.Commit(ctx); err != nil {
		var empty T
		return empty, mapRepositoryReadError(ctx, err)
	}
	return result, nil
}

func (store *Store) GetObservationStatus(ctx context.Context, source domain.ProviderRef) (domain.ObservationStatus, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.ObservationStatus, error) {
		return reader.GetObservationStatus(ctx, source)
	})
}

func (store *Store) GetCapabilities(ctx context.Context, source domain.ProviderRef) (domain.CapabilitySet, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.CapabilitySet, error) {
		return reader.GetCapabilities(ctx, source)
	})
}

func (store *Store) ListEnvironments(ctx context.Context, query domain.EnvironmentQuery) (domain.EnvironmentPage, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.EnvironmentPage, error) {
		return reader.ListEnvironments(ctx, query)
	})
}

func (store *Store) ListResources(ctx context.Context, query domain.InventoryQuery) (domain.ResourcePage, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.ResourcePage, error) {
		return reader.ListResources(ctx, query)
	})
}

func (store *Store) GetResource(ctx context.Context, query domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.ResourceSnapshot, error) {
		return reader.GetResource(ctx, query)
	})
}

func (store *Store) GetWorkloadReplicas(ctx context.Context, query domain.ResourceDetailQuery) (domain.WorkloadReplicaSnapshot, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.WorkloadReplicaSnapshot, error) {
		return reader.GetWorkloadReplicas(ctx, query)
	})
}

func (store *Store) GetHealth(ctx context.Context, query domain.HealthQuery) (domain.HealthResult, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.HealthResult, error) {
		return reader.GetHealth(ctx, query)
	})
}

func (store *Store) GetRelationships(ctx context.Context, query domain.TopologyQuery) (domain.RelationshipPage, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.RelationshipPage, error) {
		return reader.GetRelationships(ctx, query)
	})
}

func (store *Store) GetCurrentMetrics(ctx context.Context, query domain.CurrentMetricQuery) (domain.CurrentMetricResult, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.CurrentMetricResult, error) {
		return reader.GetCurrentMetrics(ctx, query)
	})
}

func (store *Store) GetMetricHistory(ctx context.Context, query domain.MetricHistoryQuery) (domain.MetricHistoryResult, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.MetricHistoryResult, error) {
		return reader.GetMetricHistory(ctx, query)
	})
}

func (store *Store) ListEvents(ctx context.Context, query domain.EventQuery) (domain.EventPage, error) {
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.EventPage, error) {
		return reader.ListEvents(ctx, query)
	})
}

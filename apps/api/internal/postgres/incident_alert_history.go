package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// GetIncidentAlertHistory uses the selected resource and half-open window in SQL.
// It never loads a source-wide alert page or follows an application-side cursor.
func (store *Store) GetIncidentAlertHistory(ctx context.Context, query domain.IncidentAlertHistoryQuery) (domain.IncidentAlertHistory, error) {
	if ctx == nil || query.Validate() != nil {
		return domain.IncidentAlertHistory{}, domain.ErrRepositoryInvalidInput
	}
	return readObservation(ctx, store, func(ctx context.Context, reader *observationRead) (domain.IncidentAlertHistory, error) {
		status, err := reader.GetObservationStatus(ctx, query.Source)
		if err != nil {
			return domain.IncidentAlertHistory{}, err
		}
		result := domain.IncidentAlertHistory{Status: status, Transitions: []domain.IncidentAlertTransition{}}
		if !status.AllowsData() {
			return result, nil
		}
		current, err := reader.GetResource(ctx, domain.ResourceDetailQuery{Source: query.Source, ResourceID: query.Resource.ID})
		if err != nil {
			return domain.IncidentAlertHistory{}, err
		}
		if current.Resource.Ref != query.Resource {
			return domain.IncidentAlertHistory{}, domain.ErrRepositoryUnavailable
		}
		if !current.Status.AllowsData() {
			result.Status = current.Status
			return result, nil
		}
		rows, err := reader.queries.Query(ctx, `SELECT transition.id,transition.alert_id,alert.resource_id,transition.from_state,transition.to_state,transition.occurred_at_us,alert.synthetic FROM alert_transitions AS transition JOIN alerts AS alert ON alert.id=transition.alert_id JOIN resources AS resource ON resource.id=alert.resource_id AND resource.source_id=alert.source_id WHERE alert.source_id=$1 AND alert.resource_id=$2 AND resource.provider=$3 AND transition.occurred_at_us>=$4 AND transition.occurred_at_us<$5 ORDER BY transition.occurred_at_us,transition.id COLLATE "C" LIMIT $6`, string(query.Source.SourceID), string(query.Resource.ID), string(query.Source.Provider), query.Range.Start.UnixMicro(), query.Range.End.UnixMicro(), query.Limit+1)
		if err != nil {
			return domain.IncidentAlertHistory{}, mapRepositoryReadError(ctx, err)
		}
		defer rows.Close()
		for rows.Next() {
			var item domain.IncidentAlertTransition
			var from sql.NullString
			var to string
			var at int64
			var synthetic int64
			if err := rows.Scan(&item.ID, &item.AlertID, &item.ResourceID, &from, &to, &at, &synthetic); err != nil {
				return domain.IncidentAlertHistory{}, domain.ErrRepositoryUnavailable
			}
			if synthetic != 0 && synthetic != 1 {
				return domain.IncidentAlertHistory{}, domain.ErrRepositoryUnavailable
			}
			if from.Valid {
				state := domain.AlertState(from.String)
				item.From = &state
			}
			item.To, item.OccurredAt = domain.AlertState(to), time.UnixMicro(at).UTC()
			item.Synthetic = synthetic == 1
			result.Transitions = append(result.Transitions, item)
		}
		if err := rows.Err(); err != nil {
			return domain.IncidentAlertHistory{}, mapRepositoryReadError(ctx, err)
		}
		if len(result.Transitions) > query.Limit {
			result.Transitions = result.Transitions[:query.Limit]
			result.MoreAvailable = true
			result.Status.Completeness, result.Status.ReasonCode = domain.CompletenessPartial, "source.alert_history_limited"
		}
		if result.ValidateFor(query) != nil {
			return domain.IncidentAlertHistory{}, domain.ErrRepositoryUnavailable
		}
		return result, nil
	})
}

var _ domain.IncidentAlertHistoryReader = (*Store)(nil)

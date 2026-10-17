package postgres

import (
	"context"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/operation"
	"strconv"
)

func (s *Store) GetExecution(ctx context.Context, actor auth.Principal, id string) (operation.ExecutionRecord, error) {
	var record operation.ExecutionRecord
	if actor.Validate() != nil || actor.Role != auth.RoleAdministrator || domain.RequestID(id).Validate() != nil {
		return record, operation.ErrInvalid
	}
	err := s.withinAuthTransaction(ctx, func(ctx context.Context, t *transaction) error {
		at, err := operationTransactionTime(ctx, t.transaction)
		if err != nil {
			return err
		}
		if err = operationActor(ctx, t.transaction, actor, at, true); err != nil {
			return err
		}
		record, err = scanOperation(t.transaction.QueryRow(ctx, `SELECT `+operationColumns+` FROM operation_executions WHERE id=$1`, id))
		return err
	})
	return record, operationRepositoryError(err)
}
func (s *Store) ListExecutions(ctx context.Context, actor auth.Principal, source domain.SourceID, page domain.PageRequest) (operation.ExecutionPage, error) {
	result := operation.ExecutionPage{Executions: []operation.ExecutionRecord{}, Page: domain.PageInfo{Limit: page.Limit}}
	if actor.Validate() != nil || actor.Role != auth.RoleAdministrator || page.Validate() != nil || page.Limit > 100 || source != "" && source.Validate() != nil {
		return result, operation.ErrInvalid
	}
	queryHash := observationQueryHash([]string{string(actor.UserID), string(source)})
	cursor, err := s.decodeObservationCursor(page.Cursor, "operations", queryHash)
	if err != nil {
		return result, operation.ErrInvalid
	}
	var after int64
	afterID := ""
	if cursor != nil {
		after, err = strconv.ParseInt(cursor.Value, 10, 64)
		if err != nil || after < 0 {
			return result, operation.ErrInvalid
		}
		afterID = cursor.ID
	}
	err = s.withinAuthTransaction(ctx, func(ctx context.Context, t *transaction) error {
		at, err := operationTransactionTime(ctx, t.transaction)
		if err != nil {
			return err
		}
		if err = operationActor(ctx, t.transaction, actor, at, true); err != nil {
			return err
		}
		rows, err := t.transaction.Query(ctx, `SELECT `+operationColumns+` FROM operation_executions WHERE ($1::text='' OR source_id=$1) AND ($2::text='' OR (created_at_us,id COLLATE "C")<($3,$2 COLLATE "C")) ORDER BY created_at_us DESC,id COLLATE "C" DESC LIMIT $4`, string(source), afterID, after, page.Limit+1)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			record, err := scanOperation(rows)
			if err != nil {
				return err
			}
			result.Executions = append(result.Executions, record)
		}
		return rows.Err()
	})
	if err != nil {
		return result, operationRepositoryError(err)
	}
	if len(result.Executions) > page.Limit {
		result.Executions = result.Executions[:page.Limit]
		result.Page.HasMore = true
		last := result.Executions[len(result.Executions)-1]
		result.Page.NextCursor, err = s.encodeObservationCursor("operations", queryHash, strconv.FormatInt(last.CreatedAt.UnixMicro(), 10), last.ID)
		if err != nil {
			return result, operation.ErrUnavailable
		}
	}
	return result, nil
}

package postgres

import (
	"context"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/jackc/pgx/v5"
)

var _ assistant.HistoryRepository = (*Store)(nil)

// State is mandatory so the existing owner/state/time index can bound the page.
// Current actor and restore generation are checked independently of old sessions.
const assistantHistorySQL = `SELECT id,source_id,COALESCE(resource_id,''),goal,state,
created_at_us,updated_at_us,from_us,until_us,tool_calls,inference_calls,usage_unknown
FROM assistant_runs
WHERE owner_id=$1 AND state=$2 AND created_at_us<=$3 AND created_at_us>=$4
AND restore_generation=$5 AND goal IN ('explain_resource','investigate_incident')
AND ($6::bigint=0 OR (created_at_us,id)<($6,$7))
ORDER BY created_at_us DESC,id DESC LIMIT $8`

func (store *Store) ListAssistantHistory(ctx context.Context, actor assistant.Actor, query assistant.HistoryQuery, at time.Time) (assistant.HistoryPage, error) {
	if ctx == nil || store == nil || !actor.Valid() || query.Validate(at) != nil {
		return assistant.HistoryPage{}, assistant.ErrInvalid
	}
	page := assistant.HistoryPage{Entries: make([]assistant.HistoryEntry, 0, query.Limit)}
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := assistantActor(ctx, tx, actor, at); err != nil {
			return err
		}
		var generation int64
		if err := tx.QueryRow(ctx, `SELECT restore_generation FROM installation_metadata WHERE singleton_id=1 FOR SHARE`).Scan(&generation); err != nil {
			return err
		}
		if query.Generation >= 0 && query.Generation != generation {
			return assistant.ErrDenied
		}
		page.Generation = generation
		before := int64(0)
		if !query.BeforeAt.IsZero() {
			before = query.BeforeAt.UnixMicro()
		}
		rows, err := tx.Query(ctx, assistantHistorySQL, actor.UserID, string(query.State), query.UpperBound.UnixMicro(), max(int64(0), at.Add(-assistant.MetadataRetention).UnixMicro()), generation, before, query.BeforeID, query.Limit+1)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var entry assistant.HistoryEntry
			var created, updated, from, until int64
			if err := rows.Scan(&entry.ID, &entry.SourceID, &entry.ResourceID, &entry.Goal, &entry.State, &created, &updated, &from, &until, &entry.ToolCalls, &entry.InferenceCalls, &entry.UsageUncertain); err != nil {
				return err
			}
			entry.CreatedAt, entry.UpdatedAt, entry.From, entry.Until = time.UnixMicro(created).UTC(), time.UnixMicro(updated).UTC(), time.UnixMicro(from).UTC(), time.UnixMicro(until).UTC()
			if entry.Validate(query, at) != nil {
				return assistant.ErrUnavailable
			}
			if len(page.Entries) == query.Limit {
				page.HasMore = true
				continue
			}
			page.Entries = append(page.Entries, entry)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows.Close()
		return assistantActor(ctx, tx, actor, at)
	})
	if err != nil {
		clear(page.Entries)
		return assistant.HistoryPage{}, err
	}
	return page, nil
}

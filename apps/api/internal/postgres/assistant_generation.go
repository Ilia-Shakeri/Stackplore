package postgres

import (
	"context"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/jackc/pgx/v5"
)

// AssistantRestoreGeneration exposes only the current metadata fence. Admission
// rechecks it atomically; this read never settles usage or modifies restore state.
func (store *Store) AssistantRestoreGeneration(ctx context.Context) (int64, error) {
	if store == nil {
		return 0, assistant.ErrUnavailable
	}
	var generation int64
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := assistantLock(ctx, tx); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT restore_generation FROM installation_metadata WHERE singleton_id=1 FOR SHARE`).Scan(&generation); err != nil {
			return assistant.ErrUnavailable
		}
		if generation < 0 {
			return assistant.ErrUnavailable
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return generation, nil
}

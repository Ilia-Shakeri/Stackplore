package postgres

import (
	"context"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/jackc/pgx/v5"
)

// GetAssistantSettings returns trusted metadata for application composition.
// It does not enable dispatch, reset usage, settle restore state or emit audit.
func (store *Store) GetAssistantSettings(ctx context.Context) (assistant.Settings, error) {
	if store == nil {
		return assistant.Settings{}, assistant.ErrUnavailable
	}
	var settings assistant.Settings
	err := store.assistantTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		control, err := assistantLock(ctx, tx)
		if err != nil {
			return err
		}
		if !control.Settings.Valid() {
			return assistant.ErrUnavailable
		}
		settings = control.Settings
		return nil
	})
	if err != nil {
		return assistant.Settings{}, err
	}
	return settings, nil
}

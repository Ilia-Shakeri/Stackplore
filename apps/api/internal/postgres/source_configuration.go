package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5"
)

const sourceConfigurationColumns = `id, provider, display_name, synthetic, enabled,
       operator_profile_id, row_version, created_at_us, updated_at_us`

func (store *Store) EnsureSourceConfigurations(ctx context.Context, configurations []domain.SourceConfiguration) error {
	if ctx == nil || len(configurations) < 1 || len(configurations) > domain.MaxRepositoryWriteBatch {
		return domain.ErrRepositoryInvalidInput
	}
	seen := make(map[domain.SourceID]struct{}, len(configurations))
	for _, configuration := range configurations {
		if configuration.Validate() != nil || configuration.Version != 1 {
			return domain.ErrRepositoryInvalidInput
		}
		if _, exists := seen[configuration.Source.ID]; exists {
			return domain.ErrRepositoryInvalidInput
		}
		seen[configuration.Source.ID] = struct{}{}
	}
	err := store.withinTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		for _, configuration := range configurations {
			source := configuration.Source
			if _, err := transaction.transaction.Exec(ctx, `INSERT INTO infrastructure_sources
(id,provider,display_name,synthetic,enabled,connection_kind,operator_profile_id,config_revision,created_at_us,updated_at_us,row_version)
VALUES($1,$2,$3,$4,$5,'operator_profile',$6,1,$7,$8,1)
ON CONFLICT(id) DO NOTHING`, string(source.ID), string(source.Provider), source.Name, boolInteger(source.Synthetic), boolInteger(configuration.Enabled), string(configuration.ProfileID), configuration.CreatedAt.UnixMicro(), configuration.UpdatedAt.UnixMicro()); err != nil {
				return err
			}
			stored, err := scanSourceConfiguration(transaction.transaction.QueryRow(ctx, `SELECT `+sourceConfigurationColumns+` FROM infrastructure_sources WHERE id=$1 AND deleted_at_us IS NULL`, string(source.ID)))
			if err != nil {
				return err
			}
			if stored.Source.Provider != source.Provider || stored.ProfileID != configuration.ProfileID || stored.Source.Synthetic != source.Synthetic {
				return domain.ErrRepositoryConflict
			}
		}
		return nil
	})
	return mapRepositoryWriteError(ctx, err)
}

func (store *Store) GetSourceConfiguration(ctx context.Context, sourceID domain.SourceID) (domain.SourceConfiguration, error) {
	if !store.available() || ctx == nil || sourceID.Validate() != nil {
		return domain.SourceConfiguration{}, domain.ErrRepositoryInvalidInput
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	configuration, err := scanSourceConfiguration(store.pool.QueryRow(ctx, `SELECT `+sourceConfigurationColumns+` FROM infrastructure_sources WHERE id=$1 AND deleted_at_us IS NULL`, string(sourceID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SourceConfiguration{}, domain.ErrRepositoryNotFound
	}
	if err != nil {
		return domain.SourceConfiguration{}, mapRepositoryReadError(ctx, err)
	}
	return configuration, nil
}

func (store *Store) ListSourceConfigurations(ctx context.Context, request domain.PageRequest) (domain.SourceConfigurationPage, error) {
	if !store.available() || ctx == nil || request.Validate() != nil {
		return domain.SourceConfigurationPage{}, domain.ErrRepositoryInvalidInput
	}
	queryHash := observationQueryHash("source_configuration")
	cursor, err := store.decodeObservationCursor(request.Cursor, "sources", queryHash)
	if err != nil {
		return domain.SourceConfigurationPage{}, err
	}
	ctx, cancel := store.queryContext(ctx, store.queryTimeout)
	defer cancel()
	after := ""
	if cursor != nil {
		after = cursor.ID
	}
	rows, err := store.pool.Query(ctx, `SELECT `+sourceConfigurationColumns+` FROM infrastructure_sources
WHERE deleted_at_us IS NULL AND id COLLATE "C" > $1 COLLATE "C"
ORDER BY id COLLATE "C" LIMIT $2`, after, request.Limit+1)
	if err != nil {
		return domain.SourceConfigurationPage{}, mapRepositoryReadError(ctx, err)
	}
	defer rows.Close()
	configurations := make([]domain.SourceConfiguration, 0, request.Limit+1)
	for rows.Next() {
		configuration, err := scanSourceConfiguration(rows)
		if err != nil {
			return domain.SourceConfigurationPage{}, domain.ErrRepositoryUnavailable
		}
		configurations = append(configurations, configuration)
	}
	if err := rows.Err(); err != nil {
		return domain.SourceConfigurationPage{}, mapRepositoryReadError(ctx, err)
	}
	page := domain.PageInfo{Limit: request.Limit}
	if len(configurations) > request.Limit {
		configurations = configurations[:request.Limit]
		page.HasMore = true
		page.NextCursor, err = store.encodeObservationCursor("sources", queryHash, "", string(configurations[len(configurations)-1].Source.ID))
		if err != nil {
			return domain.SourceConfigurationPage{}, err
		}
	}
	result := domain.SourceConfigurationPage{Configurations: configurations, Page: page}
	if result.ValidateFor(request) != nil {
		return domain.SourceConfigurationPage{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

func scanSourceConfiguration(row rowScanner) (domain.SourceConfiguration, error) {
	var result domain.SourceConfiguration
	var provider, profile string
	var synthetic, enabled, createdAt, updatedAt int64
	if err := row.Scan(&result.Source.ID, &provider, &result.Source.Name, &synthetic, &enabled, &profile, &result.Version, &createdAt, &updatedAt); err != nil {
		return domain.SourceConfiguration{}, err
	}
	if synthetic < 0 || synthetic > 1 || enabled < 0 || enabled > 1 {
		return domain.SourceConfiguration{}, domain.ErrRepositoryUnavailable
	}
	result.Source.Provider = domain.ProviderKind(provider)
	result.Source.Synthetic = synthetic == 1
	result.ProfileID = domain.SourceProfileID(profile)
	result.Enabled = enabled == 1
	result.CreatedAt = time.UnixMicro(createdAt).UTC()
	result.UpdatedAt = time.UnixMicro(updatedAt).UTC()
	if result.Validate() != nil {
		return domain.SourceConfiguration{}, domain.ErrRepositoryUnavailable
	}
	return result, nil
}

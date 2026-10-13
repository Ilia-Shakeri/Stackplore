package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/sourcesettings"
	"github.com/jackc/pgx/v5"
)

var _ sourcesettings.Repository = (*Store)(nil)

func (store *Store) CreateSourcePreview(ctx context.Context, command sourcesettings.PreviewCommand) (domain.SourceConfiguration, error) {
	if command.Validate() != nil {
		return domain.SourceConfiguration{}, sourcesettings.ErrInvalid
	}
	var configuration domain.SourceConfiguration
	err := store.withinAuthTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		tx := transaction.transaction
		if err := requireLivePrincipalTx(ctx, tx, command.Actor, command.CreatedAt, true); err != nil {
			if errors.Is(err, auth.ErrAuthRepositoryRejected) {
				return auth.ErrAuthorizationDenied
			}
			return err
		}
		var err error
		configuration, err = lockSourceConfiguration(ctx, tx, command.SourceID)
		if err != nil {
			return err
		}
		if configuration.Version != command.ExpectedVersion {
			return sourcesettings.ErrVersionConflict
		}
		if configuration.Enabled == command.Enabled {
			return sourcesettings.ErrStateUnchanged
		}
		if command.Enabled && !command.ProfileAvailable {
			return sourcesettings.ErrProfileUnavailable
		}
		if command.CreatedAt.Before(configuration.UpdatedAt) {
			return sourcesettings.ErrVersionConflict
		}
		if _, err := tx.Exec(ctx, `WITH expired AS (
SELECT source_id,session_id FROM source_update_confirmations WHERE expires_at_us<=$1
ORDER BY expires_at_us,source_id,session_id LIMIT 1000 FOR UPDATE SKIP LOCKED
) DELETE FROM source_update_confirmations AS confirmation USING expired
WHERE confirmation.source_id=expired.source_id AND confirmation.session_id=expired.session_id`, command.CreatedAt.UnixMicro()); err != nil {
			return err
		}
		var existing bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM source_update_confirmations WHERE source_id=$1 AND session_id=$2)`,
			string(command.SourceID), string(command.Actor.SessionID)).Scan(&existing); err != nil {
			return err
		}
		if !existing {
			var count int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM (SELECT 1 FROM source_update_confirmations LIMIT 4096) AS bounded`).Scan(&count); err != nil {
				return err
			}
			if count >= 4096 {
				return sourcesettings.ErrUnavailable
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO source_update_confirmations
(source_id,session_id,user_id,token_digest,expected_version,requested_enabled,created_at_us,expires_at_us)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (source_id,session_id) DO UPDATE SET
user_id=EXCLUDED.user_id, token_digest=EXCLUDED.token_digest,
expected_version=EXCLUDED.expected_version, requested_enabled=EXCLUDED.requested_enabled,
created_at_us=EXCLUDED.created_at_us, expires_at_us=EXCLUDED.expires_at_us`,
			string(command.SourceID), string(command.Actor.SessionID), string(command.Actor.UserID), command.Digest[:],
			command.ExpectedVersion, boolInteger(command.Enabled), command.CreatedAt.UnixMicro(), command.ExpiresAt.UnixMicro())
		return err
	})
	if err != nil {
		return domain.SourceConfiguration{}, sourceSettingsError(err)
	}
	return configuration, nil
}

func (store *Store) CommitSourceUpdate(ctx context.Context, command sourcesettings.UpdateCommand) (domain.SourceConfiguration, error) {
	if command.Validate() != nil || !validRecentGrantUse(command.Grant, auth.RecentAuthPurposeSourceUpdate, command.Grant.ConsumedAt) {
		return domain.SourceConfiguration{}, sourcesettings.ErrInvalid
	}
	var configuration domain.SourceConfiguration
	err := store.withinAuthTransaction(ctx, func(ctx context.Context, transaction *transaction) error {
		tx := transaction.transaction
		at := command.Grant.ConsumedAt
		var transactionAt time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&transactionAt); err != nil {
			return err
		}
		transactionAt = transactionAt.UTC().Truncate(time.Microsecond)
		if transactionAt.After(at) {
			at = transactionAt
		}
		command.Grant.ConsumedAt = at
		command.Audit.OccurredAt = at
		if err := requireLivePrincipalTx(ctx, tx, command.Actor, at, true); err != nil {
			if errors.Is(err, auth.ErrAuthRepositoryRejected) {
				return auth.ErrAuthorizationDenied
			}
			return err
		}
		var err error
		configuration, err = lockSourceConfiguration(ctx, tx, command.SourceID)
		if err != nil {
			return err
		}
		if configuration.Version != command.ExpectedVersion || at.Before(configuration.UpdatedAt) {
			return sourcesettings.ErrVersionConflict
		}
		if configuration.Enabled == command.Enabled {
			return sourcesettings.ErrStateUnchanged
		}
		if command.Enabled && !command.ProfileAvailable {
			return sourcesettings.ErrProfileUnavailable
		}
		result, err := tx.Exec(ctx, `DELETE FROM source_update_confirmations
WHERE source_id=$1 AND session_id=$2 AND user_id=$3 AND token_digest=$4
  AND expected_version=$5 AND requested_enabled=$6 AND created_at_us<=$7 AND expires_at_us>$7`,
			string(command.SourceID), string(command.Actor.SessionID), string(command.Actor.UserID), command.ConfirmationDigest[:],
			command.ExpectedVersion, boolInteger(command.Enabled), at.UnixMicro())
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return sourcesettings.ErrConfirmationRejected
		}
		if err := consumeRecentGrantTx(ctx, tx, command.Grant, auth.RecentAuthPurposeSourceUpdate); err != nil {
			if errors.Is(err, auth.ErrAuthRepositoryRejected) {
				return auth.ErrRecentAuthRejected
			}
			return err
		}
		configuration, err = scanSourceConfiguration(tx.QueryRow(ctx, `UPDATE infrastructure_sources
SET enabled=$1, config_revision=config_revision+1, row_version=row_version+1, updated_at_us=$2
WHERE id=$3 AND row_version=$4 AND deleted_at_us IS NULL
RETURNING `+sourceConfigurationColumns, boolInteger(command.Enabled), at.UnixMicro(), string(command.SourceID), command.ExpectedVersion))
		if err != nil {
			return err
		}
		return insertAuthAuditTx(ctx, tx, command.Audit)
	})
	if err != nil {
		return domain.SourceConfiguration{}, sourceSettingsError(err)
	}
	return configuration, nil
}

func lockSourceConfiguration(ctx context.Context, tx pgx.Tx, id domain.SourceID) (domain.SourceConfiguration, error) {
	configuration, err := scanSourceConfiguration(tx.QueryRow(ctx, `SELECT `+sourceConfigurationColumns+`
FROM infrastructure_sources WHERE id=$1 AND deleted_at_us IS NULL FOR UPDATE`, string(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SourceConfiguration{}, domain.ErrRepositoryNotFound
	}
	return configuration, err
}

func sourceSettingsError(err error) error {
	for _, known := range []error{sourcesettings.ErrInvalid, sourcesettings.ErrVersionConflict, sourcesettings.ErrConfirmationRejected,
		sourcesettings.ErrProfileUnavailable, sourcesettings.ErrStateUnchanged, auth.ErrAuthorizationDenied, auth.ErrRecentAuthRejected, domain.ErrRepositoryNotFound} {
		if errors.Is(err, known) {
			return known
		}
	}
	return sourcesettings.ErrUnavailable
}

package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	maximumAuthThrottleBatch        = 4
	maximumAuthThrottleKeysPerScope = 4096
	maximumAuthCursorBytes          = 1024
	maximumActivePreauth            = 4096
	authCursorVersion               = 1
)

var _ auth.Repository = (*Store)(nil)

type authRowScanner interface {
	Scan(...any) error
}

type authCursorState struct {
	Version       int    `json:"v"`
	Search        string `json:"q"`
	Role          string `json:"r"`
	Enabled       *bool  `json:"e"`
	CanonicalName string `json:"n"`
	UserID        string `json:"i"`
}

func (store *Store) SetupRequired(ctx context.Context) (bool, error) {
	if err := validateAuthRead(store, ctx); err != nil {
		return false, err
	}
	var setupCompleted sql.NullInt64
	var networkClosed sql.NullInt64
	var users int
	err := store.db.QueryRowContext(ctx, `SELECT setup_completed_at_us, network_setup_closed_at_us,
       (SELECT count(*) FROM users)
FROM installation_metadata
WHERE singleton_id = 1`).Scan(&setupCompleted, &networkClosed, &users)
	if err != nil {
		return false, mapAuthRepositoryError(ctx, err)
	}
	if !setupCompleted.Valid && !networkClosed.Valid && users == 0 {
		return true, nil
	}
	if setupCompleted.Valid && networkClosed.Valid && users > 0 {
		return false, nil
	}
	return false, auth.ErrAuthRepositoryUnavailable
}

func (store *Store) InsertPreauth(ctx context.Context, preauth auth.Preauth) error {
	if err := preauth.Validate(); err != nil {
		return auth.ErrAuthRepositoryRejected
	}
	cookieDigest, proofDigest, err := authPreauthDigestBytes(preauth)
	if err != nil {
		return auth.ErrAuthRepositoryRejected
	}
	err = store.withinTransaction(ctx, func(tx *transaction) error {
		if _, err := tx.transaction.ExecContext(ctx, `DELETE FROM preauth_challenges
WHERE consumed_at_us IS NOT NULL OR expires_at_us <= ?`, preauth.CreatedAt.UnixMicro()); err != nil {
			return err
		}
		var active int
		if err := tx.transaction.QueryRowContext(ctx, `SELECT count(*)
FROM preauth_challenges
WHERE consumed_at_us IS NULL AND expires_at_us > ?`, preauth.CreatedAt.UnixMicro()).Scan(&active); err != nil {
			return err
		}
		if active >= maximumActivePreauth {
			return auth.ErrAuthRepositoryRejected
		}
		_, err := tx.transaction.ExecContext(ctx, `INSERT INTO preauth_challenges
(id, purpose, cookie_digest, header_digest, digest_key_id, created_at_us, expires_at_us, consumed_at_us)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			preauth.ID,
			string(preauth.Purpose),
			cookieDigest,
			proofDigest,
			preauth.DigestKeyID,
			preauth.CreatedAt.UnixMicro(),
			preauth.ExpiresAt.UnixMicro(),
			nullableAuthTime(preauth.ConsumedAt),
		)
		return err
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) CheckThrottle(ctx context.Context, key auth.ThrottleKey, now time.Time) (auth.ThrottleState, error) {
	if err := validateAuthRead(store, ctx); err != nil || !validThrottleKey(key) || !validAuthTime(now) {
		if err != nil {
			return auth.ThrottleState{}, err
		}
		return auth.ThrottleState{}, auth.ErrAuthRepositoryRejected
	}
	var blockedUntil sql.NullInt64
	var expiresAt int64
	err := store.db.QueryRowContext(ctx, `SELECT blocked_until_at_us, expires_at_us
FROM authentication_throttles
WHERE scope = ? AND key_digest = ?`, string(key.Scope), key.Digest[:]).Scan(&blockedUntil, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		overflow := throttleOverflowDigest(key.Scope)
		err = store.db.QueryRowContext(ctx, `SELECT blocked_until_at_us, expires_at_us
FROM authentication_throttles
WHERE scope = ? AND key_digest = ?`, string(key.Scope), overflow[:]).Scan(&blockedUntil, &expiresAt)
		if errors.Is(err, sql.ErrNoRows) {
			return auth.ThrottleState{}, nil
		}
	}
	if err != nil {
		return auth.ThrottleState{}, mapAuthRepositoryError(ctx, err)
	}
	if expiresAt <= now.UnixMicro() || !blockedUntil.Valid || blockedUntil.Int64 <= now.UnixMicro() {
		return auth.ThrottleState{}, nil
	}
	return auth.ThrottleState{BlockedUntil: time.UnixMicro(blockedUntil.Int64).UTC()}, nil
}

func (store *Store) CompleteInitialSetup(ctx context.Context, commit auth.InitialSetupCommit) error {
	if !validInitialSetupCommit(commit) {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		result, err := tx.transaction.ExecContext(ctx, `UPDATE installation_metadata
SET setup_completed_at_us = ?, network_setup_closed_at_us = ?
WHERE singleton_id = 1
  AND setup_completed_at_us IS NULL
  AND network_setup_closed_at_us IS NULL
  AND NOT EXISTS (SELECT 1 FROM users)`,
			commit.User.CreatedAt.UnixMicro(), commit.User.CreatedAt.UnixMicro())
		if err != nil {
			return err
		}
		if !authRowsAffected(result, 1) {
			return auth.ErrAuthRepositoryConflict
		}
		if err := consumePreauthTx(ctx, tx.transaction, commit.Preauth); err != nil {
			return err
		}
		if err := deleteThrottleTx(ctx, tx.transaction, commit.ClearThrottle); err != nil {
			return err
		}
		if err := insertUserTx(ctx, tx.transaction, commit.User); err != nil {
			return err
		}
		if err := insertCredentialTx(ctx, tx.transaction, commit.User.ID, commit.Credential); err != nil {
			return err
		}
		if err := insertSessionTx(ctx, tx.transaction, commit.Session); err != nil {
			return err
		}
		return insertAuthAuditTx(ctx, tx.transaction, commit.Audit)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) CommitSetupFailure(ctx context.Context, commit auth.SetupFailureCommit) error {
	if !validSetupFailureCommit(commit) {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		if err := consumePreauthTx(ctx, tx.transaction, commit.Preauth); err != nil {
			return err
		}
		if err := applyThrottleFailureTx(ctx, tx.transaction, commit.Throttle); err != nil {
			return err
		}
		return insertAuthAuditTx(ctx, tx.transaction, commit.Audit)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) ConsumePreauth(ctx context.Context, use auth.PreauthUse) error {
	if !validPreauthUse(use) {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		return consumePreauthTx(ctx, tx.transaction, use)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) FindLoginAccount(ctx context.Context, canonicalLoginName string) (auth.LoginAccount, error) {
	if err := validateAuthRead(store, ctx); err != nil {
		return auth.LoginAccount{}, err
	}
	canonical, err := auth.CanonicalLoginName(canonicalLoginName)
	if err != nil || canonical != canonicalLoginName {
		return auth.LoginAccount{}, auth.ErrAuthRepositoryRejected
	}
	row := store.db.QueryRowContext(ctx, `SELECT
u.id, u.login_name, u.canonical_login_name, u.display_name, u.role, u.enabled,
u.auth_revision, u.created_at_us, u.updated_at_us, u.disabled_at_us, u.deleted_at_us, u.row_version,
c.algorithm, c.algorithm_version, c.time_cost, c.memory_kib, c.parallelism,
c.salt, c.password_hash, c.changed_at_us
FROM users AS u
JOIN user_password_credentials AS c ON c.user_id = u.id
WHERE u.canonical_login_name = ? AND u.deleted_at_us IS NULL`, canonicalLoginName)
	user, credential, _, err := scanLoginAccount(row)
	if err != nil {
		return auth.LoginAccount{}, mapAuthRepositoryError(ctx, err)
	}
	return auth.LoginAccount{User: user, Credential: credential}, nil
}

func (store *Store) CommitLoginSuccess(ctx context.Context, commit auth.LoginSuccessCommit) error {
	if !validLoginSuccessCommit(commit) {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		user, err := getUserTx(ctx, tx.transaction, commit.Session.UserID, false)
		if err != nil {
			return err
		}
		if !user.Enabled || user.DeletedAt != nil || user.AuthRevision != commit.Session.UserAuthRevision {
			return auth.ErrAuthRepositoryRejected
		}
		if commit.RehashedCredential != nil {
			if err := updateCredentialTx(ctx, tx.transaction, user.ID, *commit.RehashedCredential, commit.Session.CreatedAt); err != nil {
				return err
			}
		}
		for _, key := range commit.ClearThrottles {
			if err := deleteThrottleTx(ctx, tx.transaction, key); err != nil {
				return err
			}
		}
		if err := insertSessionTx(ctx, tx.transaction, commit.Session); err != nil {
			return err
		}
		return insertAuthAuditTx(ctx, tx.transaction, commit.Audit)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) CommitLoginFailure(ctx context.Context, commit auth.LoginFailureCommit) error {
	if !validLoginFailureCommit(commit) {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		for _, failure := range commit.Throttles {
			if err := applyThrottleFailureTx(ctx, tx.transaction, failure); err != nil {
				return err
			}
		}
		return insertAuthAuditTx(ctx, tx.transaction, commit.Audit)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) ResolveSession(ctx context.Context, digest auth.SessionTokenDigest, keyID string) (auth.ResolvedSession, error) {
	if err := validateAuthRead(store, ctx); err != nil {
		return auth.ResolvedSession{}, err
	}
	if digest.Validate() != nil || validateAuthIdentity(keyID) != nil {
		return auth.ResolvedSession{}, auth.ErrAuthRepositoryRejected
	}
	digestBytes, err := digest.Bytes()
	if err != nil {
		return auth.ResolvedSession{}, auth.ErrAuthRepositoryRejected
	}
	row := store.db.QueryRowContext(ctx, `SELECT
u.id, u.login_name, u.canonical_login_name, u.display_name, u.role, u.enabled,
u.auth_revision, u.created_at_us, u.updated_at_us, u.disabled_at_us, u.deleted_at_us, u.row_version,
s.id, s.user_id, s.token_digest, s.token_digest_key_id, s.csrf_digest, s.csrf_digest_key_id,
s.user_auth_revision, s.created_at_us, s.last_seen_at_us, s.idle_expires_at_us,
s.absolute_expires_at_us, s.revoked_at_us, s.revoke_reason_code, s.rotated_from_session_id
FROM sessions AS s
JOIN users AS u ON u.id = s.user_id
WHERE s.token_digest = ? AND s.token_digest_key_id = ?`, digestBytes, keyID)
	resolved, err := scanResolvedSession(row)
	if err != nil {
		return auth.ResolvedSession{}, mapAuthRepositoryError(ctx, err)
	}
	return resolved, nil
}

func (store *Store) TouchSession(ctx context.Context, touch auth.SessionTouch) error {
	if touch.SessionID.Validate() != nil || touch.UserID.Validate() != nil || touch.AuthRevision < 1 || !validAuthTime(touch.LastSeenAt) || !validAuthTime(touch.IdleExpiresAt) || !touch.IdleExpiresAt.After(touch.LastSeenAt) {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		result, err := tx.transaction.ExecContext(ctx, `UPDATE sessions
SET last_seen_at_us = ?, idle_expires_at_us = ?
WHERE id = ? AND user_id = ? AND user_auth_revision = ? AND revoked_at_us IS NULL
	  AND last_seen_at_us <= ?
	  AND idle_expires_at_us > ?
	  AND absolute_expires_at_us >= ?
  AND EXISTS (
      SELECT 1 FROM users
      WHERE users.id = sessions.user_id
        AND users.enabled = 1
        AND users.deleted_at_us IS NULL
        AND users.auth_revision = ?
  )`,
			touch.LastSeenAt.UnixMicro(),
			touch.IdleExpiresAt.UnixMicro(),
			string(touch.SessionID),
			string(touch.UserID),
			touch.AuthRevision,
			touch.LastSeenAt.UnixMicro(),
			touch.LastSeenAt.UnixMicro(),
			touch.IdleExpiresAt.UnixMicro(),
			touch.AuthRevision,
		)
		if err != nil {
			return err
		}
		if !authRowsAffected(result, 1) {
			return auth.ErrAuthRepositoryRejected
		}
		return nil
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) RevokeSession(ctx context.Context, revocation auth.SessionRevocation) error {
	if revocation.Actor.Validate() != nil || revocation.SessionID.Validate() != nil || revocation.SessionID != revocation.Actor.SessionID || !validAuthTime(revocation.RevokedAt) || !validAuthCode(revocation.Reason) || revocation.Audit.Validate() != nil || !auditMatchesActor(revocation.Audit, revocation.Actor) || !auditTargetsUser(revocation.Audit, revocation.Actor.UserID) || !revocation.Audit.OccurredAt.Equal(revocation.RevokedAt) || revocation.Audit.Outcome != domain.AuditSuccess {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		if err := requireLivePrincipalTx(ctx, tx.transaction, revocation.Actor, revocation.RevokedAt, false); err != nil {
			return err
		}
		result, err := tx.transaction.ExecContext(ctx, `UPDATE sessions
SET revoked_at_us = ?, revoke_reason_code = ?
WHERE id = ? AND user_id = ? AND revoked_at_us IS NULL`,
			revocation.RevokedAt.UnixMicro(),
			revocation.Reason,
			string(revocation.SessionID),
			string(revocation.Actor.UserID),
		)
		if err != nil {
			return err
		}
		if !authRowsAffected(result, 1) {
			return auth.ErrAuthRepositoryRejected
		}
		if err := revokeGrantsForSessionTx(ctx, tx.transaction, revocation.Actor.UserID, revocation.SessionID, revocation.RevokedAt, revocation.Reason); err != nil {
			return err
		}
		return insertAuthAuditTx(ctx, tx.transaction, revocation.Audit)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) CommitRecentGrant(ctx context.Context, commit auth.RecentGrantCommit) error {
	if commit.Actor.Validate() != nil || commit.Grant.Validate() != nil || commit.Grant.UserID != commit.Actor.UserID || commit.Grant.SessionID != commit.Actor.SessionID || !validThrottleKey(commit.ClearThrottle) || commit.ClearThrottle.Scope != auth.ThrottleScopeRecentUser || commit.Audit.Validate() != nil || !auditMatchesActor(commit.Audit, commit.Actor) || !auditTargetsUser(commit.Audit, commit.Actor.UserID) || !commit.Audit.OccurredAt.Equal(commit.Grant.CreatedAt) || commit.Audit.Outcome != domain.AuditSuccess {
		return auth.ErrAuthRepositoryRejected
	}
	digest, err := commit.Grant.Digest.Bytes()
	if err != nil {
		return auth.ErrAuthRepositoryRejected
	}
	err = store.withinTransaction(ctx, func(tx *transaction) error {
		if err := requireLivePrincipalTx(ctx, tx.transaction, commit.Actor, commit.Grant.CreatedAt, false); err != nil {
			return err
		}
		if err := deleteThrottleTx(ctx, tx.transaction, commit.ClearThrottle); err != nil {
			return err
		}
		_, err := tx.transaction.ExecContext(ctx, `INSERT INTO recent_auth_grants
(id, user_id, session_id, purpose, grant_digest, digest_key_id, created_at_us, expires_at_us,
 consumed_at_us, revoked_at_us, revoke_reason_code)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			commit.Grant.ID,
			string(commit.Grant.UserID),
			string(commit.Grant.SessionID),
			string(commit.Grant.Purpose),
			digest,
			commit.Grant.DigestKeyID,
			commit.Grant.CreatedAt.UnixMicro(),
			commit.Grant.ExpiresAt.UnixMicro(),
			nullableAuthTime(commit.Grant.ConsumedAt),
			nullableAuthTime(commit.Grant.RevokedAt),
			nullableAuthString(commit.Grant.RevokeReason),
		)
		if err != nil {
			return err
		}
		return insertAuthAuditTx(ctx, tx.transaction, commit.Audit)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) CommitRecentAuthFailure(ctx context.Context, commit auth.RecentAuthFailureCommit) error {
	if commit.Actor.Validate() != nil || commit.Audit.Validate() != nil || !auditMatchesActor(commit.Audit, commit.Actor) || !auditTargetsUser(commit.Audit, commit.Actor.UserID) || !validAuthTime(commit.Audit.OccurredAt) || commit.Audit.Outcome != domain.AuditDenied || commit.Throttle != nil && (!validThrottleFailure(*commit.Throttle) || commit.Throttle.Key.Scope != auth.ThrottleScopeRecentUser || !commit.Throttle.OccurredAt.Equal(commit.Audit.OccurredAt)) {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		if err := requireLivePrincipalTx(ctx, tx.transaction, commit.Actor, commit.Audit.OccurredAt, false); err != nil {
			return err
		}
		if commit.Throttle != nil {
			if err := applyThrottleFailureTx(ctx, tx.transaction, *commit.Throttle); err != nil {
				return err
			}
		}
		return insertAuthAuditTx(ctx, tx.transaction, commit.Audit)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) ListUsers(ctx context.Context, query auth.UserListQuery) (auth.UserPage, error) {
	if err := validateAuthRead(store, ctx); err != nil {
		return auth.UserPage{}, err
	}
	if query.Limit < 1 || query.Limit > 200 || !validUserSearch(query.Search) || query.Role != nil && query.Role.Validate() != nil {
		return auth.UserPage{}, auth.ErrAuthRepositoryRejected
	}
	cursor, err := store.decodeAuthCursor(query)
	if err != nil {
		return auth.UserPage{}, err
	}

	statement := `SELECT
id, login_name, canonical_login_name, display_name, role, enabled,
auth_revision, created_at_us, updated_at_us, disabled_at_us, deleted_at_us, row_version
FROM users
WHERE deleted_at_us IS NULL`
	arguments := make([]any, 0, 8)
	if query.Search != "" {
		statement += ` AND instr(lower(login_name || char(10) || display_name), lower(?)) > 0`
		arguments = append(arguments, query.Search)
	}
	if query.Role != nil {
		statement += ` AND role = ?`
		arguments = append(arguments, string(*query.Role))
	}
	if query.Enabled != nil {
		statement += ` AND enabled = ?`
		arguments = append(arguments, boolInteger(*query.Enabled))
	}
	if cursor != nil {
		statement += ` AND (canonical_login_name > ? OR (canonical_login_name = ? AND id > ?))`
		arguments = append(arguments, cursor.CanonicalName, cursor.CanonicalName, cursor.UserID)
	}
	statement += ` ORDER BY canonical_login_name ASC, id ASC LIMIT ?`
	arguments = append(arguments, query.Limit+1)

	rows, err := store.db.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return auth.UserPage{}, mapAuthRepositoryError(ctx, err)
	}
	defer rows.Close()
	items := make([]auth.User, 0, query.Limit+1)
	for rows.Next() {
		user, err := scanAuthUser(rows)
		if err != nil {
			return auth.UserPage{}, mapAuthRepositoryError(ctx, err)
		}
		items = append(items, user)
	}
	if err := rows.Err(); err != nil {
		return auth.UserPage{}, mapAuthRepositoryError(ctx, err)
	}
	page := auth.UserPage{}
	if len(items) > query.Limit {
		items = items[:query.Limit]
		last := items[len(items)-1]
		page.NextCursor, err = store.encodeAuthCursor(query, last)
		if err != nil {
			return auth.UserPage{}, err
		}
	}
	page.Items = items
	return page, nil
}

func (store *Store) GetUser(ctx context.Context, userID auth.UserID) (auth.User, error) {
	if err := validateAuthRead(store, ctx); err != nil {
		return auth.User{}, err
	}
	if userID.Validate() != nil {
		return auth.User{}, auth.ErrAuthRepositoryRejected
	}
	row := store.db.QueryRowContext(ctx, `SELECT
id, login_name, canonical_login_name, display_name, role, enabled,
auth_revision, created_at_us, updated_at_us, disabled_at_us, deleted_at_us, row_version
FROM users
WHERE id = ? AND deleted_at_us IS NULL`, string(userID))
	user, err := scanAuthUser(row)
	if err != nil {
		return auth.User{}, mapAuthRepositoryError(ctx, err)
	}
	return user, nil
}

func (store *Store) CommitOwnPasswordChange(ctx context.Context, commit auth.OwnPasswordChangeCommit) error {
	if !validOwnPasswordChangeCommit(commit) {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		if err := requireLivePrincipalTx(ctx, tx.transaction, commit.Actor, commit.ChangedAt, false); err != nil {
			return err
		}
		if err := consumeRecentGrantTx(ctx, tx.transaction, commit.Grant, auth.RecentAuthPurposeAccountPasswordChange); err != nil {
			return err
		}
		if err := updateCredentialTx(ctx, tx.transaction, commit.Actor.UserID, commit.Credential, commit.ChangedAt); err != nil {
			return err
		}
		result, err := tx.transaction.ExecContext(ctx, `UPDATE users
SET auth_revision = auth_revision + 1, updated_at_us = ?, row_version = row_version + 1
WHERE id = ? AND enabled = 1 AND deleted_at_us IS NULL AND auth_revision = ?`,
			commit.ChangedAt.UnixMicro(), string(commit.Actor.UserID), commit.Actor.AuthRevision)
		if err != nil {
			return err
		}
		if !authRowsAffected(result, 1) {
			return auth.ErrAuthRepositoryConflict
		}
		if err := revokeUserAuthStateTx(ctx, tx.transaction, commit.Actor.UserID, commit.ChangedAt, "password_changed"); err != nil {
			return err
		}
		if err := insertSessionTx(ctx, tx.transaction, commit.RotatedSession); err != nil {
			return err
		}
		return insertAuthAuditTx(ctx, tx.transaction, commit.Audit)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) CommitUserCreate(ctx context.Context, commit auth.UserCreateCommit) error {
	if !validUserCreateCommit(commit) {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		if err := requireLivePrincipalTx(ctx, tx.transaction, commit.Actor, commit.User.CreatedAt, true); err != nil {
			return err
		}
		if err := consumeRecentGrantTx(ctx, tx.transaction, commit.Grant, auth.RecentAuthPurposeUserCreate); err != nil {
			return err
		}
		if err := ensureCanonicalLoginAvailableTx(ctx, tx.transaction, commit.User.CanonicalLoginName, ""); err != nil {
			return err
		}
		if err := insertUserTx(ctx, tx.transaction, commit.User); err != nil {
			return err
		}
		if err := insertCredentialTx(ctx, tx.transaction, commit.User.ID, commit.Credential); err != nil {
			return err
		}
		return insertAuthAuditTx(ctx, tx.transaction, commit.Audit)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) CommitUserUpdate(ctx context.Context, commit auth.UserUpdateCommit) error {
	if !validUserUpdateCommit(commit) {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		if err := requireLivePrincipalTx(ctx, tx.transaction, commit.Actor, commit.User.UpdatedAt, true); err != nil {
			return err
		}
		if err := consumeRecentGrantTx(ctx, tx.transaction, commit.Grant, auth.RecentAuthPurposeUserUpdate); err != nil {
			return err
		}
		current, err := getUserTx(ctx, tx.transaction, commit.User.ID, false)
		if err != nil {
			return err
		}
		securityChanged, err := validateUserUpdateTransition(current, commit.User, commit.ExpectedVersion)
		if err != nil {
			return err
		}
		if err := ensureCanonicalLoginAvailableTx(ctx, tx.transaction, commit.User.CanonicalLoginName, commit.User.ID); err != nil {
			return err
		}
		if current.Enabled && current.Role == auth.RoleAdministrator && (!commit.User.Enabled || commit.User.Role != auth.RoleAdministrator) {
			if err := requireAnotherEnabledAdministratorTx(ctx, tx.transaction, current.ID); err != nil {
				return err
			}
		}
		result, err := tx.transaction.ExecContext(ctx, `UPDATE users
SET login_name = ?, canonical_login_name = ?, display_name = ?, role = ?, enabled = ?,
    auth_revision = ?, updated_at_us = ?, disabled_at_us = ?, row_version = ?
WHERE id = ? AND deleted_at_us IS NULL AND row_version = ?`,
			commit.User.LoginName,
			commit.User.CanonicalLoginName,
			commit.User.DisplayName,
			string(commit.User.Role),
			boolInteger(commit.User.Enabled),
			commit.User.AuthRevision,
			commit.User.UpdatedAt.UnixMicro(),
			nullableAuthTime(commit.User.DisabledAt),
			commit.User.Version,
			string(commit.User.ID),
			commit.ExpectedVersion,
		)
		if err != nil {
			return err
		}
		if !authRowsAffected(result, 1) {
			return auth.ErrAuthRepositoryStaleVersion
		}
		if securityChanged {
			if err := revokeUserAuthStateTx(ctx, tx.transaction, commit.User.ID, commit.User.UpdatedAt, "account_authorization_changed"); err != nil {
				return err
			}
		}
		return insertAuthAuditTx(ctx, tx.transaction, commit.Audit)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) CommitUserDelete(ctx context.Context, commit auth.UserDeleteCommit) error {
	if !validUserDeleteCommit(commit) {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		if err := requireLivePrincipalTx(ctx, tx.transaction, commit.Actor, commit.DeletedAt, true); err != nil {
			return err
		}
		if err := consumeRecentGrantTx(ctx, tx.transaction, commit.Grant, auth.RecentAuthPurposeUserDelete); err != nil {
			return err
		}
		current, err := getUserTx(ctx, tx.transaction, commit.UserID, false)
		if err != nil {
			return err
		}
		if current.Version != commit.ExpectedVersion {
			return auth.ErrAuthRepositoryStaleVersion
		}
		if commit.DeletedAt.Before(current.UpdatedAt) {
			return auth.ErrAuthRepositoryRejected
		}
		if current.Enabled && current.Role == auth.RoleAdministrator {
			if err := requireAnotherEnabledAdministratorTx(ctx, tx.transaction, current.ID); err != nil {
				return err
			}
		}
		disabledAt := current.DisabledAt
		if disabledAt == nil {
			disabledAt = &commit.DeletedAt
		}
		result, err := tx.transaction.ExecContext(ctx, `UPDATE users
SET enabled = 0, auth_revision = auth_revision + 1, updated_at_us = ?, disabled_at_us = ?,
    deleted_at_us = ?, row_version = row_version + 1
WHERE id = ? AND deleted_at_us IS NULL AND row_version = ?`,
			commit.DeletedAt.UnixMicro(),
			disabledAt.UnixMicro(),
			commit.DeletedAt.UnixMicro(),
			string(commit.UserID),
			commit.ExpectedVersion,
		)
		if err != nil {
			return err
		}
		if !authRowsAffected(result, 1) {
			return auth.ErrAuthRepositoryStaleVersion
		}
		if err := revokeUserAuthStateTx(ctx, tx.transaction, commit.UserID, commit.DeletedAt, "account_deleted"); err != nil {
			return err
		}
		return insertAuthAuditTx(ctx, tx.transaction, commit.Audit)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) CommitUserPasswordReset(ctx context.Context, commit auth.UserPasswordResetCommit) error {
	if !validUserPasswordResetCommit(commit) {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		if err := requireLivePrincipalTx(ctx, tx.transaction, commit.Actor, commit.ChangedAt, true); err != nil {
			return err
		}
		if err := consumeRecentGrantTx(ctx, tx.transaction, commit.Grant, auth.RecentAuthPurposeUserPasswordReset); err != nil {
			return err
		}
		current, err := getUserTx(ctx, tx.transaction, commit.UserID, false)
		if err != nil {
			return err
		}
		if current.Version != commit.ExpectedVersion {
			return auth.ErrAuthRepositoryStaleVersion
		}
		if commit.ChangedAt.Before(current.UpdatedAt) {
			return auth.ErrAuthRepositoryRejected
		}
		if err := updateCredentialTx(ctx, tx.transaction, commit.UserID, commit.Credential, commit.ChangedAt); err != nil {
			return err
		}
		result, err := tx.transaction.ExecContext(ctx, `UPDATE users
SET auth_revision = auth_revision + 1, updated_at_us = ?, row_version = row_version + 1
WHERE id = ? AND deleted_at_us IS NULL AND row_version = ?`,
			commit.ChangedAt.UnixMicro(), string(commit.UserID), commit.ExpectedVersion)
		if err != nil {
			return err
		}
		if !authRowsAffected(result, 1) {
			return auth.ErrAuthRepositoryStaleVersion
		}
		if err := revokeUserAuthStateTx(ctx, tx.transaction, commit.UserID, commit.ChangedAt, "password_reset"); err != nil {
			return err
		}
		return insertAuthAuditTx(ctx, tx.transaction, commit.Audit)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) CommitUserSessionsRevoke(ctx context.Context, commit auth.UserSessionsRevokeCommit) error {
	if !validUserSessionsRevokeCommit(commit) {
		return auth.ErrAuthRepositoryRejected
	}
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		if err := requireLivePrincipalTx(ctx, tx.transaction, commit.Actor, commit.RevokedAt, true); err != nil {
			return err
		}
		if err := consumeRecentGrantTx(ctx, tx.transaction, commit.Grant, auth.RecentAuthPurposeUserSessionsRevoke); err != nil {
			return err
		}
		current, err := getUserTx(ctx, tx.transaction, commit.UserID, false)
		if err != nil {
			return err
		}
		if commit.RevokedAt.Before(current.UpdatedAt) {
			return auth.ErrAuthRepositoryRejected
		}
		result, err := tx.transaction.ExecContext(ctx, `UPDATE users
SET auth_revision = auth_revision + 1, updated_at_us = ?, row_version = row_version + 1
WHERE id = ? AND deleted_at_us IS NULL AND row_version = ?`,
			commit.RevokedAt.UnixMicro(), string(commit.UserID), current.Version)
		if err != nil {
			return err
		}
		if !authRowsAffected(result, 1) {
			return auth.ErrAuthRepositoryConflict
		}
		if err := revokeUserAuthStateTx(ctx, tx.transaction, commit.UserID, commit.RevokedAt, "sessions_revoked"); err != nil {
			return err
		}
		return insertAuthAuditTx(ctx, tx.transaction, commit.Audit)
	})
	return mapAuthRepositoryError(ctx, err)
}

func (store *Store) CommitOperatorRecovery(ctx context.Context, commit auth.OperatorRecoveryCommit) (auth.User, error) {
	if !validOperatorRecoveryCommit(commit) {
		return auth.User{}, auth.ErrAuthRepositoryRejected
	}
	var recovered auth.User
	err := store.withinTransaction(ctx, func(tx *transaction) error {
		var setupCompleted sql.NullInt64
		var networkClosed sql.NullInt64
		var userCount int
		if err := tx.transaction.QueryRowContext(ctx, `SELECT setup_completed_at_us, network_setup_closed_at_us
       , (SELECT count(*) FROM users)
FROM installation_metadata
WHERE singleton_id = 1`).Scan(&setupCompleted, &networkClosed, &userCount); err != nil {
			return err
		}
		if setupCompleted.Valid != networkClosed.Valid || !setupCompleted.Valid && userCount > 0 {
			return auth.ErrAuthRepositoryUnavailable
		}

		row := tx.transaction.QueryRowContext(ctx, `SELECT
id, login_name, canonical_login_name, display_name, role, enabled,
auth_revision, created_at_us, updated_at_us, disabled_at_us, deleted_at_us, row_version
FROM users
WHERE canonical_login_name = ? AND deleted_at_us IS NULL`, commit.RequestedUser.CanonicalLoginName)
		current, lookupErr := scanAuthUser(row)
		switch {
		case lookupErr == nil:
			if commit.RequestedUser.ID != current.ID ||
				!commit.RequestedUser.CreatedAt.Equal(current.CreatedAt) ||
				commit.RequestedUser.AuthRevision != current.AuthRevision+1 ||
				commit.RequestedUser.Version != current.Version+1 ||
				commit.RecoveredAt.Before(current.UpdatedAt) {
				return auth.ErrAuthRepositoryStaleVersion
			}
			recovered = commit.RequestedUser
			result, err := tx.transaction.ExecContext(ctx, `UPDATE users
SET login_name = ?, canonical_login_name = ?, display_name = ?, role = 'administrator',
    enabled = 1, auth_revision = ?, updated_at_us = ?, disabled_at_us = NULL,
    row_version = ?
WHERE id = ? AND deleted_at_us IS NULL AND row_version = ?`,
				recovered.LoginName, recovered.CanonicalLoginName, recovered.DisplayName,
				recovered.AuthRevision, recovered.UpdatedAt.UnixMicro(), recovered.Version,
				string(recovered.ID), current.Version)
			if err != nil {
				return err
			}
			if !authRowsAffected(result, 1) {
				return auth.ErrAuthRepositoryStaleVersion
			}
			if err := updateCredentialTx(ctx, tx.transaction, recovered.ID, commit.Credential, commit.RecoveredAt); err != nil {
				return err
			}
			if err := revokeUserAuthStateTx(ctx, tx.transaction, recovered.ID, commit.RecoveredAt, "operator_recovery"); err != nil {
				return err
			}
		case errors.Is(lookupErr, sql.ErrNoRows):
			recovered = commit.RequestedUser
			if recovered.AuthRevision != 1 || recovered.Version != 1 || !recovered.CreatedAt.Equal(commit.RecoveredAt) {
				return auth.ErrAuthRepositoryStaleVersion
			}
			if err := ensureCanonicalLoginAvailableTx(ctx, tx.transaction, recovered.CanonicalLoginName, ""); err != nil {
				return err
			}
			if err := insertUserTx(ctx, tx.transaction, recovered); err != nil {
				return err
			}
			if err := insertCredentialTx(ctx, tx.transaction, recovered.ID, commit.Credential); err != nil {
				return err
			}
			if !setupCompleted.Valid {
				result, err := tx.transaction.ExecContext(ctx, `UPDATE installation_metadata
SET setup_completed_at_us = ?, network_setup_closed_at_us = ?
WHERE singleton_id = 1 AND setup_completed_at_us IS NULL AND network_setup_closed_at_us IS NULL`,
					commit.RecoveredAt.UnixMicro(), commit.RecoveredAt.UnixMicro())
				if err != nil {
					return err
				}
				if !authRowsAffected(result, 1) {
					return auth.ErrAuthRepositoryConflict
				}
			}
		default:
			return lookupErr
		}

		audit := commit.Audit
		if audit.Target.ID != "" && audit.Target.ID != string(commit.RequestedUser.ID) && audit.Target.ID != string(recovered.ID) {
			return auth.ErrAuthRepositoryRejected
		}
		audit.Target.ID = string(recovered.ID)
		return insertAuthAuditTx(ctx, tx.transaction, audit)
	})
	if err != nil {
		return auth.User{}, mapAuthRepositoryError(ctx, err)
	}
	return recovered, nil
}

func consumePreauthTx(ctx context.Context, tx *sql.Tx, use auth.PreauthUse) error {
	if !validPreauthUse(use) {
		return auth.ErrAuthRepositoryRejected
	}
	cookieDigest, err := use.CookieDigest.Bytes()
	if err != nil {
		return auth.ErrAuthRepositoryRejected
	}
	proofDigest, err := use.ProofDigest.Bytes()
	if err != nil {
		return auth.ErrAuthRepositoryRejected
	}
	result, err := tx.ExecContext(ctx, `UPDATE preauth_challenges
SET consumed_at_us = ?
WHERE purpose = ? AND cookie_digest = ? AND header_digest = ? AND digest_key_id = ?
  AND consumed_at_us IS NULL AND created_at_us <= ? AND expires_at_us > ?`,
		use.ConsumedAt.UnixMicro(),
		string(use.Purpose),
		cookieDigest,
		proofDigest,
		use.DigestKeyID,
		use.ConsumedAt.UnixMicro(),
		use.ConsumedAt.UnixMicro(),
	)
	if err != nil {
		return err
	}
	if !authRowsAffected(result, 1) {
		return auth.ErrAuthRepositoryRejected
	}
	return nil
}

func applyThrottleFailureTx(ctx context.Context, tx *sql.Tx, failure auth.ThrottleFailure) error {
	if !validThrottleFailure(failure) {
		return auth.ErrAuthRepositoryRejected
	}
	now := failure.OccurredAt.UnixMicro()
	if _, err := tx.ExecContext(ctx, `DELETE FROM authentication_throttles WHERE expires_at_us <= ?`, now); err != nil {
		return err
	}
	key := failure.Key
	var windowStarted int64
	var count int
	var blockedUntil sql.NullInt64
	var expiresAt int64
	readState := func() error {
		return tx.QueryRowContext(ctx, `SELECT window_started_at_us, failure_count, blocked_until_at_us, expires_at_us
FROM authentication_throttles
WHERE scope = ? AND key_digest = ?`, string(key.Scope), key.Digest[:]).Scan(
			&windowStarted, &count, &blockedUntil, &expiresAt,
		)
	}
	err := readState()
	if errors.Is(err, sql.ErrNoRows) {
		overflow := throttleOverflowDigest(key.Scope)
		var directKeys int
		if countErr := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM authentication_throttles
WHERE scope = ? AND key_digest <> ?`, string(key.Scope), overflow[:]).Scan(&directKeys); countErr != nil {
			return countErr
		}
		if directKeys >= maximumAuthThrottleKeysPerScope {
			key.Digest = overflow
			err = readState()
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		count = 1
		windowStarted = now
		blockedUntil = sql.NullInt64{}
		if count >= failure.MaximumFailures {
			blockedUntil = sql.NullInt64{Int64: failure.OccurredAt.Add(failure.BlockDuration).UnixMicro(), Valid: true}
		}
		expiresAt = failure.OccurredAt.Add(failure.Retention).UnixMicro()
		if blockedUntil.Valid && blockedUntil.Int64 > expiresAt {
			expiresAt = blockedUntil.Int64
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO authentication_throttles
(scope, key_digest, window_started_at_us, failure_count, blocked_until_at_us, expires_at_us)
VALUES (?, ?, ?, ?, ?, ?)`,
			string(key.Scope), key.Digest[:], windowStarted, count,
			nullableAuthInt64(blockedUntil), expiresAt)
		return err
	}
	if err != nil {
		return err
	}
	if windowStarted < 0 || count < 0 || count > 1_000_000 || expiresAt <= windowStarted || blockedUntil.Valid && (blockedUntil.Int64 < windowStarted || blockedUntil.Int64 > expiresAt) {
		return auth.ErrAuthRepositoryUnavailable
	}

	windowMicros := failure.Window.Microseconds()
	if blockedUntil.Valid && blockedUntil.Int64 <= now {
		blockedUntil = sql.NullInt64{}
	}
	if expiresAt <= now || now >= windowStarted+windowMicros {
		windowStarted = now
		count = 1
	} else if count < 1_000_000 {
		count++
	}
	if count >= failure.MaximumFailures {
		candidate := failure.OccurredAt.Add(failure.BlockDuration).UnixMicro()
		if !blockedUntil.Valid || candidate > blockedUntil.Int64 {
			blockedUntil = sql.NullInt64{Int64: candidate, Valid: true}
		}
	}
	expiresAt = failure.OccurredAt.Add(failure.Retention).UnixMicro()
	if blockedUntil.Valid && blockedUntil.Int64 > expiresAt {
		expiresAt = blockedUntil.Int64
	}
	result, err := tx.ExecContext(ctx, `UPDATE authentication_throttles
SET window_started_at_us = ?, failure_count = ?, blocked_until_at_us = ?, expires_at_us = ?
WHERE scope = ? AND key_digest = ?`,
		windowStarted, count, nullableAuthInt64(blockedUntil), expiresAt,
		string(key.Scope), key.Digest[:])
	if err != nil {
		return err
	}
	if !authRowsAffected(result, 1) {
		return auth.ErrAuthRepositoryConflict
	}
	return nil
}

func throttleOverflowDigest(scope auth.ThrottleScope) [auth.DigestBytes]byte {
	return sha256.Sum256([]byte("stackplore/auth-throttle-overflow/v1/" + string(scope)))
}

func deleteThrottleTx(ctx context.Context, tx *sql.Tx, key auth.ThrottleKey) error {
	if !validThrottleKey(key) {
		return auth.ErrAuthRepositoryRejected
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM authentication_throttles
WHERE scope = ? AND key_digest = ?`, string(key.Scope), key.Digest[:])
	return err
}

func insertUserTx(ctx context.Context, tx *sql.Tx, user auth.User) error {
	if user.Validate() != nil {
		return auth.ErrAuthRepositoryRejected
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO users
(id, login_name, canonical_login_name, display_name, role, enabled, auth_revision,
 created_at_us, updated_at_us, disabled_at_us, deleted_at_us, row_version)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(user.ID), user.LoginName, user.CanonicalLoginName, user.DisplayName,
		string(user.Role), boolInteger(user.Enabled), user.AuthRevision,
		user.CreatedAt.UnixMicro(), user.UpdatedAt.UnixMicro(), nullableAuthTime(user.DisabledAt),
		nullableAuthTime(user.DeletedAt), user.Version)
	return err
}

func insertCredentialTx(ctx context.Context, tx *sql.Tx, userID auth.UserID, credential auth.PasswordCredential) error {
	if userID.Validate() != nil || credential.Validate() != nil {
		return auth.ErrAuthRepositoryRejected
	}
	user, err := getUserTx(ctx, tx, userID, true)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_password_credentials
(user_id, algorithm, algorithm_version, time_cost, memory_kib, parallelism, salt, password_hash, changed_at_us)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(userID), credential.Algorithm, credential.AlgorithmVersion, credential.TimeCost,
		credential.MemoryKiB, credential.Parallelism, credential.Salt, credential.Hash,
		user.UpdatedAt.UnixMicro())
	return err
}

func updateCredentialTx(ctx context.Context, tx *sql.Tx, userID auth.UserID, credential auth.PasswordCredential, changedAt time.Time) error {
	if userID.Validate() != nil || credential.Validate() != nil || !validAuthTime(changedAt) {
		return auth.ErrAuthRepositoryRejected
	}
	result, err := tx.ExecContext(ctx, `UPDATE user_password_credentials
SET algorithm = ?, algorithm_version = ?, time_cost = ?, memory_kib = ?, parallelism = ?,
    salt = ?, password_hash = ?, changed_at_us = ?
WHERE user_id = ?`,
		credential.Algorithm, credential.AlgorithmVersion, credential.TimeCost, credential.MemoryKiB,
		credential.Parallelism, credential.Salt, credential.Hash, changedAt.UnixMicro(), string(userID))
	if err != nil {
		return err
	}
	if !authRowsAffected(result, 1) {
		return auth.ErrAuthRepositoryUnavailable
	}
	return nil
}

func insertSessionTx(ctx context.Context, tx *sql.Tx, session auth.Session) error {
	if session.Validate() != nil {
		return auth.ErrAuthRepositoryRejected
	}
	tokenDigest, err := session.TokenDigest.Bytes()
	if err != nil {
		return auth.ErrAuthRepositoryRejected
	}
	csrfDigest, err := session.CSRFDigest.Bytes()
	if err != nil {
		return auth.ErrAuthRepositoryRejected
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sessions
(id, user_id, token_digest, token_digest_key_id, csrf_digest, csrf_digest_key_id,
 user_auth_revision, created_at_us, last_seen_at_us, idle_expires_at_us, absolute_expires_at_us,
 revoked_at_us, revoke_reason_code, rotated_from_session_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(session.ID), string(session.UserID), tokenDigest, session.TokenDigestKeyID,
		csrfDigest, session.CSRFDigestKeyID, session.UserAuthRevision,
		session.CreatedAt.UnixMicro(), session.LastSeenAt.UnixMicro(), session.IdleExpiresAt.UnixMicro(),
		session.AbsoluteExpiresAt.UnixMicro(), nullableAuthTime(session.RevokedAt),
		nullableAuthString(session.RevokeReason), nullableSessionID(session.RotatedFromSessionID))
	return err
}

func getUserTx(ctx context.Context, tx *sql.Tx, userID auth.UserID, includeDeleted bool) (auth.User, error) {
	if userID.Validate() != nil {
		return auth.User{}, auth.ErrAuthRepositoryRejected
	}
	statement := `SELECT
id, login_name, canonical_login_name, display_name, role, enabled,
auth_revision, created_at_us, updated_at_us, disabled_at_us, deleted_at_us, row_version
FROM users
WHERE id = ?`
	if !includeDeleted {
		statement += ` AND deleted_at_us IS NULL`
	}
	return scanAuthUser(tx.QueryRowContext(ctx, statement, string(userID)))
}

func requireLivePrincipalTx(ctx context.Context, tx *sql.Tx, principal auth.Principal, now time.Time, administrator bool) error {
	if principal.Validate() != nil || !validAuthTime(now) || administrator && principal.Role != auth.RoleAdministrator {
		return auth.ErrAuthRepositoryRejected
	}
	var marker int
	err := tx.QueryRowContext(ctx, `SELECT 1
FROM users AS u
JOIN sessions AS s ON s.user_id = u.id
WHERE u.id = ? AND u.enabled = 1 AND u.deleted_at_us IS NULL
  AND u.role = ? AND u.auth_revision = ?
  AND s.id = ? AND s.user_auth_revision = ? AND s.revoked_at_us IS NULL
  AND s.created_at_us <= ? AND s.last_seen_at_us <= ?
  AND s.idle_expires_at_us > ? AND s.absolute_expires_at_us > ?`,
		string(principal.UserID), string(principal.Role), principal.AuthRevision,
		string(principal.SessionID), principal.AuthRevision,
		now.UnixMicro(), now.UnixMicro(), now.UnixMicro(), now.UnixMicro()).Scan(&marker)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.ErrAuthRepositoryRejected
	}
	if err != nil {
		return err
	}
	if marker != 1 {
		return auth.ErrAuthRepositoryUnavailable
	}
	return nil
}

func consumeRecentGrantTx(ctx context.Context, tx *sql.Tx, use auth.RecentGrantUse, purpose auth.RecentAuthPurpose) error {
	if !validRecentGrantUse(use, purpose, use.ConsumedAt) {
		return auth.ErrAuthRepositoryRejected
	}
	digest, err := use.Digest.Bytes()
	if err != nil {
		return auth.ErrAuthRepositoryRejected
	}
	result, err := tx.ExecContext(ctx, `UPDATE recent_auth_grants
SET consumed_at_us = ?
WHERE user_id = ? AND session_id = ? AND purpose = ? AND grant_digest = ? AND digest_key_id = ?
  AND consumed_at_us IS NULL AND revoked_at_us IS NULL
  AND created_at_us <= ? AND expires_at_us > ?`,
		use.ConsumedAt.UnixMicro(), string(use.Actor.UserID), string(use.Actor.SessionID),
		string(purpose), digest, use.DigestKeyID,
		use.ConsumedAt.UnixMicro(), use.ConsumedAt.UnixMicro())
	if err != nil {
		return err
	}
	if !authRowsAffected(result, 1) {
		return auth.ErrAuthRepositoryRejected
	}
	return nil
}

func requireAnotherEnabledAdministratorTx(ctx context.Context, tx *sql.Tx, excluded auth.UserID) error {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*)
FROM users
WHERE role = 'administrator' AND enabled = 1 AND deleted_at_us IS NULL AND id <> ?`, string(excluded)).Scan(&count); err != nil {
		return err
	}
	if count < 1 {
		return auth.ErrAuthRepositoryLastAdministrator
	}
	return nil
}

func ensureCanonicalLoginAvailableTx(ctx context.Context, tx *sql.Tx, canonical string, excluded auth.UserID) error {
	validated, err := auth.CanonicalLoginName(canonical)
	if err != nil || validated != canonical {
		return auth.ErrAuthRepositoryRejected
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*)
FROM users
WHERE canonical_login_name = ? AND deleted_at_us IS NULL AND id <> ?`, canonical, string(excluded)).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return auth.ErrAuthRepositoryLoginNameConflict
	}
	return nil
}

func revokeGrantsForSessionTx(ctx context.Context, tx *sql.Tx, userID auth.UserID, sessionID auth.SessionID, revokedAt time.Time, reason string) error {
	if userID.Validate() != nil || sessionID.Validate() != nil || !validAuthTime(revokedAt) || !validAuthCode(reason) {
		return auth.ErrAuthRepositoryRejected
	}
	_, err := tx.ExecContext(ctx, `UPDATE recent_auth_grants
SET revoked_at_us = ?, revoke_reason_code = ?
WHERE user_id = ? AND session_id = ? AND revoked_at_us IS NULL AND created_at_us <= ?`,
		revokedAt.UnixMicro(), reason, string(userID), string(sessionID), revokedAt.UnixMicro())
	return err
}

func revokeUserAuthStateTx(ctx context.Context, tx *sql.Tx, userID auth.UserID, revokedAt time.Time, reason string) error {
	if userID.Validate() != nil || !validAuthTime(revokedAt) || !validAuthCode(reason) {
		return auth.ErrAuthRepositoryRejected
	}
	if _, err := tx.ExecContext(ctx, `UPDATE recent_auth_grants
SET revoked_at_us = ?, revoke_reason_code = ?
WHERE user_id = ? AND revoked_at_us IS NULL AND created_at_us <= ?`,
		revokedAt.UnixMicro(), reason, string(userID), revokedAt.UnixMicro()); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE sessions
SET revoked_at_us = ?, revoke_reason_code = ?
WHERE user_id = ? AND revoked_at_us IS NULL AND created_at_us <= ?`,
		revokedAt.UnixMicro(), reason, string(userID), revokedAt.UnixMicro())
	return err
}

func insertAuthAuditTx(ctx context.Context, tx *sql.Tx, entry domain.AuditEntry) error {
	if entry.Validate() != nil || !validAuthTime(entry.OccurredAt) {
		return auth.ErrAuthRepositoryRejected
	}
	actorUserID, operatorID := auditActorColumns(entry.Actor)
	result, err := tx.ExecContext(ctx, insertAuditEntryStatement,
		string(entry.ID), entry.OccurredAt.UnixMicro(), string(entry.Actor.Kind), actorUserID,
		operatorID, string(entry.Action), string(entry.Target.Kind), optionalString(entry.Target.ID),
		string(entry.Outcome), string(entry.RequestID), string(entry.ReasonCode))
	if err != nil {
		return err
	}
	if !authRowsAffected(result, 1) {
		return auth.ErrAuthRepositoryConflict
	}
	return nil
}

func scanAuthUser(scanner authRowScanner) (auth.User, error) {
	var user auth.User
	var id string
	var role string
	var enabled int
	var createdAt int64
	var updatedAt int64
	var disabledAt sql.NullInt64
	var deletedAt sql.NullInt64
	err := scanner.Scan(
		&id, &user.LoginName, &user.CanonicalLoginName, &user.DisplayName, &role, &enabled,
		&user.AuthRevision, &createdAt, &updatedAt, &disabledAt, &deletedAt, &user.Version,
	)
	if err != nil {
		return auth.User{}, err
	}
	if enabled != 0 && enabled != 1 {
		return auth.User{}, auth.ErrAuthRepositoryUnavailable
	}
	user.ID = auth.UserID(id)
	user.Role = auth.Role(role)
	user.Enabled = enabled == 1
	user.CreatedAt = time.UnixMicro(createdAt).UTC()
	user.UpdatedAt = time.UnixMicro(updatedAt).UTC()
	user.DisabledAt = authTimeFromNull(disabledAt)
	user.DeletedAt = authTimeFromNull(deletedAt)
	if user.Validate() != nil {
		return auth.User{}, auth.ErrAuthRepositoryUnavailable
	}
	return user, nil
}

func scanLoginAccount(scanner authRowScanner) (auth.User, auth.PasswordCredential, time.Time, error) {
	var user auth.User
	var id string
	var role string
	var enabled int
	var createdAt int64
	var updatedAt int64
	var disabledAt sql.NullInt64
	var deletedAt sql.NullInt64
	var credential auth.PasswordCredential
	var timeCost int64
	var memoryKiB int64
	var parallelism int64
	var changedAt int64
	err := scanner.Scan(
		&id, &user.LoginName, &user.CanonicalLoginName, &user.DisplayName, &role, &enabled,
		&user.AuthRevision, &createdAt, &updatedAt, &disabledAt, &deletedAt, &user.Version,
		&credential.Algorithm, &credential.AlgorithmVersion, &timeCost, &memoryKiB, &parallelism,
		&credential.Salt, &credential.Hash, &changedAt,
	)
	if err != nil {
		return auth.User{}, auth.PasswordCredential{}, time.Time{}, err
	}
	if enabled != 0 && enabled != 1 || timeCost < 0 || timeCost > int64(^uint32(0)) || memoryKiB < 0 || memoryKiB > int64(^uint32(0)) || parallelism < 0 || parallelism > int64(^uint8(0)) {
		return auth.User{}, auth.PasswordCredential{}, time.Time{}, auth.ErrAuthRepositoryUnavailable
	}
	user.ID = auth.UserID(id)
	user.Role = auth.Role(role)
	user.Enabled = enabled == 1
	user.CreatedAt = time.UnixMicro(createdAt).UTC()
	user.UpdatedAt = time.UnixMicro(updatedAt).UTC()
	user.DisabledAt = authTimeFromNull(disabledAt)
	user.DeletedAt = authTimeFromNull(deletedAt)
	credential.TimeCost = uint32(timeCost)
	credential.MemoryKiB = uint32(memoryKiB)
	credential.Parallelism = uint8(parallelism)
	changed := time.UnixMicro(changedAt).UTC()
	if user.Validate() != nil || credential.Validate() != nil || !validAuthTime(changed) || changed.Before(user.CreatedAt) {
		return auth.User{}, auth.PasswordCredential{}, time.Time{}, auth.ErrAuthRepositoryUnavailable
	}
	return user, credential, changed, nil
}

func scanResolvedSession(scanner authRowScanner) (auth.ResolvedSession, error) {
	var user auth.User
	var userID string
	var role string
	var enabled int
	var userCreatedAt int64
	var userUpdatedAt int64
	var disabledAt sql.NullInt64
	var deletedAt sql.NullInt64
	var session auth.Session
	var sessionID string
	var sessionUserID string
	var tokenDigest []byte
	var csrfDigest []byte
	var sessionCreatedAt int64
	var lastSeenAt int64
	var idleExpiresAt int64
	var absoluteExpiresAt int64
	var revokedAt sql.NullInt64
	var revokeReason sql.NullString
	var rotatedFrom sql.NullString
	err := scanner.Scan(
		&userID, &user.LoginName, &user.CanonicalLoginName, &user.DisplayName, &role, &enabled,
		&user.AuthRevision, &userCreatedAt, &userUpdatedAt, &disabledAt, &deletedAt, &user.Version,
		&sessionID, &sessionUserID, &tokenDigest, &session.TokenDigestKeyID, &csrfDigest, &session.CSRFDigestKeyID,
		&session.UserAuthRevision, &sessionCreatedAt, &lastSeenAt, &idleExpiresAt, &absoluteExpiresAt,
		&revokedAt, &revokeReason, &rotatedFrom,
	)
	if err != nil {
		return auth.ResolvedSession{}, err
	}
	if enabled != 0 && enabled != 1 {
		return auth.ResolvedSession{}, auth.ErrAuthRepositoryUnavailable
	}
	user.ID = auth.UserID(userID)
	user.Role = auth.Role(role)
	user.Enabled = enabled == 1
	user.CreatedAt = time.UnixMicro(userCreatedAt).UTC()
	user.UpdatedAt = time.UnixMicro(userUpdatedAt).UTC()
	user.DisabledAt = authTimeFromNull(disabledAt)
	user.DeletedAt = authTimeFromNull(deletedAt)
	session.ID = auth.SessionID(sessionID)
	session.UserID = auth.UserID(sessionUserID)
	session.CreatedAt = time.UnixMicro(sessionCreatedAt).UTC()
	session.LastSeenAt = time.UnixMicro(lastSeenAt).UTC()
	session.IdleExpiresAt = time.UnixMicro(idleExpiresAt).UTC()
	session.AbsoluteExpiresAt = time.UnixMicro(absoluteExpiresAt).UTC()
	session.RevokedAt = authTimeFromNull(revokedAt)
	if revokeReason.Valid {
		session.RevokeReason = revokeReason.String
	}
	if rotatedFrom.Valid {
		value := auth.SessionID(rotatedFrom.String)
		session.RotatedFromSessionID = &value
	}
	var errDigest error
	session.TokenDigest, errDigest = auth.ParseSessionTokenDigest(tokenDigest)
	if errDigest != nil {
		return auth.ResolvedSession{}, auth.ErrAuthRepositoryUnavailable
	}
	session.CSRFDigest, errDigest = auth.ParseCSRFDigest(csrfDigest)
	if errDigest != nil {
		return auth.ResolvedSession{}, auth.ErrAuthRepositoryUnavailable
	}
	if user.Validate() != nil || session.Validate() != nil || session.UserID != user.ID {
		return auth.ResolvedSession{}, auth.ErrAuthRepositoryUnavailable
	}
	return auth.ResolvedSession{User: user, Session: session}, nil
}

func (store *Store) encodeAuthCursor(query auth.UserListQuery, user auth.User) (string, error) {
	if store == nil || store.closed.Load() || user.Validate() != nil {
		return "", auth.ErrAuthRepositoryUnavailable
	}
	state := authCursorState{
		Version: authCursorVersion, Search: query.Search, Enabled: cloneBool(query.Enabled),
		CanonicalName: user.CanonicalLoginName, UserID: string(user.ID),
	}
	if query.Role != nil {
		state.Role = string(*query.Role)
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return "", auth.ErrAuthRepositoryUnavailable
	}
	mac := hmac.New(sha256.New, store.authDigestKey[:])
	_, _ = mac.Write([]byte("stackplore/user-cursor/v1\x00"))
	_, _ = mac.Write(payload)
	signed := make([]byte, 0, len(payload)+sha256.Size)
	signed = append(signed, payload...)
	signed = append(signed, mac.Sum(nil)...)
	encoded := base64.RawURLEncoding.EncodeToString(signed)
	if len(encoded) > maximumAuthCursorBytes {
		return "", auth.ErrAuthRepositoryUnavailable
	}
	return encoded, nil
}

func (store *Store) decodeAuthCursor(query auth.UserListQuery) (*authCursorState, error) {
	if query.Cursor == "" {
		return nil, nil
	}
	if store == nil || store.closed.Load() || len(query.Cursor) > maximumAuthCursorBytes {
		return nil, auth.ErrAuthRepositoryRejected
	}
	signed, err := decodeCanonicalAuthBase64(query.Cursor)
	if err != nil || len(signed) <= sha256.Size || len(signed) > maximumAuthCursorBytes {
		return nil, auth.ErrAuthRepositoryRejected
	}
	payload := signed[:len(signed)-sha256.Size]
	signature := signed[len(signed)-sha256.Size:]
	mac := hmac.New(sha256.New, store.authDigestKey[:])
	_, _ = mac.Write([]byte("stackplore/user-cursor/v1\x00"))
	_, _ = mac.Write(payload)
	if subtle.ConstantTimeCompare(signature, mac.Sum(nil)) != 1 {
		return nil, auth.ErrAuthRepositoryRejected
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var state authCursorState
	if err := decoder.Decode(&state); err != nil {
		return nil, auth.ErrAuthRepositoryRejected
	}
	if err := ensureAuthJSONEnd(decoder); err != nil {
		return nil, auth.ErrAuthRepositoryRejected
	}
	role := ""
	if query.Role != nil {
		role = string(*query.Role)
	}
	canonical, canonicalErr := auth.CanonicalLoginName(state.CanonicalName)
	if state.Version != authCursorVersion || state.Search != query.Search || state.Role != role || !sameOptionalBool(state.Enabled, query.Enabled) || canonicalErr != nil || canonical != state.CanonicalName || auth.UserID(state.UserID).Validate() != nil {
		return nil, auth.ErrAuthRepositoryRejected
	}
	return &state, nil
}

func decodeCanonicalAuthBase64(value string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return nil, auth.ErrAuthRepositoryRejected
	}
	return decoded, nil
}

func ensureAuthJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return auth.ErrAuthRepositoryRejected
	}
	return nil
}

func validInitialSetupCommit(commit auth.InitialSetupCommit) bool {
	return validPreauthUse(commit.Preauth) &&
		commit.Preauth.Purpose == auth.PreauthPurposeSetup &&
		validThrottleKey(commit.ClearThrottle) &&
		commit.ClearThrottle.Scope == auth.ThrottleScopeSetupSource &&
		commit.User.Validate() == nil &&
		commit.User.Role == auth.RoleAdministrator && commit.User.Enabled &&
		commit.User.AuthRevision == 1 && commit.User.Version == 1 &&
		commit.User.CreatedAt.Equal(commit.User.UpdatedAt) &&
		commit.Credential.Validate() == nil &&
		commit.Session.Validate() == nil && commit.Session.UserID == commit.User.ID &&
		commit.Session.UserAuthRevision == commit.User.AuthRevision &&
		commit.Session.CreatedAt.Equal(commit.User.CreatedAt) && commit.Session.RevokedAt == nil &&
		commit.Audit.Validate() == nil && validAuthTime(commit.Audit.OccurredAt) &&
		commit.Audit.OccurredAt.Equal(commit.User.CreatedAt) &&
		commit.Audit.Actor.Kind == domain.AuditActorSystem && commit.Audit.Actor.ID == "" &&
		commit.Audit.Outcome == domain.AuditSuccess &&
		auditTargetsUser(commit.Audit, commit.User.ID)
}

func validSetupFailureCommit(commit auth.SetupFailureCommit) bool {
	return validPreauthUse(commit.Preauth) && commit.Preauth.Purpose == auth.PreauthPurposeSetup &&
		validThrottleFailure(commit.Throttle) && commit.Throttle.Key.Scope == auth.ThrottleScopeSetupSource &&
		commit.Throttle.OccurredAt.Equal(commit.Preauth.ConsumedAt) &&
		commit.Audit.Validate() == nil && validAuthTime(commit.Audit.OccurredAt) &&
		commit.Audit.OccurredAt.Equal(commit.Preauth.ConsumedAt) &&
		commit.Audit.Actor.Kind == domain.AuditActorSystem && commit.Audit.Actor.ID == "" &&
		commit.Audit.Target.ID == "" && commit.Audit.Outcome == domain.AuditDenied
}

func validLoginFailureCommit(commit auth.LoginFailureCommit) bool {
	if len(commit.Throttles) < 1 || len(commit.Throttles) > maximumAuthThrottleBatch ||
		commit.Audit.Validate() != nil || !validAuthTime(commit.Audit.OccurredAt) ||
		commit.Audit.Actor.Kind != domain.AuditActorSystem || commit.Audit.Actor.ID != "" ||
		commit.Audit.Target.ID != "" || commit.Audit.Outcome != domain.AuditDenied {
		return false
	}
	seen := make(map[auth.ThrottleKey]struct{}, len(commit.Throttles))
	for _, failure := range commit.Throttles {
		if !validThrottleFailure(failure) || failure.Key.Scope != auth.ThrottleScopeLoginSource && failure.Key.Scope != auth.ThrottleScopeLoginAccount ||
			!failure.OccurredAt.Equal(commit.Audit.OccurredAt) {
			return false
		}
		if _, duplicate := seen[failure.Key]; duplicate {
			return false
		}
		seen[failure.Key] = struct{}{}
	}
	return true
}

func validLoginSuccessCommit(commit auth.LoginSuccessCommit) bool {
	if commit.Session.Validate() != nil || commit.Session.RevokedAt != nil || commit.Audit.Validate() != nil ||
		!validAuthTime(commit.Audit.OccurredAt) || !commit.Audit.OccurredAt.Equal(commit.Session.CreatedAt) ||
		commit.Audit.Actor.Kind != domain.AuditActorUser || commit.Audit.Actor.ID != string(commit.Session.UserID) ||
		!auditTargetsUser(commit.Audit, commit.Session.UserID) || commit.Audit.Outcome != domain.AuditSuccess || len(commit.ClearThrottles) > maximumAuthThrottleBatch {
		return false
	}
	if commit.RehashedCredential != nil && commit.RehashedCredential.Validate() != nil {
		return false
	}
	for _, key := range commit.ClearThrottles {
		if !validThrottleKey(key) {
			return false
		}
	}
	return true
}

func validOwnPasswordChangeCommit(commit auth.OwnPasswordChangeCommit) bool {
	if commit.Actor.Validate() != nil || commit.Credential.Validate() != nil || !validAuthTime(commit.ChangedAt) ||
		!validRecentGrantUse(commit.Grant, auth.RecentAuthPurposeAccountPasswordChange, commit.ChangedAt) ||
		commit.Grant.Actor != commit.Actor || commit.RotatedSession.Validate() != nil ||
		commit.RotatedSession.UserID != commit.Actor.UserID ||
		commit.RotatedSession.UserAuthRevision != commit.Actor.AuthRevision+1 ||
		!commit.RotatedSession.CreatedAt.Equal(commit.ChangedAt) || commit.RotatedSession.RevokedAt != nil ||
		commit.RotatedSession.RotatedFromSessionID == nil || *commit.RotatedSession.RotatedFromSessionID != commit.Actor.SessionID ||
		commit.Audit.Validate() != nil || !validAuthTime(commit.Audit.OccurredAt) ||
		!commit.Audit.OccurredAt.Equal(commit.ChangedAt) || !auditMatchesActor(commit.Audit, commit.Actor) ||
		!auditTargetsUser(commit.Audit, commit.Actor.UserID) || commit.Audit.Outcome != domain.AuditSuccess {
		return false
	}
	return true
}

func validUserCreateCommit(commit auth.UserCreateCommit) bool {
	return commit.User.Validate() == nil && commit.User.Enabled && commit.User.DeletedAt == nil &&
		commit.User.AuthRevision == 1 && commit.User.Version == 1 &&
		commit.User.CreatedAt.Equal(commit.User.UpdatedAt) && commit.Credential.Validate() == nil &&
		validAdministratorMutation(commit.Actor, commit.Grant, auth.RecentAuthPurposeUserCreate, commit.Audit, commit.User.ID, commit.User.CreatedAt)
}

func validUserUpdateCommit(commit auth.UserUpdateCommit) bool {
	return commit.ExpectedVersion >= 1 && commit.User.Validate() == nil && commit.User.DeletedAt == nil &&
		validAdministratorMutation(commit.Actor, commit.Grant, auth.RecentAuthPurposeUserUpdate, commit.Audit, commit.User.ID, commit.User.UpdatedAt)
}

func validUserDeleteCommit(commit auth.UserDeleteCommit) bool {
	return commit.UserID.Validate() == nil && commit.ExpectedVersion >= 1 && validAuthTime(commit.DeletedAt) &&
		validAdministratorMutation(commit.Actor, commit.Grant, auth.RecentAuthPurposeUserDelete, commit.Audit, commit.UserID, commit.DeletedAt)
}

func validUserPasswordResetCommit(commit auth.UserPasswordResetCommit) bool {
	return commit.UserID.Validate() == nil && commit.ExpectedVersion >= 1 && commit.Credential.Validate() == nil &&
		validAuthTime(commit.ChangedAt) &&
		validAdministratorMutation(commit.Actor, commit.Grant, auth.RecentAuthPurposeUserPasswordReset, commit.Audit, commit.UserID, commit.ChangedAt)
}

func validUserSessionsRevokeCommit(commit auth.UserSessionsRevokeCommit) bool {
	return commit.UserID.Validate() == nil && validAuthTime(commit.RevokedAt) &&
		validAdministratorMutation(commit.Actor, commit.Grant, auth.RecentAuthPurposeUserSessionsRevoke, commit.Audit, commit.UserID, commit.RevokedAt)
}

func validOperatorRecoveryCommit(commit auth.OperatorRecoveryCommit) bool {
	return (commit.OperatorID == "" || validateAuthIdentity(commit.OperatorID) == nil) && commit.RequestedUser.Validate() == nil &&
		commit.RequestedUser.Role == auth.RoleAdministrator && commit.RequestedUser.Enabled &&
		commit.RequestedUser.DeletedAt == nil && commit.RequestedUser.DisabledAt == nil &&
		!commit.RequestedUser.CreatedAt.After(commit.RecoveredAt) && commit.RequestedUser.UpdatedAt.Equal(commit.RecoveredAt) &&
		commit.Credential.Validate() == nil && validAuthTime(commit.RecoveredAt) &&
		commit.Audit.Validate() == nil && validAuthTime(commit.Audit.OccurredAt) &&
		commit.Audit.OccurredAt.Equal(commit.RecoveredAt) &&
		commit.Audit.Actor.Kind == domain.AuditActorOperator && commit.Audit.Actor.ID == commit.OperatorID &&
		commit.Audit.Outcome == domain.AuditSuccess
}

func validAdministratorMutation(actor auth.Principal, grant auth.RecentGrantUse, purpose auth.RecentAuthPurpose, audit domain.AuditEntry, target auth.UserID, occurredAt time.Time) bool {
	return actor.Validate() == nil && actor.Role == auth.RoleAdministrator &&
		validRecentGrantUse(grant, purpose, occurredAt) && grant.Actor == actor &&
		audit.Validate() == nil && validAuthTime(audit.OccurredAt) && audit.OccurredAt.Equal(occurredAt) &&
		auditMatchesActor(audit, actor) && auditTargetsUser(audit, target) && audit.Outcome == domain.AuditSuccess
}

func validateUserUpdateTransition(current auth.User, next auth.User, expectedVersion int64) (bool, error) {
	if current.Validate() != nil {
		return false, auth.ErrAuthRepositoryUnavailable
	}
	if current.Version != expectedVersion {
		return false, auth.ErrAuthRepositoryStaleVersion
	}
	if next.Validate() != nil || next.ID != current.ID || !next.CreatedAt.Equal(current.CreatedAt) || next.DeletedAt != nil ||
		next.Version != expectedVersion+1 || next.UpdatedAt.Before(current.UpdatedAt) {
		return false, auth.ErrAuthRepositoryRejected
	}
	changed := next.LoginName != current.LoginName || next.DisplayName != current.DisplayName || next.Role != current.Role || next.Enabled != current.Enabled
	if !changed {
		return false, auth.ErrAuthRepositoryRejected
	}
	securityChanged := next.LoginName != current.LoginName || next.CanonicalLoginName != current.CanonicalLoginName || next.Role != current.Role || next.Enabled != current.Enabled
	wantedRevision := current.AuthRevision
	if securityChanged {
		wantedRevision++
	}
	if next.AuthRevision != wantedRevision {
		return false, auth.ErrAuthRepositoryRejected
	}
	if current.Enabled && !next.Enabled {
		if next.DisabledAt == nil || !next.DisabledAt.Equal(next.UpdatedAt) {
			return false, auth.ErrAuthRepositoryRejected
		}
	} else if !current.Enabled && !next.Enabled {
		if current.DisabledAt == nil || next.DisabledAt == nil || !next.DisabledAt.Equal(*current.DisabledAt) {
			return false, auth.ErrAuthRepositoryRejected
		}
	} else if next.DisabledAt != nil {
		return false, auth.ErrAuthRepositoryRejected
	}
	return securityChanged, nil
}

func validRecentGrantUse(use auth.RecentGrantUse, purpose auth.RecentAuthPurpose, consumedAt time.Time) bool {
	return use.Actor.Validate() == nil && purpose.Validate() == nil && use.Purpose == purpose &&
		use.Digest.Validate() == nil && validateAuthIdentity(use.DigestKeyID) == nil &&
		validAuthTime(use.ConsumedAt) && use.ConsumedAt.Equal(consumedAt)
}

func validPreauthUse(use auth.PreauthUse) bool {
	return use.Purpose.Validate() == nil && use.CookieDigest.Validate() == nil && use.ProofDigest.Validate() == nil &&
		validateAuthIdentity(use.DigestKeyID) == nil && validAuthTime(use.ConsumedAt)
}

func validThrottleKey(key auth.ThrottleKey) bool {
	if key.Scope != auth.ThrottleScopeLoginSource && key.Scope != auth.ThrottleScopeLoginAccount && key.Scope != auth.ThrottleScopeSetupSource && key.Scope != auth.ThrottleScopeRecentUser {
		return false
	}
	var zero [auth.DigestBytes]byte
	return subtle.ConstantTimeCompare(key.Digest[:], zero[:]) != 1
}

func validThrottleFailure(failure auth.ThrottleFailure) bool {
	const maximumThrottleDuration = 365 * 24 * time.Hour
	if !validThrottleKey(failure.Key) || !validAuthTime(failure.OccurredAt) ||
		failure.Window <= 0 || failure.Window > maximumThrottleDuration || failure.Window%time.Microsecond != 0 ||
		failure.BlockDuration <= 0 || failure.BlockDuration > maximumThrottleDuration || failure.BlockDuration%time.Microsecond != 0 ||
		failure.Retention <= 0 || failure.Retention > maximumThrottleDuration || failure.Retention%time.Microsecond != 0 ||
		failure.MaximumFailures < 1 || failure.MaximumFailures > 1_000_000 {
		return false
	}
	return validAuthTime(failure.OccurredAt.Add(failure.Window)) &&
		validAuthTime(failure.OccurredAt.Add(failure.BlockDuration)) &&
		validAuthTime(failure.OccurredAt.Add(failure.Retention))
}

func auditMatchesActor(entry domain.AuditEntry, actor auth.Principal) bool {
	return entry.Actor.Kind == domain.AuditActorUser && entry.Actor.ID == string(actor.UserID)
}

func auditTargetsUser(entry domain.AuditEntry, userID auth.UserID) bool {
	return entry.Target.ID == string(userID)
}

func validAuthTime(value time.Time) bool {
	if value.IsZero() || value.Year() < 1 || value.Year() > 9999 || value.Nanosecond()%int(time.Microsecond) != 0 {
		return false
	}
	_, offset := value.Zone()
	return offset == 0
}

func validateAuthIdentity(value string) error {
	if len(value) < 1 || len(value) > 128 || !authASCIIAlpha(value[0]) {
		return auth.ErrAuthRepositoryRejected
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if !authASCIIAlpha(character) && (character < '0' || character > '9') && character != '_' && character != '-' {
			return auth.ErrAuthRepositoryRejected
		}
	}
	return nil
}

func validAuthCode(value string) bool {
	if len(value) < 3 || len(value) > 96 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validUserSearch(value string) bool {
	if value == "" {
		return true
	}
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 200 || value != strings.TrimSpace(value) {
		return false
	}
	hasVisibleBase := false
	for _, character := range value {
		if unicode.IsControl(character) || unicode.In(character, unicode.Zl, unicode.Zp) || unicode.In(character, unicode.Cf) && character != '\u200c' && character != '\u200d' {
			return false
		}
		if unicode.IsLetter(character) || unicode.IsNumber(character) || unicode.IsPunct(character) || unicode.IsSymbol(character) {
			hasVisibleBase = true
		}
	}
	return hasVisibleBase
}

func validateAuthRead(store *Store, ctx context.Context) error {
	if store == nil || store.db == nil || store.closed.Load() {
		return auth.ErrAuthRepositoryUnavailable
	}
	if ctx == nil {
		return auth.ErrAuthRepositoryUnavailable
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(auth.ErrAuthRepositoryUnavailable, err)
	}
	return nil
}

func mapAuthRepositoryError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		return errors.Join(auth.ErrAuthRepositoryUnavailable, ctx.Err())
	}
	if errors.Is(err, auth.ErrAuthRepositoryConflict) || errors.Is(err, auth.ErrAuthRepositoryNotFound) ||
		errors.Is(err, auth.ErrAuthRepositoryRejected) || errors.Is(err, auth.ErrAuthRepositoryUnavailable) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		return auth.ErrAuthRepositoryNotFound
	}
	var sqliteError *sqlite.Error
	if errors.As(err, &sqliteError) {
		if sqliteError.Code()&0xff == sqlite3.SQLITE_CONSTRAINT {
			return auth.ErrAuthRepositoryConflict
		}
		return auth.ErrAuthRepositoryUnavailable
	}
	return auth.ErrAuthRepositoryUnavailable
}

func authPreauthDigestBytes(preauth auth.Preauth) ([]byte, []byte, error) {
	cookieDigest, err := preauth.CookieDigest.Bytes()
	if err != nil {
		return nil, nil, err
	}
	proofDigest, err := preauth.ProofDigest.Bytes()
	if err != nil {
		return nil, nil, err
	}
	return cookieDigest, proofDigest, nil
}

func authRowsAffected(result sql.Result, wanted int64) bool {
	if result == nil {
		return false
	}
	affected, err := result.RowsAffected()
	return err == nil && affected == wanted
}

func nullableAuthTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UnixMicro()
}

func nullableAuthString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableSessionID(value *auth.SessionID) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func nullableAuthInt64(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func authTimeFromNull(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	converted := time.UnixMicro(value.Int64).UTC()
	return &converted
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func sameOptionalBool(left *bool, right *bool) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func authASCIIAlpha(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}

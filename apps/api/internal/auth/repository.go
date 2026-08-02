package auth

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

var (
	ErrAuthRepositoryConflict    = errors.New("authentication state conflicts with stored state")
	ErrAuthRepositoryNotFound    = errors.New("authentication state was not found")
	ErrAuthRepositoryRejected    = errors.New("authentication transition was rejected")
	ErrAuthRepositoryUnavailable = errors.New("authentication repository is unavailable")

	ErrAuthRepositoryLastAdministrator = newConflictError(ErrAuthRepositoryConflict, ConflictCodeLastAdministrator)
	ErrAuthRepositoryStaleVersion      = newConflictError(ErrAuthRepositoryConflict, ConflictCodeStaleVersion)
	ErrAuthRepositoryLoginNameConflict = newConflictError(ErrAuthRepositoryConflict, ConflictCodeLoginNameConflict)
)

type ConflictCode string

const (
	ConflictCodeLastAdministrator ConflictCode = "last_administrator"
	ConflictCodeStaleVersion      ConflictCode = "stale_version"
	ConflictCodeLoginNameConflict ConflictCode = "login_name_conflict"
)

type conflictError struct {
	base error
	code ConflictCode
}

func newConflictError(base error, code ConflictCode) error {
	return &conflictError{base: base, code: code}
}

func (err *conflictError) Error() string {
	if err == nil || err.base == nil {
		return ErrAuthRepositoryConflict.Error()
	}
	return err.base.Error()
}

func (err *conflictError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.base
}

func (err *conflictError) Code() string {
	if err == nil {
		return ""
	}
	return string(err.code)
}

type ThrottleScope string

const (
	ThrottleScopeLoginSource  ThrottleScope = "login_source"
	ThrottleScopeLoginAccount ThrottleScope = "login_account"
	ThrottleScopeSetupSource  ThrottleScope = "setup_source"
	ThrottleScopeRecentUser   ThrottleScope = "recent_user"
)

type ThrottleKey struct {
	Scope  ThrottleScope
	Digest [DigestBytes]byte
}

type ThrottleState struct {
	BlockedUntil time.Time
}

type ThrottleFailure struct {
	Key             ThrottleKey
	OccurredAt      time.Time
	Window          time.Duration
	MaximumFailures int
	BlockDuration   time.Duration
	Retention       time.Duration
}

type PreauthUse struct {
	Purpose      PreauthPurpose
	CookieDigest PreauthCookieDigest
	ProofDigest  PreauthProofDigest
	DigestKeyID  string
	ConsumedAt   time.Time
}

type LoginAccount struct {
	User       User
	Credential PasswordCredential
}

type ResolvedSession struct {
	User    User
	Session Session
}

type InitialSetupCommit struct {
	Preauth       PreauthUse
	ClearThrottle ThrottleKey
	User          User
	Credential    PasswordCredential
	Session       Session
	Audit         domain.AuditEntry
}

type SetupFailureCommit struct {
	Preauth  PreauthUse
	Throttle ThrottleFailure
	Audit    domain.AuditEntry
}

type LoginSuccessCommit struct {
	Session            Session
	RehashedCredential *PasswordCredential
	ClearThrottles     []ThrottleKey
	Audit              domain.AuditEntry
}

type LoginFailureCommit struct {
	Throttles []ThrottleFailure
	Audit     domain.AuditEntry
}

type SessionTouch struct {
	SessionID     SessionID
	UserID        UserID
	AuthRevision  int64
	LastSeenAt    time.Time
	IdleExpiresAt time.Time
}

type SessionRevocation struct {
	Actor     Principal
	SessionID SessionID
	RevokedAt time.Time
	Reason    string
	Audit     domain.AuditEntry
}

type RecentGrantCommit struct {
	Actor         Principal
	Grant         RecentGrant
	ClearThrottle ThrottleKey
	Audit         domain.AuditEntry
}

type RecentAuthFailureCommit struct {
	Actor    Principal
	Throttle *ThrottleFailure
	Audit    domain.AuditEntry
}

type RecentGrantUse struct {
	Actor       Principal
	Purpose     RecentAuthPurpose
	Digest      RecentAuthDigest
	DigestKeyID string
	ConsumedAt  time.Time
}

type UserListQuery struct {
	Cursor  string
	Limit   int
	Search  string
	Role    *Role
	Enabled *bool
}

type UserPage struct {
	Items      []User
	NextCursor string
}

type OwnPasswordChangeCommit struct {
	Actor          Principal
	Grant          RecentGrantUse
	Credential     PasswordCredential
	RotatedSession Session
	ChangedAt      time.Time
	Audit          domain.AuditEntry
}

type UserCreateCommit struct {
	Actor      Principal
	Grant      RecentGrantUse
	User       User
	Credential PasswordCredential
	Audit      domain.AuditEntry
}

type UserUpdateCommit struct {
	Actor           Principal
	Grant           RecentGrantUse
	User            User
	ExpectedVersion int64
	Audit           domain.AuditEntry
}

type UserDeleteCommit struct {
	Actor           Principal
	Grant           RecentGrantUse
	UserID          UserID
	ExpectedVersion int64
	DeletedAt       time.Time
	Audit           domain.AuditEntry
}

type UserPasswordResetCommit struct {
	Actor           Principal
	Grant           RecentGrantUse
	UserID          UserID
	ExpectedVersion int64
	Credential      PasswordCredential
	ChangedAt       time.Time
	Audit           domain.AuditEntry
}

type UserSessionsRevokeCommit struct {
	Actor     Principal
	Grant     RecentGrantUse
	UserID    UserID
	RevokedAt time.Time
	Audit     domain.AuditEntry
}

type OperatorRecoveryCommit struct {
	OperatorID    string
	RequestedUser User
	Credential    PasswordCredential
	RecoveredAt   time.Time
	Audit         domain.AuditEntry
}

// Repository exposes use-case-shaped reads and atomic authentication transitions.
// Every Commit method must apply its state, grant, throttle, and audit changes in one transaction.
type Repository interface {
	SetupRequired(context.Context) (bool, error)
	InsertPreauth(context.Context, Preauth) error
	CheckThrottle(context.Context, ThrottleKey, time.Time) (ThrottleState, error)
	CompleteInitialSetup(context.Context, InitialSetupCommit) error
	CommitSetupFailure(context.Context, SetupFailureCommit) error
	ConsumePreauth(context.Context, PreauthUse) error

	FindLoginAccount(context.Context, string) (LoginAccount, error)
	CommitLoginSuccess(context.Context, LoginSuccessCommit) error
	CommitLoginFailure(context.Context, LoginFailureCommit) error

	ResolveSession(context.Context, SessionTokenDigest, string) (ResolvedSession, error)
	TouchSession(context.Context, SessionTouch) error
	RevokeSession(context.Context, SessionRevocation) error

	CommitRecentGrant(context.Context, RecentGrantCommit) error
	CommitRecentAuthFailure(context.Context, RecentAuthFailureCommit) error

	ListUsers(context.Context, UserListQuery) (UserPage, error)
	GetUser(context.Context, UserID) (User, error)
	CommitOwnPasswordChange(context.Context, OwnPasswordChangeCommit) error
	CommitUserCreate(context.Context, UserCreateCommit) error
	CommitUserUpdate(context.Context, UserUpdateCommit) error
	CommitUserDelete(context.Context, UserDeleteCommit) error
	CommitUserPasswordReset(context.Context, UserPasswordResetCommit) error
	CommitUserSessionsRevoke(context.Context, UserSessionsRevokeCommit) error
	CommitOperatorRecovery(context.Context, OperatorRecoveryCommit) (User, error)
}

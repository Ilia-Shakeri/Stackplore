package auth

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MinimumLoginNameBytes = 3
	MaximumLoginNameBytes = 64
	MaximumDisplayRunes   = 256
	maximumIdentityBytes  = 128
)

var (
	ErrInvalidAuthState = errors.New("authentication state is invalid")
	ErrInvalidLoginName = errors.New("login name is invalid")
	ErrInvalidRole      = errors.New("authentication role is invalid")
)

type Role string

const (
	RoleAdministrator Role = "administrator"
	RoleViewer        Role = "viewer"
)

func (role Role) Validate() error {
	if role != RoleAdministrator && role != RoleViewer {
		return ErrInvalidRole
	}
	return nil
}

type UserID string
type SessionID string

func (id UserID) Validate() error {
	return validateIdentity(string(id))
}

func (id SessionID) Validate() error {
	return validateIdentity(string(id))
}

// CanonicalLoginName validates one ASCII login name and returns its stable lookup form.
func CanonicalLoginName(loginName string) (string, error) {
	if len(loginName) < MinimumLoginNameBytes || len(loginName) > MaximumLoginNameBytes {
		return "", ErrInvalidLoginName
	}
	for index := range len(loginName) {
		character := loginName[index]
		if asciiLetter(character) || asciiDigit(character) || character == '.' || character == '_' || character == '-' {
			continue
		}
		return "", ErrInvalidLoginName
	}
	if !asciiAlphaNumeric(loginName[0]) || !asciiAlphaNumeric(loginName[len(loginName)-1]) {
		return "", ErrInvalidLoginName
	}
	return strings.ToLower(loginName), nil
}

type User struct {
	ID                 UserID
	LoginName          string
	CanonicalLoginName string
	DisplayName        string
	Role               Role
	Enabled            bool
	AuthRevision       int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DisabledAt         *time.Time
	DeletedAt          *time.Time
	Version            int64
}

func (user User) Validate() error {
	canonical, err := CanonicalLoginName(user.LoginName)
	if err != nil || canonical != user.CanonicalLoginName || user.ID.Validate() != nil || user.Role.Validate() != nil {
		return ErrInvalidAuthState
	}
	if !validDisplayName(user.DisplayName) || user.AuthRevision < 1 || user.Version < 1 {
		return ErrInvalidAuthState
	}
	if !validStoredTime(user.CreatedAt) || !validStoredTime(user.UpdatedAt) || user.UpdatedAt.Before(user.CreatedAt) {
		return ErrInvalidAuthState
	}
	if user.Enabled {
		if user.DisabledAt != nil || user.DeletedAt != nil {
			return ErrInvalidAuthState
		}
		return nil
	}
	if user.DisabledAt == nil || !validStoredTime(*user.DisabledAt) || user.DisabledAt.Before(user.CreatedAt) || user.DisabledAt.After(user.UpdatedAt) {
		return ErrInvalidAuthState
	}
	if user.DeletedAt != nil && (!validStoredTime(*user.DeletedAt) || user.DeletedAt.Before(*user.DisabledAt) || user.DeletedAt.After(user.UpdatedAt)) {
		return ErrInvalidAuthState
	}
	return nil
}

type Session struct {
	ID                   SessionID
	UserID               UserID
	TokenDigest          SessionTokenDigest
	TokenDigestKeyID     string
	CSRFDigest           CSRFDigest
	CSRFDigestKeyID      string
	UserAuthRevision     int64
	CreatedAt            time.Time
	LastSeenAt           time.Time
	IdleExpiresAt        time.Time
	AbsoluteExpiresAt    time.Time
	RevokedAt            *time.Time
	RevokeReason         string
	RotatedFromSessionID *SessionID
}

func (session Session) Validate() error {
	if session.ID.Validate() != nil || session.UserID.Validate() != nil || session.TokenDigest.Validate() != nil || session.CSRFDigest.Validate() != nil {
		return ErrInvalidAuthState
	}
	if validateIdentity(session.TokenDigestKeyID) != nil || validateIdentity(session.CSRFDigestKeyID) != nil || session.UserAuthRevision < 1 {
		return ErrInvalidAuthState
	}
	if !validStoredTime(session.CreatedAt) || !validStoredTime(session.LastSeenAt) || !validStoredTime(session.IdleExpiresAt) || !validStoredTime(session.AbsoluteExpiresAt) {
		return ErrInvalidAuthState
	}
	if session.LastSeenAt.Before(session.CreatedAt) || session.IdleExpiresAt.Before(session.LastSeenAt) || !session.IdleExpiresAt.After(session.CreatedAt) || session.AbsoluteExpiresAt.Before(session.IdleExpiresAt) {
		return ErrInvalidAuthState
	}
	if (session.RevokedAt == nil) != (session.RevokeReason == "") {
		return ErrInvalidAuthState
	}
	if session.RevokeReason != "" && !validBoundedCode(session.RevokeReason) {
		return ErrInvalidAuthState
	}
	if session.RevokedAt != nil && (!validStoredTime(*session.RevokedAt) || session.RevokedAt.Before(session.CreatedAt)) {
		return ErrInvalidAuthState
	}
	if session.RotatedFromSessionID != nil && (session.RotatedFromSessionID.Validate() != nil || *session.RotatedFromSessionID == session.ID) {
		return ErrInvalidAuthState
	}
	return nil
}

type PreauthPurpose string

const (
	PreauthPurposeLogin PreauthPurpose = "login"
	PreauthPurposeSetup PreauthPurpose = "setup"
)

func (purpose PreauthPurpose) Validate() error {
	if purpose != PreauthPurposeLogin && purpose != PreauthPurposeSetup {
		return ErrInvalidAuthState
	}
	return nil
}

type Preauth struct {
	ID           string
	Purpose      PreauthPurpose
	CookieDigest PreauthCookieDigest
	ProofDigest  PreauthProofDigest
	DigestKeyID  string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
}

func (preauth Preauth) Validate() error {
	if validateIdentity(preauth.ID) != nil || preauth.Purpose.Validate() != nil || preauth.CookieDigest.Validate() != nil || preauth.ProofDigest.Validate() != nil || validateIdentity(preauth.DigestKeyID) != nil {
		return ErrInvalidAuthState
	}
	if !validStoredTime(preauth.CreatedAt) || !validStoredTime(preauth.ExpiresAt) || !preauth.ExpiresAt.After(preauth.CreatedAt) {
		return ErrInvalidAuthState
	}
	if preauth.ConsumedAt != nil && (!validStoredTime(*preauth.ConsumedAt) || preauth.ConsumedAt.Before(preauth.CreatedAt) || preauth.ConsumedAt.After(preauth.ExpiresAt)) {
		return ErrInvalidAuthState
	}
	return nil
}

type RecentAuthPurpose string

const (
	RecentAuthPurposeAccountPasswordChange RecentAuthPurpose = "account.password_change"
	RecentAuthPurposeUserCreate            RecentAuthPurpose = "user.create"
	RecentAuthPurposeUserUpdate            RecentAuthPurpose = "user.update"
	RecentAuthPurposeUserDelete            RecentAuthPurpose = "user.delete"
	RecentAuthPurposeUserPasswordReset     RecentAuthPurpose = "user.password_reset"
	RecentAuthPurposeUserSessionsRevoke    RecentAuthPurpose = "user.sessions_revoke"
	RecentAuthPurposeNotificationTest      RecentAuthPurpose = "notification.test"
	RecentAuthPurposeSourceUpdate          RecentAuthPurpose = "source.update"
	RecentAuthPurposeOperationExecute      RecentAuthPurpose = "operation.execute"
	RecentAuthPurposeSettingsUpdate        RecentAuthPurpose = "settings.update"
	RecentAuthPurposeConfigurationExport   RecentAuthPurpose = "configuration.export"
	RecentAuthPurposeDataCleanup           RecentAuthPurpose = "data.cleanup"
)

func (purpose RecentAuthPurpose) Validate() error {
	switch purpose {
	case RecentAuthPurposeAccountPasswordChange,
		RecentAuthPurposeUserCreate,
		RecentAuthPurposeUserUpdate,
		RecentAuthPurposeUserDelete,
		RecentAuthPurposeUserPasswordReset,
		RecentAuthPurposeUserSessionsRevoke,
		RecentAuthPurposeNotificationTest,
		RecentAuthPurposeSourceUpdate,
		RecentAuthPurposeOperationExecute,
		RecentAuthPurposeSettingsUpdate,
		RecentAuthPurposeConfigurationExport,
		RecentAuthPurposeDataCleanup:
		return nil
	default:
		return ErrInvalidAuthState
	}
}

type RecentGrant struct {
	ID           string
	UserID       UserID
	SessionID    SessionID
	Purpose      RecentAuthPurpose
	Digest       RecentAuthDigest
	DigestKeyID  string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
	RevokedAt    *time.Time
	RevokeReason string
}

func (grant RecentGrant) Validate() error {
	if validateIdentity(grant.ID) != nil || grant.UserID.Validate() != nil || grant.SessionID.Validate() != nil || grant.Purpose.Validate() != nil || grant.Digest.Validate() != nil || validateIdentity(grant.DigestKeyID) != nil {
		return ErrInvalidAuthState
	}
	if !validStoredTime(grant.CreatedAt) || !validStoredTime(grant.ExpiresAt) || !grant.ExpiresAt.After(grant.CreatedAt) {
		return ErrInvalidAuthState
	}
	if grant.ConsumedAt != nil && (!validStoredTime(*grant.ConsumedAt) || grant.ConsumedAt.Before(grant.CreatedAt) || grant.ConsumedAt.After(grant.ExpiresAt)) {
		return ErrInvalidAuthState
	}
	if (grant.RevokedAt == nil) != (grant.RevokeReason == "") {
		return ErrInvalidAuthState
	}
	if grant.RevokeReason != "" && !validBoundedCode(grant.RevokeReason) {
		return ErrInvalidAuthState
	}
	if grant.RevokedAt != nil && (!validStoredTime(*grant.RevokedAt) || grant.RevokedAt.Before(grant.CreatedAt)) {
		return ErrInvalidAuthState
	}
	return nil
}

type Principal struct {
	UserID       UserID
	SessionID    SessionID
	Role         Role
	AuthRevision int64
}

func (principal Principal) Validate() error {
	if principal.UserID.Validate() != nil || principal.SessionID.Validate() != nil || principal.Role.Validate() != nil || principal.AuthRevision < 1 {
		return ErrInvalidAuthState
	}
	return nil
}

func validateIdentity(value string) error {
	if len(value) < 1 || len(value) > maximumIdentityBytes || !asciiLetter(value[0]) {
		return ErrInvalidAuthState
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if !asciiLetter(character) && !asciiDigit(character) && character != '_' && character != '-' {
			return ErrInvalidAuthState
		}
	}
	return nil
}

func validDisplayName(value string) bool {
	if !utf8.ValidString(value) || value == "" || value != strings.TrimSpace(value) || utf8.RuneCountInString(value) > MaximumDisplayRunes {
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

func validStoredTime(value time.Time) bool {
	if value.IsZero() || value.Year() < 1 || value.Year() > 9999 || value.Nanosecond()%int(time.Microsecond) != 0 {
		return false
	}
	_, offset := value.Zone()
	return offset == 0
}

func validBoundedCode(value string) bool {
	if len(value) < 3 || len(value) > 96 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' || asciiDigit(character) || character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func asciiAlphaNumeric(character byte) bool {
	return asciiLetter(character) || asciiDigit(character)
}

func asciiLetter(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}

func asciiDigit(character byte) bool {
	return character >= '0' && character <= '9'
}

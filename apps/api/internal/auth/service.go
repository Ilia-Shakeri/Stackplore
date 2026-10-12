package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const (
	PreauthLifetime      = 5 * time.Minute
	SessionIdleLifetime  = 30 * time.Minute
	SessionAbsoluteLife  = 12 * time.Hour
	SessionTouchInterval = time.Minute
	RecentGrantLifetime  = 5 * time.Minute

	loginSourceWindow    = 5 * time.Minute
	loginSourceBlock     = 5 * time.Minute
	loginAccountWindow   = 15 * time.Minute
	loginAccountBlock    = 15 * time.Minute
	setupSourceWindow    = 15 * time.Minute
	setupSourceBlock     = 15 * time.Minute
	recentUserWindow     = 15 * time.Minute
	recentUserBlock      = 15 * time.Minute
	throttleRetention    = time.Hour
	loginSourceFailures  = 10
	loginAccountFailures = 5
	setupSourceFailures  = 5
	recentUserFailures   = 5
)

var (
	ErrInvalidServiceOptions = errors.New("authentication service options are invalid")
	ErrInvalidServiceInput   = errors.New("authentication request is invalid")
	ErrSetupClosed           = errors.New("network setup is closed")
	ErrLoginRejected         = errors.New("sign in was rejected")
	ErrUnauthenticated       = errors.New("authentication is required")
	ErrCSRFRejected          = errors.New("request proof was rejected")
	ErrAuthorizationDenied   = errors.New("authorization was denied")
	ErrRecentAuthRejected    = errors.New("recent authentication was rejected")
	ErrAuthConflict          = errors.New("authentication state changed")
	ErrAuthNotFound          = errors.New("authentication target was not found")
	ErrAuthRateLimited       = errors.New("authentication attempt is rate limited")
	ErrAuthUnavailable       = errors.New("authentication service is unavailable")

	ErrLastAdministrator = newConflictError(ErrAuthConflict, ConflictCodeLastAdministrator)
	ErrStaleVersion      = newConflictError(ErrAuthConflict, ConflictCodeStaleVersion)
	ErrLoginNameConflict = newConflictError(ErrAuthConflict, ConflictCodeLoginNameConflict)
)

type ServiceOptions struct {
	Repository        Repository
	PasswordHasher    *PasswordHasher
	DigestKey         []byte
	DigestKeyID       string
	BootstrapRequired bool
	BootstrapGate     *BootstrapGate
	Now               func() time.Time
	Random            io.Reader
}

type Service struct {
	repository        Repository
	passwordHasher    *PasswordHasher
	digestKey         [DigestKeyBytes]byte
	digestKeyID       string
	bootstrapRequired bool
	bootstrapGate     *BootstrapGate
	now               func() time.Time
	random            io.Reader
	dummyCredential   PasswordCredential
}

type SetupStatus struct {
	Required               bool
	BootstrapProofRequired bool
}

type PreauthIssue struct {
	Cookie    OpaqueToken
	Proof     OpaqueToken
	ExpiresAt time.Time
}

type SessionContext struct {
	User              User
	CSRFProof         OpaqueToken
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
}

type SessionIssue struct {
	Token   OpaqueToken
	Context SessionContext
}

type Authentication struct {
	Principal Principal
	User      User
	Session   Session
	Context   SessionContext
	token     OpaqueToken
}

type RecentGrantIssue struct {
	Proof     OpaqueToken
	Purpose   RecentAuthPurpose
	ExpiresAt time.Time
}

type SetupInput struct {
	PreauthCookie  string
	PreauthProof   string
	BootstrapProof string
	SourceKey      string
	LoginName      string
	DisplayName    string
	Password       []byte
	RequestID      domain.RequestID
}

type LoginInput struct {
	PreauthCookie string
	PreauthProof  string
	SourceKey     string
	LoginName     string
	Password      []byte
	RequestID     domain.RequestID
}

type LogoutInput struct {
	SessionToken string
	CSRFProof    string
	RequestID    domain.RequestID
}

type RecentAuthInput struct {
	SessionToken    string
	CSRFProof       string
	CurrentPassword []byte
	Purpose         RecentAuthPurpose
	RequestID       domain.RequestID
}

type RecentAuthorizationInput struct {
	SessionToken string
	CSRFProof    string
	RecentProof  string
	Purpose      RecentAuthPurpose
	RequestID    domain.RequestID
}

type AuthenticatedReadInput struct {
	SessionToken string
}

type OwnPasswordChangeInput struct {
	SessionToken string
	CSRFProof    string
	RecentProof  string
	NewPassword  []byte
	RequestID    domain.RequestID
}

type UserCreateInput struct {
	SessionToken string
	CSRFProof    string
	RecentProof  string
	LoginName    string
	DisplayName  string
	Role         Role
	Password     []byte
	RequestID    domain.RequestID
}

type UserUpdateInput struct {
	SessionToken    string
	CSRFProof       string
	RecentProof     string
	UserID          UserID
	ExpectedVersion int64
	LoginName       *string
	DisplayName     *string
	Role            *Role
	Enabled         *bool
	RequestID       domain.RequestID
}

type UserDeleteInput struct {
	SessionToken    string
	CSRFProof       string
	RecentProof     string
	UserID          UserID
	ExpectedVersion int64
	RequestID       domain.RequestID
}

type UserPasswordResetInput struct {
	SessionToken    string
	CSRFProof       string
	RecentProof     string
	UserID          UserID
	ExpectedVersion int64
	NewPassword     []byte
	RequestID       domain.RequestID
}

type UserSessionsRevokeInput struct {
	SessionToken string
	CSRFProof    string
	RecentProof  string
	UserID       UserID
	RequestID    domain.RequestID
}

type OperatorRecoveryInput struct {
	OperatorID  string
	LoginName   string
	DisplayName string
	Password    []byte
	RequestID   domain.RequestID
}

type throttledError struct {
	until time.Time
}

func (err *throttledError) Error() string { return ErrAuthRateLimited.Error() }
func (err *throttledError) Unwrap() error { return ErrAuthRateLimited }
func (err *throttledError) RetryAt() time.Time {
	if err == nil {
		return time.Time{}
	}
	return err.until
}

// NewService builds the application authentication boundary and one dummy verifier record.
func NewService(options ServiceOptions) (*Service, error) {
	if options.Repository == nil || options.PasswordHasher == nil || len(options.DigestKey) != DigestKeyBytes ||
		validateIdentity(options.DigestKeyID) != nil || options.BootstrapGate != nil && !options.BootstrapRequired {
		return nil, ErrInvalidServiceOptions
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	randomSource := options.Random
	if randomSource == nil {
		randomSource = rand.Reader
	}
	service := &Service{
		repository:        options.Repository,
		passwordHasher:    options.PasswordHasher,
		digestKeyID:       options.DigestKeyID,
		bootstrapRequired: options.BootstrapRequired,
		bootstrapGate:     options.BootstrapGate,
		now:               now,
		random:            randomSource,
	}
	copy(service.digestKey[:], options.DigestKey)
	if _, err := service.currentTime(); err != nil {
		return nil, ErrInvalidServiceOptions
	}
	dummy, err := options.PasswordHasher.Hash(context.Background(), []byte("stackplore-invalid-password-probe"))
	if err != nil {
		return nil, ErrAuthUnavailable
	}
	service.dummyCredential = dummy
	return service, nil
}

func (service *Service) SetupStatus(ctx context.Context) (SetupStatus, error) {
	if err := validateServiceContext(ctx); err != nil || !service.valid() {
		return SetupStatus{}, ErrAuthUnavailable
	}
	required, err := service.repository.SetupRequired(ctx)
	if err != nil {
		return SetupStatus{}, mapRepositoryError(err)
	}
	return SetupStatus{Required: required, BootstrapProofRequired: required && service.bootstrapRequired}, nil
}

func (service *Service) CreatePreauth(ctx context.Context, purpose PreauthPurpose, sourceKey string) (PreauthIssue, error) {
	if err := validateServiceContext(ctx); err != nil || !service.valid() || purpose.Validate() != nil || !validSourceKey(sourceKey) {
		return PreauthIssue{}, ErrInvalidServiceInput
	}
	now, err := service.currentTime()
	if err != nil {
		return PreauthIssue{}, err
	}
	required, err := service.repository.SetupRequired(ctx)
	if err != nil {
		return PreauthIssue{}, mapRepositoryError(err)
	}
	if purpose == PreauthPurposeSetup {
		if !required {
			return PreauthIssue{}, ErrSetupClosed
		}
		key := service.throttleKey(ThrottleScopeSetupSource, "source", sourceKey)
		if err := service.requireThrottleOpen(ctx, key, now); err != nil {
			return PreauthIssue{}, err
		}
	} else if required {
		return PreauthIssue{}, ErrLoginRejected
	}

	cookie, err := generateOpaqueToken(service.random)
	if err != nil {
		return PreauthIssue{}, ErrAuthUnavailable
	}
	proof, err := generateOpaqueToken(service.random)
	if err != nil {
		return PreauthIssue{}, ErrAuthUnavailable
	}
	cookieDigest, err := DigestPreauthCookie(service.digestKey[:], cookie)
	if err != nil {
		return PreauthIssue{}, ErrAuthUnavailable
	}
	proofDigest, err := DigestPreauthProof(service.digestKey[:], proof)
	if err != nil {
		return PreauthIssue{}, ErrAuthUnavailable
	}
	id, err := service.newIdentity("pre_")
	if err != nil {
		return PreauthIssue{}, err
	}
	expiresAt := now.Add(PreauthLifetime)
	preauth := Preauth{
		ID: id, Purpose: purpose, CookieDigest: cookieDigest, ProofDigest: proofDigest,
		DigestKeyID: service.digestKeyID, CreatedAt: now, ExpiresAt: expiresAt,
	}
	if err := preauth.Validate(); err != nil {
		return PreauthIssue{}, ErrAuthUnavailable
	}
	if err := service.repository.InsertPreauth(ctx, preauth); err != nil {
		return PreauthIssue{}, mapRepositoryError(err)
	}
	return PreauthIssue{Cookie: cookie, Proof: proof, ExpiresAt: expiresAt}, nil
}

func (service *Service) Setup(ctx context.Context, input SetupInput) (SessionIssue, error) {
	if err := validateServiceContext(ctx); err != nil || !service.valid() {
		return SessionIssue{}, ErrAuthUnavailable
	}
	now, err := service.currentTime()
	if err != nil {
		return SessionIssue{}, err
	}
	preauth, err := service.parsePreauth(input.PreauthCookie, input.PreauthProof, PreauthPurposeSetup, now)
	if err != nil {
		return SessionIssue{}, err
	}
	canonical, loginErr := CanonicalLoginName(input.LoginName)
	if !validSourceKey(input.SourceKey) || input.RequestID.Validate() != nil || loginErr != nil ||
		!validDisplayName(input.DisplayName) || !validPassword(input.Password) ||
		!service.bootstrapRequired && input.BootstrapProof != "" {
		if consumeErr := service.consumeRejectedPreauth(ctx, preauth); consumeErr != nil {
			return SessionIssue{}, consumeErr
		}
		return SessionIssue{}, ErrInvalidServiceInput
	}
	setupKey := service.throttleKey(ThrottleScopeSetupSource, "source", input.SourceKey)
	if err := service.requireThrottleOpen(ctx, setupKey, now); err != nil {
		if consumeErr := service.consumeRejectedPreauth(ctx, preauth); consumeErr != nil {
			return SessionIssue{}, consumeErr
		}
		return SessionIssue{}, err
	}
	credential, err := service.passwordHasher.Hash(ctx, input.Password)
	if err != nil {
		return SessionIssue{}, mapHashError(err)
	}
	userIDText, err := service.newIdentity("usr_")
	if err != nil {
		return SessionIssue{}, err
	}
	user := User{
		ID: UserID(userIDText), LoginName: input.LoginName, CanonicalLoginName: canonical,
		DisplayName: input.DisplayName, Role: RoleAdministrator, Enabled: true,
		AuthRevision: 1, CreatedAt: now, UpdatedAt: now, Version: 1,
	}
	issue, session, err := service.newSession(user, now, nil)
	if err != nil {
		return SessionIssue{}, err
	}
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorSystem}, "auth.setup", "auth.account",
		string(user.ID), domain.AuditSuccess, input.RequestID, "auth.setup_succeeded", now,
	)
	if err != nil {
		return SessionIssue{}, err
	}
	commit := InitialSetupCommit{
		Preauth: preauth, ClearThrottle: setupKey, User: user, Credential: credential,
		Session: session, Audit: audit,
	}
	apply := func(callbackContext context.Context) error {
		return service.repository.CompleteInitialSetup(callbackContext, commit)
	}
	if service.bootstrapRequired && service.bootstrapGate == nil {
		return SessionIssue{}, ErrAuthUnavailable
	}
	if !service.bootstrapRequired {
		err = apply(ctx)
	} else {
		err = service.bootstrapGate.Use(ctx, input.BootstrapProof, apply)
	}
	if err == nil {
		return issue, nil
	}
	if errors.Is(err, ErrBootstrapDenied) {
		if recordErr := service.recordSetupFailure(ctx, preauth, setupKey, input.RequestID, now); recordErr != nil {
			return SessionIssue{}, recordErr
		}
		return SessionIssue{}, ErrAuthorizationDenied
	}
	return SessionIssue{}, mapSetupError(err)
}

func (service *Service) Login(ctx context.Context, input LoginInput) (SessionIssue, error) {
	if err := validateServiceContext(ctx); err != nil || !service.valid() {
		return SessionIssue{}, ErrAuthUnavailable
	}
	now, err := service.currentTime()
	if err != nil {
		return SessionIssue{}, err
	}
	preauth, err := service.parsePreauth(input.PreauthCookie, input.PreauthProof, PreauthPurposeLogin, now)
	if err != nil {
		return SessionIssue{}, err
	}
	if err := service.repository.ConsumePreauth(ctx, preauth); err != nil {
		return SessionIssue{}, mapPreauthError(err)
	}
	canonical, canonicalErr := CanonicalLoginName(input.LoginName)
	if !validSourceKey(input.SourceKey) || input.RequestID.Validate() != nil || !validPassword(input.Password) || canonicalErr != nil {
		return SessionIssue{}, ErrInvalidServiceInput
	}
	sourceKey := service.throttleKey(ThrottleScopeLoginSource, "source", input.SourceKey)
	accountKey := service.throttleKey(ThrottleScopeLoginAccount, "account", canonical)
	if err := service.requireThrottleOpen(ctx, sourceKey, now); err != nil {
		return SessionIssue{}, err
	}
	if err := service.requireThrottleOpen(ctx, accountKey, now); err != nil {
		return SessionIssue{}, err
	}
	account, lookupErr := service.repository.FindLoginAccount(ctx, canonical)
	credential := service.dummyCredential
	eligible := false
	if lookupErr == nil {
		if account.User.Validate() != nil || account.Credential.Validate() != nil {
			return SessionIssue{}, ErrAuthUnavailable
		}
		eligible = account.User.Enabled && account.User.DeletedAt == nil
		if eligible {
			credential = account.Credential
		}
	} else if !errors.Is(lookupErr, ErrAuthRepositoryNotFound) {
		return SessionIssue{}, mapRepositoryError(lookupErr)
	}
	verification, verifyErr := service.passwordHasher.Verify(ctx, input.Password, credential)
	if verifyErr != nil {
		return SessionIssue{}, mapHashError(verifyErr)
	}
	if !eligible || !verification.Match {
		if err := service.recordLoginFailure(ctx, sourceKey, accountKey, input.RequestID, now); err != nil {
			return SessionIssue{}, err
		}
		return SessionIssue{}, ErrLoginRejected
	}

	var rehashed *PasswordCredential
	if verification.NeedsRehash {
		updated, err := service.passwordHasher.Hash(ctx, input.Password)
		if err != nil {
			return SessionIssue{}, mapHashError(err)
		}
		rehashed = &updated
	}
	issue, session, err := service.newSession(account.User, now, nil)
	if err != nil {
		return SessionIssue{}, err
	}
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorUser, ID: string(account.User.ID)}, "auth.login",
		"auth.account", string(account.User.ID), domain.AuditSuccess, input.RequestID,
		"auth.login_succeeded", now,
	)
	if err != nil {
		return SessionIssue{}, err
	}
	commit := LoginSuccessCommit{
		Session: session, RehashedCredential: rehashed,
		ClearThrottles: []ThrottleKey{sourceKey, accountKey}, Audit: audit,
	}
	if err := service.repository.CommitLoginSuccess(ctx, commit); err != nil {
		return SessionIssue{}, mapRepositoryError(err)
	}
	return issue, nil
}

func (service *Service) Authenticate(ctx context.Context, encodedSessionToken string) (Authentication, error) {
	if err := validateServiceContext(ctx); err != nil || !service.valid() {
		return Authentication{}, ErrAuthUnavailable
	}
	token, err := ParseOpaqueToken(encodedSessionToken)
	if err != nil {
		return Authentication{}, ErrUnauthenticated
	}
	digest, err := DigestSessionToken(service.digestKey[:], token)
	if err != nil {
		return Authentication{}, ErrAuthUnavailable
	}
	resolved, err := service.repository.ResolveSession(ctx, digest, service.digestKeyID)
	if err != nil {
		if errors.Is(err, ErrAuthRepositoryNotFound) || errors.Is(err, ErrAuthRepositoryRejected) {
			return Authentication{}, ErrUnauthenticated
		}
		return Authentication{}, mapRepositoryError(err)
	}
	now, err := service.currentTime()
	if err != nil {
		return Authentication{}, err
	}
	if !validResolvedSession(resolved, digest, service.digestKeyID, now) {
		return Authentication{}, ErrUnauthenticated
	}
	if now.Sub(resolved.Session.LastSeenAt) >= SessionTouchInterval {
		idleExpiresAt := now.Add(SessionIdleLifetime)
		if idleExpiresAt.After(resolved.Session.AbsoluteExpiresAt) {
			idleExpiresAt = resolved.Session.AbsoluteExpiresAt
		}
		touch := SessionTouch{
			SessionID: resolved.Session.ID, UserID: resolved.User.ID,
			AuthRevision: resolved.User.AuthRevision, LastSeenAt: now, IdleExpiresAt: idleExpiresAt,
		}
		if err := service.repository.TouchSession(ctx, touch); err != nil {
			if errors.Is(err, ErrAuthRepositoryRejected) || errors.Is(err, ErrAuthRepositoryNotFound) {
				return Authentication{}, ErrUnauthenticated
			}
			return Authentication{}, mapRepositoryError(err)
		}
		resolved.Session.LastSeenAt = now
		resolved.Session.IdleExpiresAt = idleExpiresAt
	}
	csrfProof, err := DeriveCSRFProof(service.digestKey[:], token)
	if err != nil {
		return Authentication{}, ErrAuthUnavailable
	}
	principal := Principal{
		UserID: resolved.User.ID, SessionID: resolved.Session.ID,
		Role: resolved.User.Role, AuthRevision: resolved.User.AuthRevision,
	}
	contextValue := SessionContext{
		User: resolved.User, CSRFProof: csrfProof,
		IdleExpiresAt:     resolved.Session.IdleExpiresAt,
		AbsoluteExpiresAt: resolved.Session.AbsoluteExpiresAt,
	}
	return Authentication{
		Principal: principal, User: resolved.User, Session: resolved.Session,
		Context: contextValue, token: token,
	}, nil
}

func (service *Service) Logout(ctx context.Context, input LogoutInput) error {
	if input.RequestID.Validate() != nil {
		return ErrInvalidServiceInput
	}
	authentication, err := service.authenticateUnsafe(ctx, input.SessionToken, input.CSRFProof)
	if err != nil {
		return err
	}
	now, err := service.currentTime()
	if err != nil {
		return err
	}
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorUser, ID: string(authentication.User.ID)},
		"auth.logout", "auth.account", string(authentication.User.ID), domain.AuditSuccess,
		input.RequestID, "auth.logout_succeeded", now,
	)
	if err != nil {
		return err
	}
	revocation := SessionRevocation{
		Actor: authentication.Principal, SessionID: authentication.Session.ID,
		RevokedAt: now, Reason: "user_logout", Audit: audit,
	}
	if err := service.repository.RevokeSession(ctx, revocation); err != nil {
		if errors.Is(err, ErrAuthRepositoryNotFound) || errors.Is(err, ErrAuthRepositoryRejected) {
			return ErrUnauthenticated
		}
		return mapRepositoryError(err)
	}
	return nil
}

func (service *Service) CreateRecentGrant(ctx context.Context, input RecentAuthInput) (RecentGrantIssue, error) {
	if input.Purpose.Validate() != nil || input.RequestID.Validate() != nil || !validPassword(input.CurrentPassword) {
		return RecentGrantIssue{}, ErrInvalidServiceInput
	}
	authentication, err := service.authenticateUnsafe(ctx, input.SessionToken, input.CSRFProof)
	if err != nil {
		return RecentGrantIssue{}, err
	}
	now, err := service.currentTime()
	if err != nil {
		return RecentGrantIssue{}, err
	}
	recentKey := service.throttleKey(ThrottleScopeRecentUser, "user", string(authentication.User.ID))
	if err := service.requireThrottleOpen(ctx, recentKey, now); err != nil {
		return RecentGrantIssue{}, err
	}
	if input.Purpose != RecentAuthPurposeAccountPasswordChange && authentication.Principal.Role != RoleAdministrator {
		if err := service.recordRecentFailure(ctx, authentication.Principal, nil, input.RequestID, now); err != nil {
			return RecentGrantIssue{}, err
		}
		return RecentGrantIssue{}, ErrAuthorizationDenied
	}
	account, err := service.repository.FindLoginAccount(ctx, authentication.User.CanonicalLoginName)
	if err != nil {
		if !errors.Is(err, ErrAuthRepositoryNotFound) {
			return RecentGrantIssue{}, mapRepositoryError(err)
		}
		if err := service.recordRecentFailure(ctx, authentication.Principal, &recentKey, input.RequestID, now); err != nil {
			return RecentGrantIssue{}, err
		}
		return RecentGrantIssue{}, ErrRecentAuthRejected
	}
	if account.User.Validate() != nil || account.Credential.Validate() != nil ||
		account.User.ID != authentication.User.ID || account.User.AuthRevision != authentication.User.AuthRevision ||
		!account.User.Enabled || account.User.DeletedAt != nil {
		return RecentGrantIssue{}, ErrAuthUnavailable
	}
	verification, err := service.passwordHasher.Verify(ctx, input.CurrentPassword, account.Credential)
	if err != nil {
		return RecentGrantIssue{}, mapHashError(err)
	}
	if !verification.Match {
		if err := service.recordRecentFailure(ctx, authentication.Principal, &recentKey, input.RequestID, now); err != nil {
			return RecentGrantIssue{}, err
		}
		return RecentGrantIssue{}, ErrRecentAuthRejected
	}
	proof, err := generateOpaqueToken(service.random)
	if err != nil {
		return RecentGrantIssue{}, ErrAuthUnavailable
	}
	digest, err := DigestRecentAuthProof(service.digestKey[:], proof)
	if err != nil {
		return RecentGrantIssue{}, ErrAuthUnavailable
	}
	grantID, err := service.newIdentity("grant_")
	if err != nil {
		return RecentGrantIssue{}, err
	}
	expiresAt := now.Add(RecentGrantLifetime)
	grant := RecentGrant{
		ID: grantID, UserID: authentication.User.ID, SessionID: authentication.Session.ID,
		Purpose: input.Purpose, Digest: digest, DigestKeyID: service.digestKeyID,
		CreatedAt: now, ExpiresAt: expiresAt,
	}
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorUser, ID: string(authentication.User.ID)},
		"auth.reauthenticate", "auth.account", string(authentication.User.ID), domain.AuditSuccess,
		input.RequestID, "auth.reauthentication_succeeded", now,
	)
	if err != nil {
		return RecentGrantIssue{}, err
	}
	commit := RecentGrantCommit{
		Actor: authentication.Principal, Grant: grant, ClearThrottle: recentKey, Audit: audit,
	}
	if err := service.repository.CommitRecentGrant(ctx, commit); err != nil {
		return RecentGrantIssue{}, mapRepositoryError(err)
	}
	return RecentGrantIssue{Proof: proof, Purpose: input.Purpose, ExpiresAt: expiresAt}, nil
}

func (service *Service) AuthorizeRecent(ctx context.Context, input RecentAuthorizationInput) (Authentication, error) {
	if input.Purpose.Validate() != nil || input.RequestID.Validate() != nil {
		return Authentication{}, ErrInvalidServiceInput
	}
	authentication, err := service.authenticateAdministrator(ctx, input.SessionToken, input.CSRFProof)
	if err != nil {
		return Authentication{}, err
	}
	now, err := service.currentTime()
	if err != nil {
		return Authentication{}, err
	}
	grant, err := service.recentGrantUse(authentication.Principal, input.Purpose, input.RecentProof, now)
	if err != nil {
		return Authentication{}, err
	}
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorUser, ID: string(authentication.User.ID)},
		"auth.recent_grant_use", "auth.account", string(authentication.User.ID), domain.AuditSuccess,
		input.RequestID, "auth.recent_grant_consumed", now,
	)
	if err != nil {
		return Authentication{}, err
	}
	commit := RecentAuthorizationCommit{Actor: authentication.Principal, Grant: grant, Audit: audit}
	repository, ok := service.repository.(RecentAuthorizationRepository)
	if !ok {
		return Authentication{}, ErrAuthUnavailable
	}
	if err := repository.CommitRecentAuthorization(ctx, commit); err != nil {
		return Authentication{}, mapMutationError(err)
	}
	return authentication, nil
}

func (service *Service) ChangeOwnPassword(ctx context.Context, input OwnPasswordChangeInput) (SessionIssue, error) {
	if input.RequestID.Validate() != nil || !validPassword(input.NewPassword) {
		return SessionIssue{}, ErrInvalidServiceInput
	}
	authentication, err := service.authenticateUnsafe(ctx, input.SessionToken, input.CSRFProof)
	if err != nil {
		return SessionIssue{}, err
	}
	now, err := service.currentTime()
	if err != nil {
		return SessionIssue{}, err
	}
	grant, err := service.recentGrantUse(authentication.Principal, RecentAuthPurposeAccountPasswordChange, input.RecentProof, now)
	if err != nil {
		return SessionIssue{}, err
	}
	credential, err := service.passwordHasher.Hash(ctx, input.NewPassword)
	if err != nil {
		return SessionIssue{}, mapHashError(err)
	}
	updatedUser := authentication.User
	updatedUser.AuthRevision++
	updatedUser.Version++
	updatedUser.UpdatedAt = now
	rotatedFrom := authentication.Session.ID
	issue, rotatedSession, err := service.newSession(updatedUser, now, &rotatedFrom)
	if err != nil {
		return SessionIssue{}, err
	}
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorUser, ID: string(authentication.User.ID)},
		"auth.password-change", "auth.account", string(authentication.User.ID), domain.AuditSuccess,
		input.RequestID, "auth.password_change_succeeded", now,
	)
	if err != nil {
		return SessionIssue{}, err
	}
	commit := OwnPasswordChangeCommit{
		Actor: authentication.Principal, Grant: grant, Credential: credential,
		RotatedSession: rotatedSession, ChangedAt: now, Audit: audit,
	}
	if err := service.repository.CommitOwnPasswordChange(ctx, commit); err != nil {
		return SessionIssue{}, mapMutationError(err)
	}
	return issue, nil
}

func (service *Service) ListUsers(ctx context.Context, input AuthenticatedReadInput, query UserListQuery) (UserPage, error) {
	if !validUserListQuery(query) {
		return UserPage{}, ErrInvalidServiceInput
	}
	authentication, err := service.Authenticate(ctx, input.SessionToken)
	if err != nil {
		return UserPage{}, err
	}
	if authentication.Principal.Role != RoleAdministrator {
		return UserPage{}, ErrAuthorizationDenied
	}
	page, err := service.repository.ListUsers(ctx, query)
	if err != nil {
		if errors.Is(err, ErrAuthRepositoryRejected) {
			return UserPage{}, ErrInvalidServiceInput
		}
		return UserPage{}, mapRepositoryError(err)
	}
	for _, user := range page.Items {
		if user.Validate() != nil {
			return UserPage{}, ErrAuthUnavailable
		}
	}
	return page, nil
}

func (service *Service) GetUser(ctx context.Context, input AuthenticatedReadInput, userID UserID) (User, error) {
	if userID.Validate() != nil {
		return User{}, ErrInvalidServiceInput
	}
	authentication, err := service.Authenticate(ctx, input.SessionToken)
	if err != nil {
		return User{}, err
	}
	if authentication.Principal.Role != RoleAdministrator {
		return User{}, ErrAuthorizationDenied
	}
	user, err := service.repository.GetUser(ctx, userID)
	if err != nil {
		return User{}, mapMutationError(err)
	}
	if user.Validate() != nil {
		return User{}, ErrAuthUnavailable
	}
	return user, nil
}

func (service *Service) CreateUser(ctx context.Context, input UserCreateInput) (User, error) {
	canonical, err := CanonicalLoginName(input.LoginName)
	if err != nil || !validDisplayName(input.DisplayName) || input.Role.Validate() != nil || !validPassword(input.Password) || input.RequestID.Validate() != nil {
		return User{}, ErrInvalidServiceInput
	}
	authentication, err := service.authenticateAdministrator(ctx, input.SessionToken, input.CSRFProof)
	if err != nil {
		return User{}, err
	}
	now, err := service.currentTime()
	if err != nil {
		return User{}, err
	}
	grant, err := service.recentGrantUse(authentication.Principal, RecentAuthPurposeUserCreate, input.RecentProof, now)
	if err != nil {
		return User{}, err
	}
	credential, err := service.passwordHasher.Hash(ctx, input.Password)
	if err != nil {
		return User{}, mapHashError(err)
	}
	id, err := service.newIdentity("usr_")
	if err != nil {
		return User{}, err
	}
	user := User{
		ID: UserID(id), LoginName: input.LoginName, CanonicalLoginName: canonical,
		DisplayName: input.DisplayName, Role: input.Role, Enabled: true,
		AuthRevision: 1, CreatedAt: now, UpdatedAt: now, Version: 1,
	}
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorUser, ID: string(authentication.User.ID)},
		"user.create", "user.account", string(user.ID), domain.AuditSuccess,
		input.RequestID, "user.create_succeeded", now,
	)
	if err != nil {
		return User{}, err
	}
	commit := UserCreateCommit{Actor: authentication.Principal, Grant: grant, User: user, Credential: credential, Audit: audit}
	if err := service.repository.CommitUserCreate(ctx, commit); err != nil {
		return User{}, mapMutationError(err)
	}
	return user, nil
}

func (service *Service) UpdateUser(ctx context.Context, input UserUpdateInput) (User, error) {
	if input.UserID.Validate() != nil || input.ExpectedVersion < 1 || input.RequestID.Validate() != nil ||
		input.LoginName == nil && input.DisplayName == nil && input.Role == nil && input.Enabled == nil {
		return User{}, ErrInvalidServiceInput
	}
	authentication, err := service.authenticateAdministrator(ctx, input.SessionToken, input.CSRFProof)
	if err != nil {
		return User{}, err
	}
	now, err := service.currentTime()
	if err != nil {
		return User{}, err
	}
	grant, err := service.recentGrantUse(authentication.Principal, RecentAuthPurposeUserUpdate, input.RecentProof, now)
	if err != nil {
		return User{}, err
	}
	stored, err := service.repository.GetUser(ctx, input.UserID)
	if err != nil {
		return User{}, mapMutationError(err)
	}
	if stored.Validate() != nil || stored.DeletedAt != nil {
		return User{}, ErrAuthNotFound
	}
	updated := stored
	changed := false
	securityChanged := false
	if input.LoginName != nil {
		canonical, err := CanonicalLoginName(*input.LoginName)
		if err != nil {
			return User{}, ErrInvalidServiceInput
		}
		changed = changed || updated.LoginName != *input.LoginName
		securityChanged = updated.LoginName != *input.LoginName
		updated.LoginName = *input.LoginName
		updated.CanonicalLoginName = canonical
	}
	if input.DisplayName != nil {
		if !validDisplayName(*input.DisplayName) {
			return User{}, ErrInvalidServiceInput
		}
		changed = changed || updated.DisplayName != *input.DisplayName
		updated.DisplayName = *input.DisplayName
	}
	if input.Role != nil {
		if input.Role.Validate() != nil {
			return User{}, ErrInvalidServiceInput
		}
		changed = changed || updated.Role != *input.Role
		securityChanged = securityChanged || updated.Role != *input.Role
		updated.Role = *input.Role
	}
	if input.Enabled != nil {
		changed = changed || updated.Enabled != *input.Enabled
		securityChanged = securityChanged || updated.Enabled != *input.Enabled
		updated.Enabled = *input.Enabled
		if updated.Enabled {
			updated.DisabledAt = nil
		} else if stored.Enabled {
			disabledAt := now
			updated.DisabledAt = &disabledAt
		}
	}
	if !changed {
		return User{}, ErrInvalidServiceInput
	}
	if securityChanged {
		updated.AuthRevision++
	}
	updated.UpdatedAt = now
	updated.Version++
	if updated.Validate() != nil {
		return User{}, ErrInvalidServiceInput
	}
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorUser, ID: string(authentication.User.ID)},
		"user.update", "user.account", string(updated.ID), domain.AuditSuccess,
		input.RequestID, "user.update_succeeded", now,
	)
	if err != nil {
		return User{}, err
	}
	commit := UserUpdateCommit{
		Actor: authentication.Principal, Grant: grant, User: updated,
		ExpectedVersion: input.ExpectedVersion, Audit: audit,
	}
	if err := service.repository.CommitUserUpdate(ctx, commit); err != nil {
		return User{}, mapMutationError(err)
	}
	return updated, nil
}

func (service *Service) DeleteUser(ctx context.Context, input UserDeleteInput) error {
	if input.UserID.Validate() != nil || input.ExpectedVersion < 1 || input.RequestID.Validate() != nil {
		return ErrInvalidServiceInput
	}
	authentication, err := service.authenticateAdministrator(ctx, input.SessionToken, input.CSRFProof)
	if err != nil {
		return err
	}
	now, err := service.currentTime()
	if err != nil {
		return err
	}
	grant, err := service.recentGrantUse(authentication.Principal, RecentAuthPurposeUserDelete, input.RecentProof, now)
	if err != nil {
		return err
	}
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorUser, ID: string(authentication.User.ID)},
		"user.delete", "user.account", string(input.UserID), domain.AuditSuccess,
		input.RequestID, "user.delete_succeeded", now,
	)
	if err != nil {
		return err
	}
	commit := UserDeleteCommit{
		Actor: authentication.Principal, Grant: grant, UserID: input.UserID,
		ExpectedVersion: input.ExpectedVersion, DeletedAt: now, Audit: audit,
	}
	if err := service.repository.CommitUserDelete(ctx, commit); err != nil {
		return mapMutationError(err)
	}
	return nil
}

func (service *Service) ResetUserPassword(ctx context.Context, input UserPasswordResetInput) error {
	if input.UserID.Validate() != nil || input.ExpectedVersion < 1 || !validPassword(input.NewPassword) || input.RequestID.Validate() != nil {
		return ErrInvalidServiceInput
	}
	authentication, err := service.authenticateAdministrator(ctx, input.SessionToken, input.CSRFProof)
	if err != nil {
		return err
	}
	now, err := service.currentTime()
	if err != nil {
		return err
	}
	grant, err := service.recentGrantUse(authentication.Principal, RecentAuthPurposeUserPasswordReset, input.RecentProof, now)
	if err != nil {
		return err
	}
	credential, err := service.passwordHasher.Hash(ctx, input.NewPassword)
	if err != nil {
		return mapHashError(err)
	}
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorUser, ID: string(authentication.User.ID)},
		"user.password-reset", "user.account", string(input.UserID), domain.AuditSuccess,
		input.RequestID, "user.password_reset_succeeded", now,
	)
	if err != nil {
		return err
	}
	commit := UserPasswordResetCommit{
		Actor: authentication.Principal, Grant: grant, UserID: input.UserID,
		ExpectedVersion: input.ExpectedVersion, Credential: credential, ChangedAt: now, Audit: audit,
	}
	if err := service.repository.CommitUserPasswordReset(ctx, commit); err != nil {
		return mapMutationError(err)
	}
	return nil
}

func (service *Service) RevokeUserSessions(ctx context.Context, input UserSessionsRevokeInput) error {
	if input.UserID.Validate() != nil || input.RequestID.Validate() != nil {
		return ErrInvalidServiceInput
	}
	authentication, err := service.authenticateAdministrator(ctx, input.SessionToken, input.CSRFProof)
	if err != nil {
		return err
	}
	now, err := service.currentTime()
	if err != nil {
		return err
	}
	grant, err := service.recentGrantUse(authentication.Principal, RecentAuthPurposeUserSessionsRevoke, input.RecentProof, now)
	if err != nil {
		return err
	}
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorUser, ID: string(authentication.User.ID)},
		"user.sessions-revoke", "user.account", string(input.UserID), domain.AuditSuccess,
		input.RequestID, "user.sessions_revoke_succeeded", now,
	)
	if err != nil {
		return err
	}
	commit := UserSessionsRevokeCommit{
		Actor: authentication.Principal, Grant: grant, UserID: input.UserID,
		RevokedAt: now, Audit: audit,
	}
	if err := service.repository.CommitUserSessionsRevoke(ctx, commit); err != nil {
		return mapMutationError(err)
	}
	return nil
}

func (service *Service) RecoverAdministrator(ctx context.Context, input OperatorRecoveryInput) (User, error) {
	if validateServiceContext(ctx) != nil || !service.valid() || input.RequestID.Validate() != nil ||
		input.OperatorID != "" && validateIdentity(input.OperatorID) != nil || !validDisplayName(input.DisplayName) || !validPassword(input.Password) {
		return User{}, ErrInvalidServiceInput
	}
	canonical, err := CanonicalLoginName(input.LoginName)
	if err != nil {
		return User{}, ErrInvalidServiceInput
	}
	now, err := service.currentTime()
	if err != nil {
		return User{}, err
	}
	credential, err := service.passwordHasher.Hash(ctx, input.Password)
	if err != nil {
		return User{}, mapHashError(err)
	}
	requested := User{
		LoginName: input.LoginName, CanonicalLoginName: canonical, DisplayName: input.DisplayName,
		Role: RoleAdministrator, Enabled: true, AuthRevision: 1,
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}
	if existing, findErr := service.repository.FindLoginAccount(ctx, canonical); findErr == nil {
		if existing.User.Validate() != nil || existing.User.DeletedAt != nil {
			return User{}, ErrAuthConflict
		}
		requested = existing.User
		requested.LoginName = input.LoginName
		requested.CanonicalLoginName = canonical
		requested.DisplayName = input.DisplayName
		requested.Role = RoleAdministrator
		requested.Enabled = true
		requested.DisabledAt = nil
		requested.AuthRevision++
		requested.Version++
		requested.UpdatedAt = now
	} else if !errors.Is(findErr, ErrAuthRepositoryNotFound) {
		return User{}, mapRepositoryError(findErr)
	} else {
		id, err := service.newIdentity("usr_")
		if err != nil {
			return User{}, err
		}
		requested.ID = UserID(id)
	}
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorOperator, ID: input.OperatorID},
		"user.operator-recovery", "user.account", string(requested.ID), domain.AuditSuccess,
		input.RequestID, "user.operator_recovery_succeeded", now,
	)
	if err != nil {
		return User{}, err
	}
	commit := OperatorRecoveryCommit{
		OperatorID: input.OperatorID, RequestedUser: requested, Credential: credential,
		RecoveredAt: now, Audit: audit,
	}
	recovered, err := service.repository.CommitOperatorRecovery(ctx, commit)
	if err != nil {
		return User{}, mapMutationError(err)
	}
	if recovered.Validate() != nil || recovered.Role != RoleAdministrator || !recovered.Enabled {
		return User{}, ErrAuthUnavailable
	}
	return recovered, nil
}

func (service *Service) authenticateAdministrator(ctx context.Context, sessionToken string, csrfProof string) (Authentication, error) {
	authentication, err := service.authenticateUnsafe(ctx, sessionToken, csrfProof)
	if err != nil {
		return Authentication{}, err
	}
	if authentication.Principal.Role != RoleAdministrator {
		return Authentication{}, ErrAuthorizationDenied
	}
	return authentication, nil
}

func (service *Service) recentGrantUse(actor Principal, purpose RecentAuthPurpose, encodedProof string, consumedAt time.Time) (RecentGrantUse, error) {
	if actor.Validate() != nil || purpose.Validate() != nil || !validStoredTime(consumedAt) {
		return RecentGrantUse{}, ErrInvalidServiceInput
	}
	proof, err := ParseOpaqueToken(encodedProof)
	if err != nil {
		return RecentGrantUse{}, ErrRecentAuthRejected
	}
	digest, err := DigestRecentAuthProof(service.digestKey[:], proof)
	if err != nil {
		return RecentGrantUse{}, ErrAuthUnavailable
	}
	return RecentGrantUse{
		Actor: actor, Purpose: purpose, Digest: digest,
		DigestKeyID: service.digestKeyID, ConsumedAt: consumedAt,
	}, nil
}

func (service *Service) consumeRejectedPreauth(ctx context.Context, preauth PreauthUse) error {
	if err := service.repository.ConsumePreauth(ctx, preauth); err != nil {
		return mapPreauthError(err)
	}
	return nil
}

func validUserListQuery(query UserListQuery) bool {
	if query.Limit < 1 || query.Limit > 200 || len(query.Cursor) > 1024 {
		return false
	}
	if query.Search != "" && (utf8.RuneCountInString(query.Search) > 200 || !validDisplayName(query.Search)) {
		return false
	}
	if query.Role != nil && query.Role.Validate() != nil {
		return false
	}
	return true
}

func (service *Service) authenticateUnsafe(ctx context.Context, sessionToken string, csrfProof string) (Authentication, error) {
	authentication, err := service.Authenticate(ctx, sessionToken)
	if err != nil {
		return Authentication{}, err
	}
	proof, err := ParseOpaqueToken(csrfProof)
	if err != nil {
		return Authentication{}, ErrCSRFRejected
	}
	digest, err := DigestCSRFProof(service.digestKey[:], proof)
	if err != nil || !digest.Equal(authentication.Session.CSRFDigest) {
		return Authentication{}, ErrCSRFRejected
	}
	return authentication, nil
}

func (service *Service) AuthorizeUnsafe(ctx context.Context, sessionToken string, csrfProof string) (Authentication, error) {
	return service.authenticateUnsafe(ctx, sessionToken, csrfProof)
}

func (service *Service) parsePreauth(cookieEncoded string, proofEncoded string, purpose PreauthPurpose, consumedAt time.Time) (PreauthUse, error) {
	cookie, err := ParseOpaqueToken(cookieEncoded)
	if err != nil {
		return PreauthUse{}, ErrAuthorizationDenied
	}
	proof, err := ParseOpaqueToken(proofEncoded)
	if err != nil {
		return PreauthUse{}, ErrAuthorizationDenied
	}
	cookieDigest, err := DigestPreauthCookie(service.digestKey[:], cookie)
	if err != nil {
		return PreauthUse{}, ErrAuthUnavailable
	}
	proofDigest, err := DigestPreauthProof(service.digestKey[:], proof)
	if err != nil {
		return PreauthUse{}, ErrAuthUnavailable
	}
	return PreauthUse{
		Purpose: purpose, CookieDigest: cookieDigest, ProofDigest: proofDigest,
		DigestKeyID: service.digestKeyID, ConsumedAt: consumedAt,
	}, nil
}

func (service *Service) recordSetupFailure(ctx context.Context, preauth PreauthUse, key ThrottleKey, requestID domain.RequestID, now time.Time) error {
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorSystem}, "auth.setup", "auth.account", "",
		domain.AuditDenied, requestID, "auth.setup_denied", now,
	)
	if err != nil {
		return err
	}
	commit := SetupFailureCommit{
		Preauth: preauth,
		Throttle: ThrottleFailure{
			Key: key, OccurredAt: now, Window: setupSourceWindow,
			MaximumFailures: setupSourceFailures, BlockDuration: setupSourceBlock,
			Retention: throttleRetention,
		},
		Audit: audit,
	}
	if err := service.repository.CommitSetupFailure(ctx, commit); err != nil {
		return mapRepositoryError(err)
	}
	return nil
}

func (service *Service) recordLoginFailure(ctx context.Context, sourceKey ThrottleKey, accountKey ThrottleKey, requestID domain.RequestID, now time.Time) error {
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorSystem}, "auth.login", "auth.account", "",
		domain.AuditDenied, requestID, "auth.login_denied", now,
	)
	if err != nil {
		return err
	}
	commit := LoginFailureCommit{
		Throttles: []ThrottleFailure{
			{Key: sourceKey, OccurredAt: now, Window: loginSourceWindow, MaximumFailures: loginSourceFailures, BlockDuration: loginSourceBlock, Retention: throttleRetention},
			{Key: accountKey, OccurredAt: now, Window: loginAccountWindow, MaximumFailures: loginAccountFailures, BlockDuration: loginAccountBlock, Retention: throttleRetention},
		},
		Audit: audit,
	}
	if err := service.repository.CommitLoginFailure(ctx, commit); err != nil {
		return mapRepositoryError(err)
	}
	return nil
}

func (service *Service) recordRecentFailure(ctx context.Context, actor Principal, key *ThrottleKey, requestID domain.RequestID, now time.Time) error {
	audit, err := service.newAudit(
		domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor.UserID)},
		"auth.reauthenticate", "auth.account", string(actor.UserID), domain.AuditDenied,
		requestID, "auth.reauthentication_denied", now,
	)
	if err != nil {
		return err
	}
	commit := RecentAuthFailureCommit{Actor: actor, Audit: audit}
	if key != nil {
		commit.Throttle = &ThrottleFailure{
			Key: *key, OccurredAt: now, Window: recentUserWindow,
			MaximumFailures: recentUserFailures, BlockDuration: recentUserBlock,
			Retention: throttleRetention,
		}
	}
	if err := service.repository.CommitRecentAuthFailure(ctx, commit); err != nil {
		return mapRepositoryError(err)
	}
	return nil
}

func (service *Service) newSession(user User, now time.Time, rotatedFrom *SessionID) (SessionIssue, Session, error) {
	token, err := generateOpaqueToken(service.random)
	if err != nil {
		return SessionIssue{}, Session{}, ErrAuthUnavailable
	}
	tokenDigest, err := DigestSessionToken(service.digestKey[:], token)
	if err != nil {
		return SessionIssue{}, Session{}, ErrAuthUnavailable
	}
	csrfProof, err := DeriveCSRFProof(service.digestKey[:], token)
	if err != nil {
		return SessionIssue{}, Session{}, ErrAuthUnavailable
	}
	csrfDigest, err := DigestCSRFProof(service.digestKey[:], csrfProof)
	if err != nil {
		return SessionIssue{}, Session{}, ErrAuthUnavailable
	}
	sessionIDText, err := service.newIdentity("sess_")
	if err != nil {
		return SessionIssue{}, Session{}, err
	}
	idleExpiresAt := now.Add(SessionIdleLifetime)
	absoluteExpiresAt := now.Add(SessionAbsoluteLife)
	session := Session{
		ID: SessionID(sessionIDText), UserID: user.ID,
		TokenDigest: tokenDigest, TokenDigestKeyID: service.digestKeyID,
		CSRFDigest: csrfDigest, CSRFDigestKeyID: service.digestKeyID,
		UserAuthRevision: user.AuthRevision, CreatedAt: now, LastSeenAt: now,
		IdleExpiresAt: idleExpiresAt, AbsoluteExpiresAt: absoluteExpiresAt,
		RotatedFromSessionID: rotatedFrom,
	}
	if err := session.Validate(); err != nil {
		return SessionIssue{}, Session{}, ErrAuthUnavailable
	}
	issue := SessionIssue{
		Token: token,
		Context: SessionContext{
			User: user, CSRFProof: csrfProof, IdleExpiresAt: idleExpiresAt,
			AbsoluteExpiresAt: absoluteExpiresAt,
		},
	}
	return issue, session, nil
}

func (service *Service) requireThrottleOpen(ctx context.Context, key ThrottleKey, now time.Time) error {
	state, err := service.repository.CheckThrottle(ctx, key, now)
	if err != nil {
		return mapRepositoryError(err)
	}
	if !state.BlockedUntil.IsZero() {
		until := state.BlockedUntil.UTC().Truncate(time.Microsecond)
		if !validStoredTime(until) {
			return ErrAuthUnavailable
		}
		if now.Before(until) {
			return &throttledError{until: until}
		}
	}
	return nil
}

func (service *Service) throttleKey(scope ThrottleScope, label string, value string) ThrottleKey {
	digest := hmac.New(sha256.New, service.digestKey[:])
	_, _ = digest.Write([]byte("stackplore/auth-throttle/v1"))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(label))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(value))
	var sum [DigestBytes]byte
	copy(sum[:], digest.Sum(nil))
	return ThrottleKey{Scope: scope, Digest: sum}
}

func (service *Service) newAudit(actor domain.AuditActor, action string, targetKind string, targetID string, outcome domain.AuditOutcome, requestID domain.RequestID, reason string, occurredAt time.Time) (domain.AuditEntry, error) {
	id, err := service.newIdentity("audit_")
	if err != nil {
		return domain.AuditEntry{}, err
	}
	entry := domain.AuditEntry{
		ID: domain.AuditEntryID(id), OccurredAt: occurredAt, Actor: actor,
		Action: domain.AuditAction(action), Target: domain.AuditTarget{Kind: domain.AuditTargetKind(targetKind), ID: targetID},
		Outcome: outcome, RequestID: requestID, ReasonCode: domain.ReasonCode(reason),
	}
	if err := entry.Validate(); err != nil {
		return domain.AuditEntry{}, ErrAuthUnavailable
	}
	return entry, nil
}

func (service *Service) newIdentity(prefix string) (string, error) {
	token, err := generateOpaqueToken(service.random)
	if err != nil {
		return "", ErrAuthUnavailable
	}
	encoded, err := token.Encode()
	if err != nil {
		return "", ErrAuthUnavailable
	}
	value := prefix + encoded
	if validateIdentity(value) != nil {
		return "", ErrAuthUnavailable
	}
	return value, nil
}

func (service *Service) currentTime() (time.Time, error) {
	if service == nil || service.now == nil {
		return time.Time{}, ErrAuthUnavailable
	}
	current := service.now().UTC().Truncate(time.Microsecond)
	if !validStoredTime(current) {
		return time.Time{}, ErrAuthUnavailable
	}
	return current, nil
}

func (service *Service) valid() bool {
	return service != nil && service.repository != nil && service.passwordHasher != nil &&
		service.random != nil && service.now != nil && validateIdentity(service.digestKeyID) == nil &&
		service.dummyCredential.Validate() == nil
}

func validResolvedSession(resolved ResolvedSession, requestedDigest SessionTokenDigest, keyID string, now time.Time) bool {
	if resolved.User.Validate() != nil || resolved.Session.Validate() != nil || !resolved.User.Enabled || resolved.User.DeletedAt != nil {
		return false
	}
	return resolved.Session.UserID == resolved.User.ID &&
		resolved.Session.TokenDigest.Equal(requestedDigest) &&
		resolved.Session.UserAuthRevision == resolved.User.AuthRevision &&
		resolved.Session.TokenDigestKeyID == keyID && resolved.Session.CSRFDigestKeyID == keyID &&
		resolved.Session.RevokedAt == nil && !now.Before(resolved.Session.CreatedAt) &&
		!now.Before(resolved.Session.LastSeenAt) && !now.Before(resolved.User.CreatedAt) &&
		!now.Before(resolved.User.UpdatedAt) && now.Before(resolved.Session.IdleExpiresAt) &&
		now.Before(resolved.Session.AbsoluteExpiresAt)
}

func validSourceKey(value string) bool {
	if value == "" || len(value) > 128 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

func validateServiceContext(ctx context.Context) error {
	if ctx == nil {
		return ErrAuthUnavailable
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(ErrAuthUnavailable, err)
	}
	return nil
}

func mapHashError(err error) error {
	if errors.Is(err, ErrInvalidPassword) {
		return ErrInvalidServiceInput
	}
	return ErrAuthUnavailable
}

func mapPreauthError(err error) error {
	if errors.Is(err, ErrAuthRepositoryNotFound) || errors.Is(err, ErrAuthRepositoryRejected) || errors.Is(err, ErrAuthRepositoryConflict) {
		return ErrAuthorizationDenied
	}
	return mapRepositoryError(err)
}

func mapSetupError(err error) error {
	if errors.Is(err, ErrAuthRepositoryConflict) || errors.Is(err, ErrAuthRepositoryRejected) {
		return ErrSetupClosed
	}
	if errors.Is(err, ErrBootstrapUnavailable) {
		return ErrAuthUnavailable
	}
	return mapRepositoryError(err)
}

func mapRepositoryError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrAuthRepositoryLastAdministrator):
		return ErrLastAdministrator
	case errors.Is(err, ErrAuthRepositoryStaleVersion):
		return ErrStaleVersion
	case errors.Is(err, ErrAuthRepositoryLoginNameConflict):
		return ErrLoginNameConflict
	case errors.Is(err, ErrAuthRepositoryConflict), errors.Is(err, ErrAuthRepositoryRejected):
		return ErrAuthConflict
	case errors.Is(err, ErrAuthRepositoryNotFound):
		return ErrAuthNotFound
	default:
		return ErrAuthUnavailable
	}
}

func mapMutationError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrAuthRepositoryLastAdministrator):
		return ErrLastAdministrator
	case errors.Is(err, ErrAuthRepositoryStaleVersion):
		return ErrStaleVersion
	case errors.Is(err, ErrAuthRepositoryLoginNameConflict):
		return ErrLoginNameConflict
	case errors.Is(err, ErrAuthRepositoryNotFound):
		return ErrAuthNotFound
	case errors.Is(err, ErrAuthRepositoryRejected):
		return ErrAuthorizationDenied
	case errors.Is(err, ErrAuthRepositoryConflict):
		return ErrAuthConflict
	default:
		return ErrAuthUnavailable
	}
}

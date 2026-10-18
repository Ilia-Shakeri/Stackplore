package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

var serviceTestTime = time.Date(2026, 8, 2, 1, 2, 3, 456000000, time.UTC)

func TestUserListSearchUsesContractCodePointAndDisplayTextRules(t *testing.T) {
	valid := UserListQuery{Limit: 1, Search: strings.Repeat("\u0634", 200)}
	if !validUserListQuery(valid) {
		t.Fatal("200-code-point search was rejected")
	}
	for name, search := range map[string]string{
		"too long":       strings.Repeat("\u0634", 201),
		"leading space":  " operator",
		"control":        "operator\x00",
		"format only":    "\u200c",
		"hidden format":  "operator\u2060",
		"line separator": "operator\u2028",
	} {
		t.Run(name, func(t *testing.T) {
			if validUserListQuery(UserListQuery{Limit: 1, Search: search}) {
				t.Fatalf("search %q was accepted", search)
			}
		})
	}
}

func TestServiceMapsRejectedUserCursorToInvalidInput(t *testing.T) {
	fixture := setupServiceFixture(t)
	administrator := fixture.login(t, "request_cursor_login")
	fixture.repository.mu.Lock()
	fixture.repository.listUsersError = ErrAuthRepositoryRejected
	fixture.repository.mu.Unlock()
	_, err := fixture.service.ListUsers(
		fixture.ctx,
		AuthenticatedReadInput{SessionToken: encodeTestToken(t, administrator.Token)},
		UserListQuery{Cursor: "dGFtcGVyZWQ", Limit: 25},
	)
	if !errors.Is(err, ErrInvalidServiceInput) || errors.Is(err, ErrAuthConflict) {
		t.Fatalf("ListUsers(tampered cursor) error = %v", err)
	}
}

func TestServiceCompletesOneAtomicSetupAndAuthenticates(t *testing.T) {
	fixture := newServiceFixture(t, false)
	status, err := fixture.service.SetupStatus(fixture.ctx)
	if err != nil || !status.Required || status.BootstrapProofRequired {
		t.Fatalf("SetupStatus() = %+v, %v", status, err)
	}
	preauth := fixture.issuePreauth(t, PreauthPurposeSetup)
	issue, err := fixture.service.Setup(fixture.ctx, SetupInput{
		PreauthCookie: encodeTestToken(t, preauth.Cookie), PreauthProof: encodeTestToken(t, preauth.Proof),
		SourceKey: "192.0.2.10", LoginName: "Operator", DisplayName: "Operations",
		Password: []byte("correct horse value"), RequestID: "request_setup",
	})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if issue.Context.User.Role != RoleAdministrator || !issue.Context.User.Enabled || issue.Context.User.CanonicalLoginName != "operator" {
		t.Fatalf("setup user = %+v", issue.Context.User)
	}
	if got := issue.Context.IdleExpiresAt.Sub(serviceTestTime); got != SessionIdleLifetime {
		t.Fatalf("idle lifetime = %v", got)
	}
	if got := issue.Context.AbsoluteExpiresAt.Sub(serviceTestTime); got != SessionAbsoluteLife {
		t.Fatalf("absolute lifetime = %v", got)
	}
	authenticated, err := fixture.service.Authenticate(fixture.ctx, encodeTestToken(t, issue.Token))
	if err != nil || authenticated.Principal.Role != RoleAdministrator || authenticated.Context.User.ID != issue.Context.User.ID {
		t.Fatalf("Authenticate() = %+v, %v", authenticated, err)
	}
	status, err = fixture.service.SetupStatus(fixture.ctx)
	if err != nil || status.Required {
		t.Fatalf("closed SetupStatus() = %+v, %v", status, err)
	}
	second := fixture.issuePreauthError(PreauthPurposeSetup)
	if !errors.Is(second, ErrSetupClosed) {
		t.Fatalf("second setup preauth error = %v", second)
	}
}

func TestServiceBootstrapFailureConsumesPreauthAndSuccessBurnsProof(t *testing.T) {
	fixture := newServiceFixture(t, true)
	first := fixture.issuePreauth(t, PreauthPurposeSetup)
	input := SetupInput{
		PreauthCookie: encodeTestToken(t, first.Cookie), PreauthProof: encodeTestToken(t, first.Proof),
		BootstrapProof: fixture.wrongBootstrap, SourceKey: "192.0.2.20",
		LoginName: "operator", DisplayName: "Operations", Password: []byte("correct horse value"),
		RequestID: "request_setup_bad",
	}
	if _, err := fixture.service.Setup(fixture.ctx, input); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("Setup(wrong bootstrap) error = %v", err)
	}
	input.BootstrapProof = fixture.bootstrap
	input.RequestID = "request_setup_replay"
	if _, err := fixture.service.Setup(fixture.ctx, input); !errors.Is(err, ErrSetupClosed) && !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("Setup(consumed preauth) error = %v", err)
	}
	second := fixture.issuePreauth(t, PreauthPurposeSetup)
	input.PreauthCookie = encodeTestToken(t, second.Cookie)
	input.PreauthProof = encodeTestToken(t, second.Proof)
	input.RequestID = "request_setup_good"
	if _, err := fixture.service.Setup(fixture.ctx, input); err != nil {
		t.Fatalf("Setup(valid bootstrap) error = %v", err)
	}
}

func TestServiceLoginFailuresDoNotRevealAccountState(t *testing.T) {
	fixture := setupServiceFixture(t)
	tests := []struct {
		name     string
		login    string
		password string
		disable  bool
	}{
		{name: "unknown", login: "missing", password: "correct horse value"},
		{name: "wrong password", login: "operator", password: "wrong password value"},
		{name: "disabled", login: "operator", password: "correct horse value", disable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture.repository.mu.Lock()
			account := fixture.repository.accounts["operator"]
			account.User.Enabled = !test.disable
			if test.disable {
				disabledAt := serviceTestTime
				account.User.DisabledAt = &disabledAt
			} else {
				account.User.DisabledAt = nil
			}
			fixture.repository.accounts["operator"] = account
			fixture.repository.mu.Unlock()
			preauth := fixture.issuePreauth(t, PreauthPurposeLogin)
			_, err := fixture.service.Login(fixture.ctx, LoginInput{
				PreauthCookie: encodeTestToken(t, preauth.Cookie), PreauthProof: encodeTestToken(t, preauth.Proof),
				SourceKey: "198.51.100.8", LoginName: test.login, Password: []byte(test.password),
				RequestID: domain.RequestID("request_" + test.name[0:1]),
			})
			if !errors.Is(err, ErrLoginRejected) {
				t.Fatalf("Login() error = %v", err)
			}
		})
	}
	if got := fixture.repository.loginFailureCount; got != len(tests) {
		t.Fatalf("generic failure commits = %d", got)
	}
}

func TestServiceLoginRotatesProofAndLogoutRevokesSession(t *testing.T) {
	fixture := setupServiceFixture(t)
	preauth := fixture.issuePreauth(t, PreauthPurposeLogin)
	issue, err := fixture.service.Login(fixture.ctx, LoginInput{
		PreauthCookie: encodeTestToken(t, preauth.Cookie), PreauthProof: encodeTestToken(t, preauth.Proof),
		SourceKey: "203.0.113.7", LoginName: "OPERATOR", Password: []byte("correct horse value"),
		RequestID: "request_login",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	token := encodeTestToken(t, issue.Token)
	csrf := encodeTestToken(t, issue.Context.CSRFProof)
	if err := fixture.service.Logout(fixture.ctx, LogoutInput{SessionToken: token, CSRFProof: csrf, RequestID: "request_logout"}); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := fixture.service.Authenticate(fixture.ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate(after logout) error = %v", err)
	}
	if err := fixture.service.Logout(fixture.ctx, LogoutInput{SessionToken: token, CSRFProof: csrf, RequestID: "request_logout_again"}); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("second Logout() error = %v", err)
	}
}

func TestServiceRejectsCrossSessionCSRFAndExpiresSessions(t *testing.T) {
	fixture := setupServiceFixture(t)
	first := fixture.login(t, "request_login_one")
	second := fixture.login(t, "request_login_two")
	err := fixture.service.Logout(fixture.ctx, LogoutInput{
		SessionToken: encodeTestToken(t, first.Token),
		CSRFProof:    encodeTestToken(t, second.Context.CSRFProof), RequestID: "request_cross_csrf",
	})
	if !errors.Is(err, ErrCSRFRejected) {
		t.Fatalf("Logout(cross CSRF) error = %v", err)
	}
	fixture.clock = first.Context.IdleExpiresAt
	if _, err := fixture.service.Authenticate(fixture.ctx, encodeTestToken(t, first.Token)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate(expired) error = %v", err)
	}
}

func TestServiceSlidesIdleExpiryWithoutExtendingAbsoluteExpiry(t *testing.T) {
	fixture := setupServiceFixture(t)
	issue := fixture.login(t, "request_touch_login")
	token := encodeTestToken(t, issue.Token)
	initial, err := fixture.service.Authenticate(fixture.ctx, token)
	if err != nil {
		t.Fatalf("Authenticate(initial) error = %v", err)
	}

	fixture.clock = initial.Session.LastSeenAt.Add(SessionTouchInterval - time.Microsecond)
	untouched, err := fixture.service.Authenticate(fixture.ctx, token)
	if err != nil || !untouched.Session.LastSeenAt.Equal(initial.Session.LastSeenAt) {
		t.Fatalf("Authenticate(before touch) invariant failed; error = %v", err)
	}

	fixture.clock = initial.Session.LastSeenAt.Add(SessionTouchInterval)
	touched, err := fixture.service.Authenticate(fixture.ctx, token)
	if err != nil || !touched.Session.LastSeenAt.Equal(fixture.clock) || !touched.Session.IdleExpiresAt.Equal(fixture.clock.Add(SessionIdleLifetime)) {
		t.Fatalf("Authenticate(touch) invariant failed; error = %v", err)
	}

	nearAbsolute := initial.Session.AbsoluteExpiresAt.Add(-30 * time.Second)
	fixture.repository.mu.Lock()
	resolved := fixture.repository.sessions[initial.Session.ID]
	resolved.Session.LastSeenAt = nearAbsolute.Add(-SessionTouchInterval)
	resolved.Session.IdleExpiresAt = initial.Session.AbsoluteExpiresAt
	fixture.repository.sessions[initial.Session.ID] = resolved
	fixture.repository.mu.Unlock()
	fixture.clock = nearAbsolute
	capped, err := fixture.service.Authenticate(fixture.ctx, token)
	if err != nil || !capped.Session.IdleExpiresAt.Equal(initial.Session.AbsoluteExpiresAt) {
		t.Fatalf("Authenticate(absolute cap) invariant failed; error = %v", err)
	}
	fixture.clock = initial.Session.AbsoluteExpiresAt
	if _, err := fixture.service.Authenticate(fixture.ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate(absolute expiry) error = %v", err)
	}
}

func TestServiceCreatesPurposeBoundRecentGrantAndEnforcesRole(t *testing.T) {
	fixture := setupServiceFixture(t)
	issue := fixture.login(t, "request_login_grant")
	input := RecentAuthInput{
		SessionToken: encodeTestToken(t, issue.Token), CSRFProof: encodeTestToken(t, issue.Context.CSRFProof),
		CurrentPassword: []byte("correct horse value"), Purpose: RecentAuthPurposeUserCreate,
		RequestID: "request_grant",
	}
	grant, err := fixture.service.CreateRecentGrant(fixture.ctx, input)
	if err != nil || grant.Purpose != RecentAuthPurposeUserCreate || grant.ExpiresAt.Sub(fixture.clock) != RecentGrantLifetime {
		t.Fatalf("CreateRecentGrant() invariant failed; error = %v", err)
	}
	fixture.repository.mu.Lock()
	account := fixture.repository.accounts["operator"]
	account.User.Role = RoleViewer
	fixture.repository.accounts["operator"] = account
	for id, session := range fixture.repository.sessions {
		if session.Session.UserID == account.User.ID {
			fixture.repository.sessions[id] = ResolvedSession{User: account.User, Session: session.Session}
		}
	}
	fixture.repository.mu.Unlock()
	issue = fixture.login(t, "request_viewer_login")
	input.SessionToken = encodeTestToken(t, issue.Token)
	input.CSRFProof = encodeTestToken(t, issue.Context.CSRFProof)
	input.RequestID = "request_viewer_grant"
	fixture.repository.mu.Lock()
	auditsBeforeDeniedGrant := len(fixture.repository.audits)
	fixture.repository.mu.Unlock()
	if _, err := fixture.service.CreateRecentGrant(fixture.ctx, input); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("viewer grant error = %v", err)
	}
	fixture.repository.mu.Lock()
	auditsAfterDeniedGrant := len(fixture.repository.audits)
	fixture.repository.mu.Unlock()
	if auditsAfterDeniedGrant != auditsBeforeDeniedGrant+1 {
		t.Fatalf("denied recent grant audit count = %d, want %d", auditsAfterDeniedGrant, auditsBeforeDeniedGrant+1)
	}
	if _, err := fixture.service.ListUsers(fixture.ctx, AuthenticatedReadInput{SessionToken: input.SessionToken}, UserListQuery{Limit: 25}); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("viewer user list error = %v", err)
	}
	input.Purpose = RecentAuthPurposeAccountPasswordChange
	if _, err := fixture.service.CreateRecentGrant(fixture.ctx, input); err != nil {
		t.Fatalf("viewer self-password grant error = %v", err)
	}
}

func TestServiceConsumesNotificationTestGrantOnce(t *testing.T) {
	fixture := setupServiceFixture(t)
	administrator := fixture.login(t, "request_notification_test_login")
	grant := fixture.recentGrant(
		t, administrator, RecentAuthPurposeNotificationTest, "request_notification_test_grant",
	)
	input := RecentAuthorizationInput{
		SessionToken: encodeTestToken(t, administrator.Token),
		CSRFProof:    encodeTestToken(t, administrator.Context.CSRFProof),
		RecentProof:  encodeTestToken(t, grant.Proof), Purpose: RecentAuthPurposeNotificationTest,
		RequestID: "request_notification_test_authorize",
	}
	authorized, err := fixture.service.AuthorizeRecent(fixture.ctx, input)
	if err != nil || authorized.Principal.Role != RoleAdministrator {
		t.Fatalf("AuthorizeRecent() = (%#v, %v)", authorized, err)
	}
	input.RequestID = "request_notification_test_replay"
	if _, err := fixture.service.AuthorizeRecent(fixture.ctx, input); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("AuthorizeRecent(replay) error = %v", err)
	}
}

func TestServiceEnforcesRecentGrantPasswordPurposeSessionExpiryAndReplay(t *testing.T) {
	fixture := setupServiceFixture(t)
	administrator := fixture.login(t, "request_recent_rules_login")
	badInput := RecentAuthInput{
		SessionToken: encodeTestToken(t, administrator.Token), CSRFProof: encodeTestToken(t, administrator.Context.CSRFProof),
		CurrentPassword: []byte("wrong horse value"), Purpose: RecentAuthPurposeUserCreate, RequestID: "request_recent_wrong_password",
	}
	if _, err := fixture.service.CreateRecentGrant(fixture.ctx, badInput); !errors.Is(err, ErrRecentAuthRejected) {
		t.Fatalf("CreateRecentGrant(wrong password) error = %v", err)
	}

	purposeGrant := fixture.recentGrant(t, administrator, RecentAuthPurposeUserCreate, "request_recent_purpose_grant")
	displayName := "Wrong Purpose"
	_, err := fixture.service.UpdateUser(fixture.ctx, UserUpdateInput{
		SessionToken: badInput.SessionToken, CSRFProof: badInput.CSRFProof,
		RecentProof: encodeTestToken(t, purposeGrant.Proof), UserID: administrator.Context.User.ID,
		ExpectedVersion: administrator.Context.User.Version, DisplayName: &displayName, RequestID: "request_recent_wrong_purpose",
	})
	if !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("UpdateUser(wrong purpose) error = %v", err)
	}
	createWithGrant := func(session SessionIssue, proof OpaqueToken, loginName string, requestID domain.RequestID) error {
		_, err := fixture.service.CreateUser(fixture.ctx, UserCreateInput{
			SessionToken: encodeTestToken(t, session.Token), CSRFProof: encodeTestToken(t, session.Context.CSRFProof),
			RecentProof: encodeTestToken(t, proof), LoginName: loginName, DisplayName: "Grant Test Viewer",
			Role: RoleViewer, Password: []byte("viewer correct horse"), RequestID: requestID,
		})
		return err
	}
	if err := createWithGrant(administrator, purposeGrant.Proof, "grantpurpose", "request_recent_right_purpose"); err != nil {
		t.Fatalf("CreateUser(right purpose) error = %v", err)
	}
	if err := createWithGrant(administrator, purposeGrant.Proof, "grantreplay", "request_recent_replay"); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("CreateUser(replay) error = %v", err)
	}

	sessionGrant := fixture.recentGrant(t, administrator, RecentAuthPurposeUserCreate, "request_recent_session_grant")
	secondSession := fixture.login(t, "request_recent_second_login")
	if err := createWithGrant(secondSession, sessionGrant.Proof, "grantsession", "request_recent_wrong_session"); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("CreateUser(wrong session) error = %v", err)
	}
	if err := createWithGrant(administrator, sessionGrant.Proof, "grantsession", "request_recent_right_session"); err != nil {
		t.Fatalf("CreateUser(right session) error = %v", err)
	}

	expiredGrant := fixture.recentGrant(t, administrator, RecentAuthPurposeUserCreate, "request_recent_expired_grant")
	fixture.clock = expiredGrant.ExpiresAt
	if err := createWithGrant(administrator, expiredGrant.Proof, "grantexpired", "request_recent_expired"); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("CreateUser(expired grant) error = %v", err)
	}
}

func TestServiceDurableThrottleStopsHashPath(t *testing.T) {
	fixture := setupServiceFixture(t)
	key := fixture.service.throttleKey(ThrottleScopeLoginAccount, "account", "operator")
	fixture.repository.mu.Lock()
	fixture.repository.throttles[key] = ThrottleState{BlockedUntil: fixture.clock.Add(time.Minute)}
	fixture.repository.mu.Unlock()
	preauth := fixture.issuePreauth(t, PreauthPurposeLogin)
	_, err := fixture.service.Login(fixture.ctx, LoginInput{
		PreauthCookie: encodeTestToken(t, preauth.Cookie), PreauthProof: encodeTestToken(t, preauth.Proof),
		SourceKey: "203.0.113.12", LoginName: "operator", Password: []byte("correct horse value"),
		RequestID: "request_throttled",
	})
	if !errors.Is(err, ErrAuthRateLimited) {
		t.Fatalf("Login(throttled) error = %v", err)
	}
	var rateError interface{ RetryAt() time.Time }
	if !errors.As(err, &rateError) || rateError.RetryAt() != fixture.clock.Add(time.Minute) {
		t.Fatalf("rate error = %v", err)
	}
}

func TestServiceDurableRecentThrottleStopsPasswordHash(t *testing.T) {
	fixture := setupServiceFixture(t)
	session := fixture.login(t, "request_recent_throttle_login")
	key := fixture.service.throttleKey(
		ThrottleScopeRecentUser,
		"user",
		string(session.Context.User.ID),
	)
	blockedUntil := fixture.clock.Add(time.Minute)
	fixture.repository.mu.Lock()
	fixture.repository.throttles[key] = ThrottleState{BlockedUntil: blockedUntil}
	fixture.repository.mu.Unlock()

	_, err := fixture.service.CreateRecentGrant(fixture.ctx, RecentAuthInput{
		SessionToken:    encodeTestToken(t, session.Token),
		CSRFProof:       encodeTestToken(t, session.Context.CSRFProof),
		CurrentPassword: []byte("correct horse value"),
		Purpose:         RecentAuthPurposeAccountPasswordChange,
		RequestID:       "request_recent_throttled",
	})
	if !errors.Is(err, ErrAuthRateLimited) {
		t.Fatalf("CreateRecentGrant(throttled) error = %v", err)
	}
	var rateError interface{ RetryAt() time.Time }
	if !errors.As(err, &rateError) || !rateError.RetryAt().Equal(blockedUntil) {
		t.Fatalf("recent rate error = %v", err)
	}
}

func TestServiceReportsRequiredBootstrapButFailsClosedWithoutGate(t *testing.T) {
	fixture := newServiceFixture(t, false)
	fixture.service.bootstrapRequired = true
	status, err := fixture.service.SetupStatus(fixture.ctx)
	if err != nil || !status.Required || !status.BootstrapProofRequired {
		t.Fatalf("SetupStatus() = %+v, %v", status, err)
	}
	preauth := fixture.issuePreauth(t, PreauthPurposeSetup)
	_, err = fixture.service.Setup(fixture.ctx, SetupInput{
		PreauthCookie: encodeTestToken(t, preauth.Cookie), PreauthProof: encodeTestToken(t, preauth.Proof),
		BootstrapProof: fixture.wrongBootstrap, SourceKey: "192.0.2.44",
		LoginName: "operator", DisplayName: "Operations", Password: []byte("correct horse value"),
		RequestID: "request_missing_gate",
	})
	if !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("Setup(missing gate) error = %v", err)
	}
}

func TestServiceConsumesValidPreauthBeforeFieldRejection(t *testing.T) {
	fixture := newServiceFixture(t, false)
	setupProof := fixture.issuePreauth(t, PreauthPurposeSetup)
	_, err := fixture.service.Setup(fixture.ctx, SetupInput{
		PreauthCookie: encodeTestToken(t, setupProof.Cookie), PreauthProof: encodeTestToken(t, setupProof.Proof),
		SourceKey: "192.0.2.55", LoginName: "operator", DisplayName: "",
		Password: []byte("correct horse value"), RequestID: "request_bad_setup_field",
	})
	if !errors.Is(err, ErrInvalidServiceInput) {
		t.Fatalf("Setup(invalid field) error = %v", err)
	}
	fixture.repository.mu.Lock()
	setupConsumed := fixture.repository.preauth[len(fixture.repository.preauth)-1].ConsumedAt != nil
	fixture.repository.mu.Unlock()
	if !setupConsumed {
		t.Fatal("invalid setup left valid preauth reusable")
	}

	validSetup := fixture.issuePreauth(t, PreauthPurposeSetup)
	if _, err := fixture.service.Setup(fixture.ctx, SetupInput{
		PreauthCookie: encodeTestToken(t, validSetup.Cookie), PreauthProof: encodeTestToken(t, validSetup.Proof),
		SourceKey: "192.0.2.55", LoginName: "operator", DisplayName: "Operations",
		Password: []byte("correct horse value"), RequestID: "request_valid_setup_after_bad",
	}); err != nil {
		t.Fatalf("Setup(valid) error = %v", err)
	}
	loginProof := fixture.issuePreauth(t, PreauthPurposeLogin)
	_, err = fixture.service.Login(fixture.ctx, LoginInput{
		PreauthCookie: encodeTestToken(t, loginProof.Cookie), PreauthProof: encodeTestToken(t, loginProof.Proof),
		SourceKey: "192.0.2.55", LoginName: "bad login", Password: []byte("correct horse value"),
		RequestID: "request_bad_login_field",
	})
	if !errors.Is(err, ErrInvalidServiceInput) {
		t.Fatalf("Login(invalid field) error = %v", err)
	}
	fixture.repository.mu.Lock()
	loginConsumed := fixture.repository.preauth[len(fixture.repository.preauth)-1].ConsumedAt != nil
	fixture.repository.mu.Unlock()
	if !loginConsumed {
		t.Fatal("invalid login left valid preauth reusable")
	}
}

func TestServiceRejectsRepositorySessionDigestMismatchAndClockRollback(t *testing.T) {
	fixture := setupServiceFixture(t)
	issue := fixture.login(t, "request_digest_login")
	token := encodeTestToken(t, issue.Token)
	authenticated, err := fixture.service.Authenticate(fixture.ctx, token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	wrongToken, err := generateOpaqueToken(&counterReader{next: 240})
	if err != nil {
		t.Fatalf("wrong token error = %v", err)
	}
	wrongDigest, err := DigestSessionToken(fixture.service.digestKey[:], wrongToken)
	if err != nil {
		t.Fatalf("wrong digest error = %v", err)
	}
	tampered := ResolvedSession{User: authenticated.User, Session: authenticated.Session}
	tampered.Session.TokenDigest = wrongDigest
	fixture.repository.mu.Lock()
	fixture.repository.resolveOverride = &tampered
	fixture.repository.mu.Unlock()
	if _, err := fixture.service.Authenticate(fixture.ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate(digest mismatch) error = %v", err)
	}
	fixture.repository.mu.Lock()
	fixture.repository.resolveOverride = nil
	fixture.repository.mu.Unlock()
	fixture.clock = authenticated.Session.LastSeenAt.Add(-time.Microsecond)
	if _, err := fixture.service.Authenticate(fixture.ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate(clock rollback) error = %v", err)
	}
}

func TestServiceAdministratorUserLifecycleUsesPurposeBoundAtomicCommits(t *testing.T) {
	fixture := setupServiceFixture(t)
	administrator := fixture.login(t, "request_admin_user_login")
	createGrant := fixture.recentGrant(t, administrator, RecentAuthPurposeUserCreate, "request_user_create_grant")
	viewer, err := fixture.service.CreateUser(fixture.ctx, UserCreateInput{
		SessionToken: encodeTestToken(t, administrator.Token), CSRFProof: encodeTestToken(t, administrator.Context.CSRFProof),
		RecentProof: encodeTestToken(t, createGrant.Proof), LoginName: "viewer01", DisplayName: "On-call Viewer",
		Role: RoleViewer, Password: []byte("viewer correct horse"), RequestID: "request_user_create",
	})
	if err != nil || viewer.Role != RoleViewer || !viewer.Enabled {
		t.Fatalf("CreateUser() = %+v, %v", viewer, err)
	}
	page, err := fixture.service.ListUsers(fixture.ctx, AuthenticatedReadInput{SessionToken: encodeTestToken(t, administrator.Token)}, UserListQuery{Limit: 50})
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("ListUsers() = %+v, %v", page, err)
	}
	stored, err := fixture.service.GetUser(fixture.ctx, AuthenticatedReadInput{SessionToken: encodeTestToken(t, administrator.Token)}, viewer.ID)
	if err != nil || stored.ID != viewer.ID {
		t.Fatalf("GetUser() = %+v, %v", stored, err)
	}

	updateGrant := fixture.recentGrant(t, administrator, RecentAuthPurposeUserUpdate, "request_user_update_grant")
	newDisplay := "Night Viewer"
	updated, err := fixture.service.UpdateUser(fixture.ctx, UserUpdateInput{
		SessionToken: encodeTestToken(t, administrator.Token), CSRFProof: encodeTestToken(t, administrator.Context.CSRFProof),
		RecentProof: encodeTestToken(t, updateGrant.Proof), UserID: viewer.ID, ExpectedVersion: viewer.Version,
		DisplayName: &newDisplay, RequestID: "request_user_update",
	})
	if err != nil || updated.DisplayName != newDisplay || updated.Version != viewer.Version+1 {
		t.Fatalf("UpdateUser() = %+v, %v", updated, err)
	}

	resetGrant := fixture.recentGrant(t, administrator, RecentAuthPurposeUserPasswordReset, "request_user_reset_grant")
	if err := fixture.service.ResetUserPassword(fixture.ctx, UserPasswordResetInput{
		SessionToken: encodeTestToken(t, administrator.Token), CSRFProof: encodeTestToken(t, administrator.Context.CSRFProof),
		RecentProof: encodeTestToken(t, resetGrant.Proof), UserID: viewer.ID, ExpectedVersion: updated.Version,
		NewPassword: []byte("reset viewer horse"), RequestID: "request_user_reset",
	}); err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
	stored, err = fixture.service.GetUser(fixture.ctx, AuthenticatedReadInput{SessionToken: encodeTestToken(t, administrator.Token)}, viewer.ID)
	if err != nil {
		t.Fatalf("GetUser(after reset) error = %v", err)
	}

	revokeGrant := fixture.recentGrant(t, administrator, RecentAuthPurposeUserSessionsRevoke, "request_user_revoke_grant")
	if err := fixture.service.RevokeUserSessions(fixture.ctx, UserSessionsRevokeInput{
		SessionToken: encodeTestToken(t, administrator.Token), CSRFProof: encodeTestToken(t, administrator.Context.CSRFProof),
		RecentProof: encodeTestToken(t, revokeGrant.Proof), UserID: viewer.ID, RequestID: "request_user_revoke",
	}); err != nil {
		t.Fatalf("RevokeUserSessions() error = %v", err)
	}

	deleteGrant := fixture.recentGrant(t, administrator, RecentAuthPurposeUserDelete, "request_user_delete_grant")
	if err := fixture.service.DeleteUser(fixture.ctx, UserDeleteInput{
		SessionToken: encodeTestToken(t, administrator.Token), CSRFProof: encodeTestToken(t, administrator.Context.CSRFProof),
		RecentProof: encodeTestToken(t, deleteGrant.Proof), UserID: viewer.ID, ExpectedVersion: stored.Version,
		RequestID: "request_user_delete",
	}); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if _, err := fixture.service.GetUser(fixture.ctx, AuthenticatedReadInput{SessionToken: encodeTestToken(t, administrator.Token)}, viewer.ID); !errors.Is(err, ErrAuthNotFound) {
		t.Fatalf("GetUser(deleted) error = %v", err)
	}
}

func TestServiceProtectsLastEnabledAdministrator(t *testing.T) {
	fixture := setupServiceFixture(t)
	administrator := fixture.login(t, "request_last_admin_login")
	grant := fixture.recentGrant(t, administrator, RecentAuthPurposeUserUpdate, "request_last_admin_grant")
	viewerRole := RoleViewer
	_, err := fixture.service.UpdateUser(fixture.ctx, UserUpdateInput{
		SessionToken: encodeTestToken(t, administrator.Token), CSRFProof: encodeTestToken(t, administrator.Context.CSRFProof),
		RecentProof: encodeTestToken(t, grant.Proof), UserID: administrator.Context.User.ID,
		ExpectedVersion: administrator.Context.User.Version, Role: &viewerRole, RequestID: "request_last_admin_update",
	})
	requireConflictCode(t, err, ErrLastAdministrator, ConflictCodeLastAdministrator)
}

func TestServiceReturnsSafeAccountConflictCodes(t *testing.T) {
	fixture := setupServiceFixture(t)
	administrator := fixture.login(t, "request_conflict_login")

	createGrant := fixture.recentGrant(t, administrator, RecentAuthPurposeUserCreate, "request_conflict_create_grant")
	_, err := fixture.service.CreateUser(fixture.ctx, UserCreateInput{
		SessionToken: encodeTestToken(t, administrator.Token), CSRFProof: encodeTestToken(t, administrator.Context.CSRFProof),
		RecentProof: encodeTestToken(t, createGrant.Proof), LoginName: "operator", DisplayName: "Second Operator",
		Role: RoleViewer, Password: []byte("second correct horse"), RequestID: "request_conflict_create",
	})
	requireConflictCode(t, err, ErrLoginNameConflict, ConflictCodeLoginNameConflict)

	updateGrant := fixture.recentGrant(t, administrator, RecentAuthPurposeUserUpdate, "request_conflict_update_grant")
	displayName := "Stale Operator"
	_, err = fixture.service.UpdateUser(fixture.ctx, UserUpdateInput{
		SessionToken: encodeTestToken(t, administrator.Token), CSRFProof: encodeTestToken(t, administrator.Context.CSRFProof),
		RecentProof: encodeTestToken(t, updateGrant.Proof), UserID: administrator.Context.User.ID,
		ExpectedVersion: administrator.Context.User.Version + 1, DisplayName: &displayName, RequestID: "request_conflict_update",
	})
	requireConflictCode(t, err, ErrStaleVersion, ConflictCodeStaleVersion)
}

func TestServiceRejectsNoOpUserUpdateWithoutConsumingRecentGrant(t *testing.T) {
	fixture := setupServiceFixture(t)
	administrator := fixture.login(t, "request_noop_login")
	grant := fixture.recentGrant(t, administrator, RecentAuthPurposeUserUpdate, "request_noop_grant")
	user := administrator.Context.User
	_, err := fixture.service.UpdateUser(fixture.ctx, UserUpdateInput{
		SessionToken: encodeTestToken(t, administrator.Token), CSRFProof: encodeTestToken(t, administrator.Context.CSRFProof),
		RecentProof: encodeTestToken(t, grant.Proof), UserID: user.ID, ExpectedVersion: user.Version,
		LoginName: &user.LoginName, DisplayName: &user.DisplayName, Role: &user.Role, Enabled: &user.Enabled,
		RequestID: "request_noop_update",
	})
	if !errors.Is(err, ErrInvalidServiceInput) || errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("UpdateUser(no-op) error = %v", err)
	}
	displayName := "Updated Operator"
	updated, err := fixture.service.UpdateUser(fixture.ctx, UserUpdateInput{
		SessionToken: encodeTestToken(t, administrator.Token), CSRFProof: encodeTestToken(t, administrator.Context.CSRFProof),
		RecentProof: encodeTestToken(t, grant.Proof), UserID: user.ID, ExpectedVersion: user.Version,
		DisplayName: &displayName, RequestID: "request_after_noop_update",
	})
	if err != nil || updated.DisplayName != displayName {
		t.Fatalf("UpdateUser(after no-op) = %+v, %v", updated, err)
	}
}

func TestServiceOwnPasswordChangeRotatesCurrentAndRevokesOldSession(t *testing.T) {
	fixture := setupServiceFixture(t)
	oldIssue := fixture.login(t, "request_password_login")
	grant := fixture.recentGrant(t, oldIssue, RecentAuthPurposeAccountPasswordChange, "request_password_grant")
	newIssue, err := fixture.service.ChangeOwnPassword(fixture.ctx, OwnPasswordChangeInput{
		SessionToken: encodeTestToken(t, oldIssue.Token), CSRFProof: encodeTestToken(t, oldIssue.Context.CSRFProof),
		RecentProof: encodeTestToken(t, grant.Proof), NewPassword: []byte("new correct horse value"),
		RequestID: "request_password_change",
	})
	if err != nil {
		t.Fatalf("ChangeOwnPassword() error = %v", err)
	}
	if _, err := fixture.service.Authenticate(fixture.ctx, encodeTestToken(t, oldIssue.Token)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("old session error = %v", err)
	}
	if _, err := fixture.service.Authenticate(fixture.ctx, encodeTestToken(t, newIssue.Token)); err != nil {
		t.Fatalf("new session error = %v", err)
	}
	if _, err := fixture.service.ChangeOwnPassword(fixture.ctx, OwnPasswordChangeInput{
		SessionToken: encodeTestToken(t, newIssue.Token), CSRFProof: encodeTestToken(t, newIssue.Context.CSRFProof),
		RecentProof: encodeTestToken(t, grant.Proof), NewPassword: []byte("third correct horse"),
		RequestID: "request_password_replay",
	}); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("replayed grant error = %v", err)
	}
}

func TestServiceOperatorRecoveryNeverReopensSetup(t *testing.T) {
	fixture := setupServiceFixture(t)
	fixture.repository.mu.Lock()
	account := fixture.repository.accounts["operator"]
	disabledAt := fixture.clock
	account.User.Enabled = false
	account.User.DisabledAt = &disabledAt
	account.User.UpdatedAt = fixture.clock
	fixture.repository.accounts["operator"] = account
	fixture.repository.mu.Unlock()
	recovered, err := fixture.service.RecoverAdministrator(fixture.ctx, OperatorRecoveryInput{
		OperatorID: "local_console", LoginName: "operator", DisplayName: "Recovered Operator",
		Password: []byte("recovered horse value"), RequestID: "request_operator_recovery",
	})
	if err != nil || !recovered.Enabled || recovered.Role != RoleAdministrator || recovered.AuthRevision <= account.User.AuthRevision {
		t.Fatalf("RecoverAdministrator() = %+v, %v", recovered, err)
	}
	status, err := fixture.service.SetupStatus(fixture.ctx)
	if err != nil || status.Required {
		t.Fatalf("SetupStatus(after recovery) = %+v, %v", status, err)
	}
}

type serviceFixture struct {
	ctx            context.Context
	service        *Service
	repository     *fakeAuthRepository
	clock          time.Time
	bootstrap      string
	wrongBootstrap string
}

func newServiceFixture(t *testing.T, bootstrapRequired bool) *serviceFixture {
	t.Helper()
	repository := newFakeAuthRepository()
	hasher := mustPasswordHasher(t)
	hasher.derive = fastTestDeriver
	hasher.random = &counterReader{next: 1}
	fixture := &serviceFixture{
		ctx: context.Background(), repository: repository, clock: serviceTestTime,
	}
	var gate *BootstrapGate
	if bootstrapRequired {
		bootstrapToken, err := generateOpaqueToken(&counterReader{next: 190})
		if err != nil {
			t.Fatalf("bootstrap token error = %v", err)
		}
		fixture.bootstrap = encodeTestToken(t, bootstrapToken)
		wrongToken, err := generateOpaqueToken(&counterReader{next: 220})
		if err != nil {
			t.Fatalf("wrong bootstrap token error = %v", err)
		}
		fixture.wrongBootstrap = encodeTestToken(t, wrongToken)
		gate, err = newBootstrapGate(fixture.bootstrap, fixture.clock.Add(time.Hour), func() time.Time { return fixture.clock })
		if err != nil {
			t.Fatalf("newBootstrapGate() error = %v", err)
		}
	}
	service, err := NewService(ServiceOptions{
		Repository: repository, PasswordHasher: hasher,
		DigestKey: make([]byte, DigestKeyBytes), DigestKeyID: "key_test",
		BootstrapRequired: bootstrapRequired, BootstrapGate: gate,
		Now: func() time.Time { return fixture.clock }, Random: &counterReader{next: 10},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	fixture.service = service
	return fixture
}

func setupServiceFixture(t *testing.T) *serviceFixture {
	t.Helper()
	fixture := newServiceFixture(t, false)
	preauth := fixture.issuePreauth(t, PreauthPurposeSetup)
	_, err := fixture.service.Setup(fixture.ctx, SetupInput{
		PreauthCookie: encodeTestToken(t, preauth.Cookie), PreauthProof: encodeTestToken(t, preauth.Proof),
		SourceKey: "192.0.2.1", LoginName: "operator", DisplayName: "Operations",
		Password: []byte("correct horse value"), RequestID: "request_initial_setup",
	})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	return fixture
}

func (fixture *serviceFixture) issuePreauth(t *testing.T, purpose PreauthPurpose) PreauthIssue {
	t.Helper()
	issue, err := fixture.service.CreatePreauth(fixture.ctx, purpose, "192.0.2.1")
	if err != nil {
		t.Fatalf("CreatePreauth() error = %v", err)
	}
	return issue
}

func (fixture *serviceFixture) issuePreauthError(purpose PreauthPurpose) error {
	_, err := fixture.service.CreatePreauth(fixture.ctx, purpose, "192.0.2.1")
	return err
}

func (fixture *serviceFixture) login(t *testing.T, requestID domain.RequestID) SessionIssue {
	t.Helper()
	preauth := fixture.issuePreauth(t, PreauthPurposeLogin)
	issue, err := fixture.service.Login(fixture.ctx, LoginInput{
		PreauthCookie: encodeTestToken(t, preauth.Cookie), PreauthProof: encodeTestToken(t, preauth.Proof),
		SourceKey: "203.0.113.1", LoginName: "operator", Password: []byte("correct horse value"),
		RequestID: requestID,
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	return issue
}

func (fixture *serviceFixture) recentGrant(t *testing.T, issue SessionIssue, purpose RecentAuthPurpose, requestID domain.RequestID) RecentGrantIssue {
	t.Helper()
	grant, err := fixture.service.CreateRecentGrant(fixture.ctx, RecentAuthInput{
		SessionToken: encodeTestToken(t, issue.Token), CSRFProof: encodeTestToken(t, issue.Context.CSRFProof),
		CurrentPassword: []byte("correct horse value"), Purpose: purpose, RequestID: requestID,
	})
	if err != nil {
		t.Fatalf("CreateRecentGrant(%s) error = %v", purpose, err)
	}
	return grant
}

func encodeTestToken(t *testing.T, token OpaqueToken) string {
	t.Helper()
	encoded, err := token.Encode()
	if err != nil {
		t.Fatalf("token Encode() error = %v", err)
	}
	return encoded
}

func requireConflictCode(t *testing.T, err error, target error, code ConflictCode) {
	t.Helper()
	if !errors.Is(err, ErrAuthConflict) || !errors.Is(err, target) {
		t.Fatalf("conflict error = %v, want %v", err, target)
	}
	var coded interface{ Code() string }
	if !errors.As(err, &coded) || coded.Code() != string(code) {
		t.Fatalf("conflict code = %v, want %q", err, code)
	}
}

func fastTestDeriver(password []byte, salt []byte, _ uint32, _ uint32, _ uint8, length uint32) []byte {
	input := append(append(make([]byte, 0, len(password)+len(salt)), password...), salt...)
	digest := sha256.Sum256(input)
	result := make([]byte, length)
	for index := range result {
		result[index] = digest[index%len(digest)]
	}
	return result
}

type counterReader struct {
	mu   sync.Mutex
	next byte
}

func (reader *counterReader) Read(target []byte) (int, error) {
	if reader == nil {
		return 0, io.ErrClosedPipe
	}
	reader.mu.Lock()
	defer reader.mu.Unlock()
	for index := range target {
		target[index] = reader.next
		reader.next++
	}
	return len(target), nil
}

type fakeAuthRepository struct {
	mu                sync.Mutex
	setupRequired     bool
	preauth           []Preauth
	accounts          map[string]LoginAccount
	sessions          map[SessionID]ResolvedSession
	grants            []RecentGrant
	throttles         map[ThrottleKey]ThrottleState
	audits            []domain.AuditEntry
	loginFailureCount int
	resolveOverride   *ResolvedSession
	listUsersError    error
}

var _ Repository = (*fakeAuthRepository)(nil)

func newFakeAuthRepository() *fakeAuthRepository {
	return &fakeAuthRepository{
		setupRequired: true, accounts: make(map[string]LoginAccount),
		sessions: make(map[SessionID]ResolvedSession), throttles: make(map[ThrottleKey]ThrottleState),
	}
}

func (repository *fakeAuthRepository) SetupRequired(context.Context) (bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.setupRequired, nil
}

func (repository *fakeAuthRepository) InsertPreauth(_ context.Context, preauth Preauth) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.preauth = append(repository.preauth, preauth)
	return nil
}

func (repository *fakeAuthRepository) CheckThrottle(_ context.Context, key ThrottleKey, _ time.Time) (ThrottleState, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.throttles[key], nil
}

func (repository *fakeAuthRepository) CompleteInitialSetup(_ context.Context, commit InitialSetupCommit) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if !repository.setupRequired || !repository.consumePreauth(commit.Preauth) {
		return ErrAuthRepositoryConflict
	}
	repository.setupRequired = false
	repository.accounts[commit.User.CanonicalLoginName] = LoginAccount{User: commit.User, Credential: commit.Credential}
	repository.sessions[commit.Session.ID] = ResolvedSession{User: commit.User, Session: commit.Session}
	delete(repository.throttles, commit.ClearThrottle)
	repository.audits = append(repository.audits, commit.Audit)
	return nil
}

func (repository *fakeAuthRepository) CommitSetupFailure(_ context.Context, commit SetupFailureCommit) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if !repository.consumePreauth(commit.Preauth) {
		return ErrAuthRepositoryRejected
	}
	repository.recordThrottle(commit.Throttle)
	repository.audits = append(repository.audits, commit.Audit)
	return nil
}

func (repository *fakeAuthRepository) ConsumePreauth(_ context.Context, use PreauthUse) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if !repository.consumePreauth(use) {
		return ErrAuthRepositoryRejected
	}
	return nil
}

func (repository *fakeAuthRepository) FindLoginAccount(_ context.Context, canonical string) (LoginAccount, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	account, exists := repository.accounts[canonical]
	if !exists {
		return LoginAccount{}, ErrAuthRepositoryNotFound
	}
	return account, nil
}

func (repository *fakeAuthRepository) CommitLoginSuccess(_ context.Context, commit LoginSuccessCommit) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	account, exists := repository.accountByUserID(commit.Session.UserID)
	if !exists || !account.User.Enabled {
		return ErrAuthRepositoryRejected
	}
	if commit.RehashedCredential != nil {
		account.Credential = *commit.RehashedCredential
		repository.accounts[account.User.CanonicalLoginName] = account
	}
	repository.sessions[commit.Session.ID] = ResolvedSession{User: account.User, Session: commit.Session}
	for _, key := range commit.ClearThrottles {
		delete(repository.throttles, key)
	}
	repository.audits = append(repository.audits, commit.Audit)
	return nil
}

func (repository *fakeAuthRepository) CommitLoginFailure(_ context.Context, commit LoginFailureCommit) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, failure := range commit.Throttles {
		repository.recordThrottle(failure)
	}
	repository.loginFailureCount++
	repository.audits = append(repository.audits, commit.Audit)
	return nil
}

func (repository *fakeAuthRepository) ResolveSession(_ context.Context, digest SessionTokenDigest, keyID string) (ResolvedSession, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.resolveOverride != nil {
		return *repository.resolveOverride, nil
	}
	for _, resolved := range repository.sessions {
		if resolved.Session.TokenDigestKeyID == keyID && resolved.Session.TokenDigest.Equal(digest) {
			account, exists := repository.accountByUserID(resolved.Session.UserID)
			if !exists {
				return ResolvedSession{}, ErrAuthRepositoryNotFound
			}
			resolved.User = account.User
			return resolved, nil
		}
	}
	return ResolvedSession{}, ErrAuthRepositoryNotFound
}

func (repository *fakeAuthRepository) TouchSession(_ context.Context, touch SessionTouch) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	resolved, exists := repository.sessions[touch.SessionID]
	if !exists || resolved.User.ID != touch.UserID || resolved.User.AuthRevision != touch.AuthRevision {
		return ErrAuthRepositoryRejected
	}
	resolved.Session.LastSeenAt = touch.LastSeenAt
	resolved.Session.IdleExpiresAt = touch.IdleExpiresAt
	repository.sessions[touch.SessionID] = resolved
	return nil
}

func (repository *fakeAuthRepository) RevokeSession(_ context.Context, revocation SessionRevocation) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	resolved, exists := repository.sessions[revocation.SessionID]
	if !exists || resolved.Session.RevokedAt != nil {
		return ErrAuthRepositoryRejected
	}
	resolved.Session.RevokedAt = &revocation.RevokedAt
	resolved.Session.RevokeReason = revocation.Reason
	repository.sessions[revocation.SessionID] = resolved
	repository.audits = append(repository.audits, revocation.Audit)
	return nil
}

func (repository *fakeAuthRepository) CommitRecentGrant(_ context.Context, commit RecentGrantCommit) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	resolved, exists := repository.sessions[commit.Actor.SessionID]
	if !exists || resolved.Session.RevokedAt != nil || resolved.User.Role != commit.Actor.Role {
		return ErrAuthRepositoryRejected
	}
	repository.grants = append(repository.grants, commit.Grant)
	delete(repository.throttles, commit.ClearThrottle)
	repository.audits = append(repository.audits, commit.Audit)
	return nil
}

func (repository *fakeAuthRepository) CommitRecentAuthFailure(_ context.Context, commit RecentAuthFailureCommit) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if commit.Throttle != nil {
		repository.recordThrottle(*commit.Throttle)
	}
	repository.audits = append(repository.audits, commit.Audit)
	return nil
}

func (repository *fakeAuthRepository) CommitRecentAuthorization(_ context.Context, commit RecentAuthorizationCommit) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if commit.Actor.Role != RoleAdministrator || !repository.consumeGrant(commit.Grant) {
		return ErrAuthRepositoryRejected
	}
	repository.audits = append(repository.audits, commit.Audit)
	return nil
}

func (repository *fakeAuthRepository) ListUsers(context.Context, UserListQuery) (UserPage, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.listUsersError != nil {
		return UserPage{}, repository.listUsersError
	}
	page := UserPage{}
	for _, account := range repository.accounts {
		if account.User.DeletedAt == nil {
			page.Items = append(page.Items, account.User)
		}
	}
	return page, nil
}

func (repository *fakeAuthRepository) GetUser(_ context.Context, id UserID) (User, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	account, exists := repository.accountByUserID(id)
	if !exists || account.User.DeletedAt != nil {
		return User{}, ErrAuthRepositoryNotFound
	}
	return account.User, nil
}

func (repository *fakeAuthRepository) CommitOwnPasswordChange(_ context.Context, commit OwnPasswordChangeCommit) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	account, exists := repository.accountByUserID(commit.Actor.UserID)
	if !exists || !repository.consumeGrant(commit.Grant) {
		return ErrAuthRepositoryRejected
	}
	account.User.AuthRevision++
	account.User.Version++
	account.User.UpdatedAt = commit.ChangedAt
	account.Credential = commit.Credential
	repository.accounts[account.User.CanonicalLoginName] = account
	repository.revokeUserSessions(account.User.ID, commit.ChangedAt, "password_changed")
	repository.sessions[commit.RotatedSession.ID] = ResolvedSession{User: account.User, Session: commit.RotatedSession}
	repository.audits = append(repository.audits, commit.Audit)
	return nil
}

func (repository *fakeAuthRepository) CommitUserCreate(_ context.Context, commit UserCreateCommit) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if commit.Actor.Role != RoleAdministrator || !repository.consumeGrant(commit.Grant) {
		return ErrAuthRepositoryRejected
	}
	if _, exists := repository.accounts[commit.User.CanonicalLoginName]; exists {
		return ErrAuthRepositoryLoginNameConflict
	}
	repository.accounts[commit.User.CanonicalLoginName] = LoginAccount{User: commit.User, Credential: commit.Credential}
	repository.audits = append(repository.audits, commit.Audit)
	return nil
}

func (repository *fakeAuthRepository) CommitUserUpdate(_ context.Context, commit UserUpdateCommit) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if commit.Actor.Role != RoleAdministrator || !repository.consumeGrant(commit.Grant) {
		return ErrAuthRepositoryRejected
	}
	stored, exists := repository.accountByUserID(commit.User.ID)
	if !exists {
		return ErrAuthRepositoryNotFound
	}
	if stored.User.Version != commit.ExpectedVersion {
		return ErrAuthRepositoryStaleVersion
	}
	if stored.User.Role == RoleAdministrator && stored.User.Enabled &&
		(commit.User.Role != RoleAdministrator || !commit.User.Enabled) && repository.enabledAdministratorCount() == 1 {
		return ErrAuthRepositoryLastAdministrator
	}
	if conflicting, exists := repository.accounts[commit.User.CanonicalLoginName]; exists && conflicting.User.ID != commit.User.ID {
		return ErrAuthRepositoryLoginNameConflict
	}
	oldAuthRevision := stored.User.AuthRevision
	delete(repository.accounts, stored.User.CanonicalLoginName)
	stored.User = commit.User
	repository.accounts[commit.User.CanonicalLoginName] = stored
	if commit.User.AuthRevision != oldAuthRevision {
		repository.revokeUserSessions(commit.User.ID, commit.User.UpdatedAt, "account_changed")
	}
	repository.audits = append(repository.audits, commit.Audit)
	return nil
}

func (repository *fakeAuthRepository) CommitUserDelete(_ context.Context, commit UserDeleteCommit) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if commit.Actor.Role != RoleAdministrator || !repository.consumeGrant(commit.Grant) {
		return ErrAuthRepositoryRejected
	}
	account, exists := repository.accountByUserID(commit.UserID)
	if !exists || account.User.DeletedAt != nil {
		return ErrAuthRepositoryNotFound
	}
	if account.User.Version != commit.ExpectedVersion {
		return ErrAuthRepositoryStaleVersion
	}
	if account.User.Role == RoleAdministrator && account.User.Enabled && repository.enabledAdministratorCount() == 1 {
		return ErrAuthRepositoryLastAdministrator
	}
	account.User.Enabled = false
	account.User.DisabledAt = &commit.DeletedAt
	account.User.DeletedAt = &commit.DeletedAt
	account.User.AuthRevision++
	account.User.Version++
	account.User.UpdatedAt = commit.DeletedAt
	repository.accounts[account.User.CanonicalLoginName] = account
	repository.revokeUserSessions(account.User.ID, commit.DeletedAt, "account_deleted")
	repository.audits = append(repository.audits, commit.Audit)
	return nil
}

func (repository *fakeAuthRepository) CommitUserPasswordReset(_ context.Context, commit UserPasswordResetCommit) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if commit.Actor.Role != RoleAdministrator || !repository.consumeGrant(commit.Grant) {
		return ErrAuthRepositoryRejected
	}
	account, exists := repository.accountByUserID(commit.UserID)
	if !exists {
		return ErrAuthRepositoryNotFound
	}
	if account.User.Version != commit.ExpectedVersion {
		return ErrAuthRepositoryStaleVersion
	}
	account.Credential = commit.Credential
	account.User.AuthRevision++
	account.User.Version++
	account.User.UpdatedAt = commit.ChangedAt
	repository.accounts[account.User.CanonicalLoginName] = account
	repository.revokeUserSessions(account.User.ID, commit.ChangedAt, "password_reset")
	repository.audits = append(repository.audits, commit.Audit)
	return nil
}

func (repository *fakeAuthRepository) CommitUserSessionsRevoke(_ context.Context, commit UserSessionsRevokeCommit) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if commit.Actor.Role != RoleAdministrator || !repository.consumeGrant(commit.Grant) {
		return ErrAuthRepositoryRejected
	}
	if _, exists := repository.accountByUserID(commit.UserID); !exists {
		return ErrAuthRepositoryNotFound
	}
	repository.revokeUserSessions(commit.UserID, commit.RevokedAt, "administrator_revoked")
	repository.audits = append(repository.audits, commit.Audit)
	return nil
}

func (repository *fakeAuthRepository) CommitOperatorRecovery(_ context.Context, commit OperatorRecoveryCommit) (User, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	account := LoginAccount{User: commit.RequestedUser, Credential: commit.Credential}
	repository.accounts[account.User.CanonicalLoginName] = account
	repository.revokeUserSessions(account.User.ID, commit.RecoveredAt, "operator_recovery")
	repository.audits = append(repository.audits, commit.Audit)
	return account.User, nil
}

func (repository *fakeAuthRepository) consumePreauth(use PreauthUse) bool {
	for index, challenge := range repository.preauth {
		if challenge.ConsumedAt == nil && challenge.Purpose == use.Purpose && challenge.DigestKeyID == use.DigestKeyID &&
			challenge.CookieDigest.Equal(use.CookieDigest) && challenge.ProofDigest.Equal(use.ProofDigest) &&
			use.ConsumedAt.Before(challenge.ExpiresAt) {
			repository.preauth[index].ConsumedAt = &use.ConsumedAt
			return true
		}
	}
	return false
}

func (repository *fakeAuthRepository) accountByUserID(id UserID) (LoginAccount, bool) {
	for _, account := range repository.accounts {
		if account.User.ID == id {
			return account, true
		}
	}
	return LoginAccount{}, false
}

func (repository *fakeAuthRepository) consumeGrant(use RecentGrantUse) bool {
	for index, grant := range repository.grants {
		if grant.ConsumedAt == nil && grant.RevokedAt == nil && grant.UserID == use.Actor.UserID &&
			grant.SessionID == use.Actor.SessionID && grant.Purpose == use.Purpose &&
			grant.DigestKeyID == use.DigestKeyID && grant.Digest.Equal(use.Digest) &&
			use.ConsumedAt.Before(grant.ExpiresAt) {
			repository.grants[index].ConsumedAt = &use.ConsumedAt
			return true
		}
	}
	return false
}

func (repository *fakeAuthRepository) enabledAdministratorCount() int {
	count := 0
	for _, account := range repository.accounts {
		if account.User.Role == RoleAdministrator && account.User.Enabled && account.User.DeletedAt == nil {
			count++
		}
	}
	return count
}

func (repository *fakeAuthRepository) revokeUserSessions(userID UserID, at time.Time, reason string) {
	for id, resolved := range repository.sessions {
		if resolved.Session.UserID != userID || resolved.Session.RevokedAt != nil {
			continue
		}
		resolved.Session.RevokedAt = &at
		resolved.Session.RevokeReason = reason
		repository.sessions[id] = resolved
	}
	for index, grant := range repository.grants {
		if grant.UserID != userID || grant.ConsumedAt != nil || grant.RevokedAt != nil {
			continue
		}
		repository.grants[index].RevokedAt = &at
		repository.grants[index].RevokeReason = reason
	}
}

func (repository *fakeAuthRepository) recordThrottle(failure ThrottleFailure) {
	state := repository.throttles[failure.Key]
	if state.BlockedUntil.IsZero() {
		state.BlockedUntil = time.Time{}
	}
	repository.throttles[failure.Key] = state
}

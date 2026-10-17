package httpapi

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/go-chi/chi/v5"
)

const maximumAuthBodyBytes int64 = 8 * 1024

type authHTTP struct {
	service       authenticationBoundary
	publicOrigin  Origin
	secureCookies bool
	readiness     *Readiness
	now           func() time.Time
	bodyLimit     int64
	guestDemo     *GuestDemoOptions
}

func newAuthHTTP(service authenticationBoundary, origin Origin, readiness *Readiness, now func() time.Time, maximumBody int64) authHTTP {
	bodyLimit := min(maximumBody, maximumAuthBodyBytes)
	return authHTTP{
		service: service, publicOrigin: origin, secureCookies: origin.Scheme == "https",
		readiness: readiness, now: now, bodyLimit: bodyLimit,
	}
}

func (handler authHTTP) setupStatus(w http.ResponseWriter, r *http.Request) {
	if !requestHasNoBody(r) || !requestHasNoQuery(r) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	if handler.guestDemo != nil {
		writeJSON(w, http.StatusOK, struct {
			State          string `json:"state"`
			FixtureVersion string `json:"fixture_version"`
			ScenarioStep   int    `json:"scenario_step"`
			ObservedAt     string `json:"observed_at"`
		}{"guest_demo", handler.guestDemo.FixtureVersion, handler.guestDemo.ScenarioStep, authTimestamp(handler.guestDemo.ObservedAt)})
		return
	}
	status, err := handler.service.SetupStatus(r.Context())
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	state := "closed"
	if status.Required {
		state = "required"
	}
	writeJSON(w, http.StatusOK, setupStatusResponse{State: state, BootstrapProofRequired: status.BootstrapProofRequired})
}

func (handler authHTTP) preflight(w http.ResponseWriter, r *http.Request) {
	if !handler.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return
	}
	var request preauthRequest
	if err := decodeJSONBody(r, handler.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	source, ok := authSourceKey(r)
	if !ok {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	issue, err := handler.service.CreatePreauth(r.Context(), request.Purpose, source)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	cookie, cookieErr := issue.Cookie.Encode()
	proof, proofErr := issue.Proof.Encode()
	if cookieErr != nil || proofErr != nil {
		writeProblem(w, r, internalProblem)
		return
	}
	setAuthCookie(w, preauthCookieName, cookie, issue.ExpiresAt, handler.now(), handler.secureCookies, true)
	writeJSON(w, http.StatusCreated, preauthResponse{Proof: proof, ExpiresAt: authTimestamp(issue.ExpiresAt)})
}

func (handler authHTTP) setup(w http.ResponseWriter, r *http.Request) {
	if !handler.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return
	}
	preauthCookie, ok := readCookieValue(r, preauthCookieName)
	if !ok {
		writeProblem(w, r, forbiddenProblem)
		return
	}
	preauthProof, ok := readSingleHeader(r, preauthHeaderName)
	if !ok {
		writeProblem(w, r, forbiddenProblem)
		return
	}
	var request setupRequest
	if err := decodeJSONBody(r, handler.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	defer clearSecret(request.Password)
	defer clearSecret(request.BootstrapProof)
	source, ok := authSourceKey(r)
	if !ok {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	clearAuthCookie(w, preauthCookieName, handler.secureCookies, true)
	issue, err := handler.service.Setup(r.Context(), auth.SetupInput{
		PreauthCookie: preauthCookie, PreauthProof: preauthProof,
		BootstrapProof: string(request.BootstrapProof), SourceKey: source,
		LoginName: request.LoginName, DisplayName: request.DisplayName,
		Password: request.Password, RequestID: requestIDForAuth(r),
	})
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	handler.writeSession(w, r, http.StatusCreated, issue)
}

func (handler authHTTP) login(w http.ResponseWriter, r *http.Request) {
	if !handler.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return
	}
	preauthCookie, ok := readCookieValue(r, preauthCookieName)
	if !ok {
		writeProblem(w, r, forbiddenProblem)
		return
	}
	preauthProof, ok := readSingleHeader(r, preauthHeaderName)
	if !ok {
		writeProblem(w, r, forbiddenProblem)
		return
	}
	var request loginRequest
	if err := decodeJSONBody(r, handler.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	defer clearSecret(request.Password)
	source, ok := authSourceKey(r)
	if !ok {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	clearAuthCookie(w, preauthCookieName, handler.secureCookies, true)
	issue, err := handler.service.Login(r.Context(), auth.LoginInput{
		PreauthCookie: preauthCookie, PreauthProof: preauthProof, SourceKey: source,
		LoginName: request.LoginName, Password: request.Password, RequestID: requestIDForAuth(r),
	})
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	handler.writeSession(w, r, http.StatusOK, issue)
}

func (handler authHTTP) session(w http.ResponseWriter, r *http.Request) {
	if !requestHasNoBody(r) || !requestHasNoQuery(r) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	sessionToken, ok := readCookieValue(r, sessionCookieName)
	if !ok {
		writeProblem(w, r, unauthenticatedProblem)
		return
	}
	authentication, err := handler.service.Authenticate(r.Context(), sessionToken)
	if err != nil {
		if errors.Is(err, auth.ErrUnauthenticated) {
			clearAuthCookie(w, sessionCookieName, handler.secureCookies, true)
		}
		handler.writeError(w, r, err)
		return
	}
	response, err := authSessionResponse(authentication.Context)
	if err != nil {
		writeProblem(w, r, internalProblem)
		return
	}
	handler.writeCurrentSessionCookie(w, sessionToken, authentication.Context)
	writeJSON(w, http.StatusOK, response)
}

func (handler authHTTP) logout(w http.ResponseWriter, r *http.Request) {
	if !handler.requireSameOrigin(w, r) || !requireNoBodyAndQuery(w, r) {
		return
	}
	sessionToken, csrfProof, _, failure, ok := readAuthenticationMaterial(r, true, false)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	err := handler.service.Logout(r.Context(), auth.LogoutInput{
		SessionToken: sessionToken, CSRFProof: csrfProof, RequestID: requestIDForAuth(r),
	})
	if err != nil {
		handler.writeAuthenticatedError(w, r, sessionToken, err)
		return
	}
	clearAuthCookie(w, sessionCookieName, handler.secureCookies, true)
	writeNoContent(w)
}

func (handler authHTTP) recentGrant(w http.ResponseWriter, r *http.Request) {
	if !handler.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return
	}
	sessionToken, csrfProof, _, failure, ok := readAuthenticationMaterial(r, true, false)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	var request recentGrantRequest
	if err := decodeJSONBody(r, handler.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	defer clearSecret(request.CurrentPassword)
	issue, err := handler.service.CreateRecentGrant(r.Context(), auth.RecentAuthInput{
		SessionToken: sessionToken, CSRFProof: csrfProof, CurrentPassword: request.CurrentPassword,
		Purpose: request.Purpose, RequestID: requestIDForAuth(r),
	})
	if err != nil {
		handler.writeAuthenticatedError(w, r, sessionToken, err)
		return
	}
	proof, err := issue.Proof.Encode()
	if err != nil {
		writeProblem(w, r, internalProblem)
		return
	}
	handler.renewSessionCookie(w, r, sessionToken)
	writeJSON(w, http.StatusCreated, recentGrantResponse{
		Proof: proof, Purpose: issue.Purpose, ExpiresAt: authTimestamp(issue.ExpiresAt),
	})
}

func (handler authHTTP) changeOwnPassword(w http.ResponseWriter, r *http.Request) {
	if !handler.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return
	}
	sessionToken, csrfProof, recentProof, failure, ok := readAuthenticationMaterial(r, true, true)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	var request ownPasswordRequest
	if err := decodeJSONBody(r, handler.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	defer clearSecret(request.NewPassword)
	issue, err := handler.service.ChangeOwnPassword(r.Context(), auth.OwnPasswordChangeInput{
		SessionToken: sessionToken, CSRFProof: csrfProof, RecentProof: recentProof,
		NewPassword: request.NewPassword, RequestID: requestIDForAuth(r),
	})
	if err != nil {
		handler.writeAuthenticatedError(w, r, sessionToken, err)
		return
	}
	handler.writeSession(w, r, http.StatusOK, issue)
}

func (handler authHTTP) listUsers(w http.ResponseWriter, r *http.Request) {
	if !requestHasNoBody(r) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	sessionToken, ok := readCookieValue(r, sessionCookieName)
	if !ok {
		writeProblem(w, r, unauthenticatedProblem)
		return
	}
	query, err := parseUserListQuery(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	page, err := handler.service.ListUsers(r.Context(), auth.AuthenticatedReadInput{SessionToken: sessionToken}, query)
	if err != nil {
		handler.writeAuthenticatedError(w, r, sessionToken, err)
		return
	}
	items := make([]userResponse, len(page.Items))
	for index := range page.Items {
		items[index] = authUserResponse(page.Items[index])
	}
	handler.renewSessionCookie(w, r, sessionToken)
	writeJSON(w, http.StatusOK, userPageResponse{
		Items: items,
		Page:  pageResponse{Limit: query.Limit, HasMore: page.NextCursor != "", NextCursor: page.NextCursor},
	})
}

func (handler authHTTP) createUser(w http.ResponseWriter, r *http.Request) {
	if !handler.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return
	}
	sessionToken, csrfProof, recentProof, failure, ok := readAuthenticationMaterial(r, true, true)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	var request createUserRequest
	if err := decodeJSONBody(r, handler.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	defer clearSecret(request.Password)
	user, err := handler.service.CreateUser(r.Context(), auth.UserCreateInput{
		SessionToken: sessionToken, CSRFProof: csrfProof, RecentProof: recentProof,
		LoginName: request.LoginName, DisplayName: request.DisplayName, Role: request.Role,
		Password: request.Password, RequestID: requestIDForAuth(r),
	})
	if err != nil {
		handler.writeAuthenticatedError(w, r, sessionToken, err)
		return
	}
	handler.renewSessionCookie(w, r, sessionToken)
	setUserEntityTag(w, user.Version)
	writeJSON(w, http.StatusCreated, authUserResponse(user))
}

func (handler authHTTP) getUser(w http.ResponseWriter, r *http.Request) {
	if !requestHasNoBody(r) || !requestHasNoQuery(r) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	sessionToken, ok := readCookieValue(r, sessionCookieName)
	if !ok {
		writeProblem(w, r, unauthenticatedProblem)
		return
	}
	user, err := handler.service.GetUser(r.Context(), auth.AuthenticatedReadInput{SessionToken: sessionToken}, userIDFromRequest(r))
	if err != nil {
		handler.writeAuthenticatedError(w, r, sessionToken, err)
		return
	}
	handler.renewSessionCookie(w, r, sessionToken)
	setUserEntityTag(w, user.Version)
	writeJSON(w, http.StatusOK, authUserResponse(user))
}

func (handler authHTTP) updateUser(w http.ResponseWriter, r *http.Request) {
	if !handler.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return
	}
	sessionToken, csrfProof, recentProof, failure, ok := readAuthenticationMaterial(r, true, true)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	version, err := parseExpectedVersion(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	var request updateUserRequest
	if err := decodeJSONBody(r, handler.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	user, err := handler.service.UpdateUser(r.Context(), auth.UserUpdateInput{
		SessionToken: sessionToken, CSRFProof: csrfProof, RecentProof: recentProof,
		UserID: userIDFromRequest(r), ExpectedVersion: version, LoginName: request.LoginName,
		DisplayName: request.DisplayName, Role: request.Role, Enabled: request.Enabled,
		RequestID: requestIDForAuth(r),
	})
	if err != nil {
		handler.writeAuthenticatedError(w, r, sessionToken, err)
		return
	}
	handler.renewSessionCookie(w, r, sessionToken)
	setUserEntityTag(w, user.Version)
	writeJSON(w, http.StatusOK, authUserResponse(user))
}

func (handler authHTTP) deleteUser(w http.ResponseWriter, r *http.Request) {
	if !handler.requireSameOrigin(w, r) || !requireNoBodyAndQuery(w, r) {
		return
	}
	sessionToken, csrfProof, recentProof, failure, ok := readAuthenticationMaterial(r, true, true)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	version, err := parseExpectedVersion(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	err = handler.service.DeleteUser(r.Context(), auth.UserDeleteInput{
		SessionToken: sessionToken, CSRFProof: csrfProof, RecentProof: recentProof,
		UserID: userIDFromRequest(r), ExpectedVersion: version, RequestID: requestIDForAuth(r),
	})
	if err != nil {
		handler.writeAuthenticatedError(w, r, sessionToken, err)
		return
	}
	handler.renewSessionCookie(w, r, sessionToken)
	writeNoContent(w)
}

func (handler authHTTP) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	if !handler.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return
	}
	sessionToken, csrfProof, recentProof, failure, ok := readAuthenticationMaterial(r, true, true)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	version, err := parseExpectedVersion(r)
	if err != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	var request resetPasswordRequest
	if err := decodeJSONBody(r, handler.bodyLimit, &request); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	defer clearSecret(request.NewPassword)
	err = handler.service.ResetUserPassword(r.Context(), auth.UserPasswordResetInput{
		SessionToken: sessionToken, CSRFProof: csrfProof, RecentProof: recentProof,
		UserID: userIDFromRequest(r), ExpectedVersion: version, NewPassword: request.NewPassword,
		RequestID: requestIDForAuth(r),
	})
	if err != nil {
		handler.writeAuthenticatedError(w, r, sessionToken, err)
		return
	}
	handler.renewSessionCookie(w, r, sessionToken)
	writeNoContent(w)
}

func (handler authHTTP) revokeUserSessions(w http.ResponseWriter, r *http.Request) {
	if !handler.requireSameOrigin(w, r) || !requireNoBodyAndQuery(w, r) {
		return
	}
	sessionToken, csrfProof, recentProof, failure, ok := readAuthenticationMaterial(r, true, true)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	err := handler.service.RevokeUserSessions(r.Context(), auth.UserSessionsRevokeInput{
		SessionToken: sessionToken, CSRFProof: csrfProof, RecentProof: recentProof,
		UserID: userIDFromRequest(r), RequestID: requestIDForAuth(r),
	})
	if err != nil {
		handler.writeAuthenticatedError(w, r, sessionToken, err)
		return
	}
	handler.renewSessionCookie(w, r, sessionToken)
	writeNoContent(w)
}

func (handler authHTTP) systemReadiness(w http.ResponseWriter, r *http.Request) {
	sessionToken, ok := readCookieValue(r, sessionCookieName)
	if !ok {
		writeProblem(w, r, unauthenticatedProblem)
		return
	}
	authentication, err := handler.service.Authenticate(r.Context(), sessionToken)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	handler.writeCurrentSessionCookie(w, sessionToken, authentication.Context)
	if authentication.User.Role != auth.RoleAdministrator {
		writeProblem(w, r, forbiddenProblem)
		return
	}
	if !requestHasNoBody(r) || !requestHasNoQuery(r) {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	writeJSON(w, http.StatusOK, handler.readiness.Report())
}

func (handler authHTTP) requireSameOrigin(w http.ResponseWriter, r *http.Request) bool {
	if !validSameOrigin(r, handler.publicOrigin) {
		writeProblem(w, r, forbiddenProblem)
		return false
	}
	return true
}

func (handler authHTTP) writeSession(w http.ResponseWriter, r *http.Request, status int, issue auth.SessionIssue) {
	token, tokenErr := issue.Token.Encode()
	response, responseErr := authSessionResponse(issue.Context)
	if tokenErr != nil || responseErr != nil {
		writeProblem(w, r, internalProblem)
		return
	}
	handler.writeCurrentSessionCookie(w, token, issue.Context)
	writeJSON(w, status, response)
}

func (handler authHTTP) renewSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	authentication, err := handler.service.Authenticate(r.Context(), token)
	if err != nil {
		if errors.Is(err, auth.ErrUnauthenticated) {
			clearAuthCookie(w, sessionCookieName, handler.secureCookies, true)
		}
		return
	}
	handler.writeCurrentSessionCookie(w, token, authentication.Context)
}

func (handler authHTTP) writeCurrentSessionCookie(w http.ResponseWriter, token string, contextValue auth.SessionContext) {
	if _, err := authSessionResponse(contextValue); err != nil {
		return
	}
	expiresAt := contextValue.IdleExpiresAt
	if contextValue.AbsoluteExpiresAt.Before(expiresAt) {
		expiresAt = contextValue.AbsoluteExpiresAt
	}
	setAuthCookie(w, sessionCookieName, token, expiresAt, handler.now(), handler.secureCookies, true)
}

func (handler authHTTP) writeAuthenticatedError(w http.ResponseWriter, r *http.Request, token string, err error) {
	handler.renewSessionCookie(w, r, token)
	handler.writeError(w, r, err)
}

func (handler authHTTP) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidServiceInput):
		writeProblem(w, r, invalidRequestProblem)
	case errors.Is(err, auth.ErrLoginRejected):
		writeProblem(w, r, invalidCredentialsProblem)
	case errors.Is(err, auth.ErrUnauthenticated):
		writeProblem(w, r, unauthenticatedProblem)
	case errors.Is(err, auth.ErrCSRFRejected), errors.Is(err, auth.ErrAuthorizationDenied):
		writeProblem(w, r, forbiddenProblem)
	case errors.Is(err, auth.ErrRecentAuthRejected):
		writeProblem(w, r, recentAuthProblem)
	case errors.Is(err, auth.ErrSetupClosed):
		writeProblem(w, r, setupClosedProblem)
	case errors.Is(err, auth.ErrAuthNotFound):
		writeProblem(w, r, notFoundProblem)
	case errors.Is(err, auth.ErrAuthRateLimited):
		handler.setRetryAfter(w, err)
		writeProblem(w, r, rateLimitProblem)
	case errors.Is(err, auth.ErrAuthConflict):
		writeProblem(w, r, authConflictProblem(err))
	case errors.Is(err, auth.ErrAuthUnavailable):
		writeProblem(w, r, unavailableProblem)
	default:
		writeProblem(w, r, internalProblem)
	}
}

func (handler authHTTP) setRetryAfter(w http.ResponseWriter, err error) {
	type retryAtError interface{ RetryAt() time.Time }
	var retry retryAtError
	if !errors.As(err, &retry) {
		return
	}
	seconds := int(math.Ceil(retry.RetryAt().Sub(handler.now()).Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}

func authConflictProblem(err error) problemKind {
	type codedError interface{ Code() string }
	var coded codedError
	if !errors.As(err, &coded) {
		return conflictProblem
	}
	switch coded.Code() {
	case string(auth.ConflictCodeLastAdministrator), string(auth.ConflictCodeStaleVersion), string(auth.ConflictCodeLoginNameConflict):
		return problemKind{status: http.StatusConflict, code: coded.Code(), title: conflictProblem.title}
	default:
		return conflictProblem
	}
}

func userIDFromRequest(r *http.Request) auth.UserID {
	return auth.UserID(chi.URLParam(r, "user_id"))
}

func requestHasNoBody(r *http.Request) bool {
	return r != nil && r.ContentLength == 0 && len(r.TransferEncoding) == 0
}

func requestHasNoQuery(r *http.Request) bool {
	return r != nil && r.URL.RawQuery == ""
}

func requireNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if !requestHasNoQuery(r) {
		writeProblem(w, r, invalidRequestProblem)
		return false
	}
	return true
}

func requireNoBody(w http.ResponseWriter, r *http.Request) bool {
	if !requestHasNoBody(r) {
		writeProblem(w, r, invalidRequestProblem)
		return false
	}
	return true
}

func requireNoBodyAndQuery(w http.ResponseWriter, r *http.Request) bool {
	if !requestHasNoBody(r) || !requestHasNoQuery(r) {
		writeProblem(w, r, invalidRequestProblem)
		return false
	}
	return true
}

func writeNoContent(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

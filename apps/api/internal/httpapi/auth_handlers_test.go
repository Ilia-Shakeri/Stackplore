package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
)

const testOpaqueProof = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func TestSetupStatusAndPreflightRoutes(t *testing.T) {
	issuedAt := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	token, err := auth.ParseOpaqueToken(testOpaqueProof)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	service := &testAuthentication{
		setupStatusFunc: func(context.Context) (auth.SetupStatus, error) {
			return auth.SetupStatus{Required: true, BootstrapProofRequired: true}, nil
		},
		createPreauthFunc: func(_ context.Context, purpose auth.PreauthPurpose, source string) (auth.PreauthIssue, error) {
			called = true
			if purpose != auth.PreauthPurposeSetup || source != "198.51.100.10" {
				t.Fatalf("preflight input = (%q, %q)", purpose, source)
			}
			return auth.PreauthIssue{Cookie: token, Proof: token, ExpiresAt: issuedAt.Add(auth.PreauthLifetime)}, nil
		},
	}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = service
		options.now = func() time.Time { return issuedAt }
	})

	statusResponse := serve(handler, makeRequest(http.MethodGet, "/api/v1/setup/status", "198.51.100.10:4000"))
	if statusResponse.Code != http.StatusOK || !strings.Contains(statusResponse.Body.String(), `"state":"required"`) || !strings.Contains(statusResponse.Body.String(), `"bootstrap_proof_required":true`) {
		t.Fatal("setup status invariant failed")
	}

	request := authJSONRequest(http.MethodPost, "/api/v1/auth/preflight", `{"purpose":"setup"}`)
	response := serve(handler, request)
	if response.Code != http.StatusCreated || !called {
		t.Fatal("preflight invariant failed")
	}
	if !strings.Contains(response.Body.String(), testOpaqueProof) {
		t.Fatal("preflight body invariant failed")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != preauthCookieName || !cookies[0].HttpOnly || cookies[0].Path != authCookiePath || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("preflight cookies invariant failed")
	}

	called = false
	badOrigin := authJSONRequest(http.MethodPost, "/api/v1/auth/preflight", `{"purpose":"setup"}`)
	badOrigin.Header.Set("Origin", "http://other.test")
	badResponse := serve(handler, badOrigin)
	if badResponse.Code != http.StatusForbidden || called {
		t.Fatalf("bad origin = (%d, called=%t)", badResponse.Code, called)
	}
}

func TestLoginConsumesPreauthAndIssuesSession(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	issue := testSessionIssue(t, now, auth.RoleAdministrator)
	service := &testAuthentication{
		loginFunc: func(_ context.Context, input auth.LoginInput) (auth.SessionIssue, error) {
			if input.PreauthCookie != testOpaqueProof || input.PreauthProof != testOpaqueProof || input.SourceKey != "198.51.100.10" || input.LoginName != "operator" || string(input.Password) != "correct-horse-demo" || input.RequestID != "req_test" {
				t.Fatal("login input invariant failed")
			}
			return issue, nil
		},
	}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = service
		options.now = func() time.Time { return now }
	})
	request := authJSONRequest(http.MethodPost, "/api/v1/auth/login", `{"login_name":"operator","password":"correct-horse-demo"}`)
	request.Header.Set(preauthHeaderName, testOpaqueProof)
	request.AddCookie(&http.Cookie{Name: preauthCookieName, Value: testOpaqueProof})
	response := serve(handler, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "correct-horse-demo") {
		t.Fatal("login response invariant failed")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatal("login cookie count invariant failed")
	}
	foundClear := false
	foundSession := false
	for _, cookie := range cookies {
		if cookie.Name == preauthCookieName && cookie.MaxAge == -1 {
			foundClear = true
		}
		if cookie.Name == sessionCookieName && cookie.Value == testOpaqueProof && cookie.HttpOnly {
			foundSession = true
		}
	}
	if !foundClear || !foundSession {
		t.Fatal("login cookies invariant failed")
	}
}

func TestSessionReadRenewsCookieToCurrentServerExpiry(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	issue := testSessionIssue(t, now, auth.RoleAdministrator)
	service := &testAuthentication{
		authenticateFunc: func(_ context.Context, token string) (auth.Authentication, error) {
			if token != testOpaqueProof {
				t.Fatal("session token invariant failed")
			}
			return auth.Authentication{User: issue.Context.User, Context: issue.Context}, nil
		},
	}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = service
		options.now = func() time.Time { return now }
	})
	request := makeRequest(http.MethodGet, "/api/v1/auth/session", "198.51.100.10:4000")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: testOpaqueProof})
	response := serve(handler, request)
	if response.Code != http.StatusOK {
		t.Fatal("session response invariant failed")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName || cookies[0].Value != testOpaqueProof || !cookies[0].Expires.Equal(issue.Context.IdleExpiresAt) {
		t.Fatal("renewed session cookie invariant failed")
	}
}

func TestUserMutationForwardsGuardsAndReturnsEntityTag(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	want := testUser(now, auth.RoleViewer)
	want.Version = 8
	service := &testAuthentication{
		updateUserFunc: func(_ context.Context, input auth.UserUpdateInput) (auth.User, error) {
			if input.SessionToken != testOpaqueProof || input.CSRFProof != testOpaqueProof || input.RecentProof != testOpaqueProof || input.UserID != want.ID || input.ExpectedVersion != 7 || input.DisplayName == nil || *input.DisplayName != "Read Only" || input.RequestID != "req_test" {
				t.Fatal("update input invariant failed")
			}
			return want, nil
		},
	}
	handler, _, _ := makeHandler(t, func(options *Options) { options.Authentication = service })
	request := authJSONRequest(http.MethodPatch, "/api/v1/users/"+string(want.ID), `{"display_name":"Read Only"}`)
	request.Header.Set(csrfHeaderName, testOpaqueProof)
	request.Header.Set(recentAuthHeaderName, testOpaqueProof)
	request.Header.Set("If-Match", `"7"`)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: testOpaqueProof})
	response := serve(handler, request)
	if response.Code != http.StatusOK || response.Header().Get("ETag") != `"8"` {
		t.Fatal("update response invariant failed")
	}
	var decoded userResponse
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil || decoded.ID != want.ID || decoded.Version != 8 {
		t.Fatal("update body invariant failed")
	}
}

func TestReadinessUsesLiveRole(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	role := auth.RoleViewer
	service := &testAuthentication{
		authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			return auth.Authentication{User: testUser(now, role)}, nil
		},
	}
	handler, readiness, _ := makeHandler(t, func(options *Options) { options.Authentication = service })
	readiness.SetDatabaseReady(true)
	readiness.SetServiceReady(true)
	request := makeRequest(http.MethodGet, "/api/v1/system/readiness", "198.51.100.10:4000")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: testOpaqueProof})
	if response := serve(handler, request); response.Code != http.StatusForbidden {
		t.Fatalf("viewer readiness = %d", response.Code)
	}
	role = auth.RoleAdministrator
	response := serve(handler, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"state":"ready"`) {
		t.Fatal("administrator readiness invariant failed")
	}
}

func TestAuthErrorsStayTypedAndBounded(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	service := &testAuthentication{
		deleteUserFunc: func(context.Context, auth.UserDeleteInput) error { return auth.ErrLastAdministrator },
		createPreauthFunc: func(context.Context, auth.PreauthPurpose, string) (auth.PreauthIssue, error) {
			return auth.PreauthIssue{}, testRetryError{until: now.Add(91 * time.Second)}
		},
	}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = service
		options.now = func() time.Time { return now }
	})

	deleteRequest := makeRequest(http.MethodDelete, "/api/v1/users/usr_target", "198.51.100.10:4000")
	deleteRequest.Header.Set("Origin", "http://stackplore.test")
	deleteRequest.Header.Set(csrfHeaderName, testOpaqueProof)
	deleteRequest.Header.Set(recentAuthHeaderName, testOpaqueProof)
	deleteRequest.Header.Set("If-Match", `"1"`)
	deleteRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: testOpaqueProof})
	deleteResponse := serve(handler, deleteRequest)
	if deleteResponse.Code != http.StatusConflict || decodeProblem(t, deleteResponse).Code != string(auth.ConflictCodeLastAdministrator) {
		t.Fatal("delete response invariant failed")
	}

	preflightRequest := authJSONRequest(http.MethodPost, "/api/v1/auth/preflight", `{"purpose":"login"}`)
	preflightResponse := serve(handler, preflightRequest)
	if preflightResponse.Code != http.StatusTooManyRequests || preflightResponse.Header().Get("Retry-After") != "91" || decodeProblem(t, preflightResponse).Code != rateLimitProblem.code {
		t.Fatal("rate response invariant failed")
	}
}

func TestAuthenticatedFailureRenewsSessionCookie(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	issue := testSessionIssue(t, now, auth.RoleAdministrator)
	service := &testAuthentication{
		deleteUserFunc: func(context.Context, auth.UserDeleteInput) error { return auth.ErrLastAdministrator },
		authenticateFunc: func(_ context.Context, token string) (auth.Authentication, error) {
			if token != testOpaqueProof {
				t.Fatal("session token invariant failed")
			}
			return auth.Authentication{User: issue.Context.User, Context: issue.Context}, nil
		},
	}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = service
		options.now = func() time.Time { return now }
	})
	request := makeRequest(http.MethodDelete, "/api/v1/users/usr_target", "198.51.100.10:4000")
	request.Header.Set("Origin", "http://stackplore.test")
	request.Header.Set(csrfHeaderName, testOpaqueProof)
	request.Header.Set(recentAuthHeaderName, testOpaqueProof)
	request.Header.Set("If-Match", `"1"`)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: testOpaqueProof})
	response := serve(handler, request)
	if response.Code != http.StatusConflict {
		t.Fatal("delete response invariant failed")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName || cookies[0].Value != testOpaqueProof || !cookies[0].Expires.Equal(issue.Context.IdleExpiresAt) {
		t.Fatal("renewed failure cookie invariant failed")
	}
}

func TestLogoutReturnsEmptyNoContent(t *testing.T) {
	service := &testAuthentication{
		logoutFunc: func(_ context.Context, input auth.LogoutInput) error {
			if input.SessionToken != testOpaqueProof || input.CSRFProof != testOpaqueProof {
				t.Fatal("logout input invariant failed")
			}
			return nil
		},
	}
	handler, _, _ := makeHandler(t, func(options *Options) { options.Authentication = service })
	request := makeRequest(http.MethodPost, "/api/v1/auth/logout", "198.51.100.10:4000")
	request.Header.Set("Origin", "http://stackplore.test")
	request.Header.Set(csrfHeaderName, testOpaqueProof)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: testOpaqueProof})
	response := serve(handler, request)
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 || response.Header().Get("Content-Type") != "" {
		t.Fatal("logout response invariant failed")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName || cookies[0].MaxAge != -1 {
		t.Fatal("logout cookies invariant failed")
	}
}

func TestProtectedProofFailuresUseExactClass(t *testing.T) {
	handler, _, _ := makeHandler(t, nil)
	logout := makeRequest(http.MethodPost, "/api/v1/auth/logout", "198.51.100.10:4000")
	logout.Header.Set("Origin", "http://stackplore.test")
	logout.AddCookie(&http.Cookie{Name: sessionCookieName, Value: testOpaqueProof})
	logoutResponse := serve(handler, logout)
	if logoutResponse.Code != http.StatusForbidden || decodeProblem(t, logoutResponse).Code != forbiddenProblem.code {
		t.Fatal("missing CSRF response invariant failed")
	}

	mutation := authJSONRequest(http.MethodPost, "/api/v1/users", `{"login_name":"viewer","display_name":"Viewer","role":"viewer","password":"correct-horse-viewer"}`)
	mutation.Header.Set(csrfHeaderName, testOpaqueProof)
	mutation.AddCookie(&http.Cookie{Name: sessionCookieName, Value: testOpaqueProof})
	mutationResponse := serve(handler, mutation)
	if mutationResponse.Code != http.StatusForbidden || decodeProblem(t, mutationResponse).Code != recentAuthProblem.code {
		t.Fatal("missing recent proof response invariant failed")
	}
}

func TestAuthRouteMethodAllowSets(t *testing.T) {
	handler, _, _ := makeHandler(t, nil)
	tests := []struct {
		path  string
		allow string
	}{
		{"/api/v1/setup", http.MethodPost},
		{"/api/v1/auth/password", http.MethodPatch},
		{"/api/v1/users", "GET, POST"},
		{"/api/v1/users/usr_test", "GET, PATCH, DELETE"},
		{"/api/v1/users/usr_test/password-reset", http.MethodPost},
		{"/api/v1/users/usr_test/sessions", http.MethodDelete},
	}
	for _, test := range tests {
		response := serve(handler, makeRequest("BREW", test.path, "198.51.100.10:4000"))
		if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != test.allow {
			t.Errorf("%s response = (%d, %q); want (405, %q)", test.path, response.Code, response.Header().Get("Allow"), test.allow)
		}
	}
}

type testRetryError struct{ until time.Time }

func (err testRetryError) Error() string      { return auth.ErrAuthRateLimited.Error() }
func (err testRetryError) Unwrap() error      { return auth.ErrAuthRateLimited }
func (err testRetryError) RetryAt() time.Time { return err.until }

func authJSONRequest(method string, path string, body string) *http.Request {
	request := httptest.NewRequest(method, "http://stackplore.test"+path, strings.NewReader(body))
	request.RemoteAddr = "198.51.100.10:4000"
	request.Header.Set(requestIDHeader, "req_test")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://stackplore.test")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	return request
}

func testSessionIssue(t *testing.T, now time.Time, role auth.Role) auth.SessionIssue {
	t.Helper()
	token, err := auth.ParseOpaqueToken(testOpaqueProof)
	if err != nil {
		t.Fatal(err)
	}
	return auth.SessionIssue{
		Token: token,
		Context: auth.SessionContext{
			User: testUser(now, role), CSRFProof: token,
			IdleExpiresAt: now.Add(auth.SessionIdleLifetime), AbsoluteExpiresAt: now.Add(auth.SessionAbsoluteLife),
		},
	}
}

func testUser(now time.Time, role auth.Role) auth.User {
	return auth.User{
		ID: "usr_test", LoginName: "operator", CanonicalLoginName: "operator",
		DisplayName: "Operator", Role: role, Enabled: true, AuthRevision: 1,
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}
}

var _ error = testRetryError{}

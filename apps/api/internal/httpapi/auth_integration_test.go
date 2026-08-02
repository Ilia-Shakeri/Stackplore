package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/storage"
)

func TestAuthenticationHTTPStorageFlowKeepsRawProofsOutOfData(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "data")
	store, err := storage.Open(context.Background(), storage.Options{
		DataDirectory: dataDirectory,
		BusyTimeout:   time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = store.Close()
		}
	})
	keyID, key, err := store.AuthDigestKey()
	if err != nil {
		t.Fatal(err)
	}
	hasher, err := auth.NewPasswordHasher(auth.DefaultPasswordPolicy())
	if err != nil {
		t.Fatal(err)
	}
	bootstrapProof := testOpaqueProof
	bootstrapGate, err := auth.NewBootstrapGate(bootstrapProof, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	service, err := auth.NewService(auth.ServiceOptions{
		Repository: store, PasswordHasher: hasher, DigestKey: key, DigestKeyID: keyID,
		BootstrapRequired: true, BootstrapGate: bootstrapGate,
	})
	clear(key)
	if err != nil {
		t.Fatal(err)
	}
	handler, _, logs := makeHandler(t, func(options *Options) { options.Authentication = service })
	password := "flow-password-value-27"
	viewerPassword := "viewer-password-value-28"

	setupPreauth := integrationPreauth(t, handler, auth.PreauthPurposeSetup)
	setupRequest := authJSONRequest(http.MethodPost, "/api/v1/setup", `{"login_name":"operator","display_name":"Operations","password":"`+password+`","bootstrap_proof":"`+bootstrapProof+`"}`)
	setupRequest.Header.Set(preauthHeaderName, setupPreauth.proof)
	setupRequest.AddCookie(setupPreauth.cookie)
	setupResponse := serve(handler, setupRequest)
	if setupResponse.Code != http.StatusCreated {
		t.Fatalf("setup response = (%d, %q)", setupResponse.Code, setupResponse.Body.String())
	}
	sessionCookie := integrationCookie(t, setupResponse, sessionCookieName)
	var session sessionResponse
	if err := json.Unmarshal(setupResponse.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if session.User.Role != auth.RoleAdministrator || session.CSRFProof == "" {
		t.Fatalf("setup session = %#v", session)
	}

	statusResponse := serve(handler, makeRequest(http.MethodGet, "/api/v1/setup/status", "198.51.100.10:4000"))
	if statusResponse.Code != http.StatusOK || !bytes.Contains(statusResponse.Body.Bytes(), []byte(`"state":"closed"`)) {
		t.Fatalf("closed setup status = (%d, %q)", statusResponse.Code, statusResponse.Body.String())
	}

	recentRequest := authJSONRequest(http.MethodPost, "/api/v1/auth/recent-grants", `{"current_password":"`+password+`","purpose":"user.create"}`)
	recentRequest.Header.Set(csrfHeaderName, session.CSRFProof)
	recentRequest.AddCookie(sessionCookie)
	recentResponse := serve(handler, recentRequest)
	if recentResponse.Code != http.StatusCreated {
		t.Fatalf("recent response = (%d, %q)", recentResponse.Code, recentResponse.Body.String())
	}
	var recent recentGrantResponse
	if err := json.Unmarshal(recentResponse.Body.Bytes(), &recent); err != nil {
		t.Fatal(err)
	}
	createRequest := authJSONRequest(http.MethodPost, "/api/v1/users", `{"login_name":"viewer","display_name":"Viewer","role":"viewer","password":"`+viewerPassword+`"}`)
	createRequest.Header.Set(csrfHeaderName, session.CSRFProof)
	createRequest.Header.Set(recentAuthHeaderName, recent.Proof)
	createRequest.AddCookie(sessionCookie)
	createResponse := serve(handler, createRequest)
	if createResponse.Code != http.StatusCreated || createResponse.Header().Get("ETag") != `"1"` {
		t.Fatalf("create user response = (%d, %q, %q)", createResponse.Code, createResponse.Header().Get("ETag"), createResponse.Body.String())
	}

	logoutRequest := makeRequest(http.MethodPost, "/api/v1/auth/logout", "198.51.100.10:4000")
	logoutRequest.Header.Set("Origin", "http://stackplore.test")
	logoutRequest.Header.Set(csrfHeaderName, session.CSRFProof)
	logoutRequest.AddCookie(sessionCookie)
	if response := serve(handler, logoutRequest); response.Code != http.StatusNoContent {
		t.Fatalf("logout response = (%d, %q)", response.Code, response.Body.String())
	}

	loginPreauth := integrationPreauth(t, handler, auth.PreauthPurposeLogin)
	loginRequest := authJSONRequest(http.MethodPost, "/api/v1/auth/login", `{"login_name":"operator","password":"`+password+`"}`)
	loginRequest.Header.Set(preauthHeaderName, loginPreauth.proof)
	loginRequest.AddCookie(loginPreauth.cookie)
	loginResponse := serve(handler, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login response = (%d, %q)", loginResponse.Code, loginResponse.Body.String())
	}
	newSessionCookie := integrationCookie(t, loginResponse, sessionCookieName)
	currentRequest := makeRequest(http.MethodGet, "/api/v1/auth/session", "198.51.100.10:4000")
	currentRequest.AddCookie(newSessionCookie)
	currentResponse := serve(handler, currentRequest)
	if currentResponse.Code != http.StatusOK {
		t.Fatalf("session response = (%d, %q)", currentResponse.Code, currentResponse.Body.String())
	}
	var current sessionResponse
	if err := json.Unmarshal(currentResponse.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if len(currentResponse.Result().Cookies()) != 1 {
		t.Fatalf("session renewal cookies = %#v", currentResponse.Result().Cookies())
	}
	if current.CSRFProof == "" {
		t.Fatalf("session response = (%d, %q)", currentResponse.Code, currentResponse.Body.String())
	}

	protectedValues := [][]byte{
		[]byte(password), []byte(viewerPassword), []byte(bootstrapProof),
		[]byte(setupPreauth.proof), []byte(setupPreauth.cookie.Value),
		[]byte(loginPreauth.proof), []byte(loginPreauth.cookie.Value),
		[]byte(session.CSRFProof), []byte(current.CSRFProof), []byte(recent.Proof),
		[]byte(sessionCookie.Value), []byte(newSessionCookie.Value),
	}
	for _, protected := range protectedValues {
		if bytes.Contains(logs.Bytes(), protected) {
			t.Fatal("request logs contain protected input")
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	closed = true
	if err := filepath.WalkDir(dataDirectory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		contents, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		for _, protected := range protectedValues {
			if bytes.Contains(contents, protected) {
				t.Fatalf("protected input persisted in %s", filepath.Base(path))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

type integrationPreauthResult struct {
	cookie *http.Cookie
	proof  string
}

func integrationPreauth(t *testing.T, handler http.Handler, purpose auth.PreauthPurpose) integrationPreauthResult {
	t.Helper()
	request := authJSONRequest(http.MethodPost, "/api/v1/auth/preflight", `{"purpose":"`+string(purpose)+`"}`)
	response := serve(handler, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("preflight response = (%d, %q)", response.Code, response.Body.String())
	}
	var body preauthResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return integrationPreauthResult{cookie: integrationCookie(t, response, preauthCookieName), proof: body.Proof}
}

func integrationCookie(t *testing.T, response *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == name && cookie.MaxAge >= 0 {
			return cookie
		}
	}
	t.Fatalf("response has no %s cookie", name)
	return nil
}

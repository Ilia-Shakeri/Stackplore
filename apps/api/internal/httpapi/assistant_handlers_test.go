package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type helperReadStub struct {
	get    func(context.Context, assistant.Actor, string) (assistantservice.Snapshot, error)
	cancel func(context.Context, assistant.Actor, string) (assistant.Run, error)
}

func (stub helperReadStub) Get(ctx context.Context, actor assistant.Actor, id string) (assistantservice.Snapshot, error) {
	return stub.get(ctx, actor, id)
}
func (stub helperReadStub) Cancel(ctx context.Context, actor assistant.Actor, id string) (assistant.Run, error) {
	return stub.cancel(ctx, actor, id)
}

func helperIdentity(t *testing.T) auth.Authentication {
	t.Helper()
	digest, err := auth.ParseSessionTokenDigest(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal("create synthetic actor digest")
	}
	return auth.Authentication{Principal: auth.Principal{UserID: "user_helper", SessionID: "session_helper", Role: auth.RoleViewer, AuthRevision: 1}, Session: auth.Session{ID: "session_helper", UserID: "user_helper", TokenDigest: digest, TokenDigestKeyID: "helper_key", UserAuthRevision: 1}}
}

func helperRequest(method, path string) *http.Request {
	request := makeRequest(method, path, "198.51.100.10:4000")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: testOpaqueProof})
	if method == http.MethodPost {
		request.Header.Set("Origin", "http://stackplore.test")
		request.Header.Set(csrfHeaderName, strings.Repeat("B", 42)+"A")
	}
	return request
}

func TestAssistantReadRoutesAuthorizeActorAndPublishOnlyClosedFields(t *testing.T) {
	identity := helperIdentity(t)
	gets, cancels := 0, 0
	reader := helperReadStub{get: func(ctx context.Context, actor assistant.Actor, id string) (assistantservice.Snapshot, error) {
		gets++
		if ctx == nil || actor.UserID != "user_helper" || actor.DigestKeyID != "helper_key" || actor.AuthRevision != 1 || actor.SessionDigest != [32]byte(bytes.Repeat([]byte{1}, 32)) || id != helperRunID {
			t.Error("trusted actor or target changed")
		}
		return helperSnapshot(), nil
	}, cancel: func(ctx context.Context, actor assistant.Actor, id string) (assistant.Run, error) {
		cancels++
		run := helperSnapshot().Run
		run.State = assistant.Cancelled
		return run, nil
	}}
	handler, _, logs := makeHandler(t, func(options *Options) {
		options.AssistantRuns = reader
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity, nil }, authorizeUnsafeFunc: func(_ context.Context, token, csrf string) (auth.Authentication, error) {
			if token != testOpaqueProof || csrf != strings.Repeat("B", 42)+"A" {
				t.Error("protection material changed")
			}
			return identity, nil
		}}
	})
	base := "/api/v1/assistant/runs/" + helperRunID
	for _, path := range []string{base, base + "/citations/" + helperCitationID} {
		response := serve(handler, helperRequest(http.MethodGet, path))
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Content-Type") != "application/json" {
			t.Fatal("private read failed or was cacheable")
		}
		for _, forbidden := range []string{"internal_profile_marker", "internal_lease_marker", "user_helper", "session_helper", "helper_key", "fingerprint", "session_digest", "configuration_version", "object_id"} {
			if strings.Contains(response.Body.String(), forbidden) {
				t.Fatal("internal metadata entered wire response")
			}
		}
		if path == base && !strings.Contains(response.Body.String(), "Selected resource is available.") {
			t.Fatal("explicit authorized publication omitted answer")
		}
	}
	response := serve(handler, helperRequest(http.MethodPost, base+"/cancel"))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"state":"cancelled"`) || strings.Contains(response.Body.String(), "answer") || gets != 2 || cancels != 1 {
		t.Fatal("owner cancellation failed or released content")
	}
	for _, text := range []string{"Selected resource is available.", testOpaqueProof, "internal_profile_marker", "internal_lease_marker"} {
		if strings.Contains(logs.String(), text) {
			t.Fatal("private material entered access logs")
		}
	}
}

func TestAssistantReadRoutesRejectBeforeCallingUseCase(t *testing.T) {
	identity := helperIdentity(t)
	calls := 0
	reader := helperReadStub{get: func(context.Context, assistant.Actor, string) (assistantservice.Snapshot, error) {
		calls++
		return helperSnapshot(), nil
	}, cancel: func(context.Context, assistant.Actor, string) (assistant.Run, error) {
		calls++
		return helperSnapshot().Run, nil
	}}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.AssistantRuns = reader
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity, nil }, authorizeUnsafeFunc: func(context.Context, string, string) (auth.Authentication, error) { return identity, nil }}
	})
	base := "/api/v1/assistant/runs/" + helperRunID
	cases := map[string]struct {
		request *http.Request
		status  int
	}{
		"no_session":  {makeRequest(http.MethodGet, base, "198.51.100.10:4000"), http.StatusUnauthorized},
		"query":       {helperRequest(http.MethodGet, base+"?question=private"), http.StatusBadRequest},
		"run_id":      {helperRequest(http.MethodGet, "/api/v1/assistant/runs/invalid"), http.StatusBadRequest},
		"citation_id": {helperRequest(http.MethodGet, base+"/citations/invalid"), http.StatusBadRequest},
	}
	noCSRF := helperRequest(http.MethodPost, base+"/cancel")
	noCSRF.Header.Del(csrfHeaderName)
	cases["csrf"] = struct {
		request *http.Request
		status  int
	}{noCSRF, http.StatusForbidden}
	wrongOrigin := helperRequest(http.MethodPost, base+"/cancel")
	wrongOrigin.Header.Set("Origin", "http://other.test")
	cases["origin"] = struct {
		request *http.Request
		status  int
	}{wrongOrigin, http.StatusForbidden}
	body := helperRequest(http.MethodPost, base+"/cancel")
	body.ContentLength = 5
	cases["body"] = struct {
		request *http.Request
		status  int
	}{body, http.StatusBadRequest}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			response := serve(handler, tc.request)
			if response.Code != tc.status || calls != 0 || strings.Contains(response.Body.String(), "private") {
				t.Fatal("rejected request reached use case or exposed input")
			}
		})
	}
}

func TestAssistantReadRoutesSealErrorsAndWrongOwners(t *testing.T) {
	identity := helperIdentity(t)
	for name, tc := range map[string]struct {
		err   error
		owner string
		want  int
	}{
		"missing": {assistant.ErrDenied, "", 404}, "wrong_owner": {nil, "another_user", 404}, "quota": {assistant.ErrQuota, "", 429}, "conflict": {assistant.ErrConflict, "", 409}, "invalid": {assistant.ErrInvalid, "", 400}, "storage": {errors.New("private storage diagnostic"), "", 503},
	} {
		t.Run(name, func(t *testing.T) {
			reader := helperReadStub{get: func(context.Context, assistant.Actor, string) (assistantservice.Snapshot, error) {
				s := helperSnapshot()
				if tc.owner != "" {
					s.Run.OwnerID = tc.owner
				}
				return s, tc.err
			}}
			handler, _, logs := makeHandler(t, func(options *Options) {
				options.AssistantRuns = reader
				options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity, nil }}
			})
			response := serve(handler, helperRequest(http.MethodGet, "/api/v1/assistant/runs/"+helperRunID))
			if response.Code != tc.want || strings.Contains(response.Body.String(), "Selected resource") || strings.Contains(response.Body.String(), "diagnostic") || strings.Contains(logs.String(), "diagnostic") {
				t.Fatal("private denial or diagnostic entered publication")
			}
		})
	}
}

func TestAssistantReadRoutesRejectInconsistentAuthenticatedActor(t *testing.T) {
	for name, mutate := range map[string]func(*auth.Authentication){"digest": func(i *auth.Authentication) { i.Session.TokenDigest = auth.SessionTokenDigest{} }, "session": func(i *auth.Authentication) { i.Session.ID = "other_session" }, "revision": func(i *auth.Authentication) { i.Session.UserAuthRevision++ }, "user": func(i *auth.Authentication) { i.Session.UserID = "other_user" }} {
		t.Run(name, func(t *testing.T) {
			identity := helperIdentity(t)
			mutate(&identity)
			called := false
			reader := helperReadStub{get: func(context.Context, assistant.Actor, string) (assistantservice.Snapshot, error) {
				called = true
				return helperSnapshot(), nil
			}}
			handler, _, _ := makeHandler(t, func(options *Options) {
				options.AssistantRuns = reader
				options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity, nil }}
			})
			response := serve(handler, helperRequest(http.MethodGet, "/api/v1/assistant/runs/"+helperRunID))
			if response.Code != 503 || called {
				t.Fatal("inconsistent actor reached use case")
			}
		})
	}
}

func TestAssistantCitationRequiresRetainedReadyContent(t *testing.T) {
	identity := helperIdentity(t)
	for _, lost := range []bool{false, true} {
		t.Run(map[bool]string{true: "lost", false: "wrong_reference"}[lost], func(t *testing.T) {
			reader := helperReadStub{get: func(context.Context, assistant.Actor, string) (assistantservice.Snapshot, error) {
				s := helperSnapshot()
				if lost {
					s.Volatile = false
					s.Answer = nil
					s.Citations = nil
				}
				return s, nil
			}}
			handler, _, _ := makeHandler(t, func(options *Options) {
				options.AssistantRuns = reader
				options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity, nil }}
			})
			id := helperCitationID
			if !lost {
				id = "ev_" + strings.Repeat("d", 64)
			}
			response := serve(handler, helperRequest(http.MethodGet, "/api/v1/assistant/runs/"+helperRunID+"/citations/"+id))
			if response.Code != 404 {
				t.Fatal("unretained or unknown reference resolved")
			}
		})
	}
}

func TestAssistantReadRoutesAreAbsentByDefaultAndForbiddenInGuestComposition(t *testing.T) {
	handler, _, _ := makeHandler(t, nil)
	response := serve(handler, helperRequest(http.MethodGet, "/api/v1/assistant/runs/"+helperRunID))
	if response.Code != 404 {
		t.Fatal("default composition exposed private route")
	}
	options := testOptions(&bytes.Buffer{})
	options.ProductProviders = []ProductProvider{demo.MustNew()}
	options.GuestDemo = &GuestDemoOptions{FixtureVersion: demo.FixtureVersion, ObservedAt: demo.ObservationTime()}
	if _, err := NewHandler(options); err != nil {
		t.Fatal("baseline guest fixture invalid")
	}
	options.AssistantRuns = helperReadStub{}
	if _, err := NewHandler(options); err == nil {
		t.Fatal("guest composition accepted private helper capability")
	}
}

func TestAssistantMetadataWireHasNoContentFields(t *testing.T) {
	s := helperSnapshot()
	s.Answer = nil
	s.Citations = nil
	s.Volatile = false
	reply, err := assistantReply(s, "user_helper", helperRunID)
	if err != nil {
		t.Fatal("metadata projection failed")
	}
	wire := httptest.NewRecorder()
	writeAssistantReply(wire, reply)
	var data map[string]json.RawMessage
	if json.Unmarshal(wire.Body.Bytes(), &data) != nil || len(data) != 2 || data["run"] == nil || data["content_state"] == nil {
		t.Fatal("metadata-only wire leaked extra fields")
	}
}

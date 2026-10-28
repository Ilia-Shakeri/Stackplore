package httpapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantconversation"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
)

type helperAdmissionStub struct {
	helperReadStub
	submit func(context.Context, assistant.Actor, assistantconversation.Request, string) (assistant.Run, error)
}

func (stub helperAdmissionStub) Submit(ctx context.Context, actor assistant.Actor, request assistantconversation.Request, nonce string) (assistant.Run, error) {
	return stub.submit(ctx, actor, request, nonce)
}

const helperQuestionJSON = `{"source_id":"src_demo_01","resource_id":"r1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","window_minutes":15,"locale":"fa","question":"private_question_marker"}`

func admissionRequest(body string) *http.Request {
	r := helperRequest(http.MethodPost, "/api/v1/assistant/runs")
	r.Body = io.NopCloser(strings.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", strings.Repeat("d", 64))
	return r
}

func TestAssistantAdmissionReturnsOnlyProtectedMatchingReceipt(t *testing.T) {
	identity := helperIdentity(t)
	calls := 0
	reader := helperAdmissionStub{submit: func(ctx context.Context, actor assistant.Actor, input assistantconversation.Request, nonce string) (assistant.Run, error) {
		calls++
		if ctx == nil || actor.UserID != "user_helper" || input.Question != "private_question_marker" || input.Locale != "fa" || nonce != strings.Repeat("d", 64) {
			t.Error("admission fields or trusted actor changed")
		}
		run := helperSnapshot().Run
		run.State = assistant.Queued
		return run, nil
	}}
	handler, _, logs := makeHandler(t, func(options *Options) {
		options.AssistantRuns = reader
		options.Authentication = &testAuthentication{authorizeUnsafeFunc: func(context.Context, string, string) (auth.Authentication, error) { return identity, nil }}
	})
	response := serve(handler, admissionRequest(helperQuestionJSON))
	if response.Code != 202 || response.Header().Get("Cache-Control") != "no-store" || calls != 1 || !strings.Contains(response.Body.String(), `"state":"queued"`) {
		t.Fatal("admission did not return its private metadata receipt")
	}
	for _, private := range []string{"private_question_marker", strings.Repeat("d", 64), "internal_profile_marker", "internal_lease_marker"} {
		if strings.Contains(response.Body.String(), private) || strings.Contains(logs.String(), private) {
			t.Fatal("admission disclosed private input or internal data")
		}
	}
}

func TestAssistantAdmissionRejectsHeaderBodyAndPolicyOverridesBeforeUseCase(t *testing.T) {
	identity := helperIdentity(t)
	calls := 0
	reader := helperAdmissionStub{submit: func(context.Context, assistant.Actor, assistantconversation.Request, string) (assistant.Run, error) {
		calls++
		return helperSnapshot().Run, nil
	}}
	handler, _, logs := makeHandler(t, func(options *Options) {
		options.AssistantRuns = reader
		options.Authentication = &testAuthentication{authorizeUnsafeFunc: func(context.Context, string, string) (auth.Authentication, error) { return identity, nil }}
	})
	cases := map[string]struct {
		request *http.Request
		status  int
	}{
		"unknown":           {admissionRequest(strings.TrimSuffix(helperQuestionJSON, "}") + `,"profile_id":"private_policy_marker"}`), 400},
		"case_alias":        {admissionRequest(strings.Replace(helperQuestionJSON, `"source_id"`, `"SOURCE_ID"`, 1)), 400},
		"bad_unicode":       {admissionRequest(strings.Replace(helperQuestionJSON, `"private_question_marker"`, `"\ud800"`, 1)), 400},
		"null_conversation": {admissionRequest(strings.TrimSuffix(helperQuestionJSON, "}") + `,"conversation_id":null}`), 400},
	}
	missing := admissionRequest(helperQuestionJSON)
	missing.Header.Del("Idempotency-Key")
	cases["missing_nonce"] = struct {
		request *http.Request
		status  int
	}{missing, 400}
	duplicate := admissionRequest(helperQuestionJSON)
	duplicate.Header.Add("Idempotency-Key", strings.Repeat("e", 64))
	cases["duplicate_nonce"] = struct {
		request *http.Request
		status  int
	}{duplicate, 400}
	uppercase := admissionRequest(helperQuestionJSON)
	uppercase.Header.Set("Idempotency-Key", strings.Repeat("D", 64))
	cases["nonce_alias"] = struct {
		request *http.Request
		status  int
	}{uppercase, 400}
	csrf := admissionRequest(helperQuestionJSON)
	csrf.Header.Del(csrfHeaderName)
	cases["csrf"] = struct {
		request *http.Request
		status  int
	}{csrf, 403}
	origin := admissionRequest(helperQuestionJSON)
	origin.Header.Set("Origin", "http://other.test")
	cases["origin"] = struct {
		request *http.Request
		status  int
	}{origin, 403}
	query := admissionRequest(helperQuestionJSON)
	query.URL.RawQuery = "question=private_query_marker"
	cases["query"] = struct {
		request *http.Request
		status  int
	}{query, 400}
	media := admissionRequest(helperQuestionJSON)
	media.Header.Set("Content-Type", "text/plain")
	cases["media"] = struct {
		request *http.Request
		status  int
	}{media, 415}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			response := serve(handler, tc.request)
			if response.Code != tc.status || calls != 0 || strings.Contains(response.Body.String(), "private_") {
				t.Fatal("invalid admission reached runtime or exposed input")
			}
		})
	}
	for _, marker := range []string{"private_question_marker", "private_query_marker", "private_policy_marker"} {
		if strings.Contains(logs.String(), marker) {
			t.Fatal("rejected private input entered captured logs")
		}
	}
}

func TestAssistantAdmissionRejectsMismatchingRuntimeSelection(t *testing.T) {
	identity := helperIdentity(t)
	for name, mutate := range map[string]func(*assistant.Run){"source": func(r *assistant.Run) { r.SourceID = "other_source" }, "resource": func(r *assistant.Run) { r.ResourceID = "r1_" + strings.Repeat("B", 42) + "A" }, "window": func(r *assistant.Run) { r.From = r.Until.Add(-5 * time.Minute) }} {
		t.Run(name, func(t *testing.T) {
			reader := helperAdmissionStub{submit: func(context.Context, assistant.Actor, assistantconversation.Request, string) (assistant.Run, error) {
				run := helperSnapshot().Run
				mutate(&run)
				return run, nil
			}}
			handler, _, _ := makeHandler(t, func(options *Options) {
				options.AssistantRuns = reader
				options.Authentication = &testAuthentication{authorizeUnsafeFunc: func(context.Context, string, string) (auth.Authentication, error) { return identity, nil }}
			})
			response := serve(handler, admissionRequest(helperQuestionJSON))
			if response.Code != 503 || strings.Contains(response.Body.String(), "private_question_marker") {
				t.Fatal("mismatching runtime selection published a receipt")
			}
		})
	}
}

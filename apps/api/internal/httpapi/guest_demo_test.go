package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

func TestGuestDemoHasPublicReadAndClosedWriteBoundaryWithoutAuthentication(t *testing.T) {
	called := false
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.RateLimit.Burst = 512
		options.RateLimit.RequestsPerSecond = 1000
		options.ProductProviders = []ProductProvider{demo.MustNew()}
		options.GuestDemo = &GuestDemoOptions{FixtureVersion: demo.FixtureVersion, ObservedAt: demo.ObservationTime()}
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) {
			called = true
			return auth.Authentication{}, auth.ErrUnauthenticated
		}}
	})
	status := serve(handler, makeRequest(http.MethodGet, "/api/v1/setup/status", "198.51.100.10:4000"))
	var payload struct {
		State          string `json:"state"`
		FixtureVersion string `json:"fixture_version"`
	}
	if status.Code != http.StatusOK || json.Unmarshal(status.Body.Bytes(), &payload) != nil || payload.State != "guest_demo" || payload.FixtureVersion != demo.FixtureVersion || len(status.Result().Cookies()) > 0 {
		t.Fatal("guest discovery invented authentication or failed")
	}
	for _, path := range []string{"/sources", "/resources?limit=20", "/environments", "/topology?limit=20", "/logs?limit=20", "/metrics/current", "/events?limit=20"} {
		response := serve(handler, makeRequest(http.MethodGet, "/api/v1"+path, "198.51.100.10:4000"))
		if response.Code != http.StatusOK || len(response.Result().Cookies()) > 0 {
			t.Fatalf("public fixture read %s status %d", path, response.Code)
		}
	}
	for _, path := range []string{"/setup", "/auth/login", "/auth/logout", "/auth/recent-grants", "/users", "/settings/sources", "/settings/notifications/test", "/alert-rules", "/alerts/alert_fixture/acknowledgement", "/operations"} {
		for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodDelete} {
			response := serve(handler, makeRequest(method, "/api/v1"+path, "198.51.100.10:4000"))
			if response.Code != http.StatusForbidden {
				t.Fatalf("guest write %s %s=%d", method, path, response.Code)
			}
		}
	}
	for _, path := range []string{"/auth/session", "/users", "/audit", "/alert-rules", "/settings/sources", "/settings/notifications", "/system/readiness"} {
		if response := serve(handler, makeRequest(http.MethodGet, "/api/v1"+path, "198.51.100.10:4000")); response.Code != http.StatusForbidden {
			t.Fatalf("guest private route %s=%d", path, response.Code)
		}
	}
	if called {
		t.Fatal("guest read invoked session authentication")
	}
}

func TestGuestStreamBudgetIsIndependentAndBoundedPerPeer(t *testing.T) {
	gate := newStreamGate(3, 2)
	request := makeRequest(http.MethodGet, "/api/v1/streams/events", "198.51.100.10:4000")
	// Source identity comes from validated proxy middleware, as in normal requests.
	_, ok := guestStreamBucket(request)
	if ok {
		t.Fatal("unvalidated peer accepted")
	}
	first, ok := gate.acquireBucket("guest:fixture_peer")
	if !ok {
		t.Fatal("first stream rejected")
	}
	defer first()
	second, ok := gate.acquireBucket("guest:fixture_peer")
	if !ok {
		t.Fatal("second stream rejected")
	}
	if _, ok := gate.acquireBucket("guest:fixture_peer"); ok {
		t.Fatal("guest stream quota bypassed")
	}
	second()
	if release, ok := gate.acquireBucket("guest:fixture_peer"); !ok {
		t.Fatal("guest stream slot leaked")
	} else {
		release()
	}
}

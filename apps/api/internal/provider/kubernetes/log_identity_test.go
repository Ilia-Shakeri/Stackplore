package kubernetes

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestSDKLogReadRequiresSamePodUIDBeforeAndAfterRead(t *testing.T) {
	for _, test := range []struct {
		name         string
		beforeUID    string
		afterUID     string
		afterCode    int
		wantLines    int
		wantLogReads int
		wantState    domain.AvailabilityState
	}{
		{"stable", "pod-one", "pod-one", 200, 1, 1, domain.AvailabilityAvailable},
		{"already_recreated", "pod-two", "pod-two", 200, 0, 0, domain.AvailabilityUnavailable},
		{"recreated_during_read", "pod-one", "pod-two", 200, 0, 1, domain.AvailabilityUnavailable},
		{"permission_lost_after_read", "pod-one", "pod-one", 403, 0, 1, domain.AvailabilityPermissionDenied},
		{"deleted_after_read", "pod-one", "pod-one", 404, 0, 1, domain.AvailabilityUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			identities, logReads := 0, 0
			collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
				switch request.URL.Path {
				case "/api/v1/namespaces/shop/pods/api":
					identities++
					uid := test.beforeUID
					if identities > 1 {
						if test.afterCode != 200 {
							reason := "Forbidden"
							if test.afterCode == 404 {
								reason = "NotFound"
							}
							return jsonResponse(test.afterCode, `{"kind":"Status","reason":"`+reason+`","code":`+strconv.Itoa(test.afterCode)+`}`), nil
						}
						uid = test.afterUID
					}
					return jsonResponse(http.StatusOK, `{"metadata":{"uid":"`+uid+`","namespace":"shop","name":"api"}}`), nil
				case "/api/v1/namespaces/shop/pods/api/log":
					logReads++
					return jsonResponse(http.StatusOK, "2026-08-16T08:30:00Z private-target-line\n"), nil
				default:
					t.Fatal("identity check escaped exact Pod path")
					return nil, ErrUnavailable
				}
			})
			result, err := collector.ReadPodLogs(context.Background(), PodLogRequest{Targets: []PodLogTarget{{Key: "one", ExpectedUID: "pod-one", Namespace: "shop", PodName: "api", Container: "api"}}, Start: time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC), LineLimit: 10, ByteLimit: 1024})
			if err != nil || len(result.Lines) != test.wantLines || logReads != test.wantLogReads || result.Status.State != test.wantState {
				t.Fatalf("identity result state=%s lines=%d reads=%d error=%v", result.Status.State, len(result.Lines), logReads, err)
			}
		})
	}
}

func TestSDKIdentityGuardAllowsOnlyExactPermittedPodRead(t *testing.T) {
	guard := boundedTransport{namespaces: []string{"shop"}}
	if !guard.permits(http.MethodGet, "/api/v1/namespaces/shop/pods/api") {
		t.Fatal("exact Pod identity read blocked")
	}
	for _, path := range []string{"/api/v1/namespaces/other/pods/api", "/api/v1/pods/api", "/api/v1/namespaces/shop/pods/api/status", "/api/v1/namespaces/shop/pods/api/exec", "/api/v1/namespaces/shop/pods/api/proxy", "/api/v1/namespaces/shop/secrets/api"} {
		if guard.permits(http.MethodGet, path) {
			t.Fatal("identity guard opened another read path")
		}
	}
}

package kubernetes

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSDKProjectedTokenRefreshIsThrottledAndFailsClosed(t *testing.T) {
	now := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	reads, requests := 0, 0
	failRead := false
	rotated := false
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		requests++
		expected := "Bearer unit-test-token"
		if rotated {
			expected = "Bearer unit-test-token-rotated"
		}
		if request.Header.Get("Authorization") != expected {
			t.Fatal("stale projected credential used")
		}
		return emptyListTransport(request)
	}, func(options *APICollectorOptions) {
		options.Now = func() time.Time { return now }
		options.TokenReader = func(ctx context.Context) (string, error) {
			reads++
			if failRead {
				return "", errors.New("file unavailable")
			}
			return "unit-test-token-rotated", nil
		}
	})
	read := func() error {
		_, err := collector.core.Pods("shop").List(context.Background(), metav1.ListOptions{})
		return err
	}
	if read() != nil {
		t.Fatal("initial credential failed")
	}
	now = now.Add(59 * time.Second)
	if read() != nil || reads != 0 {
		t.Fatal("token file refreshed before bound")
	}
	now = now.Add(time.Second)
	rotated = true
	if read() != nil || reads != 1 || requests != 3 {
		t.Fatal("projected credential did not rotate at expiry")
	}
	now = now.Add(time.Minute)
	failRead = true
	if err := read(); !errors.Is(mapAPIError(err), ErrPermissionDenied) || requests != 3 {
		t.Fatal("failed refresh reused cached credential")
	}
}

func TestSDKUnauthorizedRefreshRetriesOnlyOneRead(t *testing.T) {
	for _, acceptRefreshed := range []bool{false, true} {
		t.Run(map[bool]string{false: "still_denied", true: "refreshed"}[acceptRefreshed], func(t *testing.T) {
			reads, requests := 0, 0
			collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
				requests++
				if acceptRefreshed && request.Header.Get("Authorization") == "Bearer unit-test-token-rotated" {
					return emptyListTransport(request)
				}
				return jsonResponse(http.StatusUnauthorized, `{"reason":"Unauthorized","code":401}`), nil
			}, func(options *APICollectorOptions) {
				options.TokenReader = func(context.Context) (string, error) { reads++; return "unit-test-token-rotated", nil }
			})
			_, err := collector.core.Pods("shop").List(context.Background(), metav1.ListOptions{})
			if requests != 2 || reads != 1 {
				t.Fatal("unauthorized credential retry escaped one-read bound")
			}
			if acceptRefreshed && err != nil || !acceptRefreshed && !errors.Is(mapAPIError(err), ErrPermissionDenied) {
				t.Fatal("refreshed authorization outcome lost")
			}
		})
	}
	requests, reads := 0, 0
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(http.StatusUnauthorized, `{"reason":"Unauthorized","code":401}`), nil
	}, func(options *APICollectorOptions) {
		options.TokenReader = func(context.Context) (string, error) { reads++; return "unit-test-token-rotated", nil }
	})
	_, err := collector.RestartWorkload(context.Background(), WorkloadOperation{Kind: "Deployment", Namespace: "shop", Name: "api", ResourceVersion: "17", IdempotencyKey: "operation_fixture"})
	if !errors.Is(err, ErrPermissionDenied) || requests != 1 || reads != 0 {
		t.Fatal("unauthorized write was retried")
	}
}

func TestSDKConcurrentProjectedTokenRefreshIsSingleFlightAndCloseRevokesUse(t *testing.T) {
	now := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	var reads, requests atomic.Int64
	collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
		requests.Add(1)
		if request.Header.Get("Authorization") != "Bearer unit-test-token-rotated" {
			t.Error("expired credential used by concurrent read")
		}
		return emptyListTransport(request)
	}, func(options *APICollectorOptions) {
		options.Now = func() time.Time { return now }
		options.TokenReader = func(context.Context) (string, error) { reads.Add(1); return "unit-test-token-rotated", nil }
	})
	now = now.Add(time.Minute)
	var workers sync.WaitGroup
	for index := 0; index < 12; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if _, err := collector.core.Pods("shop").List(context.Background(), metav1.ListOptions{}); err != nil {
				t.Error("concurrent refresh read failed")
			}
		}()
	}
	workers.Wait()
	if reads.Load() != 1 || requests.Load() != 12 {
		t.Fatal("concurrent refresh multiplied token reads")
	}
	provider, _ := New("source_cluster", "Scoped", collector)
	if provider.Close() != nil {
		t.Fatal("provider close failed")
	}
	if _, err := collector.core.Pods("shop").List(context.Background(), metav1.ListOptions{}); err == nil || requests.Load() != 12 {
		t.Fatal("closed collector reused credential or transport")
	}
}

func TestSDKAllDeniedNeverDowngradesAuthorizationToOutage(t *testing.T) {
	for _, code := range []int{http.StatusForbidden, http.StatusUnauthorized} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			collector := testAPICollector(t, []string{"shop"}, func(request *http.Request) (*http.Response, error) {
				reason := "Forbidden"
				if code == http.StatusUnauthorized {
					reason = "Unauthorized"
				}
				return jsonResponse(code, `{"reason":"`+reason+`","code":`+map[int]string{403: "403", 401: "401"}[code]+`}`), nil
			})
			provider, _ := New("source_cluster", "Scoped", collector)
			_, err := provider.CaptureInventory(context.Background())
			var providerError *domain.ProviderError
			if !errors.As(err, &providerError) || providerError.Code != domain.ProviderErrorPermissionDenied {
				t.Fatalf("authorization became generic outage: %v", err)
			}
		})
	}
}

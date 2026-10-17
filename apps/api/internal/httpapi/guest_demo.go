package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// GuestDemoOptions is composed only after isolated deployment checks succeed.
// It grants public observation, without inventing a user, session or CSRF proof.
type GuestDemoOptions struct {
	FixtureVersion string
	ScenarioStep   int
	ObservedAt     time.Time
}

func validGuestDemo(guest *GuestDemoOptions, providers []ProductProvider) bool {
	if guest == nil {
		return true
	}
	if len(providers) != 1 || !providers[0].Source().Synthetic || providers[0].Source().Provider != domain.ProviderDemo ||
		len(guest.FixtureVersion) < 1 || len(guest.FixtureVersion) > 64 || guest.ScenarioStep < 0 || guest.ScenarioStep > 2 ||
		guest.ObservedAt.IsZero() || guest.ObservedAt.Location() != time.UTC || guest.ObservedAt.Nanosecond()%1000 != 0 {
		return false
	}
	return true
}

func guestReadBoundary(guest *GuestDemoOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if guest == nil {
				next.ServeHTTP(w, r)
				return
			}
			if r.Method != http.MethodGet {
				writeProblem(w, r, forbiddenProblem)
				return
			}
			path := strings.TrimPrefix(r.URL.Path, "/api/v1")
			allowed := false
			switch path {
			case "/setup/status", "/sources", "/environments", "/resources", "/topology", "/logs", "/logs/download", "/metrics/current", "/metrics/history", "/health/history", "/events", "/alerts", "/notifications",
				"/streams/health", "/streams/logs", "/streams/metrics", "/streams/events", "/streams/alerts":
				allowed = true
			default:
				if strings.HasPrefix(path, "/resources/") {
					parts := strings.Split(strings.TrimPrefix(path, "/resources/"), "/")
					allowed = len(parts) == 1 || len(parts) == 2 && parts[1] == "health"
				}
			}
			if !allowed {
				writeProblem(w, r, forbiddenProblem)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func guestStreamBucket(r *http.Request) (string, bool) {
	peer, ok := authSourceKey(r)
	if !ok {
		return "", false
	}
	digest := sha256.Sum256([]byte(peer))
	// This value is a bounded per-peer rate bucket, never a persisted user ID.
	return "guest:" + hex.EncodeToString(digest[:16]), true
}

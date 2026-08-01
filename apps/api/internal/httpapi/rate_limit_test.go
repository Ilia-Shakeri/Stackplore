package httpapi

import (
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewRateLimiterValidatesConfiguration(t *testing.T) {
	t.Parallel()

	valid := RateLimitConfig{RequestsPerSecond: 1, Burst: 1, MaximumClients: 1}
	tests := []struct {
		name   string
		config RateLimitConfig
		now    func() time.Time
		valid  bool
	}{
		{name: "minimum", config: RateLimitConfig{RequestsPerSecond: 0.1, Burst: 1, MaximumClients: 1}, now: time.Now, valid: true},
		{name: "maximum", config: RateLimitConfig{RequestsPerSecond: 10000, Burst: 10000, MaximumClients: 65536}, now: time.Now, valid: true},
		{name: "not a number", config: RateLimitConfig{RequestsPerSecond: math.NaN(), Burst: 1, MaximumClients: 1}, now: time.Now},
		{name: "positive infinity", config: RateLimitConfig{RequestsPerSecond: math.Inf(1), Burst: 1, MaximumClients: 1}, now: time.Now},
		{name: "negative infinity", config: RateLimitConfig{RequestsPerSecond: math.Inf(-1), Burst: 1, MaximumClients: 1}, now: time.Now},
		{name: "rate too small", config: RateLimitConfig{RequestsPerSecond: 0.09, Burst: 1, MaximumClients: 1}, now: time.Now},
		{name: "rate too large", config: RateLimitConfig{RequestsPerSecond: 10000.01, Burst: 1, MaximumClients: 1}, now: time.Now},
		{name: "zero burst", config: RateLimitConfig{RequestsPerSecond: 1, MaximumClients: 1}, now: time.Now},
		{name: "large burst", config: RateLimitConfig{RequestsPerSecond: 1, Burst: 10001, MaximumClients: 1}, now: time.Now},
		{name: "zero client cap", config: RateLimitConfig{RequestsPerSecond: 1, Burst: 1}, now: time.Now},
		{name: "large client cap", config: RateLimitConfig{RequestsPerSecond: 1, Burst: 1, MaximumClients: 65537}, now: time.Now},
		{name: "nil clock", config: valid},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			limiter, err := newRateLimiter(test.config, test.now)
			if test.valid {
				if err != nil {
					t.Fatalf("newRateLimiter() error = %v", err)
				}
				if limiter == nil {
					t.Fatal("newRateLimiter() = nil, nil")
				}
				return
			}
			if err == nil {
				t.Fatal("newRateLimiter() error = nil; want error")
			}
		})
	}
}

func TestRateLimiterBurstAndRefill(t *testing.T) {
	t.Parallel()

	clock := newRateTestClock(time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC))
	limiter := mustRateLimiter(t, RateLimitConfig{RequestsPerSecond: 2, Burst: 2, MaximumClients: 4}, clock.Now)

	assertRateDecision(t, limiter, "192.0.2.1", true, 0)
	assertRateDecision(t, limiter, "192.0.2.1", true, 0)
	assertRateDecision(t, limiter, "192.0.2.1", false, time.Second)
	assertRateDecision(t, limiter, "192.0.2.2", true, 0)

	clock.Advance(250 * time.Millisecond)
	assertRateDecision(t, limiter, "192.0.2.1", false, time.Second)
	clock.Advance(250 * time.Millisecond)
	assertRateDecision(t, limiter, "192.0.2.1", true, 0)
}

func TestRateLimiterRejectsBlankKeyWithoutState(t *testing.T) {
	t.Parallel()

	clock := newRateTestClock(time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC))
	limiter := mustRateLimiter(t, RateLimitConfig{RequestsPerSecond: 1, Burst: 1, MaximumClients: 2}, clock.Now)

	assertRateDecision(t, limiter, "", false, time.Second)
	if len(limiter.clients) != 0 {
		t.Errorf("client count = %d; want 0", len(limiter.clients))
	}
}

func TestRateLimiterUsesBoundedOverflowBucket(t *testing.T) {
	t.Parallel()

	clock := newRateTestClock(time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC))
	limiter := mustRateLimiter(t, RateLimitConfig{RequestsPerSecond: 1, Burst: 1, MaximumClients: 2}, clock.Now)

	assertRateDecision(t, limiter, "192.0.2.1", true, 0)
	assertRateDecision(t, limiter, "192.0.2.2", true, 0)
	assertRateDecision(t, limiter, "192.0.2.3", true, 0)
	assertRateDecision(t, limiter, "192.0.2.4", false, time.Second)
	if len(limiter.clients) != 2 {
		t.Errorf("client count = %d; want 2", len(limiter.clients))
	}
	if _, exists := limiter.clients["192.0.2.3"]; exists {
		t.Error("overflow client entered bounded map")
	}

	clock.Advance(time.Second)
	assertRateDecision(t, limiter, "192.0.2.4", true, 0)
	if len(limiter.clients) != 2 {
		t.Errorf("client count after refill = %d; want 2", len(limiter.clients))
	}
}

func TestRateLimiterEvictsOnlyStaleClient(t *testing.T) {
	t.Parallel()

	clock := newRateTestClock(time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC))
	limiter := mustRateLimiter(t, RateLimitConfig{RequestsPerSecond: 1, Burst: 1, MaximumClients: 2}, clock.Now)

	assertRateDecision(t, limiter, "old", true, 0)
	clock.Advance(time.Minute)
	assertRateDecision(t, limiter, "fresh", true, 0)
	clock.Advance(staleRateClientAfter - time.Minute - time.Nanosecond)
	assertRateDecision(t, limiter, "overflow", true, 0)
	if _, exists := limiter.clients["overflow"]; exists {
		t.Error("non-stale map member was evicted early")
	}

	clock.Advance(time.Nanosecond)
	assertRateDecision(t, limiter, "replacement", true, 0)
	if _, exists := limiter.clients["old"]; exists {
		t.Error("stale client was not evicted")
	}
	if _, exists := limiter.clients["fresh"]; !exists {
		t.Error("fresh client was evicted")
	}
	if _, exists := limiter.clients["replacement"]; !exists {
		t.Error("replacement client did not enter map")
	}
	if len(limiter.clients) != 2 {
		t.Errorf("client count = %d; want 2", len(limiter.clients))
	}
}

func TestRateLimiterDefersFullMapSweepUntilExpiry(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	clock := newRateTestClock(start)
	limiter := mustRateLimiter(t, RateLimitConfig{RequestsPerSecond: 1, Burst: 1, MaximumClients: 1}, clock.Now)

	assertRateDecision(t, limiter, "stored", true, 0)
	assertRateDecision(t, limiter, "overflow-one", true, 0)
	if !limiter.nextSweep.Equal(start.Add(staleRateClientAfter)) {
		t.Fatalf("nextSweep = %s; want %s", limiter.nextSweep, start.Add(staleRateClientAfter))
	}

	clock.Advance(time.Minute)
	assertRateDecision(t, limiter, "overflow-two", true, 0)
	if !limiter.nextSweep.Equal(start.Add(staleRateClientAfter)) {
		t.Errorf("nextSweep moved early to %s", limiter.nextSweep)
	}

	clock.Set(start.Add(staleRateClientAfter))
	assertRateDecision(t, limiter, "replacement", true, 0)
	if _, exists := limiter.clients["stored"]; exists {
		t.Error("expired client remains after scheduled sweep")
	}
	if _, exists := limiter.clients["replacement"]; !exists {
		t.Error("replacement client missing after scheduled sweep")
	}
}

func TestRateLimiterDoesNotRegressOnOldClock(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	clock := newRateTestClock(start)
	limiter := mustRateLimiter(t, RateLimitConfig{RequestsPerSecond: 1, Burst: 1, MaximumClients: 1}, clock.Now)

	assertRateDecision(t, limiter, "peer", true, 0)
	clock.Advance(time.Second)
	assertRateDecision(t, limiter, "peer", true, 0)
	clock.Set(start)
	assertRateDecision(t, limiter, "peer", false, time.Second)

	bucket := limiter.clients["peer"]
	if !bucket.updated.Equal(start.Add(time.Second)) {
		t.Errorf("updated = %s; want %s", bucket.updated, start.Add(time.Second))
	}
	if !bucket.lastSeen.Equal(start.Add(time.Second)) {
		t.Errorf("lastSeen = %s; want %s", bucket.lastSeen, start.Add(time.Second))
	}

	clock.Set(start.Add(2 * time.Second))
	assertRateDecision(t, limiter, "peer", true, 0)
}

func TestRateLimiterSerializesConcurrentBurst(t *testing.T) {
	t.Parallel()

	clock := newRateTestClock(time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC))
	const burst = 128
	const requests = 512
	limiter := mustRateLimiter(t, RateLimitConfig{RequestsPerSecond: 0.1, Burst: burst, MaximumClients: 1}, clock.Now)
	var allowed atomic.Int64
	var wait sync.WaitGroup
	wait.Add(requests)
	for range requests {
		go func() {
			defer wait.Done()
			if ok, _ := limiter.allow("192.0.2.1"); ok {
				allowed.Add(1)
			}
		}()
	}
	wait.Wait()

	if got := allowed.Load(); got != burst {
		t.Errorf("allowed = %d; want %d", got, burst)
	}
}

func TestRateLimiterKeepsConcurrentClientCountBounded(t *testing.T) {
	t.Parallel()

	clock := newRateTestClock(time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC))
	const maximumClients = 32
	const requests = 512
	limiter := mustRateLimiter(t, RateLimitConfig{RequestsPerSecond: 1, Burst: 1, MaximumClients: maximumClients}, clock.Now)
	var wait sync.WaitGroup
	wait.Add(requests)
	for index := range requests {
		go func() {
			defer wait.Done()
			_, _ = limiter.allow("peer-" + strconv.Itoa(index))
		}()
	}
	wait.Wait()

	if got := len(limiter.clients); got != maximumClients {
		t.Errorf("client count = %d; want %d", got, maximumClients)
	}
}

type rateTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func newRateTestClock(now time.Time) *rateTestClock {
	return &rateTestClock{now: now}
}

func (clock *rateTestClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *rateTestClock) Advance(duration time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(duration)
	clock.mu.Unlock()
}

func (clock *rateTestClock) Set(now time.Time) {
	clock.mu.Lock()
	clock.now = now
	clock.mu.Unlock()
}

func mustRateLimiter(t *testing.T, config RateLimitConfig, now func() time.Time) *rateLimiter {
	t.Helper()
	limiter, err := newRateLimiter(config, now)
	if err != nil {
		t.Fatalf("newRateLimiter() error = %v", err)
	}
	return limiter
}

func assertRateDecision(t *testing.T, limiter *rateLimiter, key string, wantAllowed bool, wantRetry time.Duration) {
	t.Helper()
	allowed, retry := limiter.allow(key)
	if allowed != wantAllowed {
		t.Errorf("allow(%q) allowed = %t; want %t", key, allowed, wantAllowed)
	}
	if retry != wantRetry {
		t.Errorf("allow(%q) retry = %s; want %s", key, retry, wantRetry)
	}
}

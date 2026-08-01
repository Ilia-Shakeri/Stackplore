package httpapi

import (
	"errors"
	"math"
	"sync"
	"time"
)

type RateLimitConfig struct {
	RequestsPerSecond float64
	Burst             int
	MaximumClients    int
}

type rateBucket struct {
	tokens   float64
	updated  time.Time
	lastSeen time.Time
}

type rateLimiter struct {
	mu        sync.Mutex
	config    RateLimitConfig
	clients   map[string]rateBucket
	overflow  rateBucket
	nextSweep time.Time
	now       func() time.Time
}

const staleRateClientAfter = 10 * time.Minute

var errInvalidRateLimitConfig = errors.New("invalid rate limit configuration")

func newRateLimiter(config RateLimitConfig, now func() time.Time) (*rateLimiter, error) {
	if math.IsNaN(config.RequestsPerSecond) ||
		math.IsInf(config.RequestsPerSecond, 0) ||
		config.RequestsPerSecond < 0.1 ||
		config.RequestsPerSecond > 10000 ||
		config.Burst < 1 ||
		config.Burst > 10000 ||
		config.MaximumClients < 1 ||
		config.MaximumClients > 65536 ||
		now == nil {
		return nil, errInvalidRateLimitConfig
	}
	current := now()
	return &rateLimiter{
		config:  config,
		clients: make(map[string]rateBucket, config.MaximumClients),
		overflow: rateBucket{
			tokens:   float64(config.Burst),
			updated:  current,
			lastSeen: current,
		},
		now: now,
	}, nil
}

func (limiter *rateLimiter) allow(key string) (bool, time.Duration) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	if key == "" {
		return false, time.Second
	}
	now := limiter.now()
	if bucket, exists := limiter.clients[key]; exists {
		allowed, retryAfter, updated := limiter.consume(bucket, now)
		limiter.clients[key] = updated
		return allowed, retryAfter
	}

	if len(limiter.clients) >= limiter.config.MaximumClients {
		limiter.evictStale(now)
	}
	if len(limiter.clients) < limiter.config.MaximumClients {
		bucket := rateBucket{tokens: float64(limiter.config.Burst), updated: now, lastSeen: now}
		allowed, retryAfter, updated := limiter.consume(bucket, now)
		limiter.clients[key] = updated
		limiter.trackExpiry(updated.lastSeen)
		return allowed, retryAfter
	}

	allowed, retryAfter, updated := limiter.consume(limiter.overflow, now)
	limiter.overflow = updated
	return allowed, retryAfter
}

func (limiter *rateLimiter) consume(bucket rateBucket, now time.Time) (bool, time.Duration, rateBucket) {
	if now.Before(bucket.updated) {
		now = bucket.updated
	}
	elapsed := now.Sub(bucket.updated)
	if elapsed > 0 {
		bucket.tokens = math.Min(
			float64(limiter.config.Burst),
			bucket.tokens+elapsed.Seconds()*limiter.config.RequestsPerSecond,
		)
		bucket.updated = now
	}
	bucket.lastSeen = now
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true, 0, bucket
	}
	waitSeconds := (1 - bucket.tokens) / limiter.config.RequestsPerSecond
	retryAfter := time.Duration(math.Ceil(waitSeconds * float64(time.Second)))
	if retryAfter < time.Second {
		retryAfter = time.Second
	}
	return false, retryAfter, bucket
}

func (limiter *rateLimiter) evictStale(now time.Time) {
	if !limiter.nextSweep.IsZero() && now.Before(limiter.nextSweep) {
		return
	}

	limiter.nextSweep = time.Time{}
	for key, bucket := range limiter.clients {
		expiresAt := bucket.lastSeen.Add(staleRateClientAfter)
		if now.Before(expiresAt) {
			if limiter.nextSweep.IsZero() || expiresAt.Before(limiter.nextSweep) {
				limiter.nextSweep = expiresAt
			}
			continue
		}
		delete(limiter.clients, key)
	}
}

func (limiter *rateLimiter) trackExpiry(lastSeen time.Time) {
	expiresAt := lastSeen.Add(staleRateClientAfter)
	if limiter.nextSweep.IsZero() || expiresAt.Before(limiter.nextSweep) {
		limiter.nextSweep = expiresAt
	}
}

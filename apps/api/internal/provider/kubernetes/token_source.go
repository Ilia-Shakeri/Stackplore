package kubernetes

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Projected credentials rotate independently of the source configuration.
// A failed refresh must never fall back to an expired cached credential.
type bearerSource struct {
	mutex     sync.Mutex
	token     string
	expiresAt time.Time
	read      func(context.Context) (string, error)
	now       func() time.Time
	closed    bool
}

func validBearerToken(token string) bool {
	return token != "" && len(token) <= 16*1024 && strings.TrimSpace(token) == token && !strings.ContainsAny(token, "\r\n")
}

func (source *bearerSource) current(ctx context.Context) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	source.mutex.Lock()
	defer source.mutex.Unlock()
	if source.closed {
		return "", ErrUnavailable
	}
	if source.read == nil || source.now().Before(source.expiresAt) {
		return source.token, nil
	}
	token, err := source.read(ctx)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil || !validBearerToken(token) {
		return "", ErrPermissionDenied
	}
	source.token = token
	source.expiresAt = source.now().Add(time.Minute)
	return token, nil
}

func (source *bearerSource) invalidate(rejected string) {
	source.mutex.Lock()
	defer source.mutex.Unlock()
	if source.token == rejected {
		source.expiresAt = time.Time{}
	}
}
func (source *bearerSource) close() {
	source.mutex.Lock()
	defer source.mutex.Unlock()
	source.closed = true
	source.token = ""
	source.expiresAt = time.Time{}
}

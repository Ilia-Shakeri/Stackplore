package notification

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
)

type StaticWebhookTargetResolver struct {
	mu        sync.RWMutex
	reference string
	url       string
	key       []byte
	closed    bool
}

func NewStaticWebhookTargetResolver(reference, rawURL string, signingKey []byte) (*StaticWebhookTargetResolver, error) {
	if !validDeliveryID(reference) || len(signingKey) < minimumSigningKey || len(signingKey) > 128 {
		return nil, ErrDeliveryUnavailable
	}
	if _, err := parseDestination(rawURL); err != nil {
		return nil, ErrDeliveryUnavailable
	}
	return &StaticWebhookTargetResolver{
		reference: reference, url: rawURL, key: append([]byte(nil), signingKey...),
	}, nil
}

func (resolver *StaticWebhookTargetResolver) ResolveWebhookTarget(ctx context.Context, reference string) (WebhookTarget, error) {
	if resolver == nil || ctx == nil || ctx.Err() != nil {
		return WebhookTarget{}, ErrDeliveryUnavailable
	}
	resolver.mu.RLock()
	defer resolver.mu.RUnlock()
	if resolver.closed || reference != resolver.reference {
		return WebhookTarget{}, errors.New("webhook target is unavailable")
	}
	return WebhookTarget{URL: resolver.url, SigningKey: append([]byte(nil), resolver.key...)}, nil
}

func (resolver *StaticWebhookTargetResolver) Close() {
	if resolver == nil {
		return
	}
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	if resolver.closed {
		return
	}
	clear(resolver.key)
	resolver.key = nil
	resolver.url = ""
	resolver.closed = true
}

func NewDeliveryWorkerID(randomBytes []byte) (string, error) {
	if len(randomBytes) != 18 {
		return "", ErrDeliveryUnavailable
	}
	return "delivery_worker_" + base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

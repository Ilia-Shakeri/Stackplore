package notification

import (
	"context"
	"testing"
)

func TestStaticWebhookTargetResolverCopiesAndClearsSecret(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	resolver, err := NewStaticWebhookTargetResolver("webhook_primary", "https://hooks.example.test/path", key)
	if err != nil {
		t.Fatalf("NewStaticWebhookTargetResolver() error = %v", err)
	}
	key[0] = 7
	target, err := resolver.ResolveWebhookTarget(context.Background(), "webhook_primary")
	if err != nil || target.SigningKey[0] != 0 {
		t.Fatal("resolver retained caller-owned key storage")
	}
	target.SigningKey[0] = 9
	again, err := resolver.ResolveWebhookTarget(context.Background(), "webhook_primary")
	if err != nil || again.SigningKey[0] != 0 {
		t.Fatal("resolver returned shared key storage")
	}
	clear(target.SigningKey)
	clear(again.SigningKey)
	resolver.Close()
	if _, err := resolver.ResolveWebhookTarget(context.Background(), "webhook_primary"); err == nil {
		t.Fatal("closed resolver returned target material")
	}
}

func TestDeliveryWorkerIDUsesBoundedOpaqueEncoding(t *testing.T) {
	t.Parallel()
	id, err := NewDeliveryWorkerID(make([]byte, 18))
	if err != nil || !validDeliveryID(id) || len(id) > 128 {
		t.Fatalf("NewDeliveryWorkerID() = (%q, %v)", id, err)
	}
	if _, err := NewDeliveryWorkerID(make([]byte, 17)); err == nil {
		t.Fatal("short worker entropy accepted")
	}
}

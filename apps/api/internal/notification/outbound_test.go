package notification

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

func TestOutboundBlocksTransitionAndSpecialAddresses(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		"192.88.99.1", "::ffff:127.0.0.1", "64:ff9b::7f00:1", "64:ff9b:1::a00:1",
		"100::1", "100:0:0:1::1", "2001::1", "2001:2::1", "2002:7f00:1::1",
		"3fff::1", "5f00::1", "8000::1",
	} {
		if allPublicAddresses([]netip.Addr{netip.MustParseAddr(value)}) {
			t.Errorf("special address accepted: %s", value)
		}
	}
}

func TestOutboundRedirectNeverSendsCredentialsToNextOrigin(t *testing.T) {
	t.Parallel()
	var followed atomic.Int32
	second := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		followed.Add(1)
		response.WriteHeader(http.StatusNoContent)
	}))
	defer second.Close()
	var received atomic.Int32
	first := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		received.Add(1)
		http.Redirect(response, request, second.URL, http.StatusTemporaryRedirect)
	}))
	defer first.Close()
	key := make([]byte, minimumSigningKey)
	if _, err := rand.Read(key); err != nil {
		t.Fatal("generate disposable signing material")
	}
	defer clear(key)
	_, err := senderForTLSServer(t, first, 64).Send(context.Background(), WebhookTarget{URL: first.URL, SigningKey: key}, WebhookDelivery{ID: "delivery_redirect", Payload: []byte(`{"ok":true}`)})
	var failure *DeliveryError
	if !errors.As(err, &failure) || failure.Code != "notification.redirect_rejected" || received.Load() != 1 || followed.Load() != 0 {
		t.Fatal("redirect crossed the checked destination")
	}
}

func TestOutboundRechecksDNSAfterSuccessfulAttempt(t *testing.T) {
	t.Parallel()
	var received atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		received.Add(1)
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	resolver := &rebindResolver{}
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	var dialed atomic.Int32
	dialer := &net.Dialer{Timeout: time.Second}
	sender, err := NewWebhookSender(WebhookSenderOptions{
		Resolver: resolver, Dialer: dialerFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
			dialed.Add(1)
			if address != "8.8.8.8:443" {
				return nil, errors.New("fixture address changed")
			}
			return dialer.DialContext(ctx, network, server.Listener.Addr().String())
		}), Timeout: time.Second, MaximumResponseBytes: 64, RootCAs: pool,
	})
	if err != nil {
		t.Fatal("create outbound fixture")
	}
	target := WebhookTarget{URL: "https://example.com/fixture", SigningKey: make([]byte, minimumSigningKey)}
	delivery := WebhookDelivery{ID: "delivery_rebind", Payload: []byte(`{"ok":true}`)}
	if _, err := sender.Send(context.Background(), target, delivery); err != nil {
		t.Fatal("public checked attempt failed")
	}
	_, err = sender.Send(context.Background(), target, delivery)
	var failure *DeliveryError
	if !errors.As(err, &failure) || failure.Code != "notification.destination_rejected" || resolver.calls != 2 || dialed.Load() != 1 || received.Load() != 1 {
		t.Fatal("DNS rebinding reached a private destination")
	}
}

type rebindResolver struct{ calls int }

func (resolver *rebindResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	resolver.calls++
	if resolver.calls == 1 {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
}

func TestOutboundVerifiesTLSHostAndBoundsDNSCount(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		t.Error("TLS host mismatch reached receiver")
	}))
	defer server.Close()
	sender := senderForTLSServer(t, server, 64)
	_, err := sender.Send(context.Background(), WebhookTarget{URL: "https://wrong.example.test", SigningKey: make([]byte, minimumSigningKey)}, WebhookDelivery{ID: "delivery_tls", Payload: []byte(`{"ok":true}`)})
	var failure *DeliveryError
	if !errors.As(err, &failure) || failure.Code != "notification.network_failed" {
		t.Fatal("TLS host check did not fail closed")
	}
	addresses := make([]netip.Addr, 33)
	for index := range addresses {
		addresses[index] = netip.MustParseAddr("8.8.8.8")
	}
	sender = newTestSender(t, staticResolver{addresses: addresses})
	_, err = sender.Send(context.Background(), WebhookTarget{URL: "https://example.com", SigningKey: make([]byte, minimumSigningKey)}, WebhookDelivery{ID: "delivery_dns_bound", Payload: []byte(`{"ok":true}`)})
	if !errors.As(err, &failure) || failure.Code != "notification.destination_rejected" {
		t.Fatal("oversized DNS answer accepted")
	}
}

func TestNotificationTargetsAlwaysFormatRedacted(t *testing.T) {
	t.Parallel()
	key := make([]byte, minimumSigningKey)
	if _, err := rand.Read(key); err != nil {
		t.Fatal("generate disposable formatting fixture")
	}
	defer clear(key)
	for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
		if fmt.Sprintf(format, WebhookTarget{URL: "https://example.test/fixture", SigningKey: key}) != "[redacted webhook target]" ||
			fmt.Sprintf(format, MatrixTarget{Origin: "https://example.test", RoomID: "!fixture", AccessToken: key}) != "[redacted Matrix target]" {
			t.Fatal("formatted notification target is not redacted")
		}
	}
	for _, token := range []string{"=", "===", "a=b"} {
		if validMatrixToken([]byte(token)) {
			t.Fatal("padding-only or misplaced bearer padding accepted")
		}
	}
	for _, target := range []any{WebhookTarget{URL: "https://example.test/fixture", SigningKey: key}, MatrixTarget{Origin: "https://example.test", RoomID: "!fixture", AccessToken: key}} {
		encoded, err := json.Marshal(target)
		if err != nil || string(encoded) != "{}" {
			t.Fatal("target material escaped structured encoding")
		}
	}
}

type blockedDNSResolver struct{}

func (blockedDNSResolver) LookupNetIP(ctx context.Context, _, _ string) ([]netip.Addr, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestOutboundBoundsDNSAndRetriesLookupFailureBeforeSend(t *testing.T) {
	t.Parallel()
	for _, resolver := range []Resolver{blockedDNSResolver{}, staticResolver{err: errors.New("fixture lookup stopped")}} {
		var dialed atomic.Int32
		sender, err := NewWebhookSender(WebhookSenderOptions{Resolver: resolver, Dialer: dialerFunc(func(context.Context, string, string) (net.Conn, error) {
			dialed.Add(1)
			return nil, errors.New("unexpected fixture dial")
		}), Timeout: time.Second, MaximumResponseBytes: 64})
		if err != nil {
			t.Fatal("create bounded DNS fixture")
		}
		started := time.Now()
		_, err = sender.Send(context.Background(), WebhookTarget{URL: "https://example.com", SigningKey: make([]byte, minimumSigningKey)}, WebhookDelivery{ID: "delivery_dns_failure", Payload: []byte(`{"ok":true}`)})
		var failure *DeliveryError
		if !errors.As(err, &failure) || !failure.Retryable || failure.Uncertain || dialed.Load() != 0 || time.Since(started) > 2*time.Second {
			t.Fatal("DNS failure sent bytes or lacked bounded safe retry")
		}
		if failure.Code != "notification.dns_failed" && failure.Code != "notification.timeout" {
			t.Fatal("DNS failure escaped fixed codes")
		}
	}
}

package notification

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type staticResolver struct {
	addresses []netip.Addr
	err       error
}

type dialerFunc func(context.Context, string, string) (net.Conn, error)

func (dial dialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return dial(ctx, network, address)
}

func (resolver staticResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return resolver.addresses, resolver.err
}

func TestWebhookRejectsUnsafeDestinationOnEveryAttempt(t *testing.T) {
	sender := newTestSender(t, staticResolver{addresses: []netip.Addr{netip.MustParseAddr("127.0.0.1")}})
	_, err := sender.Send(context.Background(), WebhookTarget{
		URL: "https://hooks.example.test/delivery", SigningKey: make([]byte, 32),
	}, WebhookDelivery{ID: "delivery_one", Payload: []byte(`{"ok":true}`)})
	var failure *DeliveryError
	if !errors.As(err, &failure) || failure.Code != "notification.destination_rejected" || failure.Retryable {
		t.Fatalf("private destination error = %#v", err)
	}

	sender = newTestSender(t, staticResolver{addresses: []netip.Addr{
		netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.1"),
	}})
	_, err = sender.Send(context.Background(), WebhookTarget{
		URL: "https://hooks.example.test/delivery", SigningKey: make([]byte, 32),
	}, WebhookDelivery{ID: "delivery_one", Payload: []byte(`{"ok":true}`)})
	if !errors.As(err, &failure) || failure.Code != "notification.destination_rejected" {
		t.Fatalf("mixed destination error = %#v", err)
	}
}

func TestWebhookURLAndSignatureAreStrictAndStable(t *testing.T) {
	for _, raw := range []string{
		"http://hooks.example.test/path",
		"https://user@hooks.example.test/path",
		"https://hooks.example.test/path#fragment",
		" https://hooks.example.test/path",
	} {
		if _, err := parseDestination(raw); err == nil {
			t.Errorf("unsafe URL accepted: %q", raw)
		}
	}
	delivery := WebhookDelivery{ID: "delivery_one", Payload: []byte(`{"state":"firing"}`)}
	key := []byte("01234567890123456789012345678901")
	first := signPayload(key, delivery)
	second := signPayload(key, delivery)
	if first == "" || first != second || len(first) != 64 {
		t.Fatalf("signature = (%q, %q)", first, second)
	}
	delivery.ID = "delivery_two"
	if signPayload(key, delivery) == first {
		t.Fatal("delivery identifier is not bound into signature")
	}
}

func TestReservedAddressRangesAreRejected(t *testing.T) {
	for _, value := range []string{
		"0.1.2.3", "100.64.0.1", "169.254.1.1", "192.0.2.1", "198.18.0.1",
		"198.51.100.1", "203.0.113.1", "2001:db8::1", "fc00::1", "fe80::1",
	} {
		if allPublicAddresses([]netip.Addr{netip.MustParseAddr(value)}) {
			t.Errorf("reserved address accepted: %s", value)
		}
	}
	if !allPublicAddresses([]netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("2606:4700:4700::1111")}) {
		t.Fatal("public addresses were rejected")
	}
}

func TestWebhookSendsStableHeadersAndBoundsRemoteResponse(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	delivery := WebhookDelivery{ID: "delivery_one", Payload: []byte(`{"state":"firing"}`)}
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		if request.Method != http.MethodPost || string(body) != string(delivery.Payload) ||
			request.Header.Get("Idempotency-Key") != delivery.ID ||
			request.Header.Get("X-Stackplore-Signature") != "sha256="+signPayload(key, delivery) {
			t.Errorf("webhook request = (%s, %q, %#v)", request.Method, body, request.Header)
		}
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	sender := senderForTLSServer(t, server, 64)
	result, err := sender.Send(context.Background(), WebhookTarget{URL: server.URL, SigningKey: key}, delivery)
	if err != nil || result.Status != http.StatusNoContent {
		t.Fatalf("Send() = (%#v, %v)", result, err)
	}

	large := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(strings.Repeat("x", 65)))
	}))
	defer large.Close()
	sender = senderForTLSServer(t, large, 64)
	_, err = sender.Send(context.Background(), WebhookTarget{URL: large.URL, SigningKey: key}, delivery)
	var failure *DeliveryError
	if !errors.As(err, &failure) || failure.Code != "notification.response_too_large" || failure.Retryable {
		t.Fatalf("large response error = %#v", err)
	}
}

func TestWebhookRejectsRedirectAndClassifiesRemoteRetry(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	delivery := WebhookDelivery{ID: "delivery_one", Payload: []byte(`{"state":"firing"}`)}
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, "/other", http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	_, err := senderForTLSServer(t, redirect, 64).Send(
		context.Background(), WebhookTarget{URL: redirect.URL, SigningKey: key}, delivery,
	)
	var failure *DeliveryError
	if !errors.As(err, &failure) || failure.Code != "notification.redirect_rejected" || failure.Retryable {
		t.Fatalf("redirect error = %#v", err)
	}

	rejected := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer rejected.Close()
	result, err := senderForTLSServer(t, rejected, 64).Send(
		context.Background(), WebhookTarget{URL: rejected.URL, SigningKey: key}, delivery,
	)
	if result.Status != http.StatusServiceUnavailable || !errors.As(err, &failure) || !failure.Retryable {
		t.Fatalf("remote rejection = (%#v, %#v)", result, err)
	}
}

func TestWebhookClassifiesUnknownNetworkResultAsUncertain(t *testing.T) {
	t.Parallel()
	sender, err := NewWebhookSender(WebhookSenderOptions{
		Resolver: staticResolver{addresses: []netip.Addr{netip.MustParseAddr("8.8.8.8")}},
		Dialer: dialerFunc(func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("network stopped")
		}),
		Timeout: 2 * time.Second, MaximumResponseBytes: 64,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = sender.Send(context.Background(), WebhookTarget{
		URL: "https://hooks.example.test/delivery", SigningKey: make([]byte, 32),
	}, WebhookDelivery{ID: "delivery_one", Payload: []byte(`{"ok":true}`)})
	var failure *DeliveryError
	if !errors.As(err, &failure) || !failure.Retryable || !failure.Uncertain ||
		failure.Code != "notification.network_failed" {
		t.Fatalf("network failure = %#v; want retryable uncertain", err)
	}
}

func TestWebhookAttemptsReleaseConnectionsAfterSuccess(t *testing.T) {
	closed := make(chan struct{}, 3)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			closed <- struct{}{}
		}
	}
	server.StartTLS()
	defer server.Close()
	sender := senderForTLSServer(t, server, 64)
	for range 3 {
		_, err := sender.Send(context.Background(), WebhookTarget{
			URL: server.URL, SigningKey: []byte("01234567890123456789012345678901"),
		}, WebhookDelivery{ID: "delivery_connection_test", Payload: []byte(`{"ok":true}`)})
		if err != nil {
			t.Fatal("webhook fixture send failed")
		}
	}
	for range 3 {
		select {
		case <-closed:
		case <-time.After(2 * time.Second):
			t.Fatal("completed webhook attempt retained its connection")
		}
	}
}

func newTestSender(t *testing.T, resolver Resolver) *WebhookSender {
	t.Helper()
	sender, err := NewWebhookSender(WebhookSenderOptions{
		Resolver: resolver, Dialer: &net.Dialer{Timeout: time.Second},
		Timeout: 2 * time.Second, MaximumResponseBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	return sender
}

func senderForTLSServer(t *testing.T, server *httptest.Server, maximum int64) *WebhookSender {
	t.Helper()
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	dialer := &net.Dialer{Timeout: time.Second}
	sender, err := NewWebhookSender(WebhookSenderOptions{
		Resolver: staticResolver{addresses: []netip.Addr{netip.MustParseAddr("8.8.8.8")}},
		Dialer: dialerFunc(func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, server.Listener.Addr().String())
		}),
		Timeout: 2 * time.Second, MaximumResponseBytes: maximum,
	})
	if err != nil {
		t.Fatal(err)
	}
	sender.rootCAs = pool
	return sender
}

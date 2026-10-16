package notification

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func matrixFixturePayload(t *testing.T) []byte {
	t.Helper()
	payload, err := RenderMatrixPayload(domain.Notification{ID: "notification_fixture", Kind: "alert.firing", Severity: domain.SeverityWarning, SummaryCode: "alert.cpu_threshold", CreatedAt: time.Date(2026, time.October, 13, 10, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal("render Matrix fixture")
	}
	return payload
}

func matrixSenderForReceiver(t *testing.T, server *httptest.Server, maximum int64) *MatrixSender {
	t.Helper()
	dialer := &net.Dialer{Timeout: time.Second}
	sender, err := NewMatrixSender(MatrixSenderOptions{Resolver: staticResolver{addresses: []netip.Addr{netip.MustParseAddr("8.8.8.8")}}, Dialer: dialerFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "8.8.8.8:443" {
			return nil, errors.New("fixture dial changed")
		}
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}), Timeout: time.Second, MaximumResponseBytes: maximum})
	if err != nil {
		t.Fatal("create Matrix fixture sender")
	}
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	sender.rootCAs = pool
	return sender
}

func TestMatrixActualHTTPSReceiverUsesStableTransactionAndPlainNotice(t *testing.T) {
	t.Parallel()
	payload := matrixFixturePayload(t)
	token := []byte("synthetic_receiver_token")
	var mu sync.Mutex
	requests := 0
	seen := make(map[string][]byte)
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(io.LimitReader(request.Body, maximumWebhookPayload+1))
		if err != nil {
			t.Error("read Matrix fixture request")
			return
		}
		if request.Method != http.MethodPut || request.URL.EscapedPath() != "/_matrix/client/v3/rooms/%21fixture:example.test/send/m.room.message/delivery_fixture" || request.URL.RawQuery != "" || request.Header.Get("Authorization") != "Bearer "+string(token) || string(body) != string(payload) {
			t.Error("Matrix fixture method, path, auth, or canonical body differs")
		}
		var value matrixPayload
		if json.Unmarshal(body, &value) != nil || value.MsgType != "m.notice" || strings.Contains(value.Body, "<") {
			t.Error("Matrix fixture payload is not plain notice")
		}
		mu.Lock()
		requests++
		if prior, ok := seen[request.URL.Path]; ok && string(prior) != string(body) {
			t.Error("Matrix retry payload changed")
		}
		seen[request.URL.Path] = append([]byte(nil), body...)
		mu.Unlock()
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"event_id":"$fixture"}`))
	}))
	defer server.Close()
	sender := matrixSenderForReceiver(t, server, 1024)
	for range 2 {
		result, err := sender.Send(context.Background(), MatrixTarget{Origin: "https://example.com", RoomID: "!fixture:example.test", AccessToken: token}, MatrixDelivery{ID: "delivery_fixture", Payload: payload})
		if err != nil || result.Status != 200 {
			t.Fatal("actual Matrix HTTPS receiver did not confirm send")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if requests != 2 || len(seen) != 1 {
		t.Fatal("Matrix retries did not retain one transaction")
	}
}

func TestMatrixRejectsRedirectBoundsBodyAndMarksUncertainResponse(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		handler   http.HandlerFunc
		code      string
		uncertain bool
	}{
		{"redirect", func(response http.ResponseWriter, request *http.Request) {
			http.Redirect(response, request, "https://elsewhere.example.test/steal", 307)
		}, "notification.redirect_rejected", false},
		{"large", func(response http.ResponseWriter, _ *http.Request) {
			_, _ = response.Write([]byte(strings.Repeat("x", 65)))
		}, "notification.response_too_large", true},
		{"invalid-success", func(response http.ResponseWriter, _ *http.Request) { _, _ = response.Write([]byte(`{"ok":true}`)) }, "notification.invalid_response", true},
		{"timeout", func(response http.ResponseWriter, request *http.Request) {
			select {
			case <-request.Context().Done():
			case <-time.After(1500 * time.Millisecond):
			}
		}, "notification.timeout", true},
		{"body-timeout", func(response http.ResponseWriter, request *http.Request) {
			response.WriteHeader(200)
			response.(http.Flusher).Flush()
			select {
			case <-request.Context().Done():
			case <-time.After(1500 * time.Millisecond):
			}
		}, "notification.timeout", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(test.handler)
			defer server.Close()
			sender := matrixSenderForReceiver(t, server, 64)
			_, err := sender.Send(context.Background(), MatrixTarget{Origin: "https://example.com", RoomID: "!fixture", AccessToken: []byte("synthetic_fixture_token")}, MatrixDelivery{ID: "delivery_fixture", Payload: matrixFixturePayload(t)})
			var failure *DeliveryError
			if !errors.As(err, &failure) || failure.Code != test.code || failure.Uncertain != test.uncertain {
				t.Fatalf("Matrix fixture result differs; safe code=%v", err)
			}
		})
	}
}

func TestMatrixRejectsUnsafeDNSOnEveryAttemptAndNeverDials(t *testing.T) {
	t.Parallel()
	resolver := &changingResolver{}
	dialed := false
	sender, err := NewMatrixSender(MatrixSenderOptions{Resolver: resolver, Dialer: dialerFunc(func(context.Context, string, string) (net.Conn, error) {
		dialed = true
		return nil, errors.New("fixture network unavailable")
	}), Timeout: time.Second, MaximumResponseBytes: 64})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		_, _ = sender.Send(context.Background(), MatrixTarget{Origin: "https://matrix.example.test", RoomID: "!fixture", AccessToken: []byte("synthetic_fixture")}, MatrixDelivery{ID: "delivery_fixture", Payload: matrixFixturePayload(t)})
	}
	if resolver.calls != 2 || dialed {
		t.Fatal("Matrix DNS was not checked and rejected on each attempt")
	}
}

type changingResolver struct{ calls int }

func (resolver *changingResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	resolver.calls++
	if resolver.calls == 1 {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")}, nil
	}
	return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
}

func TestMatrixTargetCloseAndInputBounds(t *testing.T) {
	t.Parallel()
	resolver, err := NewStaticMatrixTargetResolver("matrix_fixture", "https://matrix.example.test", "!fixture:example.test", []byte("synthetic_fixture"))
	if err != nil {
		t.Fatal(err)
	}
	target, err := resolver.ResolveMatrixTarget(context.Background(), "matrix_fixture")
	if err != nil {
		t.Fatal(err)
	}
	clear(target.AccessToken)
	target, err = resolver.ResolveMatrixTarget(context.Background(), "matrix_fixture")
	if err != nil || len(target.AccessToken) == 0 || target.AccessToken[0] == 0 {
		t.Fatal("target resolver copied token incorrectly")
	}
	resolver.Close()
	if _, err := resolver.ResolveMatrixTarget(context.Background(), "matrix_fixture"); err == nil {
		t.Fatal("closed Matrix target still resolves")
	}
	for _, payload := range []string{`{"msgtype":"m.text","body":"x"}`, `{"msgtype":"m.notice","body":"x","formatted_body":"<b>x</b>"}`, `{"msgtype":"m.notice","body":"x"} garbage`} {
		if validMatrixPayload([]byte(payload)) {
			t.Fatal("unbounded or formatted Matrix payload accepted")
		}
	}
}

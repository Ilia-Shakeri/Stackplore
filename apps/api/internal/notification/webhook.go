package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maximumWebhookPayload = 64 * 1024
	minimumSigningKey     = 32
)

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type ContextDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

type WebhookSenderOptions struct {
	Resolver             Resolver
	Dialer               ContextDialer
	Timeout              time.Duration
	MaximumResponseBytes int64
}

type WebhookSender struct {
	resolver             Resolver
	dialer               ContextDialer
	timeout              time.Duration
	maximumResponseBytes int64
	rootCAs              *x509.CertPool
}

type WebhookTarget struct {
	URL        string
	SigningKey []byte
}

type WebhookDelivery struct {
	ID      string
	Payload []byte
}

type WebhookResult struct {
	Status int
}

type DeliveryError struct {
	Code      string
	Retryable bool
	Uncertain bool
}

func (failure *DeliveryError) Error() string {
	if failure == nil || failure.Code == "" {
		return "notification delivery failed"
	}
	return failure.Code
}

func NewWebhookSender(options WebhookSenderOptions) (*WebhookSender, error) {
	if options.Resolver == nil || options.Dialer == nil || options.Timeout < time.Second ||
		options.Timeout > 30*time.Second || options.MaximumResponseBytes < 1 ||
		options.MaximumResponseBytes > 1024*1024 {
		return nil, errors.New("webhook sender options are invalid")
	}
	return &WebhookSender{
		resolver: options.Resolver, dialer: options.Dialer, timeout: options.Timeout,
		maximumResponseBytes: options.MaximumResponseBytes,
	}, nil
}

func (sender *WebhookSender) Send(ctx context.Context, target WebhookTarget, delivery WebhookDelivery) (WebhookResult, error) {
	if sender == nil || ctx == nil || ctx.Err() != nil || !validDeliveryID(delivery.ID) ||
		len(delivery.Payload) < 2 || len(delivery.Payload) > maximumWebhookPayload ||
		!json.Valid(delivery.Payload) || len(target.SigningKey) < minimumSigningKey || len(target.SigningKey) > 128 {
		return WebhookResult{}, deliveryFailure("notification.invalid_request", false)
	}
	destination, err := parseDestination(target.URL)
	if err != nil {
		return WebhookResult{}, deliveryFailure("notification.destination_rejected", false)
	}
	attemptContext, cancel := context.WithTimeout(ctx, sender.timeout)
	defer cancel()
	host := strings.ToLower(strings.TrimSuffix(destination.Hostname(), "."))
	addresses, err := sender.resolver.LookupNetIP(attemptContext, "ip", host)
	if err != nil || len(addresses) == 0 || !allPublicAddresses(addresses) {
		return WebhookResult{}, deliveryFailure("notification.destination_rejected", false)
	}
	request, err := http.NewRequestWithContext(attemptContext, http.MethodPost, destination.String(), bytes.NewReader(delivery.Payload))
	if err != nil {
		return WebhookResult{}, deliveryFailure("notification.invalid_request", false)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", delivery.ID)
	request.Header.Set("X-Stackplore-Delivery-ID", delivery.ID)
	request.Header.Set("X-Stackplore-Signature", "sha256="+signPayload(target.SigningKey, delivery))
	client := sender.client(host, addresses)
	defer client.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(err, errWebhookRedirect) {
			return WebhookResult{}, deliveryFailure("notification.redirect_rejected", false)
		}
		if attemptContext.Err() != nil {
			return WebhookResult{}, uncertainDeliveryFailure("notification.timeout")
		}
		return WebhookResult{}, uncertainDeliveryFailure("notification.network_failed")
	}
	defer response.Body.Close()
	reader := io.LimitReader(response.Body, sender.maximumResponseBytes+1)
	body, err := io.ReadAll(reader)
	if err != nil {
		return WebhookResult{}, uncertainDeliveryFailure("notification.network_failed")
	}
	if int64(len(body)) > sender.maximumResponseBytes {
		return WebhookResult{}, deliveryFailure("notification.response_too_large", false)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return WebhookResult{Status: response.StatusCode}, deliveryFailure(
			"notification.remote_rejected", response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500,
		)
	}
	return WebhookResult{Status: response.StatusCode}, nil
}

func (sender *WebhookSender) client(host string, addresses []netip.Addr) *http.Client {
	transport := &http.Transport{
		Proxy: nil, DisableCompression: true, DisableKeepAlives: true, ForceAttemptHTTP2: false,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: sender.rootCAs},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			requestedHost, port, err := net.SplitHostPort(address)
			if err != nil || !strings.EqualFold(strings.TrimSuffix(requestedHost, "."), host) {
				return nil, errors.New("webhook dial target changed")
			}
			return sender.dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].String(), port))
		},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   sender.timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errWebhookRedirect
		},
	}
}

var errWebhookRedirect = errors.New("webhook redirects are rejected")

func parseDestination(raw string) (*url.URL, error) {
	if len(raw) < 1 || len(raw) > 2048 || raw != strings.TrimSpace(raw) {
		return nil, errors.New("webhook URL is invalid")
	}
	destination, err := url.Parse(raw)
	if err != nil || destination.Scheme != "https" || destination.Hostname() == "" ||
		destination.User != nil || destination.Fragment != "" || destination.RawQuery == "" && strings.Contains(raw, "?") {
		return nil, errors.New("webhook URL is invalid")
	}
	if destination.Port() != "" {
		port, err := strconv.Atoi(destination.Port())
		if err != nil || port < 1 || port > 65535 {
			return nil, errors.New("webhook port is invalid")
		}
	}
	return destination, nil
}

func allPublicAddresses(addresses []netip.Addr) bool {
	for _, address := range addresses {
		address = address.Unmap()
		if !address.IsValid() || address.Zone() != "" || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
			address.IsLinkLocalUnicast() || reservedAddress(address) {
			return false
		}
	}
	return true
}

func reservedAddress(address netip.Addr) bool {
	for _, prefix := range reservedPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

var reservedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

func signPayload(key []byte, delivery WebhookDelivery) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(delivery.ID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(delivery.Payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func validDeliveryID(value string) bool {
	if len(value) < 1 || len(value) > 128 || !asciiLetter(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		char := value[index]
		if !asciiLetter(char) && (char < '0' || char > '9') && char != '_' && char != '-' {
			return false
		}
	}
	return true
}

func asciiLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func deliveryFailure(code string, retryable bool) error {
	return &DeliveryError{Code: code, Retryable: retryable}
}

func uncertainDeliveryFailure(code string) error {
	return &DeliveryError{Code: code, Retryable: true, Uncertain: true}
}

var _ error = (*DeliveryError)(nil)

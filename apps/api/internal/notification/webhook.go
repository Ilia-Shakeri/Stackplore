package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	RootCAs              *x509.CertPool
}

type WebhookSender struct {
	resolver             Resolver
	dialer               ContextDialer
	timeout              time.Duration
	maximumResponseBytes int64
	rootCAs              *x509.CertPool
}

type WebhookTarget struct {
	URL        string `json:"-"`
	SigningKey []byte `json:"-"`
}

func (WebhookTarget) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("[redacted webhook target]"))
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
	if !validOutboundOptions(outboundOptions{resolver: options.Resolver, dialer: options.Dialer, timeout: options.Timeout, maximumResponseBytes: options.MaximumResponseBytes}) {
		return nil, errors.New("webhook sender options are invalid")
	}
	return &WebhookSender{
		resolver: options.Resolver, dialer: options.Dialer, timeout: options.Timeout,
		maximumResponseBytes: options.MaximumResponseBytes,
		rootCAs:              options.RootCAs,
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
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Idempotency-Key", delivery.ID)
	headers.Set("X-Stackplore-Delivery-ID", delivery.ID)
	headers.Set("X-Stackplore-Signature", "sha256="+signPayload(target.SigningKey, delivery))
	status, _, err := sendOutbound(ctx, outboundOptions{resolver: sender.resolver, dialer: sender.dialer,
		timeout: sender.timeout, maximumResponseBytes: sender.maximumResponseBytes, rootCAs: sender.rootCAs},
		destination, http.MethodPost, delivery.Payload, headers)
	return WebhookResult{Status: status}, err
}

var errWebhookRedirect = errors.New("webhook redirects are rejected")

func parseDestination(raw string) (*url.URL, error) {
	if len(raw) < 1 || len(raw) > 2048 || raw != strings.TrimSpace(raw) {
		return nil, errors.New("webhook URL is invalid")
	}
	destination, err := url.Parse(raw)
	if err != nil || destination.Scheme != "https" || destination.Hostname() == "" || destination.Opaque != "" ||
		destination.User != nil || strings.Contains(raw, "#") || destination.RawQuery == "" && strings.Contains(raw, "?") || destination.String() != raw ||
		!validDestinationHost(destination.Hostname()) {
		return nil, errors.New("webhook URL is invalid")
	}
	if destination.Port() != "" {
		if !validDestinationPort(destination.Port()) {
			return nil, errors.New("webhook port is invalid")
		}
	}
	return destination, nil
}

func validDestinationHost(host string) bool {
	if host == "" || len(host) > 253 || host != strings.ToLower(host) || strings.HasSuffix(host, ".") {
		return false
	}
	if address, err := netip.ParseAddr(host); err == nil {
		return address.Zone() == "" && address.String() == host
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if character < 'a' || character > 'z' {
				if character < '0' || character > '9' {
					if character != '-' {
						return false
					}
				}
			}
		}
	}
	return true
}

func validDestinationPort(port string) bool {
	if len(port) < 1 || len(port) > 5 || len(port) > 1 && port[0] == '0' {
		return false
	}
	value, err := strconv.Atoi(port)
	return err == nil && value >= 1 && value <= 65535
}

func allPublicAddresses(addresses []netip.Addr) bool {
	for _, address := range addresses {
		address = address.Unmap()
		if !address.IsValid() || address.Zone() != "" || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
			address.IsLinkLocalUnicast() || address.Is6() && !publicIPv6Prefix.Contains(address) || reservedAddress(address) {
			return false
		}
	}
	return true
}

var publicIPv6Prefix = netip.MustParsePrefix("2000::/3")

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
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	// Protocol and transition ranges can encode another destination address.
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
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

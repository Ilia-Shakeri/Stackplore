package httpapi

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestNewProxyPolicyValidatesInput(t *testing.T) {
	t.Parallel()

	validPrefix := netip.MustParsePrefix("10.0.0.0/8")
	tests := []struct {
		name      string
		origin    Origin
		prefixes  []netip.Prefix
		wantError bool
	}{
		{
			name:     "valid",
			origin:   Origin{Scheme: "https", Authority: "example.test:8443"},
			prefixes: []netip.Prefix{validPrefix},
		},
		{
			name:      "bad scheme",
			origin:    Origin{Scheme: "ftp", Authority: "example.test"},
			wantError: true,
		},
		{
			name:      "blank authority",
			origin:    Origin{Scheme: "https"},
			wantError: true,
		},
		{
			name:      "invalid prefix",
			origin:    Origin{Scheme: "https", Authority: "example.test"},
			prefixes:  []netip.Prefix{{}},
			wantError: true,
		},
		{
			name:      "default route prefix",
			origin:    Origin{Scheme: "https", Authority: "example.test"},
			prefixes:  []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")},
			wantError: true,
		},
		{
			name:      "unmasked prefix",
			origin:    Origin{Scheme: "https", Authority: "example.test"},
			prefixes:  []netip.Prefix{netip.PrefixFrom(netip.MustParseAddr("10.0.0.1"), 8)},
			wantError: true,
		},
		{
			name:      "mapped prefix",
			origin:    Origin{Scheme: "https", Authority: "example.test"},
			prefixes:  []netip.Prefix{netip.MustParsePrefix("::ffff:192.0.2.0/120")},
			wantError: true,
		},
		{
			name:      "duplicate prefix",
			origin:    Origin{Scheme: "https", Authority: "example.test"},
			prefixes:  []netip.Prefix{validPrefix, validPrefix},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			policy, err := newProxyPolicy(test.origin, test.prefixes)
			if test.wantError {
				if err == nil {
					t.Fatal("newProxyPolicy() error = nil; want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("newProxyPolicy() error = %v", err)
			}
			if policy.origin != test.origin {
				t.Errorf("origin = %#v; want %#v", policy.origin, test.origin)
			}
			test.prefixes[0] = netip.MustParsePrefix("192.0.2.0/24")
			if policy.trusted[0] != validPrefix {
				t.Error("trusted prefixes share caller storage")
			}
		})
	}
}

func TestProxyResolveDirectClientAddresses(t *testing.T) {
	t.Parallel()

	policy := mustProxyPolicy(t, Origin{Scheme: "https", Authority: "example.test"})
	tests := []struct {
		name       string
		remoteAddr string
		wantIP     string
	}{
		{name: "IPv4", remoteAddr: "192.0.2.10:43100", wantIP: "192.0.2.10"},
		{name: "IPv6", remoteAddr: "[2001:db8::10]:43100", wantIP: "2001:db8::10"},
		{name: "mapped IPv4", remoteAddr: "[::ffff:192.0.2.10]:43100", wantIP: "192.0.2.10"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, "https://example.test/api/v1", nil)
			request.RemoteAddr = test.remoteAddr
			request.TLS = &tls.ConnectionState{}

			info := policy.resolve(request)
			assertNetworkInfo(t, info, test.wantIP, "https", "example.test", true)
			if info.untrustedForwarding {
				t.Error("untrustedForwarding = true; want false")
			}
		})
	}
}

func TestProxyResolveMarksUntrustedForwarding(t *testing.T) {
	t.Parallel()

	policy := mustProxyPolicy(t, Origin{Scheme: "http", Authority: "example.test"})
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "standard field", key: "Forwarded", value: "for=203.0.113.10"},
		{name: "empty standard field", key: "Forwarded", value: ""},
		{name: "client field", key: "X-Forwarded-For", value: "203.0.113.10"},
		{name: "host field", key: "X-Forwarded-Host", value: "example.test"},
		{name: "scheme field", key: "X-Forwarded-Proto", value: "https"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, "http://example.test/api/v1", nil)
			request.RemoteAddr = "192.0.2.10:43100"
			request.Header.Add(test.key, test.value)

			info := policy.resolve(request)
			if info.err != nil {
				t.Fatalf("resolve() error = %v", info.err)
			}
			if !info.untrustedForwarding {
				t.Error("untrustedForwarding = false; want true")
			}
			if info.clientVerified {
				t.Error("clientVerified = true; want false")
			}
			if info.clientIP.String() != "192.0.2.10" {
				t.Errorf("clientIP = %q; want %q", info.clientIP, "192.0.2.10")
			}
		})
	}
}

func TestProxyResolveTrustedChain(t *testing.T) {
	t.Parallel()

	policy := mustProxyPolicy(
		t,
		Origin{Scheme: "https", Authority: "example.test"},
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("2001:db8:ffff::/48"),
	)
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		wantIP     string
		verified   bool
	}{
		{
			name:       "IPv4 chain",
			remoteAddr: "10.0.0.5:43100",
			forwarded:  "203.0.113.10, 10.0.0.4",
			wantIP:     "203.0.113.10",
			verified:   true,
		},
		{
			name:       "IPv6 chain",
			remoteAddr: "[2001:db8:ffff::5]:43100",
			forwarded:  "2001:db8:1::10, 2001:db8:ffff::4",
			wantIP:     "2001:db8:1::10",
			verified:   true,
		},
		{
			name:       "mapped peer and client",
			remoteAddr: "[::ffff:10.0.0.5]:43100",
			forwarded:  "::ffff:192.0.2.10, 10.0.0.4",
			wantIP:     "192.0.2.10",
			verified:   true,
		},
		{
			name:       "all trusted",
			remoteAddr: "10.0.0.5:43100",
			forwarded:  "10.0.0.3, 10.0.0.4",
			wantIP:     "10.0.0.3",
			verified:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, "http://internal/api/v1", nil)
			request.RemoteAddr = test.remoteAddr
			request.Header.Set("X-Forwarded-For", test.forwarded)
			request.Header.Set("X-Forwarded-Host", "EXAMPLE.TEST:443")
			request.Header.Set("X-Forwarded-Proto", "https")

			info := policy.resolve(request)
			assertNetworkInfo(t, info, test.wantIP, "https", "example.test", test.verified)
			if info.untrustedForwarding {
				t.Error("untrustedForwarding = true; want false")
			}
		})
	}
}

func TestProxyResolveTrustedPeerWithoutClientChainIsUnverified(t *testing.T) {
	t.Parallel()

	policy := mustProxyPolicy(
		t,
		Origin{Scheme: "https", Authority: "example.test"},
		netip.MustParsePrefix("10.0.0.0/8"),
	)
	request := httptest.NewRequest(http.MethodGet, "http://example.test/api/v1", nil)
	request.RemoteAddr = "10.0.0.5:43100"
	request.Header.Set("X-Forwarded-Host", "example.test")
	request.Header.Set("X-Forwarded-Proto", "https")

	info := policy.resolve(request)
	assertNetworkInfo(t, info, "10.0.0.5", "https", "example.test", false)
}

func TestProxyResolveRejectsBadTrustedForwarding(t *testing.T) {
	t.Parallel()

	policy := mustProxyPolicy(
		t,
		Origin{Scheme: "https", Authority: "example.test"},
		netip.MustParsePrefix("10.0.0.0/8"),
	)
	tests := []struct {
		name   string
		values http.Header
	}{
		{name: "standard field", values: http.Header{"Forwarded": {""}}},
		{name: "mixed standard fields", values: http.Header{"Forwarded": {"for=203.0.113.10"}, "X-Forwarded-For": {"203.0.113.10"}}},
		{name: "empty scheme", values: http.Header{"X-Forwarded-Proto": {""}}},
		{name: "duplicate scheme", values: http.Header{"X-Forwarded-Proto": {"https", "https"}}},
		{name: "scheme chain", values: http.Header{"X-Forwarded-Proto": {"https,http"}}},
		{name: "uppercase scheme", values: http.Header{"X-Forwarded-Proto": {"HTTPS"}}},
		{name: "empty host", values: http.Header{"X-Forwarded-Host": {""}}},
		{name: "duplicate host", values: http.Header{"X-Forwarded-Host": {"example.test", "example.test"}}},
		{name: "host chain", values: http.Header{"X-Forwarded-Host": {"example.test,internal"}}},
		{name: "empty client", values: http.Header{"X-Forwarded-For": {""}}},
		{name: "client with port", values: http.Header{"X-Forwarded-For": {"203.0.113.10:43100"}}},
		{name: "bracketed client", values: http.Header{"X-Forwarded-For": {"[2001:db8::10]"}}},
		{name: "zoned client", values: http.Header{"X-Forwarded-For": {"fe80::1%eth0"}}},
		{name: "unspecified client", values: http.Header{"X-Forwarded-For": {"0.0.0.0"}}},
		{name: "multicast client", values: http.Header{"X-Forwarded-For": {"224.0.0.1"}}},
		{name: "too many clients", values: http.Header{"X-Forwarded-For": {strings.Repeat("192.0.2.1,", maximumForwardedHops) + "192.0.2.1"}}},
		{name: "client field too long", values: http.Header{"X-Forwarded-For": {strings.Repeat("1", 1025)}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, "http://example.test/api/v1", nil)
			request.RemoteAddr = "10.0.0.5:43100"
			request.Header = test.values.Clone()

			info := policy.resolve(request)
			if info.err == nil {
				t.Fatal("resolve() error = nil; want error")
			}
			if info.clientVerified {
				t.Error("clientVerified = true; want false")
			}
		})
	}
}

func TestCanonicalAuthority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     string
		scheme    string
		want      string
		wantError bool
	}{
		{name: "host", value: "example.test", scheme: "https", want: "example.test"},
		{name: "case and root dot", value: "EXAMPLE.TEST.", scheme: "https", want: "example.test"},
		{name: "default HTTP port", value: "example.test:80", scheme: "http", want: "example.test"},
		{name: "default HTTPS port", value: "example.test:443", scheme: "https", want: "example.test"},
		{name: "custom port", value: "example.test:8443", scheme: "https", want: "example.test:8443"},
		{name: "IPv6", value: "[2001:db8::1]:8443", scheme: "https", want: "[2001:db8::1]:8443"},
		{name: "mapped IPv4", value: "[::ffff:192.0.2.1]:8080", scheme: "http", want: "192.0.2.1:8080"},
		{name: "blank", wantError: true},
		{name: "path", value: "example.test/path", scheme: "https", wantError: true},
		{name: "user info", value: "user@example.test", scheme: "https", wantError: true},
		{name: "service port", value: "example.test:http", scheme: "http", wantError: true},
		{name: "zero port", value: "example.test:0", scheme: "https", wantError: true},
		{name: "large port", value: "example.test:65536", scheme: "https", wantError: true},
		{name: "leading zero port", value: "example.test:08443", scheme: "https", wantError: true},
		{name: "zone", value: "[fe80::1%25eth0]", scheme: "https", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := canonicalAuthority(test.value, test.scheme)
			if test.wantError {
				if err == nil {
					t.Fatalf("canonicalAuthority() = %q, nil; want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("canonicalAuthority() error = %v", err)
			}
			if got != test.want {
				t.Errorf("canonicalAuthority() = %q; want %q", got, test.want)
			}
		})
	}
}

func TestProxyPublicOriginGate(t *testing.T) {
	t.Parallel()

	policy := mustProxyPolicy(
		t,
		Origin{Scheme: "https", Authority: "example.test"},
		netip.MustParsePrefix("10.0.0.0/8"),
	)
	passed := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := policy.middleware(policy.requirePublicOrigin(passed))
	tests := []struct {
		name       string
		url        string
		remoteAddr string
		tls        bool
		headers    http.Header
		wantStatus int
	}{
		{
			name:       "direct HTTPS",
			url:        "https://example.test/api/v1",
			remoteAddr: "192.0.2.10:43100",
			tls:        true,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "trusted edge",
			url:        "http://internal/api/v1",
			remoteAddr: "10.0.0.5:43100",
			headers: http.Header{
				"X-Forwarded-For":   {"203.0.113.10"},
				"X-Forwarded-Host":  {"example.test"},
				"X-Forwarded-Proto": {"https"},
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "wrong scheme",
			url:        "http://example.test/api/v1",
			remoteAddr: "192.0.2.10:43100",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong host",
			url:        "https://other.test/api/v1",
			remoteAddr: "192.0.2.10:43100",
			tls:        true,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "untrusted forward hint",
			url:        "https://example.test/api/v1",
			remoteAddr: "192.0.2.10:43100",
			tls:        true,
			headers:    http.Header{"X-Forwarded-For": {"203.0.113.10"}},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, test.url, nil)
			request.RemoteAddr = test.remoteAddr
			request.Header = test.headers.Clone()
			if test.tls {
				request.TLS = &tls.ConnectionState{}
			}
			request = request.WithContext(context.WithValue(request.Context(), requestIDContextKey{}, "req_proxy_test"))
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Errorf("status = %d; want %d", response.Code, test.wantStatus)
			}
		})
	}
}

func TestRemoteIPRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"",
		"192.0.2.1",
		"host:80",
		"0.0.0.0:80",
		"224.0.0.1:80",
		"[::]:80",
		"[ff02::1]:80",
		"[fe80::1%eth0]:80",
		"192.0.2.1:",
		"192.0.2.1:0",
		"192.0.2.1:080",
		"192.0.2.1:http",
		"192.0.2.1:65536",
		"192.0.2.1:0:1",
	} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			if _, err := remoteIP(value); err == nil {
				t.Fatalf("remoteIP(%q) error = nil; want error", value)
			}
		})
	}
}

func mustProxyPolicy(t *testing.T, origin Origin, trusted ...netip.Prefix) proxyPolicy {
	t.Helper()
	policy, err := newProxyPolicy(origin, trusted)
	if err != nil {
		t.Fatalf("newProxyPolicy() error = %v", err)
	}
	return policy
}

func assertNetworkInfo(
	t *testing.T,
	info networkInfo,
	wantIP string,
	wantScheme string,
	wantAuthority string,
	wantVerified bool,
) {
	t.Helper()
	if info.err != nil {
		t.Fatalf("resolve() error = %v", info.err)
	}
	if info.clientIP.String() != wantIP {
		t.Errorf("clientIP = %q; want %q", info.clientIP, wantIP)
	}
	if info.scheme != wantScheme {
		t.Errorf("scheme = %q; want %q", info.scheme, wantScheme)
	}
	if info.authority != wantAuthority {
		t.Errorf("authority = %q; want %q", info.authority, wantAuthority)
	}
	if info.clientVerified != wantVerified {
		t.Errorf("clientVerified = %t; want %t", info.clientVerified, wantVerified)
	}
}

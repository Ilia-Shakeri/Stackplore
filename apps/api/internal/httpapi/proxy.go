package httpapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

const maximumForwardedHops = 16

var errInvalidRequestNetwork = errors.New("invalid request network metadata")

type Origin struct {
	Scheme    string
	Authority string
}

type networkContextKey struct{}

type networkInfo struct {
	clientIP            netip.Addr
	scheme              string
	authority           string
	untrustedForwarding bool
	clientVerified      bool
	err                 error
}

type proxyPolicy struct {
	origin  Origin
	trusted []netip.Prefix
}

func newProxyPolicy(origin Origin, trusted []netip.Prefix) (proxyPolicy, error) {
	if origin.Scheme != "http" && origin.Scheme != "https" {
		return proxyPolicy{}, errInvalidRequestNetwork
	}
	authority, err := canonicalAuthority(origin.Authority, origin.Scheme)
	if err != nil {
		return proxyPolicy{}, err
	}
	seen := make(map[netip.Prefix]struct{}, len(trusted))
	for _, prefix := range trusted {
		if !prefix.IsValid() ||
			prefix.Bits() == 0 ||
			prefix != prefix.Masked() ||
			prefix.Addr().Is4In6() {
			return proxyPolicy{}, errInvalidRequestNetwork
		}
		if _, exists := seen[prefix]; exists {
			return proxyPolicy{}, errInvalidRequestNetwork
		}
		seen[prefix] = struct{}{}
	}
	return proxyPolicy{
		origin:  Origin{Scheme: origin.Scheme, Authority: authority},
		trusted: append([]netip.Prefix(nil), trusted...),
	}, nil
}

func (policy proxyPolicy) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := policy.resolve(r)
		ctx := context.WithValue(r.Context(), networkContextKey{}, info)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (policy proxyPolicy) requirePublicOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := networkInfoFromContext(r.Context())
		if info.err != nil || info.untrustedForwarding || info.scheme != policy.origin.Scheme || info.authority != policy.origin.Authority {
			writeProblem(w, r, invalidRequestProblem)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (policy proxyPolicy) resolve(r *http.Request) networkInfo {
	remote, err := remoteIP(r.RemoteAddr)
	if err != nil {
		return networkInfo{err: err}
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	authority := r.Host
	forwardedPresent := hasForwardingHeaders(r.Header)
	trustedPeer := policy.containsTrusted(remote)
	if forwardedPresent && !trustedPeer {
		canonical, canonicalErr := canonicalAuthority(authority, scheme)
		return networkInfo{
			clientIP:            remote,
			scheme:              scheme,
			authority:           canonical,
			untrustedForwarding: true,
			clientVerified:      false,
			err:                 canonicalErr,
		}
	}

	clientIP := remote
	clientVerified := !trustedPeer
	if trustedPeer {
		if len(r.Header.Values("Forwarded")) > 0 {
			return networkInfo{clientIP: remote, err: errInvalidRequestNetwork}
		}
		if values := r.Header.Values("X-Forwarded-Proto"); len(values) > 0 {
			value := values[0]
			if len(values) != 1 || strings.Contains(value, ",") || value != "http" && value != "https" {
				return networkInfo{clientIP: remote, err: errInvalidRequestNetwork}
			}
			scheme = value
		}
		if values := r.Header.Values("X-Forwarded-Host"); len(values) > 0 {
			value := values[0]
			if len(values) != 1 || value == "" || strings.Contains(value, ",") {
				return networkInfo{clientIP: remote, err: errInvalidRequestNetwork}
			}
			authority = value
		}
		if values := r.Header.Values("X-Forwarded-For"); len(values) > 0 {
			value := strings.Join(values, ",")
			chain, parseErr := parseForwardedFor(value)
			if parseErr != nil {
				return networkInfo{clientIP: remote, err: parseErr}
			}
			chain = append(chain, remote)
			clientIP = policy.firstUntrustedFromRight(chain)
			clientVerified = !policy.containsTrusted(clientIP)
		}
	}

	canonical, err := canonicalAuthority(authority, scheme)
	return networkInfo{
		clientIP:       clientIP,
		scheme:         scheme,
		authority:      canonical,
		clientVerified: clientVerified,
		err:            err,
	}
}

func (policy proxyPolicy) containsTrusted(address netip.Addr) bool {
	for _, prefix := range policy.trusted {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func (policy proxyPolicy) firstUntrustedFromRight(chain []netip.Addr) netip.Addr {
	index := len(chain) - 1
	for index > 0 && policy.containsTrusted(chain[index]) {
		index--
	}
	return chain[index]
}

func networkInfoFromContext(ctx context.Context) networkInfo {
	info, _ := ctx.Value(networkContextKey{}).(networkInfo)
	return info
}

func remoteIP(value string) (netip.Addr, error) {
	host, port, err := net.SplitHostPort(value)
	if err != nil || !validNetworkPort(port) {
		return netip.Addr{}, errInvalidRequestNetwork
	}
	address, err := netip.ParseAddr(host)
	if err != nil || address.Zone() != "" || address.IsUnspecified() || address.IsMulticast() {
		return netip.Addr{}, errInvalidRequestNetwork
	}
	return address.Unmap(), nil
}

func hasForwardingHeaders(header http.Header) bool {
	for _, name := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto"} {
		if len(header.Values(name)) > 0 {
			return true
		}
	}
	return false
}

func parseForwardedFor(value string) ([]netip.Addr, error) {
	if len(value) > 1024 {
		return nil, errInvalidRequestNetwork
	}
	parts := strings.Split(value, ",")
	if len(parts) < 1 || len(parts) > maximumForwardedHops {
		return nil, errInvalidRequestNetwork
	}
	addresses := make([]netip.Addr, 0, len(parts))
	for _, part := range parts {
		address, err := netip.ParseAddr(strings.TrimSpace(part))
		if err != nil || address.Zone() != "" || address.IsUnspecified() || address.IsMulticast() {
			return nil, errInvalidRequestNetwork
		}
		addresses = append(addresses, address.Unmap())
	}
	return addresses, nil
}

func canonicalAuthority(value string, scheme string) (string, error) {
	if value == "" || len(value) > 255 || strings.ContainsAny(value, "\\/?#@\x00\r\n\t ") {
		return "", errInvalidRequestNetwork
	}
	parsed, err := url.Parse("//" + value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Path != "" {
		return "", errInvalidRequestNetwork
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if !validHost(host) {
		return "", errInvalidRequestNetwork
	}
	port := parsed.Port()
	if port != "" && !validNetworkPort(port) {
		return "", errInvalidRequestNetwork
	}
	if (port == "80" && scheme == "http") || (port == "443" && scheme == "https") {
		port = ""
	}
	if address, err := netip.ParseAddr(host); err == nil {
		host = address.Unmap().String()
		if address.Unmap().Is6() {
			host = "[" + host + "]"
		}
	}
	if port != "" {
		return net.JoinHostPort(strings.Trim(host, "[]"), port), nil
	}
	return host, nil
}

func validNetworkPort(value string) bool {
	if value == "" || len(value) > 5 || len(value) > 1 && value[0] == '0' {
		return false
	}
	for index := range len(value) {
		if !asciiDigit(value[index]) {
			return false
		}
	}
	port, err := strconv.ParseUint(value, 10, 16)
	return err == nil && port != 0
}

func validHost(host string) bool {
	if host == "" {
		return false
	}
	if address, err := netip.ParseAddr(host); err == nil {
		return address.Zone() == ""
	}
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for index := range len(label) {
			character := label[index]
			if !asciiLetter(character) && !asciiDigit(character) && character != '-' {
				return false
			}
		}
	}
	return true
}

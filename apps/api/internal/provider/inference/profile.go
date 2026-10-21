// Package inference implements an operator-bound local inference adapter.
package inference

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	port "github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type Dialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

// Profile is operator-owned. No value in this type is accepted from a browser.
type Profile struct {
	ID                     string
	Origin                 string
	ApprovedAddresses      []netip.Addr
	ExpectedAlias          string
	ExpectedBuild          string
	ExpectedTemplateSHA256 string
	ExpectedContextTokens  int
	Generation             uint64
}

func (Profile) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected inference profile]")
}
func (Profile) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type Options struct {
	Resolver Resolver
	Dialer   Dialer
	RootCAs  *x509.CertPool
	Timeout  time.Duration
}

type Local struct {
	profile   Profile
	origin    *url.URL
	addresses map[netip.Addr]bool
	resolver  Resolver
	dialer    Dialer
	rootCAs   *x509.CertPool
	timeout   time.Duration
	revision  string
}

// NewLocal permits only exact loopback or private destinations, not remote egress.
func NewLocal(profile Profile, options Options) (*Local, error) {
	origin, err := url.Parse(profile.Origin)
	if err != nil || origin == nil || len(profile.Origin) > 512 || origin.Scheme != "https" && origin.Scheme != "http" || origin.Hostname() == "" || origin.User != nil || origin.RawQuery != "" || origin.ForceQuery || origin.Fragment != "" || origin.RawFragment != "" || origin.RawPath != "" || origin.Path != "" && origin.Path != "/" || strings.ContainsAny(origin.Hostname(), "%\\\x00\r\n") {
		return nil, port.ErrInvalid
	}
	if !safeCode(profile.ID, 64) || profile.Generation == 0 || !strings.HasPrefix(profile.ExpectedAlias, "sha256-") || !isDigest(strings.TrimPrefix(profile.ExpectedAlias, "sha256-")) || !safeText(profile.ExpectedBuild, 256) || !isDigest(profile.ExpectedTemplateSHA256) || profile.ExpectedContextTokens < port.MaximumInputTokens+port.MaximumOutputTokens || len(profile.ApprovedAddresses) < 1 || len(profile.ApprovedAddresses) > 16 {
		return nil, port.ErrInvalid
	}
	if origin.Port() != "" {
		n, parseErr := strconv.Atoi(origin.Port())
		if parseErr != nil || n < 1 || n > 65535 || strconv.Itoa(n) != origin.Port() {
			return nil, port.ErrInvalid
		}
	}
	approved := make(map[netip.Addr]bool, len(profile.ApprovedAddresses))
	for _, address := range profile.ApprovedAddresses {
		address = address.Unmap()
		if !allowedLocalAddress(address) || origin.Scheme == "http" && !address.IsLoopback() || approved[address] {
			return nil, port.ErrInvalid
		}
		approved[address] = true
	}
	if options.Timeout == 0 {
		options.Timeout = port.MaximumCallDuration
	}
	if options.Timeout < time.Millisecond || options.Timeout > port.MaximumCallDuration {
		return nil, port.ErrInvalid
	}
	if options.Resolver == nil {
		options.Resolver = net.DefaultResolver
	}
	if options.Dialer == nil {
		options.Dialer = &net.Dialer{Timeout: min(options.Timeout, 5*time.Second), KeepAlive: -1}
	}
	if options.RootCAs != nil {
		options.RootCAs = options.RootCAs.Clone()
	}
	profile.ApprovedAddresses = append([]netip.Addr(nil), profile.ApprovedAddresses...)
	origin.Path = ""
	revision := sha256.Sum256([]byte(profile.ID + "\x00" + profile.Origin + "\x00" + profile.ExpectedAlias + "\x00" + profile.ExpectedBuild + "\x00" + profile.ExpectedTemplateSHA256 + "\x00" + strconv.Itoa(profile.ExpectedContextTokens) + "\x00" + strconv.FormatUint(profile.Generation, 10)))
	return &Local{profile: profile, origin: origin, addresses: approved, resolver: options.Resolver, dialer: options.Dialer, rootCAs: options.RootCAs, timeout: options.Timeout, revision: hex.EncodeToString(revision[:])}, nil
}

func allowedLocalAddress(address netip.Addr) bool {
	return address.IsValid() && address.Zone() == "" && address != privateMetadataAddress && !address.IsUnspecified() && !address.IsMulticast() && !address.IsLinkLocalUnicast() && !address.IsLinkLocalMulticast() && (address.IsLoopback() || address.IsPrivate())
}

var privateMetadataAddress = netip.MustParseAddr("fd00:ec2::254")

func isDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

func safeCode(value string, maximum int) bool {
	if len(value) < 1 || len(value) > maximum {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func safeText(value string, maximum int) bool {
	return len(value) > 0 && len(value) <= maximum && !strings.ContainsAny(value, "\x00\r\n")
}

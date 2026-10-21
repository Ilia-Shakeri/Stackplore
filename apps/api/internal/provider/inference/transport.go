package inference

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"

	port "github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

const maximumWireBytes = 1024 * 1024

var errRedirect = errors.New("inference.redirect_rejected")

func (local *Local) exchange(ctx context.Context, path string, payload []byte) ([]byte, error) {
	if local == nil || ctx == nil || ctx.Err() != nil {
		return nil, port.ErrCancelled
	}
	method := http.MethodPost
	switch path {
	case "/props", "/v1/models":
		method = http.MethodGet
	case "/apply-template", "/tokenize", "/completion":
	default:
		return nil, port.ErrInvalid
	}
	if len(payload) > maximumWireBytes {
		return nil, port.ErrInputLimit
	}
	work, cancel := context.WithTimeout(ctx, local.timeout)
	defer cancel()
	host := local.origin.Hostname()
	addresses, err := local.resolve(work, host)
	if err != nil {
		return nil, err
	}
	portNumber := local.origin.Port()
	if portNumber == "" {
		portNumber = "443"
		if local.origin.Scheme == "http" {
			portNumber = "80"
		}
	}
	target := *local.origin
	target.Path = path
	request, err := http.NewRequestWithContext(work, method, target.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, port.ErrInvalid
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	transport := &http.Transport{
		Proxy: nil, DisableCompression: true, DisableKeepAlives: true, ForceAttemptHTTP2: false,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: local.rootCAs, ServerName: host},
		ResponseHeaderTimeout: local.timeout, MaxResponseHeaderBytes: 16 * 1024,
		DialContext: func(dialContext context.Context, network, address string) (net.Conn, error) {
			requestedHost, requestedPort, splitErr := net.SplitHostPort(address)
			if splitErr != nil || !strings.EqualFold(requestedHost, host) || requestedPort != portNumber {
				return nil, port.ErrInvalid
			}
			return local.dialer.DialContext(dialContext, network, net.JoinHostPort(addresses[0].String(), portNumber))
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: local.timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errRedirect }}
	response, err := client.Do(request)
	if err != nil {
		return nil, port.ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > maximumWireBytes || response.Header.Get("Content-Encoding") != "" && response.Header.Get("Content-Encoding") != "identity" {
		return nil, port.ErrUnavailable
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, maximumWireBytes+1))
	if err != nil || len(contents) > maximumWireBytes {
		clear(contents)
		return nil, port.ErrUnavailable
	}
	return contents, nil
}

func (local *Local) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	var addresses []netip.Addr
	if literal, err := netip.ParseAddr(host); err == nil {
		addresses = []netip.Addr{literal.Unmap()}
	} else {
		var lookupErr error
		addresses, lookupErr = local.resolver.LookupNetIP(ctx, "ip", host)
		if lookupErr != nil {
			return nil, port.ErrUnavailable
		}
	}
	if len(addresses) == 0 || len(addresses) > 16 {
		return nil, port.ErrUnavailable
	}
	checked := make([]netip.Addr, len(addresses))
	for index, address := range addresses {
		address = address.Unmap()
		if !allowedLocalAddress(address) || !local.addresses[address] || local.origin.Scheme == "http" && !address.IsLoopback() {
			return nil, port.ErrUnavailable
		}
		checked[index] = address
	}
	return checked, nil
}

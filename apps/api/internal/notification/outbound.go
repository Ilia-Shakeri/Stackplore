package notification

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type outboundOptions struct {
	resolver             Resolver
	dialer               ContextDialer
	timeout              time.Duration
	maximumResponseBytes int64
	rootCAs              *x509.CertPool
}

func validOutboundOptions(options outboundOptions) bool {
	return options.resolver != nil && options.dialer != nil && options.timeout >= time.Second &&
		options.timeout <= 30*time.Second && options.maximumResponseBytes >= 1 && options.maximumResponseBytes <= 1024*1024
}

// Each attempt resolves and pins its public addresses before sending any credential.
func sendOutbound(ctx context.Context, options outboundOptions, destination *url.URL, method string, payload []byte, headers http.Header) (int, []byte, error) {
	attemptContext, cancel := context.WithTimeout(ctx, options.timeout)
	defer cancel()
	host := destination.Hostname()
	addresses, err := options.resolver.LookupNetIP(attemptContext, "ip", host)
	if err != nil {
		if attemptContext.Err() != nil {
			return 0, nil, deliveryFailure("notification.timeout", true)
		}
		return 0, nil, deliveryFailure("notification.dns_failed", true)
	}
	if len(addresses) == 0 || len(addresses) > 32 || !allPublicAddresses(addresses) {
		return 0, nil, deliveryFailure("notification.destination_rejected", false)
	}
	request, err := http.NewRequestWithContext(attemptContext, method, destination.String(), bytes.NewReader(payload))
	if err != nil {
		return 0, nil, deliveryFailure("notification.invalid_request", false)
	}
	request.Header = headers.Clone()
	client := outboundClient(options, host, destination.Port(), addresses)
	defer client.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(err, errWebhookRedirect) {
			return 0, nil, deliveryFailure("notification.redirect_rejected", false)
		}
		if attemptContext.Err() != nil {
			return 0, nil, uncertainDeliveryFailure("notification.timeout")
		}
		return 0, nil, uncertainDeliveryFailure("notification.network_failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, options.maximumResponseBytes+1))
	if err != nil {
		if attemptContext.Err() != nil {
			return response.StatusCode, nil, uncertainDeliveryFailure("notification.timeout")
		}
		return response.StatusCode, nil, uncertainDeliveryFailure("notification.network_failed")
	}
	if int64(len(body)) > options.maximumResponseBytes {
		return response.StatusCode, nil, uncertainDeliveryFailure("notification.response_too_large")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return response.StatusCode, nil, deliveryFailure("notification.remote_rejected", response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500)
	}
	return response.StatusCode, body, nil
}

func outboundClient(options outboundOptions, host, port string, addresses []netip.Addr) *http.Client {
	if port == "" {
		port = "443"
	}
	transport := &http.Transport{
		Proxy: nil, DisableCompression: true, DisableKeepAlives: true, ForceAttemptHTTP2: false,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: options.rootCAs},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			requestedHost, requestedPort, err := net.SplitHostPort(address)
			if err != nil || !strings.EqualFold(requestedHost, host) || requestedPort != port {
				return nil, errors.New("notification dial target changed")
			}
			return options.dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].Unmap().String(), port))
		},
	}
	return &http.Client{Transport: transport, Timeout: options.timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errWebhookRedirect }}
}

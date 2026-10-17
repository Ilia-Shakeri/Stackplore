package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/docker"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/kubernetes"
)

func configuredProductProviders(settings config.Config) (providers []httpapi.ProductProvider, err error) {
	if config.Validate(settings) != nil {
		return nil, errProductStart
	}
	providers = []httpapi.ProductProvider{demo.MustNew()}
	ownedProviders := providers
	defer func() {
		if err != nil {
			closeProductProviders(ownedProviders)
		}
	}()
	if settings.DockerEnabled {
		transport, origin, transportErr := dockerProviderTransport(settings)
		if transportErr != nil {
			return nil, errProductStart
		}
		collector, collectorErr := docker.NewEngineCollector(docker.EngineCollectorOptions{Client: &http.Client{Transport: transport, Timeout: settings.RequestTimeout}, BaseURL: origin, MaximumItems: 10000, MaximumResponse: 8 << 20, InspectConcurrency: 8})
		if collectorErr != nil {
			transport.CloseIdleConnections()
			return nil, errProductStart
		}
		var provider httpapi.ProductProvider
		var providerErr error
		if settings.DockerWriteEnabled {
			writerTransport, writerOrigin, writerErr := dockerWriterTransport(settings)
			if writerErr != nil {
				_ = collector.Close()
				return nil, errProductStart
			}
			writer, writerErr := docker.NewSDKWriter(docker.WriterOptions{Client: &http.Client{Transport: writerTransport, Timeout: settings.RequestTimeout}, BaseURL: writerOrigin, MaximumResponse: 8 << 20})
			if writerErr != nil {
				writerTransport.CloseIdleConnections()
				_ = collector.Close()
				return nil, errProductStart
			}
			provider, providerErr = docker.NewWithOperations("source_docker", "Docker Engine", collector, writer)
		} else {
			provider, providerErr = docker.New("source_docker", "Docker Engine", collector)
		}
		if providerErr != nil {
			_ = collector.Close()
			return nil, errProductStart
		}
		providers = append(providers, provider)
		ownedProviders = providers
	}
	if settings.KubernetesEnabled {
		tokenBytes, readErr := readBoundedFile(settings.KubernetesTokenFile, 16<<10)
		if readErr != nil {
			return nil, errProductStart
		}
		token := strings.TrimSpace(string(tokenBytes))
		clear(tokenBytes)
		roots, readErr := providerCertPool(settings.KubernetesCAFile)
		if readErr != nil {
			return nil, errProductStart
		}
		transport := boundedProviderTransport()
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "kubernetes.default.svc"}
		collector, collectorErr := kubernetes.NewAPICollector(kubernetes.APICollectorOptions{Client: &http.Client{Transport: transport, Timeout: settings.RequestTimeout}, BearerToken: token,
			TokenReader: func(ctx context.Context) (string, error) {
				if ctx == nil || ctx.Err() != nil {
					return "", errProductStart
				}
				data, err := readBoundedFile(settings.KubernetesTokenFile, 16<<10)
				if err != nil {
					return "", errProductStart
				}
				defer clear(data)
				if ctx.Err() != nil {
					return "", errProductStart
				}
				return strings.TrimSpace(string(data)), nil
			},
			Namespaces: settings.KubernetesNamespaces, ClusterScoped: settings.KubernetesClusterObservation, MaximumItems: 10000, MaximumResponse: 8 << 20})
		if collectorErr != nil {
			transport.CloseIdleConnections()
			return nil, errProductStart
		}
		var provider httpapi.ProductProvider
		var providerErr error
		if settings.KubernetesWriteEnabled {
			writerTokenBytes, tokenErr := readBoundedFile(settings.KubernetesWriterTokenFile, 16<<10)
			if tokenErr != nil {
				_ = collector.Close()
				return nil, errProductStart
			}
			writerToken := strings.TrimSpace(string(writerTokenBytes))
			clear(writerTokenBytes)
			writerRoots, certErr := providerCertPool(settings.KubernetesWriterCAFile)
			if certErr != nil {
				_ = collector.Close()
				return nil, errProductStart
			}
			writerTransport := boundedProviderTransport()
			writerTransport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: writerRoots, ServerName: "kubernetes.default.svc"}
			writer, writerErr := kubernetes.NewSDKWriter(kubernetes.WriterOptions{Client: &http.Client{Transport: writerTransport, Timeout: settings.RequestTimeout}, BearerToken: writerToken,
				TokenReader: func(ctx context.Context) (string, error) {
					if ctx == nil || ctx.Err() != nil {
						return "", errProductStart
					}
					data, readErr := readBoundedFile(settings.KubernetesWriterTokenFile, 16<<10)
					if readErr != nil {
						return "", errProductStart
					}
					defer clear(data)
					if ctx.Err() != nil {
						return "", errProductStart
					}
					return strings.TrimSpace(string(data)), nil
				}, Namespaces: settings.KubernetesWriterNamespaces, MaximumResponse: 256 << 10})
			if writerErr != nil {
				writerTransport.CloseIdleConnections()
				_ = collector.Close()
				return nil, errProductStart
			}
			provider, providerErr = kubernetes.NewWithOperations("source_kubernetes", "Kubernetes", collector, writer)
		} else {
			provider, providerErr = kubernetes.New("source_kubernetes", "Kubernetes", collector)
		}
		if providerErr != nil {
			_ = collector.Close()
			return nil, errProductStart
		}
		providers = append(providers, provider)
		ownedProviders = providers
	}
	return providers, nil
}

func closeProductProviders(providers []httpapi.ProductProvider) {
	for _, provider := range providers {
		if closer, ok := provider.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}
}

func boundedProviderTransport() *http.Transport {
	return &http.Transport{Proxy: nil, DisableCompression: true, MaxIdleConns: 8, MaxIdleConnsPerHost: 8, MaxConnsPerHost: 8,
		DialContext: (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, IdleConnTimeout: 30 * time.Second, MaxResponseHeaderBytes: 64 << 10}
}

func dockerProviderTransport(settings config.Config) (*http.Transport, string, error) {
	transport := boundedProviderTransport()
	if settings.DockerAccessProfile != config.DockerProfileRestrictedProxy {
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "unix", settings.DockerSocketPath)
		}
		return transport, "http://docker", nil
	}
	origin, err := url.Parse(settings.DockerProxyOrigin)
	if err != nil || origin.Scheme != "https" || origin.Hostname() == "" {
		return nil, "", errProductStart
	}
	roots, err := providerCertPool(settings.DockerProxyCAFile)
	if err != nil {
		return nil, "", errProductStart
	}
	certificateBytes, err := readBoundedFile(settings.DockerProxyCertFile, 1<<20)
	if err != nil {
		return nil, "", errProductStart
	}
	defer clear(certificateBytes)
	keyBytes, err := readBoundedFile(settings.DockerProxyKeyFile, 1<<20)
	if err != nil {
		return nil, "", errProductStart
	}
	defer clear(keyBytes)
	identity, err := tls.X509KeyPair(certificateBytes, keyBytes)
	if err != nil {
		return nil, "", errProductStart
	}
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{identity}, ServerName: origin.Hostname()}
	return transport, origin.String(), nil
}

func dockerWriterTransport(settings config.Config) (*http.Transport, string, error) {
	origin, err := url.Parse(settings.DockerWriterOrigin)
	if err != nil || origin.Scheme != "https" || origin.Hostname() == "" {
		return nil, "", errProductStart
	}
	roots, err := providerCertPool(settings.DockerWriterCAFile)
	if err != nil {
		return nil, "", errProductStart
	}
	certificateBytes, err := readBoundedFile(settings.DockerWriterCertFile, 1<<20)
	if err != nil {
		return nil, "", errProductStart
	}
	defer clear(certificateBytes)
	keyBytes, err := readBoundedFile(settings.DockerWriterKeyFile, 1<<20)
	if err != nil {
		return nil, "", errProductStart
	}
	defer clear(keyBytes)
	identity, err := tls.X509KeyPair(certificateBytes, keyBytes)
	if err != nil {
		return nil, "", errProductStart
	}
	transport := boundedProviderTransport()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{identity}, ServerName: origin.Hostname()}
	return transport, origin.String(), nil
}

func providerCertPool(path string) (*x509.CertPool, error) {
	data, err := readBoundedFile(path, 1<<20)
	if err != nil {
		return nil, errProductStart
	}
	defer clear(data)
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(data) {
		return nil, errProductStart
	}
	return roots, nil
}

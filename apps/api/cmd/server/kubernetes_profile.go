package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"strings"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/kubernetes"
)

func configuredKubernetesCollector(settings config.Config) (*kubernetes.APICollector, error) {
	transport := boundedProviderTransport()
	options := kubernetes.APICollectorOptions{Client: &http.Client{Transport: transport, Timeout: settings.RequestTimeout}, Namespaces: settings.KubernetesNamespaces, ClusterScoped: settings.KubernetesClusterObservation, MaximumItems: 10000, MaximumResponse: 8 << 20}
	if settings.KubernetesAccessProfile == config.KubernetesProfileKubeconfig {
		ctx, cancel := context.WithTimeout(context.Background(), settings.RequestTimeout)
		defer cancel()
		collector, err := kubernetes.NewMountedKubeconfigCollector(ctx, kubernetes.MountedKubeconfigOptions{File: settings.KubernetesKubeconfigFile, Context: settings.KubernetesContext, Roots: settings.KubernetesReferenceRoots, Collector: options})
		if err != nil {
			transport.CloseIdleConnections()
			return nil, errProductStart
		}
		return collector, nil
	}
	tokenBytes, err := readBoundedFile(settings.KubernetesTokenFile, 16<<10)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, errProductStart
	}
	token := strings.TrimSpace(string(tokenBytes))
	clear(tokenBytes)
	roots, err := providerCertPool(settings.KubernetesCAFile)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, errProductStart
	}
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "kubernetes.default.svc"}
	options.BearerToken = token
	options.TokenReader = func(ctx context.Context) (string, error) {
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
	}
	collector, err := kubernetes.NewAPICollector(options)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, errProductStart
	}
	return collector, nil
}

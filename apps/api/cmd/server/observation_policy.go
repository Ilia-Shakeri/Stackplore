package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func ensureObservationPolicy(ctx context.Context, repository observationDatabase, settings config.Config, source domain.InfrastructureSource) error {
	if source.Provider == domain.ProviderDemo {
		return nil
	}
	policy, ok := repository.(interface {
		EnsureObservationPolicy(context.Context, domain.ProviderRef, string) error
	})
	if !ok {
		return errObservationStart
	}
	// Only digests of reviewed operator authority are persisted. No endpoint,
	// filesystem reference, provider credential or raw profile reaches the database.
	namespaces := slices.Clone(settings.KubernetesNamespaces)
	slices.Sort(namespaces)
	var fields []string
	if source.Provider == domain.ProviderKubernetes {
		fields = append([]string{"policy/v1", string(source.Provider)}, namespaces...)
		if settings.KubernetesClusterObservation {
			fields = append(fields, "cluster-observation")
		}
		fields = append(fields, settings.KubernetesTokenFile, settings.KubernetesCAFile)
	} else {
		fields = []string{"policy/v1", string(source.Provider), string(settings.DockerAccessProfile), settings.DockerSocketPath, settings.DockerProxyOrigin, settings.DockerProxyCAFile, settings.DockerProxyCertFile, settings.DockerProxyKeyFile}
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return errObservationStart
	}
	digest := sha256.Sum256(encoded)
	return policy.EnsureObservationPolicy(ctx, domain.ProviderRef{Provider: source.Provider, SourceID: source.ID}, hex.EncodeToString(digest[:]))
}

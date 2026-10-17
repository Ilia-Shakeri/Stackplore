package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/appsettings"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/operation"
)

func buildOperationRuntime(ctx context.Context, settings config.Config, database databaseRuntime, providers []httpapi.ProductProvider) (*operation.Service, error) {
	if ctx == nil || ctx.Err() != nil || database == nil {
		return nil, errProductStart
	}
	repository, ok := database.(operation.Repository)
	if !ok {
		if settings.MutationsEnabled {
			return nil, errProductStart
		}
		return nil, nil
	}
	_, key, err := database.AuthDigestKey()
	if err != nil {
		clear(key)
		return nil, errProductStart
	}
	defer clear(key)
	registrations := []operation.ProviderRegistration{}
	for _, provider := range providers {
		writer, ok := provider.(domain.OperationProvider)
		if !ok {
			continue
		}
		source := provider.Source()
		enabled := settings.DockerWriteEnabled && source.Provider == domain.ProviderDocker || settings.KubernetesWriteEnabled && source.Provider == domain.ProviderKubernetes
		registrations = append(registrations, operation.ProviderRegistration{Source: domain.ProviderRef{Provider: source.Provider, SourceID: source.ID}, Provider: writer, WriteEnabled: enabled})
	}
	service, err := operation.NewService(operation.ServiceOptions{Repository: repository, Providers: registrations, MutationsEnabled: settings.MutationsEnabled && !settings.DemoDeployment, DigestKey: key, Now: time.Now, ProviderTimeout: min(settings.RequestTimeout, 20*time.Second), Lifetime: ctx})
	if err != nil {
		return nil, errProductStart
	}
	return service, nil
}

func buildApplicationSettingsRuntime(settings config.Config, database databaseRuntime) (*appsettings.Service, error) {
	if _, ok := database.(interface{ isFakeDatabase() }); ok {
		return nil, nil
	}
	repository, ok := database.(appsettings.Repository)
	if !ok {
		return nil, errHTTPBoundary
	}
	_, key, err := database.AuthDigestKey()
	if err != nil {
		clear(key)
		return nil, errHTTPBoundary
	}
	defer clear(key)
	policy := settings.AuditPolicyConfig.LifecyclePolicy()
	policyContext, cancel := context.WithTimeout(context.Background(), settings.DatabaseHealthTimeout)
	err = repository.ConfigureAuditLifecyclePolicy(policyContext, policy)
	cancel()
	if err != nil {
		return nil, errProductStart
	}
	integrations := []appsettings.ExportIntegration{
		{Kind: "webhook", Enabled: settings.WebhookEnabled, Target: "[masked]", CredentialReference: "[masked]"},
		{Kind: "matrix", Enabled: settings.MatrixEnabled, Target: "[masked]", CredentialReference: "[masked]"},
	}
	service, err := appsettings.New(repository, key, time.Now, config.ApplicationVersion, integrations)
	if err != nil {
		return nil, errHTTPBoundary
	}
	return service, nil
}

func runAuditRetention(ctx context.Context, settings config.Config, service *appsettings.Service, logger *slog.Logger) {
	if ctx == nil || service == nil || logger == nil {
		return
	}
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	prune := func() {
		for batch := 0; batch < 8 && ctx.Err() == nil; batch++ {
			work, cancel := context.WithTimeout(ctx, min(settings.RequestTimeout, 10*time.Second))
			removed, err := service.PruneAuditHistory(work, 1000)
			cancel()
			if err != nil {
				if ctx.Err() == nil {
					logger.Warn("audit retention delayed", "reason", "bounded_retention_failed")
				}
				return
			}
			if removed < 1000 {
				return
			}
		}
	}
	prune()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prune()
		}
	}
}

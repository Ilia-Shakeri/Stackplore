package sourcesettings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type fakeRepository struct {
	Repository
	configuration domain.SourceConfiguration
	previewCalls  int
	command       PreviewCommand
}

func (repository *fakeRepository) GetSourceConfiguration(context.Context, domain.SourceID) (domain.SourceConfiguration, error) {
	return repository.configuration, nil
}
func (repository *fakeRepository) CreateSourcePreview(_ context.Context, command PreviewCommand) (domain.SourceConfiguration, error) {
	repository.previewCalls++
	repository.command = command
	return repository.configuration, nil
}

func TestPreviewUsesExactOperatorProfileAndOnlyPersistsDigest(t *testing.T) {
	at := time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC)
	repository := &fakeRepository{configuration: domain.SourceConfiguration{Source: domain.InfrastructureSource{ID: "source_docker", Provider: domain.ProviderDocker, Name: "Docker"},
		ProfileID: domain.SourceProfileDockerSocketDevelopment, Enabled: true, Version: 4, CreatedAt: at, UpdatedAt: at}}
	service, err := New(repository, map[domain.SourceID]domain.SourceProfileID{"source_demo": domain.SourceProfileDemoFixture}, func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Principal{UserID: "usr_admin", SessionID: "sess_admin", Role: auth.RoleAdministrator, AuthRevision: 1}
	if _, err := service.Preview(context.Background(), actor, "source_docker", 4, true); !errors.Is(err, ErrProfileUnavailable) || repository.previewCalls != 0 {
		t.Fatal("missing operator profile enabled")
	}
	preview, err := service.Preview(context.Background(), actor, "source_docker", 4, false)
	if err != nil || repository.previewCalls != 1 || preview.RequestedEnabled || preview.ExpiresAt.Sub(at) != ConfirmationLifetime {
		t.Fatal("missing profile could not be disabled")
	}
	digest, err := confirmationDigest(preview.ConfirmationToken)
	if err != nil || repository.command.Digest != digest || repository.command.ProfileAvailable {
		t.Fatal("preview digest binding failed")
	}
	if service.ProfileAvailable(repository.configuration) {
		t.Fatal("missing runtime profile was advertised")
	}
}

func TestConfirmationRejectsMalformedProof(t *testing.T) {
	for _, input := range []string{"", "browser endpoint", "__________________________________________=", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB"} {
		if _, err := confirmationDigest(input); err == nil {
			t.Fatal("noncanonical confirmation accepted")
		}
	}
}

func TestRestrictedDockerProfileCanBeEnabled(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	repository := &fakeRepository{configuration: domain.SourceConfiguration{
		Source:    domain.InfrastructureSource{ID: "source_docker", Provider: domain.ProviderDocker, Name: "Docker"},
		ProfileID: domain.SourceProfileDockerRestrictedProxy, Version: 1, CreatedAt: at, UpdatedAt: at,
	}}
	profiles := map[domain.SourceID]domain.SourceProfileID{
		"source_demo":       domain.SourceProfileDemoFixture,
		"source_docker":     domain.SourceProfileDockerRestrictedProxy,
		"source_kubernetes": domain.SourceProfileKubernetesInCluster,
	}
	service, err := New(repository, profiles, func() time.Time { return at })
	if err != nil {
		t.Fatal("configured restricted Docker profile prevented source settings startup")
	}
	actor := auth.Principal{UserID: "usr_admin", SessionID: "sess_admin", Role: auth.RoleAdministrator, AuthRevision: 1}
	preview, err := service.Preview(context.Background(), actor, "source_docker", 1, true)
	if err != nil || !preview.RequestedEnabled || repository.previewCalls != 1 || !repository.command.ProfileAvailable {
		t.Fatal("configured restricted Docker profile could not preview collection enablement")
	}
	repository.configuration.ProfileID = domain.SourceProfileDockerSocketDevelopment
	if service.ProfileAvailable(repository.configuration) {
		t.Fatal("restricted profile advertised a different socket profile")
	}
	if _, err := New(repository, map[domain.SourceID]domain.SourceProfileID{"source_docker": "browser_proxy"}, func() time.Time { return at }); !errors.Is(err, ErrInvalid) {
		t.Fatal("unknown operator profile accepted")
	}
}

func TestMountedKubernetesProfileCanBeEnabledWithoutAcceptingOtherProfiles(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	repository := &fakeRepository{configuration: domain.SourceConfiguration{
		Source:    domain.InfrastructureSource{ID: "source_kubernetes", Provider: domain.ProviderKubernetes, Name: "Kubernetes"},
		ProfileID: domain.SourceProfileKubernetesKubeconfig, Version: 1, CreatedAt: at, UpdatedAt: at,
	}}
	service, err := New(repository, map[domain.SourceID]domain.SourceProfileID{"source_kubernetes": domain.SourceProfileKubernetesKubeconfig}, func() time.Time { return at })
	if err != nil {
		t.Fatal("mounted Kubernetes profile prevented source settings startup")
	}
	actor := auth.Principal{UserID: "usr_admin", SessionID: "sess_admin", Role: auth.RoleAdministrator, AuthRevision: 1}
	preview, err := service.Preview(context.Background(), actor, "source_kubernetes", 1, true)
	if err != nil || !preview.RequestedEnabled || repository.previewCalls != 1 || !repository.command.ProfileAvailable {
		t.Fatal("mounted Kubernetes profile could not preview collection enablement")
	}
	repository.configuration.ProfileID = domain.SourceProfileKubernetesInCluster
	if service.ProfileAvailable(repository.configuration) {
		t.Fatal("mounted Kubernetes profile advertised in-cluster credentials")
	}
	if _, err := New(repository, map[domain.SourceID]domain.SourceProfileID{"source_kubernetes": "kubernetes_unknown"}, func() time.Time { return at }); !errors.Is(err, ErrInvalid) {
		t.Fatal("unreviewed Kubernetes operator profile accepted")
	}
}

func TestSourceSettingsAcceptsOnlyReviewedOperatorProfiles(t *testing.T) {
	for _, profile := range []domain.SourceProfileID{domain.SourceProfileDemoFixture, domain.SourceProfileDockerSocketDevelopment, domain.SourceProfileDockerRestrictedProxy, domain.SourceProfileKubernetesInCluster, domain.SourceProfileKubernetesKubeconfig} {
		t.Run(string(profile), func(t *testing.T) {
			if _, err := New(&fakeRepository{}, map[domain.SourceID]domain.SourceProfileID{"source_reviewed": profile}, time.Now); err != nil {
				t.Fatal("reviewed operator profile prevented settings startup")
			}
		})
	}
	for _, profile := range []domain.SourceProfileID{"", "kubernetes_unknown", "docker_arbitrary_proxy", "browser_endpoint"} {
		if _, err := New(&fakeRepository{}, map[domain.SourceID]domain.SourceProfileID{"source_reviewed": profile}, time.Now); !errors.Is(err, ErrInvalid) {
			t.Fatal("unreviewed operator profile accepted")
		}
	}
}

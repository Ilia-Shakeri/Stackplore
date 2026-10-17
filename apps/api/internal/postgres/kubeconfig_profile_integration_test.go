package postgres

import (
	"context"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"testing"
	"time"
)

func TestMountedKubernetesProfileMigrationAndFencePreserveAdministratorChoice(t *testing.T) {
	store, _, _ := openIntegrationStore(t)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Microsecond)
	configuration := domain.SourceConfiguration{Source: domain.InfrastructureSource{ID: "source_kubeconfig_upgrade", Provider: domain.ProviderKubernetes, Name: "Operator cluster"}, ProfileID: domain.SourceProfileKubernetesInCluster, Enabled: false, Version: 1, CreatedAt: at, UpdatedAt: at}
	if err := store.EnsureSourceConfigurations(ctx, []domain.SourceConfiguration{configuration}); err != nil {
		t.Fatal(err)
	}
	source := domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: configuration.Source.ID}
	if err := store.ReconcileOperatorProfile(ctx, source, domain.SourceProfileKubernetesKubeconfig); err != nil {
		t.Fatal(err)
	}
	after, err := store.GetSourceConfiguration(ctx, source.SourceID)
	if err != nil || after.ProfileID != domain.SourceProfileKubernetesKubeconfig || after.Enabled || after.Version != 2 {
		t.Fatal("profile reconciliation changed administrator enablement or failed migration")
	}
	status, err := store.GetObservationStatus(ctx, source)
	if err != nil || status.State != domain.AvailabilityPermissionDenied || status.ReasonCode != "source.profile_changed" {
		t.Fatal("profile switch served prior cache")
	}
	if err := store.ReconcileOperatorProfile(ctx, source, domain.SourceProfileKubernetesInCluster); err != nil {
		t.Fatal(err)
	}
	final, err := store.GetSourceConfiguration(ctx, source.SourceID)
	if err != nil || final.ProfileID != configuration.ProfileID || final.Enabled || final.Version != 3 {
		t.Fatal("reverse profile switch lost choice or version fence")
	}
}

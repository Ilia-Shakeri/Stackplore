package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestOperatorProfileReconciliationPreservesDisabledSourceAndReferences(t *testing.T) {
	store, _, _ := openIntegrationStore(t)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Microsecond)
	configuration := domain.SourceConfiguration{Source: domain.InfrastructureSource{ID: "source_docker_upgrade", Provider: domain.ProviderDocker, Name: "Operator engine"}, ProfileID: domain.SourceProfileDockerSocketDevelopment, Enabled: false, Version: 1, CreatedAt: at, UpdatedAt: at}
	if err := store.EnsureSourceConfigurations(ctx, []domain.SourceConfiguration{configuration}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `INSERT INTO source_connection_references(source_id,purpose,reference_kind,reference_id,display_label,created_at_us,updated_at_us) VALUES($1,'observation','operator_profile','profile_fixture','Operator reference',$2,$2)`, string(configuration.Source.ID), at.UnixMicro()); err != nil {
		t.Fatal(err)
	}
	source := domain.ProviderRef{SourceID: configuration.Source.ID, Provider: configuration.Source.Provider}
	if err := store.ReconcileOperatorProfile(ctx, source, domain.SourceProfileDockerRestrictedProxy); err != nil {
		t.Fatal(err)
	}
	after, err := store.GetSourceConfiguration(ctx, source.SourceID)
	if err != nil || after.ProfileID != domain.SourceProfileDockerRestrictedProxy || after.Enabled || after.Version != 2 {
		t.Fatalf("operator profile changed administrator choice: %#v, %v", after, err)
	}
	if !after.CreatedAt.Equal(configuration.CreatedAt) || !after.UpdatedAt.After(configuration.UpdatedAt) {
		t.Fatal("profile change did not preserve creation time and advance update time")
	}
	status, err := store.GetObservationStatus(ctx, source)
	if err != nil || status.State != domain.AvailabilityPermissionDenied || status.ReasonCode != "source.profile_changed" {
		t.Fatal("profile selection exposed prior authority")
	}
	if err := store.ReconcileOperatorProfile(ctx, source, domain.SourceProfileDockerRestrictedProxy); err != nil {
		t.Fatal(err)
	}
	unchanged, err := store.GetSourceConfiguration(ctx, source.SourceID)
	if err != nil || unchanged.Version != 2 || !unchanged.UpdatedAt.Equal(after.UpdatedAt) {
		t.Fatal("same profile repeated configuration revision")
	}
	var referenceKind, referenceID, label string
	var audits int
	if err := store.pool.QueryRow(ctx, `SELECT reference_kind,reference_id,display_label,(SELECT count(*) FROM audit_entries WHERE action='source.profile_update') FROM source_connection_references WHERE source_id=$1 AND purpose='observation'`, string(source.SourceID)).Scan(&referenceKind, &referenceID, &label, &audits); err != nil || referenceKind != "operator_profile" || referenceID != "profile_fixture" || label != "Operator reference" || audits != 1 {
		t.Fatal("profile upgrade changed references or audit count")
	}
	for _, invalid := range []struct {
		source  domain.ProviderRef
		profile domain.SourceProfileID
	}{
		{source, "docker_unknown"}, {domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: source.SourceID}, domain.SourceProfileDockerRestrictedProxy},
	} {
		if err := store.ReconcileOperatorProfile(ctx, invalid.source, invalid.profile); !errors.Is(err, domain.ErrRepositoryInvalidInput) {
			t.Fatal("unreviewed operator profile accepted")
		}
	}
	if err := store.ReconcileOperatorProfile(ctx, source, domain.SourceProfileDockerSocketDevelopment); err != nil {
		t.Fatal(err)
	}
	restored, err := store.GetSourceConfiguration(ctx, source.SourceID)
	if err != nil || restored.ProfileID != configuration.ProfileID || restored.Enabled || restored.Version != 3 {
		t.Fatal("reviewed reverse profile switch lost administrator choice")
	}
}

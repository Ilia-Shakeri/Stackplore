package domain_test

import (
	"context"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

func TestInventoryCaptureBoundsAndBindsNormalizedEvidence(t *testing.T) {
	provider := demo.MustNew()
	source := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: demo.SourceID}
	capture, err := provider.CaptureInventory(context.Background())
	if err != nil || capture.ValidateFor(source) != nil {
		t.Fatal("valid normalized capture rejected")
	}
	capture.Source.SourceID = "source_other"
	if capture.ValidateFor(source) == nil {
		t.Fatal("capture accepted another source")
	}
	capture, _ = provider.CaptureInventory(context.Background())
	capture.Resources = append(capture.Resources, capture.Resources[0])
	if capture.ValidateFor(source) == nil {
		t.Fatal("capture accepted duplicate resource")
	}
	capture, _ = provider.CaptureInventory(context.Background())
	capture.Health[0].ResourceID = "r1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if capture.ValidateFor(source) == nil {
		t.Fatal("capture accepted health outside its inventory")
	}
	capture, _ = provider.CaptureInventory(context.Background())
	capture.Relationships[0].Evidence = append([]domain.RelationshipEvidence(nil), capture.Relationships[0].Evidence...)
	capture.Relationships[0].Evidence[0].Synthetic = false
	if capture.ValidateFor(source) == nil {
		t.Fatal("capture dropped synthetic evidence boundary")
	}
	capture, _ = provider.CaptureInventory(context.Background())
	capture.Status.Completeness = domain.CompletenessPartial
	capture.Status.ReasonCode = "fixture.capture.partial"
	if capture.ValidateFor(source) == nil {
		t.Fatal("capture claimed complete inventory with partial status")
	}
	capture, _ = provider.CaptureInventory(context.Background())
	capture.Resources = make([]domain.ResourceSnapshot, domain.MaxInventoryCaptureItems+1)
	if capture.ValidateFor(source) == nil {
		t.Fatal("capture exceeded allocation bound")
	}
}

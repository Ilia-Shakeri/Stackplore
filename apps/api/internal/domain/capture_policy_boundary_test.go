package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

func TestCaptureBatchKeepsEachObservedKindBoundToGeneration(t *testing.T) {
	source := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: demo.SourceID}
	for name, change := range map[string]func(*domain.CaptureBatch){
		"valid":                    func(*domain.CaptureBatch) {},
		"duplicate environment":    func(b *domain.CaptureBatch) { b.Environments = append(b.Environments, b.Environments[0]) },
		"other environment source": func(b *domain.CaptureBatch) { b.Environments[0].SourceID = "source_other" },
		"future environment": func(b *domain.CaptureBatch) {
			b.Environments[0].Status.ObservedAt = b.Token.ObservedAt.Add(time.Second)
		},
		"duplicate resource":       func(b *domain.CaptureBatch) { b.Resources = append(b.Resources, b.Resources[0]) },
		"synthetic resource":       func(b *domain.CaptureBatch) { b.Resources[0].Resource.Synthetic = false },
		"duplicate health":         func(b *domain.CaptureBatch) { b.Health = append(b.Health, b.Health[0]) },
		"future health":            func(b *domain.CaptureBatch) { b.Health[0].Status.ObservedAt = b.Token.ObservedAt.Add(time.Second) },
		"unsafe health provenance": func(b *domain.CaptureBatch) { b.Health[0].Evidence[0].Synthetic = false },
		"duplicate edge":           func(b *domain.CaptureBatch) { b.Relationships = append(b.Relationships, b.Relationships[0]) },
		"other edge source":        func(b *domain.CaptureBatch) { b.Relationships[0].Evidence[0].SourceID = "source_other" },
		"future edge":              func(b *domain.CaptureBatch) { b.Relationships[0].ObservedAt = b.Token.ObservedAt.Add(time.Second) },
	} {
		t.Run(name, func(t *testing.T) {
			capture, err := demo.MustNew().CaptureInventory(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			batch := domain.CaptureBatch{Token: domain.CaptureToken{Source: source, ConfigurationVersion: 1, Generation: 1, ObservedAt: capture.Status.ObservedAt}, Environments: capture.Environments, Resources: capture.Resources, Health: capture.Health, Relationships: capture.Relationships, WorkloadReplicas: capture.WorkloadReplicas}
			change(&batch)
			err = batch.Validate()
			if name == "valid" && err != nil {
				t.Fatal(err)
			}
			if name != "valid" && err == nil {
				t.Fatal("invalid generation evidence accepted")
			}
		})
	}
}

func TestCaptureCompletionNeverPublishesFutureOrPartialAsComplete(t *testing.T) {
	source := domain.ProviderRef{Provider: domain.ProviderDemo, SourceID: demo.SourceID}
	provider := demo.MustNew()
	capture, err := provider.CaptureInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	caps, err := provider.GetCapabilities(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	base := domain.CaptureCompletion{Token: domain.CaptureToken{Source: source, ConfigurationVersion: 1, Generation: 1, ObservedAt: capture.Status.ObservedAt}, CollectedAt: capture.Status.ObservedAt, Status: capture.Status, Capabilities: caps, Complete: true}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*domain.CaptureCompletion){
		"receipt before capture": func(c *domain.CaptureCompletion) { c.CollectedAt = c.Token.ObservedAt.Add(-time.Second) },
		"status after capture":   func(c *domain.CaptureCompletion) { c.Status.ObservedAt = c.Token.ObservedAt.Add(time.Second) },
		"partial complete": func(c *domain.CaptureCompletion) {
			c.Status.Completeness = domain.CompletenessPartial
			c.Status.ReasonCode = "source.partial"
		},
		"future capability": func(c *domain.CaptureCompletion) {
			c.Capabilities.Capabilities[0].Status.ObservedAt = c.Token.ObservedAt.Add(time.Second)
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := base
			c.Capabilities.Capabilities = append([]domain.CapabilityStatus(nil), base.Capabilities.Capabilities...)
			change(&c)
			if c.Validate() == nil {
				t.Fatal("invalid completion accepted")
			}
		})
	}
}

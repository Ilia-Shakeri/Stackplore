package assistantevidence

import (
	"context"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestHistoryReadWithoutConfiguredPortIsExplicitlyUnsupported(t *testing.T) {
	for _, kind := range []assistant.StepKind{assistant.AlertHistory, assistant.WorkloadHistory} {
		f := newFixture(t)
		f.request.Kind = kind
		f.request.Scope.Resources = f.request.Scope.Resources[:1]
		proof, err := f.reader.Read(context.Background(), f.request)
		if err != nil || proof.Bundle().Summary().Availability != domain.AvailabilityUnsupported || proof.Bundle().RecordCount() != 0 {
			t.Fatal("missing history port fabricated data", err)
		}
		if err := f.reader.AuthorizeEvidence(context.Background(), f.request.Actor, f.request.Run, proof); err != nil {
			t.Fatal("empty history proof lost its current authority fence", err)
		}
	}
}

func TestHistoryReadRejectsRemoteAndWideScopeBeforeIO(t *testing.T) {
	for _, remote := range []bool{false, true} {
		f := newFixture(t)
		f.request.Kind = assistant.AlertHistory
		if remote {
			f.request.Scope.Resources = f.request.Scope.Resources[:1]
			f.request.Destination = contextbuilder.Remote
		}
		if _, err := f.reader.Read(context.Background(), f.request); err != ErrDenied || f.gate.checks.Load() != 0 || f.live.calls.Load() != 0 || f.reads.resourceCalls != 0 {
			t.Fatal("remote or wide history request reached I/O", err)
		}
	}
}

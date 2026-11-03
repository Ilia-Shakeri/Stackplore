package assistantevidence

import (
	"context"
	"errors"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type historyFixture struct{ calls int }

func (history *historyFixture) GetIncidentAlertHistory(_ context.Context, query domain.IncidentAlertHistoryQuery) (domain.IncidentAlertHistory, error) {
	history.calls++
	return domain.IncidentAlertHistory{Status: domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: query.Range.End}, Transitions: []domain.IncidentAlertTransition{{ID: "transition_fixture", AlertID: "alert_fixture", ResourceID: query.Resource.ID, To: domain.AlertFiring, OccurredAt: query.Range.Start, Synthetic: true}}}, nil
}

func (history *historyFixture) GetIncidentWorkloadHistory(_ context.Context, query domain.IncidentWorkloadHistoryQuery) (domain.IncidentWorkloadHistory, error) {
	history.calls++
	status := domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: query.Range.Start}
	return domain.IncidentWorkloadHistory{Status: status, Samples: []domain.WorkloadReplicaSnapshot{{Resource: query.Resource, Status: status, Desired: 3, Ready: 2}}}, nil
}

func TestConfiguredHistoryReadSealsFactsAndRechecksAuthority(t *testing.T) {
	for _, kind := range []assistant.StepKind{assistant.AlertHistory, assistant.WorkloadHistory} {
		f := newFixture(t)
		f.request.Kind = kind
		f.request.Scope.Resources = f.request.Scope.Resources[:1]
		history := &historyFixture{}
		source := f.reader.sources[f.request.Scope.Source.SourceID]
		source.History = history
		f.reader.sources[source.Reference.SourceID] = source
		proof, err := f.reader.Read(context.Background(), f.request)
		if err != nil || history.calls != 1 || proof.Bundle().RecordCount() != 1 {
			t.Fatal("configured bounded history was not collected", err)
		}
		observations, err := proof.Bundle().Observations()
		if err != nil || len(observations) != 1 || observations[0].Reference.ResourceID != f.request.Scope.Resources[0].ID || !observations[0].Time.Equal(f.request.Scope.Range.Start) {
			t.Fatal("history proof lost selected resource or actual time", err)
		}
		if err := f.reader.AuthorizeEvidence(context.Background(), f.request.Actor, f.request.Run, proof); err != nil {
			t.Fatal("current history proof rejected", err)
		}
		completed := f.request.Run
		completed.State, completed.Version, completed.Fence = assistant.Completed, completed.Version+1, completed.Fence+1
		locator, err := f.reader.Resolve(context.Background(), f.request.Actor, completed, proof, observations[0].Reference.ID)
		if err != nil || locator.ResourceID != f.request.Scope.Resources[0].ID || locator.Kind != observations[0].Reference.Kind {
			t.Fatal("completed history citation lost its exact resource binding", err)
		}
		f.live.err = ErrDenied
		if err := f.reader.AuthorizeEvidence(context.Background(), f.request.Actor, f.request.Run, proof); !errors.Is(err, ErrDenied) {
			t.Fatal("revoked history proof remained authorized", err)
		}
		if _, err := f.reader.Resolve(context.Background(), f.request.Actor, completed, proof, observations[0].Reference.ID); !errors.Is(err, ErrDenied) {
			t.Fatal("revoked history citation remained readable", err)
		}
	}
}

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

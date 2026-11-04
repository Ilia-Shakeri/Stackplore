package assistantservice

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (reads *fixtureInventoryReads) GetIncidentAlertHistory(ctx context.Context, query domain.IncidentAlertHistoryQuery) (domain.IncidentAlertHistory, error) {
	if ctx.Err() != nil || query.Validate() != nil || query.Resource != reads.resource || query.Source != reads.source || query.Limit != 8 || reads.denyHistory.Load() {
		return domain.IncidentAlertHistory{}, assistantevidence.ErrDenied
	}
	status := reads.status()
	result := domain.IncidentAlertHistory{Status: status}
	if !reads.historyEnabled {
		result.Status.State, result.Status.ReasonCode = domain.AvailabilityUnsupported, "fixture.history_unsupported"
	} else if !reads.historyEmpty {
		result.Transitions = []domain.IncidentAlertTransition{{ID: "history_transition", AlertID: "history_alert", ResourceID: query.Resource.ID, To: domain.AlertFiring, OccurredAt: query.Range.Start.Add(time.Second), Synthetic: true}}
	}
	return result, nil
}

func (reads *fixtureInventoryReads) GetIncidentWorkloadHistory(ctx context.Context, query domain.IncidentWorkloadHistoryQuery) (domain.IncidentWorkloadHistory, error) {
	if ctx.Err() != nil || query.Validate() != nil || query.Resource != reads.resource || query.Source != reads.source || query.Limit != 8 || reads.denyHistory.Load() {
		return domain.IncidentWorkloadHistory{}, assistantevidence.ErrDenied
	}
	result := domain.IncidentWorkloadHistory{Status: reads.status()}
	if !reads.historyEnabled {
		result.Status.State, result.Status.ReasonCode = domain.AvailabilityUnsupported, "fixture.history_unsupported"
	} else if !reads.historyEmpty {
		for index, ready := range []uint64{1, 3} {
			status := reads.status()
			status.ObservedAt = query.Range.Start.Add(time.Duration(index+1) * time.Second)
			result.Samples = append(result.Samples, domain.WorkloadReplicaSnapshot{Resource: query.Resource, Status: status, Desired: 3, Ready: ready})
		}
	}
	return result, nil
}

func TestIncidentHistoryFlowsThroughReservedProofsAndReport(t *testing.T) {
	for _, empty := range []bool{false, true} {
		service, repository, authority, model := fixtureService(t, nil)
		authority.reads.historyEnabled, authority.reads.historyEmpty = true, empty
		input := inputFixture(t, 1)
		input.Admission.Goal = assistant.InvestigateIncident
		if _, err := service.Submit(context.Background(), input); err != nil {
			t.Fatal(err)
		}
		if _, err := service.runOne(context.Background(), "worker"); err != nil {
			t.Fatal(err)
		}
		snapshot, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID)
		if err != nil || snapshot.Report == nil || !snapshot.HistoryCoverage.AlertsRead || !snapshot.HistoryCoverage.WorkloadRead || snapshot.Run.ToolCalls != 7 || model.calls.Load() != 1 {
			t.Fatal("history did not cross reserved current proofs", err)
		}
		if slices.Contains(snapshot.Report.Gaps, "alerts_not_read") || slices.Contains(snapshot.Report.Gaps, "workload_history_not_read") {
			t.Fatal("readable empty history became unread")
		}
		for _, kind := range []string{"alert_history", "workload_history"} {
			begin, read, finish := slices.Index(repository.trace, "begin:"+kind), slices.Index(repository.trace, "read:"+kind), slices.Index(repository.trace, "finish:"+kind)
			if begin < 0 || read <= begin || finish <= read {
				t.Fatal("history escaped durable reservation")
			}
		}
		if !empty {
			if len(snapshot.Report.Timeline) != 4 || len(snapshot.Report.Hypotheses) != 1 || snapshot.Report.Hypotheses[0].Code != "workload_shortfall_possible" || len(snapshot.Report.Hypotheses[0].Contrary) != 1 {
				t.Fatal("history chronology or recovery lost")
			}
			service.mu.Lock()
			proofs := service.evidenceSetLocked(service.entries[input.Admission.ID])
			service.mu.Unlock()
			for _, proof := range proofs {
				if proof.ReadKind() != assistant.AlertHistory && proof.ReadKind() != assistant.WorkloadHistory {
					continue
				}
				for _, ref := range proof.References() {
					if _, err := authority.reader.Resolve(context.Background(), input.Admission.Actor, snapshot.Run, proof, ref.ID); err != nil {
						t.Fatal("history witness failed current authority", err)
					}
				}
			}
		}
		authority.reads.denyHistory.Store(true)
		if _, err := service.Get(context.Background(), input.Admission.Actor, input.Admission.ID); err != assistant.ErrDenied {
			t.Fatal("revoked history retained report", err)
		}
	}
}

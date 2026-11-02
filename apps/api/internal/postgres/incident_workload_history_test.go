package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestIncidentWorkloadHistoryUsesSelectedResourceWindowAndPolicy(t *testing.T) {
	fixture := newObservationFixture(t)
	query := domain.IncidentWorkloadHistoryQuery{Source: fixture.token.Source, Resource: fixture.batch.Resources[0].Resource.Ref, Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: fixture.at.Add(time.Minute)}, Limit: 1}
	history, err := fixture.store.GetIncidentWorkloadHistory(fixture.ctx, query)
	if err != nil || history.ValidateFor(query) != nil || len(history.Samples) != 1 || history.Samples[0].Desired != 3 || history.Samples[0].Ready != 2 || history.MoreAvailable {
		t.Fatal("selected workload sample was not retained", err)
	}
	window := query
	window.Range.Start = history.Samples[0].Status.ObservedAt
	window.Range.End = window.Range.Start.Add(time.Microsecond)
	inside, err := fixture.store.GetIncidentWorkloadHistory(fixture.ctx, window)
	if err != nil || len(inside.Samples) != 1 {
		t.Fatal("inclusive window start lost the sample", err)
	}
	window.Range.Start = window.Range.Start.Add(-time.Microsecond)
	window.Range.End = history.Samples[0].Status.ObservedAt
	out, err := fixture.store.GetIncidentWorkloadHistory(fixture.ctx, window)
	if err != nil || len(out.Samples) != 0 {
		t.Fatal("exclusive window end returned a sample", err)
	}
	token, err := fixture.store.BeginCapture(fixture.ctx, fixture.source.Source.ID, 1, fixture.at.Add(time.Second))
	if err != nil {
		t.Fatal("begin later capture")
	}
	batch := fixture.batch
	batch.Token = token
	batch.WorkloadReplicas = append([]domain.WorkloadReplicaSnapshot{}, fixture.batch.WorkloadReplicas...)
	batch.WorkloadReplicas[0].Status.ObservedAt = fixture.at.Add(time.Second)
	batch.WorkloadReplicas[0].Ready = 3
	if err := fixture.store.SaveCapture(fixture.ctx, batch); err != nil {
		t.Fatal("save later capture")
	}
	completion := fixture.completion
	completion.Token = token
	if err := fixture.store.FinishCapture(fixture.ctx, completion); err != nil {
		t.Fatal("finish later capture")
	}
	capped, err := fixture.store.GetIncidentWorkloadHistory(fixture.ctx, query)
	if err != nil || len(capped.Samples) != 1 || !capped.MoreAvailable || capped.Status.Completeness != domain.CompletenessPartial || capped.Samples[0].Ready != 2 {
		t.Fatal("bounded history hid a later recovery or rewrote the earlier count", err)
	}
	query.Limit = 2
	complete, err := fixture.store.GetIncidentWorkloadHistory(fixture.ctx, query)
	if err != nil || len(complete.Samples) != 2 || complete.MoreAvailable || complete.Samples[1].Ready != 3 {
		t.Fatal("history lost actual recovery sample", err)
	}
	foreign := query
	foreign.Source.SourceID = "foreign_source"
	if _, err := fixture.store.GetIncidentWorkloadHistory(fixture.ctx, foreign); !errors.Is(err, domain.ErrRepositoryInvalidInput) {
		t.Fatal("foreign source reached history")
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `UPDATE infrastructure_sources SET operator_policy_pending=true WHERE id=$1`, string(query.Source.SourceID)); err != nil {
		t.Fatal("set current policy fence")
	}
	denied, err := fixture.store.GetIncidentWorkloadHistory(fixture.ctx, query)
	if err != nil || denied.Status.State != domain.AvailabilityPermissionDenied || len(denied.Samples) != 0 || denied.MoreAvailable {
		t.Fatal("policy loss returned workload history", err)
	}
}

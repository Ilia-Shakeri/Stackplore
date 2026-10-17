package postgres

import (
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"testing"
	"time"
)

func TestHealthHistoryReadsRetainedExactResourceAndClosesRevokedAuthority(t *testing.T) {
	fixture := newObservationFixture(t)
	resource := fixture.batch.Resources[0].Resource.Ref
	query := domain.HealthHistoryQuery{Source: fixture.token.Source, Resource: resource, Range: domain.TimeRange{Start: fixture.at.Add(-time.Minute), End: fixture.at.Add(time.Second)}, Limit: 1}
	result, err := fixture.store.GetHealthHistory(fixture.ctx, query)
	if err != nil || len(result.Samples) != 1 || result.Samples[0].ResourceID != resource.ID || result.ValidateFor(query) != nil {
		t.Fatalf("retained health read failed: %v", err)
	}
	secondAt := fixture.batch.Health[0].Status.ObservedAt.Add(-time.Second).UnixMicro()
	if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO health_history(id,resource_id,health_state,availability,completeness,observed_at_us,reason_code,synthetic) VALUES('health_older',$1,'unknown','available','partial',$2,'source.partial',1)`, string(resource.ID), secondAt); err != nil {
		t.Fatal(err)
	}
	result, err = fixture.store.GetHealthHistory(fixture.ctx, query)
	if err != nil || len(result.Samples) != 1 || result.Status.Completeness != domain.CompletenessPartial {
		t.Fatal("health limit hid truncation")
	}
	query.Limit = 10
	result, err = fixture.store.GetHealthHistory(fixture.ctx, query)
	if err != nil || len(result.Samples) != 2 || result.Samples[1].State != domain.HealthUnknown {
		t.Fatal("retained partial health classified as known")
	}
	if err := fixture.store.EnsureObservationPolicy(fixture.ctx, query.Source, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	result, err = fixture.store.GetHealthHistory(fixture.ctx, query)
	if err != nil || result.Status.State != domain.AvailabilityPermissionDenied || len(result.Samples) != 0 {
		t.Fatal("changed authority exposed retained health")
	}
}

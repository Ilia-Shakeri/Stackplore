package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestPostgreSQLRetentionIsBoundedAndProtectsAudit(t *testing.T) {
	store, _, _ := openIntegrationStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	for index := range 5 {
		if err := store.InsertInfrastructureEvents(ctx, []domain.InfrastructureEvent{{
			ID:       domain.EventID(fmt.Sprintf("event_prune_%d", index)),
			Provider: domain.ProviderDemo, SourceID: "source_retention", Kind: "demo.health.changed",
			Severity: domain.SeverityInfo, ReasonCode: "demo.health.changed", Summary: "health changed",
			OccurredAt: now.Add(-time.Hour), ObservedAt: now, Synthetic: true,
		}}); err == nil {
			t.Fatal("missing source foreign key was accepted")
		}
	}
	if _, err := store.pool.Exec(ctx, `INSERT INTO infrastructure_sources
(id,provider,display_name,synthetic,enabled,connection_kind,created_at_us,updated_at_us)
VALUES('source_retention','demo','Retention fixture',1,1,'fixture',$1,$1)`, now.UnixMicro()); err != nil {
		t.Fatal("seed retention source")
	}
	for index := range 5 {
		if err := store.InsertInfrastructureEvents(ctx, []domain.InfrastructureEvent{{
			ID:       domain.EventID(fmt.Sprintf("event_prune_%d", index)),
			Provider: domain.ProviderDemo, SourceID: "source_retention", Kind: "demo.health.changed",
			Severity: domain.SeverityInfo, ReasonCode: "demo.health.changed", Summary: "health changed",
			OccurredAt: now.Add(-time.Hour), ObservedAt: now, Synthetic: true,
		}}); err != nil {
			t.Fatal("seed retention event")
		}
	}
	audit := domain.AuditEntry{
		ID: "audit_retention_protected", OccurredAt: now.Add(-24 * time.Hour), Actor: domain.AuditActor{Kind: domain.AuditActorSystem},
		Action: "collector.persist", Target: domain.AuditTarget{Kind: "source.observation", ID: "source_retention"},
		Outcome: domain.AuditSuccess, RequestID: "request_retention", ReasonCode: "collector.persisted",
	}
	if err := store.AppendAuditEntries(ctx, []domain.AuditEntry{audit}); err != nil {
		t.Fatal("seed protected audit")
	}
	if removed, err := store.Prune(ctx, RetentionInfrastructureEvents, now, 2); err != nil || removed != 2 {
		t.Fatal("retention did not honor batch bound")
	}
	for _, class := range []RetentionClass{RetentionMetricSamples, RetentionHealthHistory, RetentionTerminalNotificationDeliveries} {
		if removed, err := store.Prune(ctx, class, now, 2); err != nil || removed != 0 {
			t.Fatal("empty retention class rejected")
		}
	}
	if _, err := store.Prune(ctx, RetentionClass("audit_entries"), now, 2); !errors.Is(err, ErrInvalidOptions) {
		t.Fatal("generic cleanup accepted audit history")
	}
	var count int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries WHERE id=$1`, string(audit.ID)).Scan(&count); err != nil || count != 1 {
		t.Fatal("generic cleanup removed protected audit")
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM infrastructure_events`).Scan(&count); err != nil || count != 3 {
		t.Fatal("retention removed more than the requested batch")
	}
}

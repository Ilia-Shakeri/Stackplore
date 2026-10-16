package postgres

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/notification"
)

func TestMatrixTestIntentStaysInOneLaneAndAuditsLeaseLoss(t *testing.T) {
	fixture := newObservationFixture(t)
	if _, err := fixture.store.pool.Exec(fixture.ctx, `INSERT INTO users(id,login_name,canonical_login_name,display_name,role,enabled,created_at_us,updated_at_us) VALUES('user_delivery_test','deliverytest','deliverytest','Delivery fixture','administrator',1,$1,$1)`, fixture.at.UnixMicro()); err != nil {
		t.Fatal("seed test delivery actor")
	}
	for index, outcome := range []string{"success", "uncertain"} {
		at := fixture.at.Add(time.Duration(index) * 2 * time.Second)
		suffix := outcome
		item := domain.Notification{ID: domain.NotificationID("notification_matrix_test_" + suffix), Kind: notification.TestNotificationKind, Severity: domain.SeverityInfo, SummaryCode: notification.TestNotificationReason, CreatedAt: at}
		command := notification.WebhookTestCommand{Notification: item, DestinationReferenceID: "destination_matrix_test", MaximumAttempts: 1, Audit: domain.AuditEntry{ID: domain.AuditEntryID("audit_matrix_test_" + suffix), OccurredAt: at, Actor: domain.AuditActor{Kind: domain.AuditActorUser, ID: "user_delivery_test"}, Action: "notification.test", Target: domain.AuditTarget{Kind: "notification.destination", ID: "destination_matrix_test"}, Outcome: domain.AuditIntent, RequestID: domain.RequestID("request_matrix_test_" + suffix), ReasonCode: notification.TestNotificationReason}}
		if err := fixture.store.CreateNotificationTest(fixture.ctx, notification.DeliveryAdapterMatrix, command); err != nil {
			t.Fatal("create Matrix test intent")
		}
		for _, adapter := range []notification.DeliveryAdapter{notification.DeliveryAdapterWebhook, notification.DeliveryAdapterMatrix} {
			if created, err := fixture.store.EnqueueDeliveries(fixture.ctx, adapter, "destination_other", 3, at, 32); err != nil || created != 0 {
				t.Fatal("test notification copied to another destination")
			}
		}
		tasks, err := fixture.store.ClaimDeliveries(fixture.ctx, notification.DeliveryAdapterMatrix, "destination_matrix_test", "worker_matrix_test", at, time.Second, 1)
		if err != nil || len(tasks) != 1 || tasks[0].MaximumAttempts != 1 || !strings.Contains(string(tasks[0].Payload), `"msgtype":"m.notice"`) {
			t.Fatal("Matrix test was not scoped to its one attempt")
		}
		if outcome == "success" {
			status := http.StatusOK
			if err := fixture.store.CompleteDelivery(fixture.ctx, notification.DeliveryAdapterMatrix, "destination_matrix_test", notification.DeliveryCompletion{DeliveryID: tasks[0].ID, AttemptNumber: 1, LeaseOwner: "worker_matrix_test", FinishedAt: at, Outcome: notification.DeliverySucceeded, ResponseStatus: &status}); err != nil {
				t.Fatal("complete Matrix test result")
			}
		} else {
			if tasks, err := fixture.store.ClaimDeliveries(fixture.ctx, notification.DeliveryAdapterMatrix, "destination_matrix_test", "worker_matrix_recovered", at.Add(time.Second), time.Second, 1); err != nil || len(tasks) != 0 {
				t.Fatal("lost one-attempt test was retried")
			}
		}
		var intents, results int
		if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM audit_entries WHERE id=$1 AND outcome='intent'`, string(command.Audit.ID)).Scan(&intents); err != nil || intents != 1 {
			t.Fatal("test intent audit absent")
		}
		if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM audit_entries WHERE action='notification.test' AND request_id=$1 AND outcome=$2`, string(command.Audit.RequestID), outcome).Scan(&results); err != nil || results != 1 {
			t.Fatal("test terminal result audit absent or duplicated")
		}
	}
}

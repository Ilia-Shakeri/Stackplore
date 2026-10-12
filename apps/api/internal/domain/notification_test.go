package domain

import (
	"testing"
	"time"
)

func TestAlertMutationBindsTransitionAndNotification(t *testing.T) {
	now := fixtureTime()
	alert := validPageAlert("alert_one", now)
	transition := AlertTransition{
		ID: "transition_one", AlertID: alert.ID, To: AlertFiring, OccurredAt: now,
		Actor: AuditActor{Kind: AuditActorSystem}, ReasonCode: "alert.condition_matched",
		EvidenceRange: alert.EvidenceRange,
	}
	notification := Notification{
		ID: "notification_one", SourceID: &alert.SourceID, AlertID: &alert.ID,
		Kind: "notification.alert_firing", Severity: alert.Severity,
		SummaryCode: alert.SummaryCode, CreatedAt: now, Synthetic: alert.Synthetic,
	}
	mutation := AlertMutation{
		Alert: alert, DedupeKey: "resource_health", Transition: &transition, Notification: &notification,
	}
	if err := mutation.Validate(); err != nil {
		t.Fatal(err)
	}
	wrong := notification
	wrongAlertID := AlertID("alert_other")
	wrong.AlertID = &wrongAlertID
	mutation.Notification = &wrong
	if mutation.Validate() == nil {
		t.Fatal("cross-alert notification was accepted")
	}

	previous := now.Add(-time.Minute)
	from := AlertPending
	transition.From = &from
	mutation = AlertMutation{
		Alert: alert, DedupeKey: "resource_health", ExpectedUpdatedAt: &previous,
		ExpectedState: &from, Transition: &transition,
	}
	if err := mutation.Validate(); err != nil {
		t.Fatal(err)
	}
	mutation.ExpectedState = nil
	if mutation.Validate() == nil {
		t.Fatal("alert update without expected state was accepted")
	}
}

package postgres

import (
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/notification"
)

func TestNotificationTestAuditResultMapsEveryTerminalOutcome(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   notification.DeliveryOutcome
		outcome domain.AuditOutcome
		reason  domain.ReasonCode
	}{
		{notification.DeliverySucceeded, domain.AuditSuccess, "notification.test_delivery_succeeded"},
		{notification.DeliveryFailed, domain.AuditFailure, "notification.test_delivery_failed"},
		{notification.DeliveryUncertain, domain.AuditUncertain, "notification.test_delivery_uncertain"},
	}
	for _, test := range tests {
		outcome, reason, ok := notificationTestAuditResult(test.input)
		if !ok || outcome != test.outcome || reason != test.reason {
			t.Fatalf("notificationTestAuditResult(%q) = (%q, %q, %t)", test.input, outcome, reason, ok)
		}
	}
	if _, _, ok := notificationTestAuditResult("pending"); ok {
		t.Fatal("non-terminal delivery outcome received an audit result")
	}
}

package notification

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type testRepositoryStub struct{ commands []WebhookTestCommand }

func (repository *testRepositoryStub) CreateWebhookTest(_ context.Context, command WebhookTestCommand) error {
	repository.commands = append(repository.commands, command)
	return nil
}

type testIdentifiersStub struct{}

func (testIdentifiersStub) NewNotificationID() (domain.NotificationID, error) {
	return "notification_test", nil
}
func (testIdentifiersStub) NewAuditID() (domain.AuditEntryID, error) { return "audit_test", nil }

func TestServiceCreatesDurableOneAttemptTestIntent(t *testing.T) {
	at := time.Date(2026, time.August, 21, 15, 0, 0, 123000, time.UTC)
	repository := &testRepositoryStub{}
	service, err := NewTestService(repository, testIdentifiersStub{}, "webhook_primary", func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	item, err := service.RequestTest(context.Background(), "user_admin", "request_test_delivery")
	if err != nil || item.ID != "notification_test" || len(repository.commands) != 1 {
		t.Fatalf("RequestTest() = (%#v, %v), commands = %d", item, err, len(repository.commands))
	}
	command := repository.commands[0]
	if command.Validate() != nil || command.MaximumAttempts != 1 || command.Audit.Outcome != domain.AuditIntent {
		t.Fatalf("test command = %#v", command)
	}
}

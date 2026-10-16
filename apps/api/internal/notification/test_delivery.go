package notification

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const (
	TestNotificationKind   domain.ReasonCode = "notification.test_delivery"
	TestNotificationReason domain.ReasonCode = "notification.test_delivery_requested"
	maximumTestAttempts                      = 1
)

type WebhookTestCommand struct {
	Notification           domain.Notification
	DestinationReferenceID string
	MaximumAttempts        int
	Audit                  domain.AuditEntry
}

func (command WebhookTestCommand) Validate() error {
	item := command.Notification
	if item.Validate() != nil || item.SourceID != nil || item.AlertID != nil || item.ResourceID != nil ||
		item.Kind != TestNotificationKind || item.Severity != domain.SeverityInfo ||
		item.SummaryCode != TestNotificationReason || item.Synthetic ||
		!validDeliveryID(command.DestinationReferenceID) || command.MaximumAttempts != maximumTestAttempts ||
		command.Audit.Validate() != nil || command.Audit.Actor.Kind != domain.AuditActorUser ||
		command.Audit.Action != "notification.test" || command.Audit.Target.Kind != "notification.destination" ||
		command.Audit.Target.ID != command.DestinationReferenceID || command.Audit.Outcome != domain.AuditIntent ||
		command.Audit.RequestID.Validate() != nil || command.Audit.ReasonCode != TestNotificationReason ||
		command.Audit.OccurredAt != item.CreatedAt {
		return fmt.Errorf("webhook test command is invalid")
	}
	return nil
}

type TestRepository interface {
	CreateWebhookTest(context.Context, WebhookTestCommand) error
}

type AdapterTestRepository interface {
	CreateNotificationTest(context.Context, DeliveryAdapter, WebhookTestCommand) error
}
type scopedTestRepository struct {
	repository AdapterTestRepository
	adapter    DeliveryAdapter
}

func (repository scopedTestRepository) CreateWebhookTest(ctx context.Context, command WebhookTestCommand) error {
	return repository.repository.CreateNotificationTest(ctx, repository.adapter, command)
}

func NewAdapterTestService(repository AdapterTestRepository, identifiers TestIdentifierFactory, adapter DeliveryAdapter, reference string, now func() time.Time) (*TestService, error) {
	if repository == nil || !adapter.Valid() {
		return nil, ErrDeliveryUnavailable
	}
	return NewTestService(scopedTestRepository{repository: repository, adapter: adapter}, identifiers, reference, now)
}

type TestIdentifierFactory interface {
	NewNotificationID() (domain.NotificationID, error)
	NewAuditID() (domain.AuditEntryID, error)
}

type SecureTestIdentifierFactory struct{}

func (SecureTestIdentifierFactory) NewNotificationID() (domain.NotificationID, error) {
	value, err := secureTestIdentifier("notification_")
	return domain.NotificationID(value), err
}

func (SecureTestIdentifierFactory) NewAuditID() (domain.AuditEntryID, error) {
	value, err := secureTestIdentifier("audit_")
	return domain.AuditEntryID(value), err
}

func secureTestIdentifier(prefix string) (string, error) {
	randomBytes := make([]byte, 18)
	defer clear(randomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", ErrDeliveryUnavailable
	}
	value := prefix + base64.RawURLEncoding.EncodeToString(randomBytes)
	return value, nil
}

type TestService struct {
	repository             TestRepository
	identifiers            TestIdentifierFactory
	destinationReferenceID string
	now                    func() time.Time
}

func NewTestService(repository TestRepository, identifiers TestIdentifierFactory, destinationReferenceID string, now func() time.Time) (*TestService, error) {
	if repository == nil || identifiers == nil || !validDeliveryID(destinationReferenceID) || now == nil {
		return nil, ErrDeliveryUnavailable
	}
	return &TestService{
		repository: repository, identifiers: identifiers,
		destinationReferenceID: destinationReferenceID, now: now,
	}, nil
}

func (service *TestService) RequestTest(ctx context.Context, actor domain.UserID, requestID domain.RequestID) (domain.Notification, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || actor.Validate() != nil || requestID.Validate() != nil {
		return domain.Notification{}, ErrDeliveryUnavailable
	}
	at := service.now().UTC().Truncate(time.Microsecond)
	if at.IsZero() {
		return domain.Notification{}, ErrDeliveryUnavailable
	}
	notificationID, err := service.identifiers.NewNotificationID()
	if err != nil {
		return domain.Notification{}, ErrDeliveryUnavailable
	}
	auditID, err := service.identifiers.NewAuditID()
	if err != nil {
		return domain.Notification{}, ErrDeliveryUnavailable
	}
	item := domain.Notification{
		ID: notificationID, Kind: TestNotificationKind, Severity: domain.SeverityInfo,
		SummaryCode: TestNotificationReason, CreatedAt: at,
	}
	command := WebhookTestCommand{
		Notification: item, DestinationReferenceID: service.destinationReferenceID,
		MaximumAttempts: maximumTestAttempts,
		Audit: domain.AuditEntry{
			ID: auditID, OccurredAt: at, Actor: domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor)},
			Action: "notification.test", Target: domain.AuditTarget{Kind: "notification.destination", ID: service.destinationReferenceID},
			Outcome: domain.AuditIntent, RequestID: requestID, ReasonCode: TestNotificationReason,
		},
	}
	if command.Validate() != nil {
		return domain.Notification{}, ErrDeliveryUnavailable
	}
	if err := service.repository.CreateWebhookTest(ctx, command); err != nil {
		return domain.Notification{}, ErrDeliveryUnavailable
	}
	return item, nil
}

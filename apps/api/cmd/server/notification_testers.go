package main

import (
	"context"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/notification"
)

type notificationTesters struct{ webhook, matrix httpapi.NotificationTester }

func (testers *notificationTesters) RequestTest(ctx context.Context, user domain.UserID, request domain.RequestID) (domain.Notification, error) {
	if testers.webhook == nil {
		return domain.Notification{}, notification.ErrDeliveryUnavailable
	}
	return testers.webhook.RequestTest(ctx, user, request)
}
func (testers *notificationTesters) RequestMatrixTest(ctx context.Context, user domain.UserID, request domain.RequestID) (domain.Notification, error) {
	if testers.matrix == nil {
		return domain.Notification{}, notification.ErrDeliveryUnavailable
	}
	return testers.matrix.RequestTest(ctx, user, request)
}

func buildNotificationTesters(settings config.Config, reader productHistoryReader) (httpapi.NotificationTester, error) {
	if !settings.WebhookEnabled && !settings.MatrixEnabled {
		return nil, nil
	}
	repository, ok := any(reader).(notification.AdapterTestRepository)
	if !ok {
		return nil, nil
	}
	testers := &notificationTesters{}
	var err error
	if settings.WebhookEnabled {
		testers.webhook, err = notification.NewAdapterTestService(repository, notification.SecureTestIdentifierFactory{}, notification.DeliveryAdapterWebhook, settings.WebhookDestinationReferenceID, time.Now)
		if err != nil {
			return nil, err
		}
	}
	if settings.MatrixEnabled {
		testers.matrix, err = notification.NewAdapterTestService(repository, notification.SecureTestIdentifierFactory{}, notification.DeliveryAdapterMatrix, settings.MatrixDestinationReferenceID, time.Now)
		if err != nil {
			return nil, err
		}
	}
	return testers, nil
}

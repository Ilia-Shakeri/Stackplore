package main

import (
	"crypto/rand"
	"log/slog"
	"net"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/notification"
)

func buildMatrixDeliveryWorker(settings config.Config, database databaseRuntime, logger *slog.Logger) (*notification.DeliveryWorker, *notification.StaticMatrixTargetResolver, error) {
	if !settings.MatrixEnabled {
		return nil, nil, nil
	}
	repository, ok := database.(notification.AdapterDeliveryRepository)
	if !ok || logger == nil {
		return nil, nil, errDeliveryStart
	}
	token := []byte(settings.MatrixAccessToken.Reveal())
	defer clear(token)
	targets, err := notification.NewStaticMatrixTargetResolver(settings.MatrixDestinationReferenceID, settings.MatrixOrigin.Reveal(), settings.MatrixRoomID.Reveal(), token)
	if err != nil {
		return nil, nil, errDeliveryStart
	}
	randomBytes := make([]byte, 18)
	if _, err := rand.Read(randomBytes); err != nil {
		clear(randomBytes)
		targets.Close()
		return nil, nil, errDeliveryStart
	}
	workerID, err := notification.NewDeliveryWorkerID(randomBytes)
	clear(randomBytes)
	if err != nil {
		targets.Close()
		return nil, nil, errDeliveryStart
	}
	sender, err := notification.NewMatrixSender(notification.MatrixSenderOptions{Resolver: net.DefaultResolver, Dialer: &net.Dialer{Timeout: deliveryAttemptTimeout}, Timeout: deliveryAttemptTimeout, MaximumResponseBytes: 64 * 1024})
	if err != nil {
		targets.Close()
		return nil, nil, errDeliveryStart
	}
	worker, err := notification.NewDeliveryWorker(notification.DeliveryWorkerOptions{
		AdapterRepository: repository, Adapter: notification.DeliveryAdapterMatrix, Attempt: notification.MatrixAttempt(targets, sender),
		DestinationReferenceID: settings.MatrixDestinationReferenceID, LeaseOwner: workerID,
		Now: time.Now, PollInterval: deliveryPollInterval, LeaseDuration: deliveryLeaseDuration,
		BatchSize: deliveryBatchSize, MaximumAttempts: 3, RetrySchedule: []time.Duration{5 * time.Second, 30 * time.Second, 2 * time.Minute},
		OnError: func(error) { logger.Error("notification delivery failed", "reason", "delivery_unavailable") },
	})
	if err != nil {
		targets.Close()
		return nil, nil, errDeliveryStart
	}
	return worker, targets, nil
}

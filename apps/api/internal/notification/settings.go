package notification

import (
	"context"
	"fmt"
)

const WebhookAdapter = "generic_webhook"

type RuntimeSettings struct {
	Enabled                bool
	DestinationReferenceID string
}

func NewRuntimeSettings(enabled bool, destinationReferenceID string) (*RuntimeSettings, error) {
	if enabled {
		if !validDeliveryID(destinationReferenceID) {
			return nil, fmt.Errorf("notification runtime settings are invalid")
		}
	} else if destinationReferenceID != "" {
		return nil, fmt.Errorf("disabled notification settings contain a destination")
	}
	return &RuntimeSettings{Enabled: enabled, DestinationReferenceID: destinationReferenceID}, nil
}

func (settings *RuntimeSettings) GetNotificationSettings(ctx context.Context) (RuntimeSettings, error) {
	if settings == nil || ctx == nil || ctx.Err() != nil {
		return RuntimeSettings{}, ErrDeliveryUnavailable
	}
	return *settings, nil
}

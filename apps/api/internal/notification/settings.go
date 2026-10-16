package notification

import (
	"context"
	"fmt"
)

const WebhookAdapter = "generic_webhook"

type RuntimeSettings struct {
	Enabled                      bool
	DestinationReferenceID       string
	MatrixEnabled                bool
	MatrixDestinationReferenceID string
}

func NewCombinedRuntimeSettings(webhookEnabled bool, webhookReference string, matrixEnabled bool, matrixReference string) (*RuntimeSettings, error) {
	settings, err := NewRuntimeSettings(webhookEnabled, webhookReference)
	if err != nil {
		return nil, err
	}
	if matrixEnabled && !validDeliveryID(matrixReference) || !matrixEnabled && matrixReference != "" || matrixEnabled && webhookEnabled && matrixReference == webhookReference {
		return nil, ErrDeliveryUnavailable
	}
	settings.MatrixEnabled, settings.MatrixDestinationReferenceID = matrixEnabled, matrixReference
	return settings, nil
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

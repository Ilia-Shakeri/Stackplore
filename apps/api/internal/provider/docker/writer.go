package docker

import (
	"context"
	"errors"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"net/http"
	"strings"
	"time"
)

type WriterOptions struct {
	Client          *http.Client
	BaseURL         string
	MaximumResponse int64
	Now             func() time.Time
}
type SDKWriter struct {
	sdk       *client.Client
	transport *engineWriteTransport
}

func NewSDKWriter(o WriterOptions) (*SDKWriter, error) {
	boundary, base, err := engineClientBoundary(EngineCollectorOptions{Client: o.Client, BaseURL: o.BaseURL, MaximumItems: 1, MaximumResponse: o.MaximumResponse, InspectConcurrency: 1, Now: o.Now})
	if err != nil {
		return nil, err
	}
	read := &engineReadTransport{base: base, next: boundary.Transport, maximum: o.MaximumResponse}
	transport := &engineWriteTransport{read: read}
	boundary.Transport = transport
	boundary.Timeout = min(boundary.Timeout, 20*time.Second)
	sdk, err := client.New(client.WithHost(o.BaseURL), client.WithScheme(base.Scheme), client.WithHTTPClient(boundary))
	if err != nil {
		transport.CloseIdleConnections()
		return nil, ErrUnavailable
	}
	return &SDKWriter{sdk: sdk, transport: transport}, nil
}
func (w *SDKWriter) Close() error {
	if w == nil {
		return nil
	}
	w.transport.CloseIdleConnections()
	return w.sdk.Close()
}
func (w *SDKWriter) inspect(ctx context.Context, id, expected string) (container.InspectResponse, error) {
	if w == nil || ctx == nil || ctx.Err() != nil || !exactDockerContainerID(id) || domain.ValidateOperationVersion(expected) != nil {
		return container.InspectResponse{}, ErrUnavailable
	}
	result, err := w.sdk.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return container.InspectResponse{}, engineError(ctx, err)
	}
	value := result.Container
	if value.ID != id || value.State == nil || dockerContainerVersion(id, value) != expected {
		return container.InspectResponse{}, ErrVersionConflict
	}
	return value, nil
}
func (w *SDKWriter) PreviewContainer(ctx context.Context, id, version string) (string, error) {
	if ctx == nil {
		return "", ErrUnavailable
	}
	value, err := w.inspect(engineObservationContext(ctx), id, version)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(value.Name, "/"), nil
}
func (w *SDKWriter) RestartContainer(ctx context.Context, id, version, operationID string) (string, error) {
	if ctx == nil || domain.RequestID(operationID).Validate() != nil {
		return "", ErrUnavailable
	}
	ctx = engineObservationContext(ctx)
	before, err := w.inspect(ctx, id, version)
	if err != nil {
		return "", err
	}
	timeout := 10
	_, err = w.sdk.ContainerRestart(ctx, id, client.ContainerRestartOptions{Timeout: &timeout, Signal: "SIGTERM"})
	if err != nil {
		return "", err
	}
	after, err := w.sdk.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil || after.Container.ID != id || after.Container.State == nil || after.Container.State.StartedAt == before.State.StartedAt {
		return "", errors.Join(ErrUnavailable, domain.ErrOperationOutcomeUncertain)
	}
	current := dockerContainerVersion(id, after.Container)
	if current == "" || current == version {
		return "", errors.Join(ErrUnavailable, domain.ErrOperationOutcomeUncertain)
	}
	return current, nil
}

var _ OperationCollector = (*SDKWriter)(nil)

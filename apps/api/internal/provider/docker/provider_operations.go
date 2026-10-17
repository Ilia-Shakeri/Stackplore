package docker

import (
	"context"
	"errors"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"time"
)

type OperationalProvider struct {
	*Provider
	writer OperationCollector
}

func NewWithOperations(id domain.SourceID, name string, observer Collector, writer OperationCollector) (*OperationalProvider, error) {
	if writer == nil {
		return nil, ErrUnavailable
	}
	reader, err := New(id, name, observer)
	if err != nil {
		if closer, ok := writer.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		return nil, err
	}
	return &OperationalProvider{Provider: reader, writer: writer}, nil
}
func (p *OperationalProvider) Close() error {
	first := p.Provider.Close()
	if closer, ok := p.writer.(interface{ Close() error }); ok {
		return errors.Join(first, closer.Close())
	}
	return first
}
func (p *OperationalProvider) GetCapabilities(ctx context.Context, source domain.ProviderRef) (domain.CapabilitySet, error) {
	set, err := p.Provider.GetCapabilities(ctx, source)
	if err != nil {
		return set, err
	}
	for i := range set.Capabilities {
		if set.Capabilities[i].Name == domain.CapabilityDockerRestart {
			set.Capabilities[i].Status = domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: time.Now().UTC().Truncate(time.Microsecond)}
		}
	}
	return set, set.ValidateFor(source)
}
func (provider *OperationalProvider) PreviewOperation(ctx context.Context, request domain.OperationRequest) (domain.OperationImpact, error) {
	if err := provider.validate(ctx, request.Source); err != nil || request.Validate() != nil ||
		request.Action != domain.OperationDockerRestart || request.Resource.Identity.Kind != "docker.container" {
		return domain.OperationImpact{}, providerError("docker.operation.preview", firstError(err, request.Validate()))
	}
	if provider.writer == nil {
		return domain.OperationImpact{}, providerError("docker.operation.preview", ErrUnsupported)
	}
	if previewer, ok := provider.writer.(interface {
		PreviewContainer(context.Context, string, string) (string, error)
	}); ok {
		name, err := previewer.PreviewContainer(ctx, string(request.Resource.Identity.NativeID()), request.ExpectedVersion)
		if err != nil {
			return domain.OperationImpact{}, providerError("docker.operation.preview", err)
		}
		impact := domain.OperationImpact{Request: request, DisplayName: name, EffectCode: "operation.docker_restart", RiskCode: "operation.brief_interruption"}
		return impact, impact.Validate()
	}
	state, err := provider.observe(ctx)
	if err != nil {
		return domain.OperationImpact{}, err
	}
	for _, item := range state.resources {
		if item.Resource.Ref.ID != request.Resource.ID {
			continue
		}
		if item.Resource.Version == "" || item.Resource.Version != request.ExpectedVersion {
			return domain.OperationImpact{}, providerError("docker.operation.preview", ErrVersionConflict)
		}
		impact := domain.OperationImpact{
			Request: request, DisplayName: item.Resource.DisplayName,
			EffectCode: "operation.docker_restart", RiskCode: "operation.brief_interruption",
		}
		return impact, impact.Validate()
	}
	return domain.OperationImpact{}, providerError("docker.operation.preview", errors.New("resource not found"))
}

func (provider *OperationalProvider) ExecuteOperation(ctx context.Context, execution domain.OperationExecution) (domain.OperationProviderResult, error) {
	request := execution.Request
	if err := provider.validate(ctx, request.Source); err != nil || execution.Validate() != nil ||
		request.Action != domain.OperationDockerRestart || request.Resource.Identity.Kind != "docker.container" {
		return domain.OperationProviderResult{}, providerError("docker.operation.execute", firstError(err, execution.Validate()))
	}
	collector := provider.writer
	if collector == nil {
		return domain.OperationProviderResult{}, providerError("docker.operation.execute", ErrUnsupported)
	}
	version, err := collector.RestartContainer(
		ctx, string(request.Resource.Identity.NativeID()), request.ExpectedVersion, execution.OperationID,
	)
	if err != nil {
		return domain.OperationProviderResult{}, providerError("docker.operation.execute", err)
	}
	result := domain.OperationProviderResult{
		OperationID: execution.OperationID, Action: request.Action, ResourceID: request.Resource.ID, PreviousVersion: request.ExpectedVersion,
		CurrentVersion: version, EffectCode: "operation.docker_restart_started",
	}
	return result, result.ValidateFor(execution)
}

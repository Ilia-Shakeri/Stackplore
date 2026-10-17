package kubernetes

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type OperationalProvider struct {
	*Provider
	writer OperationCollector
}

func NewWithOperations(sourceID domain.SourceID, name string, observer Collector, writer OperationCollector) (*OperationalProvider, error) {
	if writer == nil {
		return nil, ErrUnavailable
	}
	provider, err := New(sourceID, name, observer)
	if err != nil {
		if closer, ok := writer.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		return nil, err
	}
	return &OperationalProvider{Provider: provider, writer: writer}, nil
}

func (provider *OperationalProvider) Close() error {
	var writerError error
	if closer, ok := provider.writer.(interface{ Close() error }); ok {
		writerError = closer.Close()
	}
	return errors.Join(provider.Provider.Close(), writerError)
}

func (provider *OperationalProvider) GetCapabilities(ctx context.Context, source domain.ProviderRef) (domain.CapabilitySet, error) {
	set, err := provider.Provider.GetCapabilities(ctx, source)
	if err != nil {
		return set, err
	}
	for index, item := range set.Capabilities {
		if item.Name == domain.CapabilityWorkloadRestart || item.Name == domain.CapabilityWorkloadScale {
			set.Capabilities[index].Status = domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: item.Status.ObservedAt}
		}
	}
	return set, set.ValidateFor(source)
}

func (provider *OperationalProvider) PreviewOperation(ctx context.Context, request domain.OperationRequest) (domain.OperationImpact, error) {
	if err := provider.validate(ctx, request.Source); err != nil || request.Validate() != nil {
		return domain.OperationImpact{}, providerError("kubernetes.operation.preview", firstError(err, request.Validate()))
	}
	if provider.writer == nil {
		return domain.OperationImpact{}, providerError("kubernetes.operation.preview", ErrUnsupported)
	}
	_, snapshot, err := provider.observe(ctx)
	if err != nil {
		return domain.OperationImpact{}, err
	}
	workload, ok := operationWorkload(snapshot, request.Resource)
	if !ok || !workloadSupports(request.Action, workload.Kind) {
		return domain.OperationImpact{}, providerError("kubernetes.operation.preview", ErrUnsupported)
	}
	if workload.Version == "" || kubernetesOperationVersion(workload.Version) != request.ExpectedVersion {
		return domain.OperationImpact{}, providerError("kubernetes.operation.preview", ErrVersionConflict)
	}
	if err := provider.writer.PreviewWorkload(ctx, WorkloadOperation{Kind: workload.Kind, Namespace: workload.Namespace, Name: workload.Name, ExpectedUID: workload.UID, ResourceVersion: workload.Version, RequestedReplicas: request.Parameters.Replicas}); err != nil {
		return domain.OperationImpact{}, providerError("kubernetes.operation.preview", err)
	}
	impact := domain.OperationImpact{
		Request: request, DisplayName: workload.Name,
		EffectCode: "operation.kubernetes_restart", RiskCode: "operation.brief_interruption",
	}
	if request.Action == domain.OperationKubernetesScale {
		current := workload.Desired
		impact.CurrentReplicas = &current
		impact.EffectCode = "operation.kubernetes_scale"
		impact.RiskCode = "operation.capacity_change"
	}
	return impact, impact.Validate()
}

func (provider *OperationalProvider) ExecuteOperation(ctx context.Context, execution domain.OperationExecution) (domain.OperationProviderResult, error) {
	request := execution.Request
	if err := provider.validate(ctx, request.Source); err != nil || execution.Validate() != nil {
		return domain.OperationProviderResult{}, providerError("kubernetes.operation.execute", firstError(err, execution.Validate()))
	}
	collector := provider.writer
	if collector == nil {
		return domain.OperationProviderResult{}, providerError("kubernetes.operation.execute", ErrUnsupported)
	}
	_, snapshot, err := provider.observe(ctx)
	if err != nil {
		return domain.OperationProviderResult{}, err
	}
	workload, ok := operationWorkload(snapshot, request.Resource)
	if !ok || !workloadSupports(request.Action, workload.Kind) {
		return domain.OperationProviderResult{}, providerError("kubernetes.operation.execute", ErrUnsupported)
	}
	if workload.Version == "" || kubernetesOperationVersion(workload.Version) != request.ExpectedVersion {
		return domain.OperationProviderResult{}, providerError("kubernetes.operation.execute", ErrVersionConflict)
	}
	operation := WorkloadOperation{
		Kind: workload.Kind, Namespace: workload.Namespace, Name: workload.Name,
		ResourceVersion: workload.Version, ExpectedUID: workload.UID, OperationID: execution.OperationID,
		RequestedReplicas: request.Parameters.Replicas,
	}
	var version string
	if request.Action == domain.OperationKubernetesScale {
		version, err = collector.ScaleWorkload(ctx, operation)
	} else {
		version, err = collector.RestartWorkload(ctx, operation)
	}
	if err != nil {
		if errors.Is(err, domain.ErrOperationOutcomeUncertain) {
			return domain.OperationProviderResult{}, domain.ErrOperationOutcomeUncertain
		}
		return domain.OperationProviderResult{}, providerError("kubernetes.operation.execute", err)
	}
	result := domain.OperationProviderResult{
		OperationID: execution.OperationID,
		Action:      request.Action, ResourceID: request.Resource.ID, PreviousVersion: request.ExpectedVersion,
		CurrentVersion: kubernetesOperationVersion(version), EffectCode: "operation.kubernetes_restart_started",
	}
	if request.Action == domain.OperationKubernetesScale {
		result.EffectCode = "operation.kubernetes_scale_accepted"
	}
	return result, result.ValidateFor(execution)
}

func operationWorkload(snapshot Snapshot, resource domain.ResourceRef) (Workload, bool) {
	for _, workload := range snapshot.Workloads {
		kind, err := workloadResourceKind(workload.Kind)
		if err == nil && workload.UID == string(resource.Identity.NativeID()) && kind == resource.Identity.Kind && resource.Identity.Scope == namespaceScope(workload.Namespace) {
			return workload, true
		}
	}
	return Workload{}, false
}

func (provider *OperationalProvider) ReconcileOperation(ctx context.Context, execution domain.OperationExecution) (domain.OperationProviderResult, bool, error) {
	if err := provider.validate(ctx, execution.Request.Source); err != nil || execution.Validate() != nil {
		return domain.OperationProviderResult{}, false, ErrUnavailable
	}
	reconciler, ok := provider.writer.(interface {
		ReconcileWorkload(context.Context, WorkloadOperation) (string, bool, error)
	})
	if !ok {
		return domain.OperationProviderResult{}, false, nil
	}
	_, snapshot, err := provider.observe(ctx)
	if err != nil {
		return domain.OperationProviderResult{}, false, err
	}
	workload, ok := operationWorkload(snapshot, execution.Request.Resource)
	if !ok || !workloadSupports(execution.Request.Action, workload.Kind) {
		return domain.OperationProviderResult{}, false, nil
	}
	version, accepted, err := reconciler.ReconcileWorkload(ctx, WorkloadOperation{Kind: workload.Kind, Namespace: workload.Namespace, Name: workload.Name, ExpectedUID: workload.UID, ResourceVersion: workload.Version, OperationID: execution.OperationID, RequestedReplicas: execution.Request.Parameters.Replicas})
	if err != nil || !accepted {
		return domain.OperationProviderResult{}, false, err
	}
	effect := domain.ReasonCode("operation.kubernetes_restart_started")
	if execution.Request.Action == domain.OperationKubernetesScale {
		effect = "operation.kubernetes_scale_accepted"
	}
	result := domain.OperationProviderResult{OperationID: execution.OperationID, Action: execution.Request.Action, ResourceID: execution.Request.Resource.ID, PreviousVersion: execution.Request.ExpectedVersion, CurrentVersion: kubernetesOperationVersion(version), EffectCode: effect}
	if result.ValidateFor(execution) != nil {
		return domain.OperationProviderResult{}, false, ErrUnavailable
	}
	return result, true, nil
}

var _ domain.OperationProvider = (*OperationalProvider)(nil)
var _ domain.OperationReconciler = (*OperationalProvider)(nil)

func workloadSupports(action domain.OperationAction, kind string) bool {
	switch action {
	case domain.OperationKubernetesRestart:
		return kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet"
	case domain.OperationKubernetesScale:
		return kind == "Deployment" || kind == "StatefulSet"
	default:
		return false
	}
}

func kubernetesOperationVersion(resourceVersion string) string {
	digest := sha256.Sum256([]byte("stackplore-kubernetes-version/v1\x00" + resourceVersion))
	return "kv_" + base64.RawURLEncoding.EncodeToString(digest[:])
}

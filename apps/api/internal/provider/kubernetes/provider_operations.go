package kubernetes

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (provider *Provider) PreviewOperation(ctx context.Context, request domain.OperationRequest) (domain.OperationImpact, error) {
	if err := provider.validate(ctx, request.Source); err != nil || request.Validate() != nil {
		return domain.OperationImpact{}, providerError("kubernetes.operation.preview", firstError(err, request.Validate()))
	}
	if _, ok := provider.collector.(OperationCollector); !ok {
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

func (provider *Provider) ExecuteOperation(ctx context.Context, execution domain.OperationExecution) (domain.OperationProviderResult, error) {
	request := execution.Request
	if err := provider.validate(ctx, request.Source); err != nil || execution.Validate() != nil {
		return domain.OperationProviderResult{}, providerError("kubernetes.operation.execute", firstError(err, execution.Validate()))
	}
	collector, ok := provider.collector.(OperationCollector)
	if !ok {
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
		ResourceVersion: workload.Version, IdempotencyKey: execution.IdempotencyKey,
		RequestedReplicas: request.Parameters.Replicas,
	}
	var version string
	if request.Action == domain.OperationKubernetesScale {
		version, err = collector.ScaleWorkload(ctx, operation)
	} else {
		version, err = collector.RestartWorkload(ctx, operation)
	}
	if err != nil {
		return domain.OperationProviderResult{}, providerError("kubernetes.operation.execute", err)
	}
	result := domain.OperationProviderResult{
		Action: request.Action, ResourceID: request.Resource.ID, PreviousVersion: request.ExpectedVersion,
		CurrentVersion: kubernetesOperationVersion(version), EffectCode: "operation.kubernetes_restart_started",
	}
	if request.Action == domain.OperationKubernetesScale {
		result.EffectCode = "operation.kubernetes_scale_accepted"
	}
	return result, result.ValidateFor(execution)
}

func operationWorkload(snapshot Snapshot, resource domain.ResourceRef) (Workload, bool) {
	for _, workload := range snapshot.Workloads {
		if workload.UID == string(resource.Identity.NativeID()) {
			return workload, true
		}
	}
	return Workload{}, false
}

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

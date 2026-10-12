package domain

import "fmt"

const MaxOperationReplicas = 10_000

type OperationAction string

const (
	OperationDockerRestart     OperationAction = "docker.container_restart"
	OperationKubernetesRestart OperationAction = "kubernetes.workload_restart"
	OperationKubernetesScale   OperationAction = "kubernetes.workload_scale"
)

func (action OperationAction) Validate() error {
	switch action {
	case OperationDockerRestart, OperationKubernetesRestart, OperationKubernetesScale:
		return nil
	default:
		return fmt.Errorf("unknown operation action %q", action)
	}
}

func ValidateOperationVersion(version string) error {
	return validateToken("operation version", version, maxIDBytes)
}

type OperationParameters struct {
	Replicas *int
}

func (parameters OperationParameters) ValidateFor(action OperationAction) error {
	if err := action.Validate(); err != nil {
		return err
	}
	if action != OperationKubernetesScale {
		if parameters.Replicas != nil {
			return fmt.Errorf("restart operation must not have replicas")
		}
		return nil
	}
	if parameters.Replicas == nil || *parameters.Replicas < 0 || *parameters.Replicas > MaxOperationReplicas {
		return fmt.Errorf("scale operation replicas are invalid")
	}
	return nil
}

type OperationRequest struct {
	Source          ProviderRef
	Resource        ResourceRef
	ExpectedVersion string
	Action          OperationAction
	Parameters      OperationParameters
}

func (request OperationRequest) Validate() error {
	if err := request.Source.Validate(); err != nil {
		return err
	}
	if err := request.Resource.Validate(); err != nil {
		return err
	}
	if !request.Source.Matches(request.Resource) {
		return fmt.Errorf("operation resource belongs to another source")
	}
	if err := ValidateOperationVersion(request.ExpectedVersion); err != nil {
		return err
	}
	return request.Parameters.ValidateFor(request.Action)
}

type OperationImpact struct {
	Request         OperationRequest
	DisplayName     string
	EffectCode      ReasonCode
	RiskCode        ReasonCode
	CurrentReplicas *int
}

type OperationExecution struct {
	Request        OperationRequest
	IdempotencyKey string
}

func (execution OperationExecution) Validate() error {
	if err := execution.Request.Validate(); err != nil {
		return err
	}
	return validateToken("operation idempotency key", execution.IdempotencyKey, maxIDBytes)
}

func (impact OperationImpact) Validate() error {
	if err := impact.Request.Validate(); err != nil {
		return err
	}
	if err := validateDisplayText("operation target name", impact.DisplayName, maxDisplayNameLength, false); err != nil {
		return err
	}
	if err := impact.EffectCode.Validate(); err != nil {
		return err
	}
	if err := impact.RiskCode.Validate(); err != nil {
		return err
	}
	if impact.Request.Action == OperationKubernetesScale {
		if impact.CurrentReplicas == nil || *impact.CurrentReplicas < 0 || *impact.CurrentReplicas > MaxOperationReplicas {
			return fmt.Errorf("scale impact current replicas are invalid")
		}
	} else if impact.CurrentReplicas != nil {
		return fmt.Errorf("restart impact must not have current replicas")
	}
	return nil
}

type OperationProviderResult struct {
	Action          OperationAction
	ResourceID      ResourceID
	PreviousVersion string
	CurrentVersion  string
	EffectCode      ReasonCode
}

func (result OperationProviderResult) ValidateFor(execution OperationExecution) error {
	if err := execution.Validate(); err != nil {
		return err
	}
	request := execution.Request
	if result.Action != request.Action || result.ResourceID != request.Resource.ID || result.PreviousVersion != request.ExpectedVersion {
		return fmt.Errorf("operation result does not match request")
	}
	if err := ValidateOperationVersion(result.CurrentVersion); err != nil {
		return err
	}
	return result.EffectCode.Validate()
}

package domain

import (
	"fmt"
	"math"
)

type KubernetesContainerState string

const (
	KubernetesContainerUnknown    KubernetesContainerState = "unknown"
	KubernetesContainerWaiting    KubernetesContainerState = "waiting"
	KubernetesContainerRunning    KubernetesContainerState = "running"
	KubernetesContainerTerminated KubernetesContainerState = "terminated"
)

type KubernetesConditionType string
type KubernetesConditionStatus string

const (
	KubernetesConditionReady           KubernetesConditionType   = "ready"
	KubernetesConditionContainersReady KubernetesConditionType   = "containers_ready"
	KubernetesConditionInitialized     KubernetesConditionType   = "initialized"
	KubernetesConditionScheduled       KubernetesConditionType   = "scheduled"
	KubernetesConditionAvailable       KubernetesConditionType   = "available"
	KubernetesConditionProgressing     KubernetesConditionType   = "progressing"
	KubernetesConditionReplicaFailure  KubernetesConditionType   = "replica_failure"
	KubernetesConditionComplete        KubernetesConditionType   = "complete"
	KubernetesConditionFailed          KubernetesConditionType   = "failed"
	KubernetesConditionTrue            KubernetesConditionStatus = "true"
	KubernetesConditionFalse           KubernetesConditionStatus = "false"
	KubernetesConditionUnknown         KubernetesConditionStatus = "unknown"
)

type KubernetesContainerMetadata struct {
	Name               string                   `json:"name"`
	State              KubernetesContainerState `json:"state"`
	Ready              bool                     `json:"ready"`
	RestartCount       uint64                   `json:"restart_count"`
	ReasonCode         ReasonCode               `json:"reason_code,omitempty"`
	CPURequestCores    *float64                 `json:"cpu_request_cores,omitempty"`
	CPULimitCores      *float64                 `json:"cpu_limit_cores,omitempty"`
	MemoryRequestBytes *uint64                  `json:"memory_request_bytes,omitempty"`
	MemoryLimitBytes   *uint64                  `json:"memory_limit_bytes,omitempty"`
}

type KubernetesConditionMetadata struct {
	Type       KubernetesConditionType   `json:"type"`
	Status     KubernetesConditionStatus `json:"status"`
	ReasonCode ReasonCode                `json:"reason_code,omitempty"`
}

// Only reviewed facts belong here. Provider messages, environment values,
// commands, credentials and arbitrary labels have no metadata field.
type KubernetesResourceMetadata struct {
	Containers []KubernetesContainerMetadata `json:"containers"`
	Conditions []KubernetesConditionMetadata `json:"conditions"`
}

func (metadata KubernetesResourceMetadata) Validate() error {
	if metadata.Containers == nil || metadata.Conditions == nil || len(metadata.Containers) > 64 || len(metadata.Conditions) > 16 {
		return fmt.Errorf("Kubernetes metadata exceeds its bound")
	}
	names := make(map[string]struct{}, len(metadata.Containers))
	for _, container := range metadata.Containers {
		if err := container.Validate(); err != nil {
			return err
		}
		if _, duplicate := names[container.Name]; duplicate {
			return fmt.Errorf("duplicate Kubernetes container metadata")
		}
		names[container.Name] = struct{}{}
	}
	types := make(map[KubernetesConditionType]struct{}, len(metadata.Conditions))
	for _, condition := range metadata.Conditions {
		if err := condition.Validate(); err != nil {
			return err
		}
		if _, duplicate := types[condition.Type]; duplicate {
			return fmt.Errorf("duplicate Kubernetes condition metadata")
		}
		types[condition.Type] = struct{}{}
	}
	return nil
}

func (container KubernetesContainerMetadata) Validate() error {
	if !safeKubernetesContainerName(container.Name) || container.RestartCount > 1<<53-1 {
		return fmt.Errorf("invalid Kubernetes container metadata")
	}
	switch container.State {
	case KubernetesContainerUnknown, KubernetesContainerWaiting, KubernetesContainerRunning, KubernetesContainerTerminated:
	default:
		return fmt.Errorf("invalid Kubernetes container state")
	}
	switch container.ReasonCode {
	case "", "kubernetes.container.waiting", "kubernetes.container.running", "kubernetes.container.terminated",
		"kubernetes.container.crash_loop", "kubernetes.container.image_pull", "kubernetes.container.config_error",
		"kubernetes.container.creating", "kubernetes.container.out_of_memory", "kubernetes.container.completed", "kubernetes.container.error":
	default:
		return fmt.Errorf("invalid Kubernetes container reason")
	}
	for _, value := range []*float64{container.CPURequestCores, container.CPULimitCores} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value > 1e9) {
			return fmt.Errorf("invalid Kubernetes CPU quantity")
		}
	}
	for _, value := range []*uint64{container.MemoryRequestBytes, container.MemoryLimitBytes} {
		if value != nil && *value > 1<<53-1 {
			return fmt.Errorf("invalid Kubernetes memory quantity")
		}
	}
	return nil
}

func (condition KubernetesConditionMetadata) Validate() error {
	switch condition.Type {
	case KubernetesConditionReady, KubernetesConditionContainersReady, KubernetesConditionInitialized, KubernetesConditionScheduled,
		KubernetesConditionAvailable, KubernetesConditionProgressing, KubernetesConditionReplicaFailure, KubernetesConditionComplete, KubernetesConditionFailed:
	default:
		return fmt.Errorf("invalid Kubernetes condition type")
	}
	switch condition.Status {
	case KubernetesConditionTrue, KubernetesConditionFalse, KubernetesConditionUnknown:
	default:
		return fmt.Errorf("invalid Kubernetes condition status")
	}
	switch condition.ReasonCode {
	case "", "kubernetes.condition.true", "kubernetes.condition.false", "kubernetes.condition.unknown":
	default:
		return fmt.Errorf("invalid Kubernetes condition reason")
	}
	return nil
}

func safeKubernetesContainerName(value string) bool {
	if len(value) < 1 || len(value) > 63 {
		return false
	}
	for index, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			continue
		}
		if character != '-' || index == 0 || index == len(value)-1 {
			return false
		}
	}
	return true
}

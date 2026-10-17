package domain

import (
	"math"
	"testing"
)

func TestKubernetesMetadataAllowsOnlyBoundedReviewedFacts(t *testing.T) {
	cpu, memory := 0.25, uint64(128<<20)
	valid := KubernetesResourceMetadata{Containers: []KubernetesContainerMetadata{{Name: "api", State: KubernetesContainerRunning, Ready: true, RestartCount: 2, CPURequestCores: &cpu, MemoryLimitBytes: &memory}},
		Conditions: []KubernetesConditionMetadata{{Type: KubernetesConditionReady, Status: KubernetesConditionTrue, ReasonCode: "kubernetes.condition.true"}}}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, state := range []KubernetesContainerState{KubernetesContainerUnknown, KubernetesContainerWaiting, KubernetesContainerTerminated} {
		container := valid.Containers[0]
		container.State = state
		if err := container.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, invalid := range []float64{-1, 1e9 + 1, math.NaN(), math.Inf(1)} {
		container := valid.Containers[0]
		container.CPULimitCores = &invalid
		if container.Validate() == nil {
			t.Fatal("unsafe CPU quantity accepted")
		}
	}
	for _, mutate := range []func(*KubernetesResourceMetadata){
		func(value *KubernetesResourceMetadata) { value.Containers = nil },
		func(value *KubernetesResourceMetadata) { value.Conditions = nil },
		func(value *KubernetesResourceMetadata) {
			value.Containers = append(value.Containers, value.Containers[0])
		},
		func(value *KubernetesResourceMetadata) { value.Containers[0].Name = "raw/host" },
		func(value *KubernetesResourceMetadata) { value.Containers[0].State = "raw-state" },
		func(value *KubernetesResourceMetadata) { value.Containers[0].ReasonCode = "raw-private-message" },
		func(value *KubernetesResourceMetadata) { value.Containers[0].RestartCount = 1 << 53 },
		func(value *KubernetesResourceMetadata) {
			n := uint64(1 << 53)
			value.Containers[0].MemoryLimitBytes = &n
		},
		func(value *KubernetesResourceMetadata) { value.Conditions[0].Type = "raw-type" },
		func(value *KubernetesResourceMetadata) { value.Conditions[0].Status = "raw-status" },
		func(value *KubernetesResourceMetadata) { value.Conditions[0].ReasonCode = "private-message" },
		func(value *KubernetesResourceMetadata) {
			value.Conditions = append(value.Conditions, value.Conditions[0])
		},
		func(value *KubernetesResourceMetadata) { value.Containers = make([]KubernetesContainerMetadata, 65) },
		func(value *KubernetesResourceMetadata) { value.Conditions = make([]KubernetesConditionMetadata, 17) },
	} {
		value := KubernetesResourceMetadata{Containers: append([]KubernetesContainerMetadata(nil), valid.Containers...), Conditions: append([]KubernetesConditionMetadata(nil), valid.Conditions...)}
		mutate(&value)
		if value.Validate() == nil {
			t.Fatal("unsafe metadata accepted")
		}
	}
}

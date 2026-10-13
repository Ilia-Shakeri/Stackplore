package domain

import "fmt"

type CapabilityName string

const (
	CapabilityInventoryList   CapabilityName = "inventory.list"
	CapabilityInventoryDetail CapabilityName = "inventory.detail"
	CapabilityHealth          CapabilityName = "health.read"
	CapabilityTopology        CapabilityName = "topology.read"
	CapabilityLogRead         CapabilityName = "log.read"
	CapabilityLogTail         CapabilityName = "log.tail"
	CapabilityMetricCurrent   CapabilityName = "metric.current"
	CapabilityMetricHistory   CapabilityName = "metric.history"
	CapabilityEventHistory    CapabilityName = "event.history"
	CapabilityEventWatch      CapabilityName = "event.watch"
	CapabilityDockerRestart   CapabilityName = "operation.docker_restart"
	CapabilityWorkloadRestart CapabilityName = "operation.workload_restart"
	CapabilityWorkloadScale   CapabilityName = "operation.workload_scale"
)

var requiredCapabilities = [...]CapabilityName{
	CapabilityInventoryList,
	CapabilityInventoryDetail,
	CapabilityHealth,
	CapabilityTopology,
	CapabilityLogRead,
	CapabilityLogTail,
	CapabilityMetricCurrent,
	CapabilityMetricHistory,
	CapabilityEventHistory,
	CapabilityEventWatch,
	CapabilityDockerRestart,
	CapabilityWorkloadRestart,
	CapabilityWorkloadScale,
}

// CapabilityNames returns the fixed safe capability order without shared mutable state.
func CapabilityNames() []CapabilityName {
	return append([]CapabilityName(nil), requiredCapabilities[:]...)
}

func (name CapabilityName) Validate() error {
	for _, known := range requiredCapabilities {
		if name == known {
			return nil
		}
	}
	return fmt.Errorf("unknown capability %q", name)
}

type CapabilityStatus struct {
	Name   CapabilityName
	Status ObservationStatus
}

func (capability CapabilityStatus) Validate() error {
	if err := capability.Name.Validate(); err != nil {
		return err
	}
	return capability.Status.Validate()
}

type CapabilitySet struct {
	Source       ProviderRef
	Capabilities []CapabilityStatus
}

func (set CapabilitySet) Validate() error {
	if err := set.Source.Validate(); err != nil {
		return err
	}
	if len(set.Capabilities) != len(requiredCapabilities) {
		return fmt.Errorf("capability set must report every known capability")
	}
	for index, capability := range set.Capabilities {
		if err := capability.Validate(); err != nil {
			return err
		}
		if capability.Name != requiredCapabilities[index] {
			return fmt.Errorf("capability set must use the fixed capability order")
		}
	}
	return nil
}

func (set CapabilitySet) ValidateFor(source ProviderRef) error {
	if err := source.Validate(); err != nil {
		return err
	}
	if err := set.Validate(); err != nil {
		return err
	}
	if set.Source != source {
		return fmt.Errorf("capability set belongs to another source")
	}
	return nil
}

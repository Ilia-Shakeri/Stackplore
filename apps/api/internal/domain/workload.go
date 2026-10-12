package domain

import "fmt"

type WorkloadReplicaSnapshot struct {
	Resource ResourceRef
	Desired  uint64
	Ready    uint64
	Status   ObservationStatus
}

func (snapshot WorkloadReplicaSnapshot) ValidateFor(query ResourceDetailQuery) error {
	if err := query.Validate(); err != nil {
		return err
	}
	if err := snapshot.Resource.Validate(); err != nil {
		return err
	}
	if err := snapshot.Status.Validate(); err != nil {
		return err
	}
	if !query.Source.Matches(snapshot.Resource) || query.ResourceID != snapshot.Resource.ID {
		return fmt.Errorf("workload replica snapshot does not match requested resource")
	}
	if snapshot.Ready > snapshot.Desired {
		return fmt.Errorf("workload ready replica count exceeds desired count")
	}
	return nil
}

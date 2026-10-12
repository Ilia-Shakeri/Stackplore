package domain

import "testing"

func TestWorkloadReplicaSnapshotBindsResourceAndCounts(t *testing.T) {
	resource := fixtureResourceRef(t, "workload-alpha")
	query := ResourceDetailQuery{Source: fixtureProvider(), ResourceID: resource.ID}
	snapshot := WorkloadReplicaSnapshot{Resource: resource, Desired: 3, Ready: 2, Status: fixtureStatus()}
	if err := snapshot.ValidateFor(query); err != nil {
		t.Fatalf("ValidateFor() error = %v", err)
	}

	snapshot.Ready = 4
	requireInvalid(t, snapshot.ValidateFor(query))
	snapshot.Ready = 2
	query.ResourceID = fixtureResourceRef(t, "workload-beta").ID
	requireInvalid(t, snapshot.ValidateFor(query))
}

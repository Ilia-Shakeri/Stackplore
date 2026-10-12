package domain

import "testing"

func TestOperationRequestBindsExactActionVersionAndParameters(t *testing.T) {
	t.Parallel()
	resource := fixtureResourceRef(t, "native_operation")
	replicas := 3
	request := OperationRequest{
		Source: fixtureProvider(), Resource: resource, ExpectedVersion: "rv_17",
		Action: OperationKubernetesScale, Parameters: OperationParameters{Replicas: &replicas},
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	request.Action = OperationDockerRestart
	if request.Validate() == nil {
		t.Fatal("restart request accepted scale parameters")
	}
	request.Action = OperationKubernetesScale
	tooMany := MaxOperationReplicas + 1
	request.Parameters.Replicas = &tooMany
	if request.Validate() == nil {
		t.Fatal("scale request accepted excessive replicas")
	}
}

package domain

import "testing"

func TestOperationRequestBindsExactActionVersionAndParameters(t *testing.T) {
	t.Parallel()
	identity := fixtureIdentity("native_operation")
	identity.Provider, identity.SourceID, identity.Kind = ProviderKubernetes, "source_kubernetes", "kubernetes.deployment"
	identity.Scope = Scope{Kind: ScopeNamespace, Name: "shop"}
	resource, err := NewResourceRef(identity)
	if err != nil {
		t.Fatal(err)
	}
	replicas := 3
	request := OperationRequest{
		Source: ProviderRef{Provider: ProviderKubernetes, SourceID: "source_kubernetes"}, Resource: resource, ExpectedVersion: "rv_17",
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

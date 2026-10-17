package domain

// ResourceMetadata contains only reviewed facts. Provider payloads and arbitrary
// maps cannot cross this boundary.
type ResourceMetadata struct {
	Docker     *DockerResourceMetadata     `json:"docker,omitempty"`
	Kubernetes *KubernetesResourceMetadata `json:"kubernetes,omitempty"`
}

func (metadata ResourceMetadata) ValidateFor(provider ProviderKind) error {
	if provider == ProviderDocker && metadata.Docker != nil && metadata.Kubernetes == nil {
		return metadata.Docker.Validate()
	}
	if provider == ProviderKubernetes && metadata.Kubernetes != nil && metadata.Docker == nil {
		return metadata.Kubernetes.Validate()
	}
	return ErrRepositoryInvalidInput
}

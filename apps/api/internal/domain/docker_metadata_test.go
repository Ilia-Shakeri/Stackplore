package domain

import (
	"math"
	"testing"
	"time"
)

func TestDockerMetadataRejectsUnsafeAndUnboundedFacts(t *testing.T) {
	valid := func() DockerResourceMetadata {
		return DockerResourceMetadata{EngineVersion: "29.0.0", State: "running", Health: "healthy", Image: "registry.example:5000/shop/api:1.0",
			Labels: []DockerLabelMetadata{{Key: "com.docker.compose.project", Value: "shop"}},
			Mounts: []DockerMountMetadata{{Type: "bind", Target: "/data", ReadOnly: true}},
			Ports:  []DockerPortMetadata{{ContainerPort: 8080, Protocol: "tcp"}}}
	}
	if err := valid().Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*DockerResourceMetadata){
		"registry credential": func(value *DockerResourceMetadata) { value.Image = "user:password@registry.example/shop/api" },
		"URL image":           func(value *DockerResourceMetadata) { value.Image = "https://registry.example/shop/api" },
		"running exit":        func(value *DockerResourceMetadata) { code := int32(1); value.ExitCode = &code },
		"control":             func(value *DockerResourceMetadata) { value.EngineVersion = "29\nunsafe" },
		"secret label":        func(value *DockerResourceMetadata) { value.Labels[0].Key = "secret.token" },
		"label value":         func(value *DockerResourceMetadata) { value.Labels[0].Value = "project\nunsafe" },
		"unknown state":       func(value *DockerResourceMetadata) { value.State = "anything" },
		"unknown health":      func(value *DockerResourceMetadata) { value.Health = "anything" },
		"port zero":           func(value *DockerResourceMetadata) { value.Ports[0].ContainerPort = 0 },
		"publication":         func(value *DockerResourceMetadata) { value.Ports[0].Published = true },
		"large ports":         func(value *DockerResourceMetadata) { value.Ports = make([]DockerPortMetadata, MaxDockerPorts+1) },
		"mount source":        func(value *DockerResourceMetadata) { value.Mounts[0].Type = "host" },
		"relative mount":      func(value *DockerResourceMetadata) { value.Mounts[0].Target = "relative" },
		"nan limit":           func(value *DockerResourceMetadata) { limit := math.NaN(); value.CPULimitCores = &limit },
		"zero limit":          func(value *DockerResourceMetadata) { limit := uint64(0); value.MemoryLimitBytes = &limit },
		"timestamp precision": func(value *DockerResourceMetadata) {
			at := time.Date(2026, 10, 17, 0, 0, 0, 1, time.UTC)
			value.StartedAt = &at
		},
	} {
		t.Run(name, func(t *testing.T) {
			value := valid()
			change(&value)
			if value.Validate() == nil {
				t.Fatal("unsafe metadata accepted")
			}
		})
	}
}

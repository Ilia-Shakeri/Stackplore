package domain

import (
	"math"
	"regexp"
	"strings"
	"time"
)

const (
	MaxDockerPorts  = 64
	MaxDockerMounts = 64
	MaxDockerLabels = 16
)

type DockerResourceMetadata struct {
	EngineVersion    string                `json:"engine_version,omitempty"`
	State            string                `json:"state"`
	Image            string                `json:"image,omitempty"`
	Health           string                `json:"health"`
	RestartCount     uint64                `json:"restart_count"`
	StartedAt        *time.Time            `json:"started_at,omitempty"`
	ExitCode         *int32                `json:"exit_code,omitempty"`
	Ports            []DockerPortMetadata  `json:"ports"`
	Mounts           []DockerMountMetadata `json:"mounts"`
	Labels           []DockerLabelMetadata `json:"labels"`
	CPULimitCores    *float64              `json:"cpu_limit_cores,omitempty"`
	MemoryLimitBytes *uint64               `json:"memory_limit_bytes,omitempty"`
}

type DockerPortMetadata struct {
	ContainerPort uint16  `json:"container_port"`
	HostPort      *uint16 `json:"host_port,omitempty"`
	Protocol      string  `json:"protocol"`
	Published     bool    `json:"published"`
}

type DockerMountMetadata struct {
	Type     string `json:"type"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only"`
}

type DockerLabelMetadata struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var dockerPublicToken = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
var dockerImageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$`)
var dockerImageDigest = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func (metadata DockerResourceMetadata) Validate() error {
	if validateDisplayText("Docker version", metadata.EngineVersion, 64, true) != nil ||
		metadata.Ports == nil || metadata.Mounts == nil || metadata.Labels == nil ||
		len(metadata.Ports) > MaxDockerPorts || len(metadata.Mounts) > MaxDockerMounts || len(metadata.Labels) > MaxDockerLabels ||
		metadata.RestartCount > 1<<53-1 {
		return ErrRepositoryInvalidInput
	}
	switch metadata.State {
	case "created", "running", "paused", "restarting", "exited", "removing", "dead", "unknown":
	default:
		return ErrRepositoryInvalidInput
	}
	switch metadata.Health {
	case "healthy", "unhealthy", "starting", "none", "unknown":
	default:
		return ErrRepositoryInvalidInput
	}
	if metadata.Image != "" {
		name, digest, found := strings.Cut(metadata.Image, "@")
		if len(metadata.Image) > 256 || strings.Contains(name, "//") || !dockerImageName.MatchString(name) || found && !dockerImageDigest.MatchString(digest) {
			return ErrRepositoryInvalidInput
		}
	}
	if metadata.ExitCode != nil && metadata.State != "exited" && metadata.State != "dead" {
		return ErrRepositoryInvalidInput
	}
	if metadata.StartedAt != nil && (metadata.StartedAt.IsZero() || metadata.StartedAt.Location() != time.UTC ||
		metadata.StartedAt.Nanosecond()%int(time.Microsecond) != 0 || !repositoryTimeSupported(*metadata.StartedAt)) {
		return ErrRepositoryInvalidInput
	}
	if metadata.CPULimitCores != nil && (math.IsNaN(*metadata.CPULimitCores) || math.IsInf(*metadata.CPULimitCores, 0) ||
		*metadata.CPULimitCores <= 0 || *metadata.CPULimitCores > 1<<20) ||
		metadata.MemoryLimitBytes != nil && (*metadata.MemoryLimitBytes == 0 || *metadata.MemoryLimitBytes > 1<<53-1) {
		return ErrRepositoryInvalidInput
	}
	type portKey struct {
		container, host uint16
		protocol        string
		published       bool
	}
	ports := map[portKey]struct{}{}
	for _, port := range metadata.Ports {
		if port.ContainerPort == 0 || port.Protocol != "tcp" && port.Protocol != "udp" && port.Protocol != "sctp" ||
			port.Published != (port.HostPort != nil) || port.HostPort != nil && *port.HostPort == 0 {
			return ErrRepositoryInvalidInput
		}
		// A value key avoids treating equal host ports at distinct pointers as unique.
		key := portKey{container: port.ContainerPort, protocol: port.Protocol, published: port.Published}
		if port.HostPort != nil {
			key.host = *port.HostPort
		}
		if _, duplicate := ports[key]; duplicate {
			return ErrRepositoryInvalidInput
		}
		ports[key] = struct{}{}
	}
	for _, mount := range metadata.Mounts {
		if mount.Type != "volume" && mount.Type != "bind" && mount.Type != "tmpfs" ||
			!strings.HasPrefix(mount.Target, "/") || validateDisplayText("Docker mount target", mount.Target, 256, false) != nil {
			return ErrRepositoryInvalidInput
		}
	}
	labels := map[string]struct{}{}
	for _, label := range metadata.Labels {
		if _, duplicate := labels[label.Key]; duplicate || len(label.Value) > 256 {
			return ErrRepositoryInvalidInput
		}
		labels[label.Key] = struct{}{}
		switch label.Key {
		case "com.docker.compose.project", "com.docker.compose.service":
			if !dockerPublicToken.MatchString(label.Value) {
				return ErrRepositoryInvalidInput
			}
		case "stackplore.io/depends-on":
			items := strings.Split(label.Value, ",")
			if len(items) > 32 {
				return ErrRepositoryInvalidInput
			}
			for _, item := range items {
				if !dockerPublicToken.MatchString(item) {
					return ErrRepositoryInvalidInput
				}
			}
		default:
			return ErrRepositoryInvalidInput
		}
	}
	return nil
}

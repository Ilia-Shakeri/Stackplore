package docker

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func containerMetadata(inspection container.InspectResponse, entry Container) (*domain.DockerResourceMetadata, bool) {
	metadata := &domain.DockerResourceMetadata{State: entry.State, Health: "unknown", Ports: []domain.DockerPortMetadata{}, Mounts: []domain.DockerMountMetadata{}, Labels: []domain.DockerLabelMetadata{}}
	if inspection.State == nil {
		metadata.State = "unknown"
		return metadata, false
	}
	if inspection.RestartCount < 0 {
		return nil, false
	}
	metadata.RestartCount = uint64(inspection.RestartCount)
	metadata.Health = entry.Health
	if metadata.Health == "" {
		metadata.Health = "none"
	}
	if at, err := time.Parse(time.RFC3339Nano, inspection.State.StartedAt); err == nil && !at.IsZero() {
		at = at.UTC().Truncate(time.Microsecond)
		metadata.StartedAt = &at
	}
	if entry.State == "exited" || entry.State == "dead" {
		if inspection.State.ExitCode < -1<<31 || inspection.State.ExitCode > 1<<31-1 {
			return nil, false
		}
		value := int32(inspection.State.ExitCode)
		metadata.ExitCode = &value
	}
	if inspection.Config != nil {
		metadata.Image = inspection.Config.Image
	}
	if inspection.HostConfig != nil {
		limits := inspection.HostConfig.Resources
		if limits.NanoCPUs > 0 {
			value := float64(limits.NanoCPUs) / 1e9
			metadata.CPULimitCores = &value
		} else if limits.CPUQuota > 0 && limits.CPUPeriod > 0 {
			value := float64(limits.CPUQuota) / float64(limits.CPUPeriod)
			metadata.CPULimitCores = &value
		}
		if limits.Memory > 0 {
			value := uint64(limits.Memory)
			metadata.MemoryLimitBytes = &value
		}
	}
	ports := map[network.Port][]network.PortBinding{}
	if inspection.Config != nil {
		for port := range inspection.Config.ExposedPorts {
			ports[port] = nil
		}
	}
	if inspection.NetworkSettings != nil {
		for port, bindings := range inspection.NetworkSettings.Ports {
			ports[port] = bindings
		}
	}
	type key struct {
		port, host uint16
		protocol   string
	}
	seen := map[key]struct{}{}
	for port, bindings := range ports {
		if !port.IsValid() {
			continue
		}
		if len(bindings) == 0 {
			seen[key{port: port.Num(), protocol: string(port.Proto())}] = struct{}{}
		}
		for _, binding := range bindings {
			host, err := strconv.ParseUint(binding.HostPort, 10, 16)
			if err == nil && host != 0 {
				seen[key{port: port.Num(), host: uint16(host), protocol: string(port.Proto())}] = struct{}{}
			}
		}
	}
	keys := make([]key, 0, len(seen))
	for item := range seen {
		keys = append(keys, item)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].port != keys[j].port {
			return keys[i].port < keys[j].port
		}
		if keys[i].protocol != keys[j].protocol {
			return keys[i].protocol < keys[j].protocol
		}
		return keys[i].host < keys[j].host
	})
	complete := len(keys) <= domain.MaxDockerPorts && len(inspection.Mounts) <= domain.MaxDockerMounts
	for _, item := range keys[:min(len(keys), domain.MaxDockerPorts)] {
		port := domain.DockerPortMetadata{ContainerPort: item.port, Protocol: item.protocol, Published: item.host != 0}
		if item.host != 0 {
			host := item.host
			port.HostPort = &host
		}
		metadata.Ports = append(metadata.Ports, port)
	}
	for _, mount := range inspection.Mounts[:min(len(inspection.Mounts), domain.MaxDockerMounts)] {
		if mount.Type == "volume" || mount.Type == "bind" || mount.Type == "tmpfs" {
			metadata.Mounts = append(metadata.Mounts, domain.DockerMountMetadata{Type: string(mount.Type), Target: mount.Destination, ReadOnly: !mount.RW})
		}
	}
	sort.Slice(metadata.Mounts, func(i, j int) bool { return metadata.Mounts[i].Target < metadata.Mounts[j].Target })
	for _, label := range []domain.DockerLabelMetadata{
		{Key: "com.docker.compose.project", Value: entry.ComposeProject},
		{Key: "com.docker.compose.service", Value: entry.ComposeService},
		{Key: "stackplore.io/depends-on", Value: strings.Join(entry.DependsOn, ",")},
	} {
		if label.Value != "" && len(label.Value) <= 256 {
			metadata.Labels = append(metadata.Labels, label)
		}
	}
	if metadata.Validate() != nil {
		return nil, false
	}
	return metadata, complete
}

package demo

import (
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"time"
)

func (provider *Provider) metricSample(resource domain.ResourceRef, name domain.MetricName, at time.Time) domain.MetricSample {
	profile := provider.profiles[resource.ID]
	tick := int(at.Sub(observedAt) / scenarioTick)
	wave := ((tick % 4) + 4) % 4
	cpu := profile.cpu
	if resource.Identity.NativeID() == "checkout" && tick > 0 {
		cpu = 0.36
	}
	var value float64
	switch name {
	case domain.MetricCPUUsageCores:
		value = cpu + float64(wave)/100
	case domain.MetricMemoryUsageBytes:
		value = float64(profile.memoryMiB+wave*2) * 1024 * 1024
	case domain.MetricRestartCount:
		value = float64(profile.restarts)
		if resource.Identity.NativeID() == "checkout" && tick > 0 {
			value++
		}
	case domain.MetricNetworkReceiveBytesTotal:
		value = float64(profile.networkMiB*1024*1024 + (tick+12)*256*1024)
	case domain.MetricNetworkSendBytesTotal:
		value = float64(profile.networkMiB*512*1024 + (tick+12)*128*1024)
	case domain.MetricCPURequestCores:
		value = 0.5
	case domain.MetricCPULimitCores:
		value = 1
	case domain.MetricMemoryRequestBytes:
		value = float64(profile.memoryMiB) * 1024 * 1024
	case domain.MetricMemoryLimitBytes:
		value = float64(profile.memoryMiB*2) * 1024 * 1024
	}
	return domain.MetricSample{ID: domain.MetricSampleID(factID("metric", string(resource.ID), string(name), at.Format(time.RFC3339Nano))), ResourceID: resource.ID, Name: name, Unit: metricUnit(name), Value: value, SampledAt: at, Interval: scenarioTick, Synthetic: true}
}

package domain

import (
	"fmt"
	"math"
	"time"
)

const (
	MaxMetricTargets       = 100
	MaxMetricHistoryPoints = 200
	MaxMetricLookback      = 30 * 24 * time.Hour
)

type MetricName string

const (
	MetricCPUUsageCores            MetricName = "cpu.usage.cores"
	MetricMemoryUsageBytes         MetricName = "memory.usage.bytes"
	MetricNetworkReceiveBytesTotal MetricName = "network.receive.bytes_total"
	MetricNetworkSendBytesTotal    MetricName = "network.send.bytes_total"
	MetricRestartCount             MetricName = "restart.count"
	MetricCPURequestCores          MetricName = "cpu.request.cores"
	MetricCPULimitCores            MetricName = "cpu.limit.cores"
	MetricMemoryRequestBytes       MetricName = "memory.request.bytes"
	MetricMemoryLimitBytes         MetricName = "memory.limit.bytes"
)

type MetricUnit string

const (
	MetricUnitCores MetricUnit = "cores"
	MetricUnitBytes MetricUnit = "bytes"
	MetricUnitCount MetricUnit = "count"
)

func (name MetricName) Validate() error {
	_, exists := metricUnits[name]
	if !exists {
		return fmt.Errorf("unknown metric name %q", name)
	}
	return nil
}

func (unit MetricUnit) Validate() error {
	switch unit {
	case MetricUnitCores, MetricUnitBytes, MetricUnitCount:
		return nil
	default:
		return fmt.Errorf("unknown metric unit %q", unit)
	}
}

var metricUnits = map[MetricName]MetricUnit{
	MetricCPUUsageCores:            MetricUnitCores,
	MetricMemoryUsageBytes:         MetricUnitBytes,
	MetricNetworkReceiveBytesTotal: MetricUnitBytes,
	MetricNetworkSendBytesTotal:    MetricUnitBytes,
	MetricRestartCount:             MetricUnitCount,
	MetricCPURequestCores:          MetricUnitCores,
	MetricCPULimitCores:            MetricUnitCores,
	MetricMemoryRequestBytes:       MetricUnitBytes,
	MetricMemoryLimitBytes:         MetricUnitBytes,
}

type MetricSample struct {
	ID         MetricSampleID
	ResourceID ResourceID
	Name       MetricName
	Unit       MetricUnit
	Value      float64
	SampledAt  time.Time
	Interval   time.Duration
	Synthetic  bool
}

func (sample MetricSample) Validate() error {
	if err := sample.ID.Validate(); err != nil {
		return err
	}
	if err := sample.ResourceID.Validate(); err != nil {
		return err
	}
	if err := sample.Name.Validate(); err != nil {
		return err
	}
	if err := sample.Unit.Validate(); err != nil {
		return err
	}
	if expected := metricUnits[sample.Name]; sample.Unit != expected {
		return fmt.Errorf("metric %q must use unit %q", sample.Name, expected)
	}
	if math.IsNaN(sample.Value) || math.IsInf(sample.Value, 0) || sample.Value < 0 {
		return fmt.Errorf("metric value must be finite and non-negative")
	}
	if sample.Value > 1<<53-1 {
		return fmt.Errorf("metric value exceeds the safe number range")
	}
	if sample.Unit != MetricUnitCores && math.Trunc(sample.Value) != sample.Value {
		return fmt.Errorf("byte and count metrics must use whole values")
	}
	if err := validateUTCTime("metric sample time", sample.SampledAt); err != nil {
		return err
	}
	if sample.Interval < 0 || sample.Interval > 24*time.Hour {
		return fmt.Errorf("metric interval is outside its allowed range")
	}
	if sample.Interval > 0 && sample.Interval < time.Second {
		return fmt.Errorf("metric interval must be zero or at least one second")
	}
	if sample.Interval%time.Second != 0 {
		return fmt.Errorf("metric interval must use whole seconds")
	}
	return nil
}

type CurrentMetricQuery struct {
	Source    ProviderRef
	Resources []ResourceRef
	Names     []MetricName
}

func (query CurrentMetricQuery) Validate() error {
	if err := query.Source.Validate(); err != nil {
		return err
	}
	if len(query.Resources) == 0 || len(query.Resources) > MaxMetricTargets {
		return fmt.Errorf("metric query must have between 1 and %d resources", MaxMetricTargets)
	}
	if len(query.Names) == 0 || len(query.Names) > len(metricUnits) {
		return fmt.Errorf("metric query has an invalid metric count")
	}
	seenResources := make(map[ResourceID]struct{}, len(query.Resources))
	for _, resource := range query.Resources {
		if err := resource.Validate(); err != nil {
			return err
		}
		if !query.Source.Matches(resource) {
			return fmt.Errorf("metric resource does not belong to query source")
		}
		if _, exists := seenResources[resource.ID]; exists {
			return fmt.Errorf("metric query has duplicate resource %q", resource.ID)
		}
		seenResources[resource.ID] = struct{}{}
	}
	seenNames := make(map[MetricName]struct{}, len(query.Names))
	for _, name := range query.Names {
		if err := name.Validate(); err != nil {
			return err
		}
		if _, exists := seenNames[name]; exists {
			return fmt.Errorf("metric query has duplicate metric %q", name)
		}
		seenNames[name] = struct{}{}
	}
	return nil
}

type CurrentMetricResult struct {
	Status ObservationStatus
	Sets   []CurrentMetricSet
}

type CurrentMetricSet struct {
	ResourceID ResourceID
	Status     ObservationStatus
	Samples    []MetricSample
}

func (result CurrentMetricResult) ValidateFor(query CurrentMetricQuery) error {
	if err := query.Validate(); err != nil {
		return err
	}
	if err := result.Status.Validate(); err != nil {
		return err
	}
	if len(result.Sets) > len(query.Resources) {
		return fmt.Errorf("metric result exceeds requested resources")
	}
	if !result.Status.AllowsData() && len(result.Sets) != 0 {
		return fmt.Errorf("unavailable metric result must not contain data")
	}
	if result.Status.AllowsData() && result.Status.Completeness == CompletenessComplete && len(result.Sets) != len(query.Resources) {
		return fmt.Errorf("complete metric result must report every resource")
	}
	resources := make(map[ResourceID]struct{}, len(query.Resources))
	for _, resource := range query.Resources {
		resources[resource.ID] = struct{}{}
	}
	names := make(map[MetricName]struct{}, len(query.Names))
	for _, name := range query.Names {
		names[name] = struct{}{}
	}
	seenSets := make(map[ResourceID]struct{}, len(result.Sets))
	seenSampleIDs := make(map[MetricSampleID]struct{})
	for _, set := range result.Sets {
		if err := set.ResourceID.Validate(); err != nil {
			return err
		}
		if _, exists := resources[set.ResourceID]; !exists {
			return fmt.Errorf("metric result contains an unrequested resource")
		}
		if _, exists := seenSets[set.ResourceID]; exists {
			return fmt.Errorf("metric result contains a duplicate resource set")
		}
		seenSets[set.ResourceID] = struct{}{}
		if err := set.Status.Validate(); err != nil {
			return err
		}
		if set.Status.ObservedAt.After(result.Status.ObservedAt) {
			return fmt.Errorf("metric set cannot be newer than its result")
		}
		if len(set.Samples) > len(query.Names) {
			return fmt.Errorf("metric set exceeds requested metrics")
		}
		if !set.Status.AllowsData() && len(set.Samples) != 0 {
			return fmt.Errorf("unavailable metric set must not contain samples")
		}
		if set.Status.AllowsData() && set.Status.Completeness == CompletenessComplete && len(set.Samples) != len(query.Names) {
			return fmt.Errorf("complete metric set must report every requested metric")
		}
		seenNames := make(map[MetricName]struct{}, len(set.Samples))
		for _, sample := range set.Samples {
			if err := sample.Validate(); err != nil {
				return err
			}
			if sample.ResourceID != set.ResourceID {
				return fmt.Errorf("metric sample does not belong to its resource set")
			}
			if _, exists := names[sample.Name]; !exists {
				return fmt.Errorf("metric result contains an unrequested metric")
			}
			if sample.SampledAt.After(set.Status.ObservedAt) {
				return fmt.Errorf("metric sample cannot be newer than its resource set")
			}
			if _, exists := seenNames[sample.Name]; exists {
				return fmt.Errorf("metric resource set contains a duplicate metric")
			}
			seenNames[sample.Name] = struct{}{}
			if _, exists := seenSampleIDs[sample.ID]; exists {
				return fmt.Errorf("metric result contains a duplicate sample ID")
			}
			seenSampleIDs[sample.ID] = struct{}{}
		}
	}
	return nil
}

type MetricHistoryQuery struct {
	Source     ProviderRef
	Resources  []ResourceRef
	Names      []MetricName
	Range      TimeRange
	PointLimit int
}

func (query MetricHistoryQuery) Validate() error {
	current := CurrentMetricQuery{Source: query.Source, Resources: query.Resources, Names: query.Names}
	if err := current.Validate(); err != nil {
		return err
	}
	if err := query.Range.ValidateLookback(MaxMetricLookback); err != nil {
		return err
	}
	if query.PointLimit < 1 || query.PointLimit > MaxMetricHistoryPoints {
		return fmt.Errorf("metric history point limit must be between 1 and %d", MaxMetricHistoryPoints)
	}
	return nil
}

type MetricHistoryResult struct {
	Status  ObservationStatus
	Samples []MetricSample
}

func (result MetricHistoryResult) ValidateFor(query MetricHistoryQuery) error {
	if err := query.Validate(); err != nil {
		return err
	}
	if err := result.Status.Validate(); err != nil {
		return err
	}
	if len(result.Samples) > query.PointLimit {
		return fmt.Errorf("metric history exceeds requested point limit")
	}
	if !result.Status.AllowsData() && len(result.Samples) != 0 {
		return fmt.Errorf("unavailable metric history must not contain data")
	}
	resources := make(map[ResourceID]struct{}, len(query.Resources))
	for _, resource := range query.Resources {
		resources[resource.ID] = struct{}{}
	}
	names := make(map[MetricName]struct{}, len(query.Names))
	for _, name := range query.Names {
		names[name] = struct{}{}
	}
	seen := make(map[MetricSampleID]struct{}, len(result.Samples))
	for index, sample := range result.Samples {
		if err := sample.Validate(); err != nil {
			return err
		}
		if _, exists := resources[sample.ResourceID]; !exists {
			return fmt.Errorf("metric history contains an unrequested resource")
		}
		if _, exists := names[sample.Name]; !exists {
			return fmt.Errorf("metric history contains an unrequested metric")
		}
		if sample.SampledAt.Before(query.Range.Start) || !sample.SampledAt.Before(query.Range.End) {
			return fmt.Errorf("metric history sample is outside the requested range")
		}
		if sample.SampledAt.After(result.Status.ObservedAt) {
			return fmt.Errorf("metric history sample cannot be newer than its result")
		}
		if _, exists := seen[sample.ID]; exists {
			return fmt.Errorf("metric history contains a duplicate sample ID")
		}
		if index > 0 && metricSampleOrder(result.Samples[index-1], sample) > 0 {
			return fmt.Errorf("metric history must use ascending time and identity order")
		}
		seen[sample.ID] = struct{}{}
	}
	return nil
}

func metricSampleOrder(left, right MetricSample) int {
	if comparison := left.SampledAt.Compare(right.SampledAt); comparison != 0 {
		return comparison
	}
	if left.ResourceID < right.ResourceID {
		return -1
	}
	if left.ResourceID > right.ResourceID {
		return 1
	}
	if left.Name < right.Name {
		return -1
	}
	if left.Name > right.Name {
		return 1
	}
	if left.ID < right.ID {
		return -1
	}
	if left.ID > right.ID {
		return 1
	}
	return 0
}

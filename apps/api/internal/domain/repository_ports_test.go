package domain

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"
)

type repositoryContract struct{}

func (repositoryContract) InsertMetricSamples(context.Context, []MetricSample) error {
	return nil
}

func (repositoryContract) InsertInfrastructureEvents(context.Context, []InfrastructureEvent) error {
	return nil
}

func (repositoryContract) AppendAuditEntries(context.Context, []AuditEntry) error {
	return nil
}

func (repositoryContract) SaveAlertMutation(context.Context, AlertMutation) error {
	return nil
}

func (repository repositoryContract) WithinUnitOfWork(ctx context.Context, work func(Repositories) error) error {
	return work(repository)
}

var _ Repositories = repositoryContract{}
var _ RepositoryUnitOfWork = repositoryContract{}
var _ ProductRepositories = repositoryContract{}

func TestRepositoryPortsStayFocused(t *testing.T) {
	tests := map[string]struct {
		port    any
		methods int
	}{
		"metric samples":        {port: (*MetricSampleRepository)(nil), methods: 1},
		"infrastructure events": {port: (*InfrastructureEventRepository)(nil), methods: 1},
		"audit entries":         {port: (*AuditEntryRepository)(nil), methods: 1},
		"alert mutation":        {port: (*AlertMutationRepository)(nil), methods: 1},
		"repositories":          {port: (*Repositories)(nil), methods: 3},
		"product repositories":  {port: (*ProductRepositories)(nil), methods: 4},
		"unit of work":          {port: (*RepositoryUnitOfWork)(nil), methods: 1},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			portType := reflect.TypeOf(test.port).Elem()
			if portType.NumMethod() != test.methods {
				t.Fatalf("method count = %d, want %d", portType.NumMethod(), test.methods)
			}
		})
	}
}

func TestRepositoryBatchValidation(t *testing.T) {
	metric := validRepositoryMetricSample()
	event := validRepositoryEvent()
	audit := validRepositoryAuditEntry()

	tests := map[string]struct {
		validate func() error
		want     error
	}{
		"metric valid": {validate: func() error { return ValidateMetricSampleWriteBatch([]MetricSample{metric}) }},
		"event valid":  {validate: func() error { return ValidateInfrastructureEventWriteBatch([]InfrastructureEvent{event}) }},
		"audit valid":  {validate: func() error { return ValidateAuditEntryWriteBatch([]AuditEntry{audit}) }},
		"metric empty": {validate: func() error { return ValidateMetricSampleWriteBatch(nil) }, want: ErrRepositoryInvalidInput},
		"event empty":  {validate: func() error { return ValidateInfrastructureEventWriteBatch(nil) }, want: ErrRepositoryInvalidInput},
		"audit empty":  {validate: func() error { return ValidateAuditEntryWriteBatch(nil) }, want: ErrRepositoryInvalidInput},
		"metric duplicate": {validate: func() error {
			return ValidateMetricSampleWriteBatch([]MetricSample{metric, metric})
		}, want: ErrRepositoryConflict},
		"event duplicate": {validate: func() error {
			return ValidateInfrastructureEventWriteBatch([]InfrastructureEvent{event, event})
		}, want: ErrRepositoryConflict},
		"audit duplicate": {validate: func() error {
			return ValidateAuditEntryWriteBatch([]AuditEntry{audit, audit})
		}, want: ErrRepositoryConflict},
		"metric invalid": {validate: func() error {
			invalid := metric
			invalid.Value = -1
			return ValidateMetricSampleWriteBatch([]MetricSample{invalid})
		}, want: ErrRepositoryInvalidInput},
		"event invalid synthetic marker": {validate: func() error {
			invalid := event
			invalid.Synthetic = false
			return ValidateInfrastructureEventWriteBatch([]InfrastructureEvent{invalid})
		}, want: ErrRepositoryInvalidInput},
		"audit invalid": {validate: func() error {
			invalid := audit
			invalid.RequestID = ""
			return ValidateAuditEntryWriteBatch([]AuditEntry{invalid})
		}, want: ErrRepositoryInvalidInput},
		"metric before epoch": {validate: func() error {
			invalid := metric
			invalid.SampledAt = time.Date(1969, time.December, 31, 23, 59, 59, 0, time.UTC)
			return ValidateMetricSampleWriteBatch([]MetricSample{invalid})
		}, want: ErrRepositoryInvalidInput},
		"event before epoch": {validate: func() error {
			invalid := event
			invalid.OccurredAt = time.Date(1969, time.December, 31, 23, 59, 59, 0, time.UTC)
			return ValidateInfrastructureEventWriteBatch([]InfrastructureEvent{invalid})
		}, want: ErrRepositoryInvalidInput},
		"audit before epoch": {validate: func() error {
			invalid := audit
			invalid.OccurredAt = time.Date(1969, time.December, 31, 23, 59, 59, 0, time.UTC)
			return ValidateAuditEntryWriteBatch([]AuditEntry{invalid})
		}, want: ErrRepositoryInvalidInput},
		"metric below microsecond precision": {validate: func() error {
			invalid := metric
			invalid.SampledAt = invalid.SampledAt.Add(time.Nanosecond)
			return ValidateMetricSampleWriteBatch([]MetricSample{invalid})
		}, want: ErrRepositoryInvalidInput},
		"event below microsecond precision": {validate: func() error {
			invalid := event
			invalid.ObservedAt = invalid.ObservedAt.Add(time.Nanosecond)
			return ValidateInfrastructureEventWriteBatch([]InfrastructureEvent{invalid})
		}, want: ErrRepositoryInvalidInput},
		"audit below microsecond precision": {validate: func() error {
			invalid := audit
			invalid.OccurredAt = invalid.OccurredAt.Add(time.Nanosecond)
			return ValidateAuditEntryWriteBatch([]AuditEntry{invalid})
		}, want: ErrRepositoryInvalidInput},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := test.validate()
			if test.want == nil && err != nil {
				t.Fatalf("validation error = %v", err)
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("validation error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestRepositoryBatchLimitIsEnforced(t *testing.T) {
	samples := make([]MetricSample, MaxRepositoryWriteBatch)
	for index := range samples {
		samples[index] = validRepositoryMetricSample()
		samples[index].ID = MetricSampleID("metric_" + strconv.Itoa(index))
	}
	if err := ValidateMetricSampleWriteBatch(samples); err != nil {
		t.Fatalf("maximum batch validation error = %v", err)
	}
	samples = append(samples, validRepositoryMetricSample())
	if err := ValidateMetricSampleWriteBatch(samples); !errors.Is(err, ErrRepositoryInvalidInput) {
		t.Fatalf("oversized batch error = %v", err)
	}
}

func validRepositoryMetricSample() MetricSample {
	resourceID := repositoryResourceID()
	return MetricSample{
		ID:         "metric_one",
		ResourceID: resourceID,
		Name:       MetricCPUUsageCores,
		Unit:       MetricUnitCores,
		Value:      0.5,
		SampledAt:  time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC),
		Interval:   10 * time.Second,
		Synthetic:  true,
	}
}

func validRepositoryEvent() InfrastructureEvent {
	environmentID := EnvironmentID("environment_one")
	resourceID := repositoryResourceID()
	return InfrastructureEvent{
		ID:            "event_one",
		Provider:      ProviderDemo,
		SourceID:      "source_one",
		EnvironmentID: &environmentID,
		ResourceID:    &resourceID,
		Kind:          "resource.changed",
		Severity:      SeverityInfo,
		ReasonCode:    "resource.updated",
		Summary:       "Resource changed",
		OccurredAt:    time.Date(2026, time.August, 1, 11, 59, 0, 0, time.UTC),
		ObservedAt:    time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC),
		Synthetic:     true,
	}
}

func repositoryResourceID() ResourceID {
	identity, err := NewResourceIdentity(
		ProviderDemo,
		"source_one",
		"environment_one",
		"demo.service",
		Scope{Kind: ScopeGlobal},
		"native-one",
	)
	if err != nil {
		panic(err)
	}
	resourceID, err := NewResourceID(identity)
	if err != nil {
		panic(err)
	}
	return resourceID
}

func validRepositoryAuditEntry() AuditEntry {
	return AuditEntry{
		ID:         "audit_one",
		OccurredAt: time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC),
		Actor:      AuditActor{Kind: AuditActorOperator, ID: "operator_one"},
		Action:     "source.test",
		Target:     AuditTarget{Kind: "source.connection", ID: "source_one"},
		Outcome:    AuditSuccess,
		RequestID:  "request_one",
		ReasonCode: "source.test_succeeded",
	}
}

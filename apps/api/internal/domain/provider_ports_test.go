package domain

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type contractProvider struct{}

func (contractProvider) ListEnvironments(context.Context, EnvironmentQuery) (EnvironmentPage, error) {
	return EnvironmentPage{}, nil
}

func (contractProvider) ListResources(context.Context, InventoryQuery) (ResourcePage, error) {
	return ResourcePage{}, nil
}

func (contractProvider) GetResource(context.Context, ResourceDetailQuery) (ResourceSnapshot, error) {
	return ResourceSnapshot{}, nil
}

func (contractProvider) GetHealth(context.Context, HealthQuery) (HealthResult, error) {
	return HealthResult{}, nil
}

func (contractProvider) GetRelationships(context.Context, TopologyQuery) (RelationshipPage, error) {
	return RelationshipPage{}, nil
}

func (contractProvider) ReadLogs(context.Context, LogReadQuery) (LogPage, error) {
	return LogPage{}, nil
}

func (contractProvider) TailLogs(context.Context, LogTailQuery) (LogTail, error) {
	return newContractLogTail(), nil
}

func (contractProvider) GetCurrentMetrics(context.Context, CurrentMetricQuery) (CurrentMetricResult, error) {
	return CurrentMetricResult{}, nil
}

func (contractProvider) ListEvents(context.Context, EventQuery) (EventPage, error) {
	return EventPage{}, nil
}

func (contractProvider) WatchEvents(context.Context, EventWatchQuery) (EventWatch, error) {
	return newContractEventWatch(), nil
}

func (contractProvider) GetCapabilities(context.Context, ProviderRef) (CapabilitySet, error) {
	return CapabilitySet{}, nil
}

var _ InventoryProvider = contractProvider{}
var _ HealthProvider = contractProvider{}
var _ TopologyProvider = contractProvider{}
var _ LogReadProvider = contractProvider{}
var _ LogTailProvider = contractProvider{}
var _ CurrentMetricProvider = contractProvider{}
var _ EventHistoryProvider = contractProvider{}
var _ EventWatchProvider = contractProvider{}
var _ CapabilityProvider = contractProvider{}

type contractLogTail struct {
	done      chan struct{}
	started   chan struct{}
	startOnce sync.Once
	closeOnce sync.Once
}

func newContractLogTail() *contractLogTail {
	return &contractLogTail{done: make(chan struct{}), started: make(chan struct{})}
}

func (tail *contractLogTail) Next(ctx context.Context) (LogRecord, error) {
	tail.startOnce.Do(func() { close(tail.started) })
	select {
	case <-ctx.Done():
		return LogRecord{}, ctx.Err()
	case <-tail.done:
		return LogRecord{}, io.EOF
	}
}

func (tail *contractLogTail) Close() error {
	tail.closeOnce.Do(func() { close(tail.done) })
	return nil
}

type contractEventWatch struct {
	done      chan struct{}
	started   chan struct{}
	startOnce sync.Once
	closeOnce sync.Once
}

func newContractEventWatch() *contractEventWatch {
	return &contractEventWatch{done: make(chan struct{}), started: make(chan struct{})}
}

func (watch *contractEventWatch) Next(ctx context.Context) (InfrastructureEvent, error) {
	watch.startOnce.Do(func() { close(watch.started) })
	select {
	case <-ctx.Done():
		return InfrastructureEvent{}, ctx.Err()
	case <-watch.done:
		return InfrastructureEvent{}, io.EOF
	}
}

func (watch *contractEventWatch) Close() error {
	watch.closeOnce.Do(func() { close(watch.done) })
	return nil
}

func TestProviderPortsStayFocused(t *testing.T) {
	tests := map[string]struct {
		port    any
		methods int
	}{
		"inventory":     {port: (*InventoryProvider)(nil), methods: 3},
		"health":        {port: (*HealthProvider)(nil), methods: 1},
		"topology":      {port: (*TopologyProvider)(nil), methods: 1},
		"log read":      {port: (*LogReadProvider)(nil), methods: 1},
		"log tail":      {port: (*LogTailProvider)(nil), methods: 1},
		"metric":        {port: (*CurrentMetricProvider)(nil), methods: 1},
		"event history": {port: (*EventHistoryProvider)(nil), methods: 1},
		"event watch":   {port: (*EventWatchProvider)(nil), methods: 1},
		"capability":    {port: (*CapabilityProvider)(nil), methods: 1},
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

func TestReferenceStreamsHonorLifecycleContract(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	logTail := newContractLogTail()
	if _, err := logTail.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("log tail error = %v, want cancellation", err)
	}

	logTail = newContractLogTail()
	logResult := make(chan error, 1)
	go func() {
		_, err := logTail.Next(context.Background())
		logResult <- err
	}()
	<-logTail.started
	if err := logTail.Close(); err != nil {
		t.Fatalf("first log close error = %v", err)
	}
	if err := logTail.Close(); err != nil {
		t.Fatalf("second log close error = %v", err)
	}
	select {
	case err := <-logResult:
		if !errors.Is(err, io.EOF) {
			t.Fatalf("closed log tail error = %v, want EOF", err)
		}
	case <-time.After(time.Second):
		t.Fatal("log tail Close did not unblock Next")
	}

	eventWatch := newContractEventWatch()
	if _, err := eventWatch.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("event watch error = %v, want cancellation", err)
	}

	eventWatch = newContractEventWatch()
	eventResult := make(chan error, 1)
	go func() {
		_, err := eventWatch.Next(context.Background())
		eventResult <- err
	}()
	<-eventWatch.started
	if err := eventWatch.Close(); err != nil {
		t.Fatalf("first event close error = %v", err)
	}
	if err := eventWatch.Close(); err != nil {
		t.Fatalf("second event close error = %v", err)
	}
	select {
	case err := <-eventResult:
		if !errors.Is(err, io.EOF) {
			t.Fatalf("closed event watch error = %v, want EOF", err)
		}
	case <-time.After(time.Second):
		t.Fatal("event watch Close did not unblock Next")
	}
}

func TestProviderErrorKeepsPublicTextSafeAndCauseMatchable(t *testing.T) {
	cause := errors.New("credential-value-must-stay-hidden")
	providerError, err := NewProviderError("inventory.list", ProviderErrorUnavailable, true, cause)
	if err != nil {
		t.Fatalf("NewProviderError() error = %v", err)
	}
	if strings.Contains(providerError.Error(), cause.Error()) {
		t.Fatal("provider error exposed its cause text")
	}
	if !errors.Is(providerError, cause) {
		t.Fatal("provider error lost its cause chain")
	}
	_, err = NewProviderError("inventory.list", ProviderErrorPermissionDenied, true, cause)
	requireInvalid(t, err)
}

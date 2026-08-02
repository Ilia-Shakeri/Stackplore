package auth

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestBootstrapGateConsumesOnlySuccessfulCallback(t *testing.T) {
	current := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	proof, err := mustCoreToken(t, 1).Encode()
	if err != nil {
		t.Fatal(err)
	}
	gate, err := newBootstrapGate(proof, current.Add(5*time.Minute), func() time.Time { return current })
	if err != nil {
		t.Fatal(err)
	}

	callbackFailure := errors.New("setup transaction failed")
	var calls atomic.Int32
	err = gate.Use(context.Background(), proof, func(context.Context) error {
		calls.Add(1)
		return callbackFailure
	})
	if !errors.Is(err, callbackFailure) {
		t.Fatalf("first Use() error = %v", err)
	}
	err = gate.Use(context.Background(), proof, func(context.Context) error {
		calls.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("retry Use() error = %v", err)
	}
	err = gate.Use(context.Background(), proof, func(context.Context) error {
		calls.Add(1)
		return nil
	})
	if !errors.Is(err, ErrBootstrapDenied) {
		t.Fatalf("replay Use() error = %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("callback calls = %d; want 2", got)
	}
}

func TestBootstrapGateRejectsWrongExpiredAndMalformedProofs(t *testing.T) {
	current := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	proof, err := mustCoreToken(t, 1).Encode()
	if err != nil {
		t.Fatal(err)
	}
	wrong, err := mustCoreToken(t, 2).Encode()
	if err != nil {
		t.Fatal(err)
	}
	gate, err := newBootstrapGate(proof, current.Add(time.Minute), func() time.Time { return current })
	if err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	callback := func(context.Context) error {
		calls.Add(1)
		return nil
	}
	for _, candidate := range []string{wrong, "short", proof + "="} {
		if err := gate.Use(context.Background(), candidate, callback); !errors.Is(err, ErrBootstrapDenied) {
			t.Fatalf("wrong proof error = %v", err)
		}
	}
	current = current.Add(time.Minute)
	if err := gate.Use(context.Background(), proof, callback); !errors.Is(err, ErrBootstrapDenied) {
		t.Fatalf("expired proof error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("denied proof reached callback")
	}

	if _, err := newBootstrapGate("short", current.Add(time.Minute), func() time.Time { return current }); !errors.Is(err, ErrInvalidBootstrapGate) {
		t.Fatalf("malformed constructor error = %v", err)
	}
	if _, err := newBootstrapGate(proof, current, func() time.Time { return current }); !errors.Is(err, ErrInvalidBootstrapGate) {
		t.Fatalf("expired constructor error = %v", err)
	}
}

func TestBootstrapGateSerializesSuccessfulUse(t *testing.T) {
	current := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	proof, err := mustCoreToken(t, 3).Encode()
	if err != nil {
		t.Fatal(err)
	}
	gate, err := newBootstrapGate(proof, current.Add(time.Minute), func() time.Time { return current })
	if err != nil {
		t.Fatal(err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	firstResult := make(chan error, 1)
	go func() {
		firstResult <- gate.Use(context.Background(), proof, func(context.Context) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	secondResult := make(chan error, 1)
	go func() {
		secondResult <- gate.Use(context.Background(), proof, func(context.Context) error {
			return errors.New("second callback must not run")
		})
	}()
	select {
	case err := <-secondResult:
		t.Fatalf("second use escaped held gate: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-firstResult; err != nil {
		t.Fatalf("first Use() error = %v", err)
	}
	if err := <-secondResult; !errors.Is(err, ErrBootstrapDenied) {
		t.Fatalf("second Use() error = %v", err)
	}
}

func TestBootstrapGateHonorsCanceledContext(t *testing.T) {
	current := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	proof, err := mustCoreToken(t, 4).Encode()
	if err != nil {
		t.Fatal(err)
	}
	gate, err := newBootstrapGate(proof, current.Add(time.Minute), func() time.Time { return current })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := gate.Use(ctx, proof, func(context.Context) error { return nil }); !errors.Is(err, context.Canceled) || !errors.Is(err, ErrBootstrapUnavailable) {
		t.Fatalf("canceled Use() error = %v", err)
	}
}

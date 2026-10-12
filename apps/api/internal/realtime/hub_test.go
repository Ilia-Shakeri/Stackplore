package realtime

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManagerFansOutOneProducerAndStopsAfterLastSubscriber(t *testing.T) {
	manager := testManager(t, 4, 2)
	defer manager.Close()
	started := make(chan struct{})
	stopped := make(chan struct{})
	producer := func(ctx context.Context, publish func(string, []byte) error) {
		close(started)
		if err := publish("health.snapshot.v1", []byte(`{"state":"ready"}`)); err != nil {
			t.Errorf("publish error = %v", err)
		}
		<-ctx.Done()
		close(stopped)
	}
	first, err := manager.Subscribe("health/source", 0, producer)
	if err != nil {
		t.Fatalf("first subscribe: %v", err)
	}
	<-started
	second, err := manager.Subscribe("health/source", 0, producer)
	if err != nil {
		t.Fatalf("second subscribe: %v", err)
	}
	frame := receiveFrame(t, first.C)
	if frame.ID != 1 || frame.Event != "health.snapshot.v1" || string(frame.Data) != `{"state":"ready"}` {
		t.Fatalf("frame = %#v", frame)
	}
	first.Close()
	if manager.ActiveFeeds() != 1 {
		t.Fatalf("active feeds = %d, want 1", manager.ActiveFeeds())
	}
	second.Close()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("producer did not stop")
	}
	if manager.ActiveFeeds() != 0 {
		t.Fatalf("active feeds = %d, want 0", manager.ActiveFeeds())
	}
}

func TestManagerReplaysAvailableFramesAndSignalsGap(t *testing.T) {
	manager := testManager(t, 2, 2)
	defer manager.Close()
	publishReady := make(chan func(string, []byte) error, 1)
	producer := func(ctx context.Context, publish func(string, []byte) error) {
		publishReady <- publish
		<-ctx.Done()
	}
	first, err := manager.Subscribe("events/source", 0, producer)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	publish := <-publishReady
	for index := 1; index <= 3; index++ {
		if err := publish("event.snapshot.v1", []byte(`{"value":1}`)); err != nil {
			t.Fatalf("publish %d: %v", index, err)
		}
		_ = receiveFrame(t, first.C)
	}
	replay, err := manager.Subscribe("events/source", 1, producer)
	if err != nil || replay.Reset {
		t.Fatalf("replay = %#v, error = %v", replay, err)
	}
	if receiveFrame(t, replay.C).ID != 2 || receiveFrame(t, replay.C).ID != 3 {
		t.Fatal("replay order mismatch")
	}
	if err := publish("event.snapshot.v1", []byte(`{"value":2}`)); err != nil {
		t.Fatalf("publish 4: %v", err)
	}
	_ = receiveFrame(t, first.C)
	gap, err := manager.Subscribe("events/source", 1, producer)
	if err != nil || !gap.Reset {
		t.Fatalf("gap = %#v, error = %v", gap, err)
	}
	first.Close()
	replay.Close()
	gap.Close()
}

func TestManagerCutsSlowSubscriberAndRejectsBadFrames(t *testing.T) {
	manager := testManager(t, 2, 1)
	defer manager.Close()
	publishReady := make(chan func(string, []byte) error, 1)
	producer := func(ctx context.Context, publish func(string, []byte) error) {
		publishReady <- publish
		<-ctx.Done()
	}
	subscription, err := manager.Subscribe("logs/source", 0, producer)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	publish := <-publishReady
	if err := publish("bad event", []byte(`{}`)); !errors.Is(err, ErrInvalidFrame) {
		t.Fatalf("bad event error = %v", err)
	}
	if err := publish("log.snapshot.v1", []byte(`not-json`)); !errors.Is(err, ErrInvalidFrame) {
		t.Fatalf("bad data error = %v", err)
	}
	if err := publish("log.snapshot.v1", []byte(`{"line":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := publish("log.snapshot.v1", []byte(`{"line":2}`)); err != nil {
		t.Fatal(err)
	}
	_ = receiveFrame(t, subscription.C)
	select {
	case _, open := <-subscription.C:
		if open {
			t.Fatal("slow subscriber remained open")
		}
	case <-time.After(time.Second):
		t.Fatal("slow subscriber was not cut")
	}
	subscription.Close()
}

func testManager(t *testing.T, replay, queue int) *Manager {
	t.Helper()
	manager, err := NewManager(ManagerOptions{
		MaximumFeeds: 4, ReplaySize: replay, QueueSize: queue, MaximumFrame: 1024,
		Now: func() time.Time { return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}

func receiveFrame(t *testing.T, frames <-chan Frame) Frame {
	t.Helper()
	select {
	case frame, open := <-frames:
		if !open {
			t.Fatal("frame channel closed")
		}
		return frame
	case <-time.After(time.Second):
		t.Fatal("frame timed out")
		return Frame{}
	}
}

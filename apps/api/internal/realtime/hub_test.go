package realtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestManagerFansOutOneProducerAndStopsAfterLastSubscriber(t *testing.T) {
	manager := testManager(t, 4, 2)
	defer manager.Close()
	started := make(chan struct{})
	stopped := make(chan struct{})
	producer := func(ctx context.Context, publish func(string, []byte) error) error {
		close(started)
		if err := publish("health.snapshot.v1", []byte(`{"state":"ready"}`)); err != nil {
			t.Errorf("publish error = %v", err)
		}
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
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
	producer := func(ctx context.Context, publish func(string, []byte) error) error {
		publishReady <- publish
		<-ctx.Done()
		return ctx.Err()
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
	producer := func(ctx context.Context, publish func(string, []byte) error) error {
		publishReady <- publish
		<-ctx.Done()
		return ctx.Err()
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

func TestManagerInvalidationDiscardsQueuedReplayAndFencesOldProducer(t *testing.T) {
	manager := testManager(t, 4, 4)
	t.Cleanup(manager.Close)
	ready := make(chan func(string, []byte) error, 1)
	releaseOld := make(chan struct{})
	releaseOldProducer := sync.OnceFunc(func() { close(releaseOld) })
	t.Cleanup(releaseOldProducer)
	producer := func(ctx context.Context, publish func(string, []byte) error) error {
		ready <- publish
		<-ctx.Done()
		<-releaseOld
		return ErrRevoked
	}
	first, err := manager.Subscribe("events/source", 0, producer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(first.Close)
	oldFeed := manager.feeds["events/source"]
	publish := <-ready
	for range 2 {
		if err := publish("events.snapshot.v1", []byte(`{"value":1}`)); err != nil {
			t.Fatal(err)
		}
	}
	replay, err := manager.Subscribe("events/source", 1, producer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(replay.Close)
	unrelated, err := manager.Subscribe("health/other", 0, func(ctx context.Context, _ func(string, []byte) error) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unrelated.Close)
	manager.Invalidate("events/source")
	for _, subscription := range []*Subscription{first, replay} {
		if !subscription.Revoked() {
			t.Fatal("invalidated subscription retained authority")
		}
		if _, open := <-subscription.C; open {
			t.Fatal("invalidated subscription retained queued history")
		}
	}
	if !errors.Is(publish("events.snapshot.v1", []byte(`{"value":2}`)), ErrClosed) || unrelated.Revoked() || manager.ActiveFeeds() != 1 {
		t.Fatal("scope invalidation failed to fence only the previous producer")
	}
	recovered, err := manager.Subscribe("events/source", 1, func(ctx context.Context, publish func(string, []byte) error) error {
		if err := publish("events.snapshot.v1", []byte(`{"value":3}`)); err != nil {
			return err
		}
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil || !recovered.Reset {
		t.Fatal("recovery did not start a fresh replay ring")
	}
	t.Cleanup(recovered.Close)
	releaseOldProducer()
	select {
	case <-oldFeed.done:
	case <-time.After(time.Second):
		t.Fatal("revoked producer did not stop")
	}
	first.Close()
	replay.Close()
	if recovered.Revoked() || manager.ActiveFeeds() != 2 {
		t.Fatal("late completion or closure of revoked feed affected its replacement")
	}
	frame := receiveFrame(t, recovered.C)
	if frame.ID != 1 || string(frame.Data) != `{"value":3}` {
		t.Fatal("recovered feed returned history from previous authority")
	}
}

func TestManagerRevocationCommitsQueuedStateBeforeProducerCancellation(t *testing.T) {
	manager := testManager(t, 2, 2)
	t.Cleanup(manager.Close)
	started := make(chan struct{})
	canceled := make(chan struct{})
	subscription, err := manager.Subscribe("health/source", 0, func(ctx context.Context, publish func(string, []byte) error) error {
		if err := publish("health.snapshot.v1", []byte(`{"state":"ready"}`)); err != nil {
			return err
		}
		close(started)
		<-ctx.Done()
		close(canceled)
		return ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(subscription.Close)
	<-started
	feed := manager.feeds["health/source"]
	feed.hub.mu.Lock()
	invalidated := make(chan struct{})
	go func() {
		manager.Invalidate("health/source")
		close(invalidated)
	}()
	var cancellationObserved bool
	select {
	case <-canceled:
		cancellationObserved = true
	case <-time.After(time.Second):
	}
	feed.hub.mu.Unlock()
	select {
	case <-invalidated:
	case <-time.After(time.Second):
		t.Fatal("invalidation did not complete after releasing the hub")
	}
	if cancellationObserved {
		t.Fatal("producer cancellation preceded the locked revocation and queue commit")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("revoked producer was not canceled after committing queued state")
	}
	if _, open := <-subscription.C; open || !subscription.Revoked() {
		t.Fatal("revoked subscription did not close")
	}
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

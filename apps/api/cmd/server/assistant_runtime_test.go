package main

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantworker"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
)

type databaseWithoutAssistant struct{ databaseRuntime }

// Shadow one promoted method while retaining every other server repository.
type databaseWithoutAssistantRecovery struct {
	*fakeDatabase
	RecoverAssistant struct{}
}

func TestBuildAssistantMaintenanceRequiresProductionRepository(t *testing.T) {
	t.Parallel()
	for _, database := range []databaseRuntime{nil, databaseWithoutAssistant{&fakeDatabase{}}} {
		if worker, err := buildAssistantMaintenance(database); worker != nil || !errors.Is(err, errStorageStart) {
			t.Fatal("missing production maintenance repository accepted")
		}
	}
	if worker, err := buildAssistantMaintenance(&fakeDatabase{}); worker == nil || err != nil {
		t.Fatal("metadata maintenance was omitted from a complete repository")
	}
}

func TestRunRefusesMissingAssistantRepositoryBeforeListen(t *testing.T) {
	t.Parallel()
	database := &fakeDatabase{}
	listened := false
	err := runWithStorage(context.Background(), defaultSettings(t), discardLogger(t),
		func(string, string) (net.Listener, error) {
			listened = true
			return nil, errors.New("unexpected metadata-maintenance listener")
		}, func(context.Context, config.Config) (databaseRuntime, error) {
			return databaseWithoutAssistantRecovery{fakeDatabase: database}, nil
		})
	if !errors.Is(err, errStorageStart) || listened || !database.closed.Load() {
		t.Fatal("missing maintenance port did not seal startup before listening")
	}
}

type assistantMaintenanceDatabase struct {
	*fakeDatabase
	lost            chan struct{}
	started         chan struct{}
	cancelled       chan struct{}
	release         chan struct{}
	closedSignal    chan struct{}
	pruned          chan struct{}
	blockRecovery   bool
	onPrune         context.CancelFunc
	active          atomic.Bool
	closedDuringIO  atomic.Bool
	invalidBounds   atomic.Bool
	cleanupTimedOut atomic.Bool
	recoveryCalls   atomic.Int32
	retentionCalls  atomic.Int32
	startedOnce     sync.Once
	cancelledOnce   sync.Once
	prunedOnce      sync.Once
	closedOnce      sync.Once
}

func newAssistantMaintenanceDatabase(block bool) *assistantMaintenanceDatabase {
	return &assistantMaintenanceDatabase{fakeDatabase: &fakeDatabase{}, lost: make(chan struct{}), started: make(chan struct{}), cancelled: make(chan struct{}), release: make(chan struct{}), closedSignal: make(chan struct{}), pruned: make(chan struct{}), blockRecovery: block}
}

func (database *assistantMaintenanceDatabase) OwnershipLost() <-chan struct{} { return database.lost }

func (database *assistantMaintenanceDatabase) checkBounds(ctx context.Context, at time.Time, limit, maximum int) {
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 3*time.Second || limit != maximum || at.Location() != time.UTC || !at.Equal(at.Truncate(time.Microsecond)) {
		database.invalidBounds.Store(true)
	}
}

func (database *assistantMaintenanceDatabase) RecoverAssistant(ctx context.Context, at time.Time, limit int) (int, error) {
	database.checkBounds(ctx, at, limit, assistantworker.MaximumRecoveryBatch)
	database.recoveryCalls.Add(1)
	database.active.Store(true)
	defer database.active.Store(false)
	database.startedOnce.Do(func() { close(database.started) })
	if !database.blockRecovery {
		return 0, nil
	}
	<-ctx.Done()
	database.cancelledOnce.Do(func() { close(database.cancelled) })
	// Hold a bounded cleanup interval after cancellation to observe the join.
	select {
	case <-database.release:
	case <-time.After(time.Second):
		database.cleanupTimedOut.Store(true)
	}
	return 0, ctx.Err()
}

func (database *assistantMaintenanceDatabase) PruneAssistant(ctx context.Context, at time.Time, limit int) (int, error) {
	database.checkBounds(ctx, at, limit, assistantworker.MaximumRetentionBatch)
	database.retentionCalls.Add(1)
	database.prunedOnce.Do(func() { close(database.pruned) })
	if database.onPrune != nil {
		database.onPrune()
	}
	return 0, nil
}

func (database *assistantMaintenanceDatabase) Close() error {
	if database.active.Load() {
		database.closedDuringIO.Store(true)
	}
	database.closedOnce.Do(func() { close(database.closedSignal) })
	return database.fakeDatabase.Close()
}

func TestAssistantMaintenanceRunsWithoutDispatchConfiguration(t *testing.T) {
	t.Parallel()
	database := newAssistantMaintenanceDatabase(false)
	worker, err := buildAssistantMaintenance(database)
	if err != nil || worker == nil {
		t.Fatal("metadata maintenance was not composed without dispatch configuration")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	database.onPrune = cancel
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	select {
	case <-database.pruned:
	case <-time.After(time.Second):
		t.Fatal("disabled-dispatch maintenance did not recover and prune")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal("metadata maintenance did not stop cleanly")
		}
	case <-time.After(time.Second):
		t.Fatal("metadata maintenance did not join")
	}
	if database.invalidBounds.Load() || database.recoveryCalls.Load() != 1 || database.retentionCalls.Load() != 1 || worker.LastResult().Recovery.Outcome != assistantworker.Succeeded {
		t.Fatal("maintenance composition lost its independent bounds or safe outcome")
	}
}

// This listener never opens a socket or accepts a connection.
type assistantLifecycleListener struct {
	closed chan struct{}
	once   sync.Once
}

func (listener *assistantLifecycleListener) Accept() (net.Conn, error) {
	<-listener.closed
	return nil, net.ErrClosed
}
func (listener *assistantLifecycleListener) Close() error {
	listener.once.Do(func() { close(listener.closed) })
	return nil
}
func (*assistantLifecycleListener) Addr() net.Addr { return testAddress("metadata-only") }

func TestAssistantMaintenanceJoinsBeforeDatabaseCloseOnShutdownAndOwnershipLoss(t *testing.T) {
	for _, ownershipLoss := range []bool{false, true} {
		name := "shutdown"
		if ownershipLoss {
			name = "ownership_loss"
		}
		t.Run(name, func(t *testing.T) {
			database := newAssistantMaintenanceDatabase(true)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var release sync.Once
			finishCleanup := func() { release.Do(func() { close(database.release) }) }
			defer finishCleanup()
			listener := &assistantLifecycleListener{closed: make(chan struct{})}
			defer listener.Close()
			done := make(chan error, 1)
			settings := defaultSettings(t)
			logger := discardLogger(t)
			go func() {
				done <- runWithStorage(ctx, settings, logger, func(string, string) (net.Listener, error) { return listener, nil },
					func(context.Context, config.Config) (databaseRuntime, error) { return database, nil })
			}()
			select {
			case <-database.started:
			case err := <-done:
				t.Fatalf("server stopped before metadata maintenance: %v", err)
			case <-time.After(3 * time.Second):
				t.Fatal("server did not start metadata maintenance")
			}
			if ownershipLoss {
				close(database.lost)
			} else {
				cancel()
			}
			select {
			case <-database.cancelled:
			case <-time.After(time.Second):
				t.Fatal("service lifetime did not cancel active maintenance I/O")
			}
			select {
			case <-database.closedSignal:
				t.Fatal("database closed while maintenance cleanup was still active")
			case <-done:
				t.Fatal("server returned before metadata maintenance joined")
			case <-time.After(20 * time.Millisecond):
			}
			finishCleanup()
			select {
			case err := <-done:
				if ownershipLoss && !errors.Is(err, errStorageOwnership) || !ownershipLoss && err != nil {
					t.Fatalf("server lifetime returned an incorrect sealed result: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("server did not join maintenance after bounded cleanup")
			}
			if !database.closed.Load() || database.closedDuringIO.Load() || database.active.Load() || database.invalidBounds.Load() || database.cleanupTimedOut.Load() || database.retentionCalls.Load() != 0 {
				t.Fatal("database close order, cancellation or maintenance bounds were lost")
			}
		})
	}
}

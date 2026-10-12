package main

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
)

type ownershipDatabase struct {
	*fakeDatabase
	lost chan struct{}
}

func (database *ownershipDatabase) OwnershipLost() <-chan struct{} { return database.lost }

func TestRuntimeStopsAndFailsOnOwnershipLoss(t *testing.T) {
	database := &ownershipDatabase{fakeDatabase: &fakeDatabase{}, lost: make(chan struct{})}
	settings := defaultSettings(t)
	err := runWithStorage(context.Background(), settings, discardLogger(t),
		func(_, _ string) (net.Listener, error) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			close(database.lost)
			return listener, err
		},
		func(context.Context, config.Config) (databaseRuntime, error) { return database, nil },
	)
	if !errors.Is(err, errStorageOwnership) || !database.closed.Load() || lifecycleReason(err) != "storage_ownership_lost" {
		t.Fatal("runtime did not stop and seal ownership loss")
	}
}

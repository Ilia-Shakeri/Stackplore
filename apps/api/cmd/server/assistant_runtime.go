package main

import (
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantworker"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/postgres"
)

var _ assistantworker.Repository = (*postgres.Store)(nil)

// Maintenance has no admission, inference or provider-read capability and runs
// independently of future product dispatch enablement.
func buildAssistantMaintenance(database databaseRuntime) (*assistantworker.Worker, error) {
	if database == nil {
		return nil, errStorageStart
	}
	repository, ok := database.(assistantworker.Repository)
	if !ok {
		return nil, errStorageStart
	}
	worker, err := assistantworker.New(assistantworker.Options{Repository: repository})
	if err != nil {
		return nil, errStorageStart
	}
	return worker, nil
}

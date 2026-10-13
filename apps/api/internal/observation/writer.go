package observation

import (
	"context"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func (worker *Worker) save(ctx context.Context, token domain.CaptureToken, collected collection) error {
	batch := domain.CaptureBatch{Token: token}
	count := 0
	flush := func() error {
		if count == 0 {
			return nil
		}
		if err := batch.Validate(); err != nil {
			return err
		}
		if err := worker.repository.SaveCapture(ctx, batch); err != nil {
			return err
		}
		batch, count = domain.CaptureBatch{Token: token}, 0
		return nil
	}
	ready := func() error {
		if count >= domain.MaxRepositoryWriteBatch {
			return flush()
		}
		return nil
	}
	for _, item := range collected.inventory.Environments {
		if err := ready(); err != nil {
			return err
		}
		batch.Environments, count = append(batch.Environments, item), count+1
	}
	for _, item := range collected.inventory.Resources {
		if err := ready(); err != nil {
			return err
		}
		batch.Resources, count = append(batch.Resources, item), count+1
	}
	for _, item := range collected.inventory.Health {
		if err := ready(); err != nil {
			return err
		}
		batch.Health, count = append(batch.Health, item), count+1
	}
	for _, item := range collected.inventory.WorkloadReplicas {
		if err := ready(); err != nil {
			return err
		}
		batch.WorkloadReplicas, count = append(batch.WorkloadReplicas, item), count+1
	}
	for _, item := range collected.inventory.Relationships {
		if err := ready(); err != nil {
			return err
		}
		batch.Relationships, count = append(batch.Relationships, item), count+1
	}
	for _, item := range collected.metrics {
		if err := ready(); err != nil {
			return err
		}
		batch.Metrics, count = append(batch.Metrics, item), count+1
	}
	for _, item := range collected.events {
		if err := ready(); err != nil {
			return err
		}
		batch.Events, count = append(batch.Events, item), count+1
	}
	return flush()
}

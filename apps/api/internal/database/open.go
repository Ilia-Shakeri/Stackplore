// Package database builds the primary PostgreSQL runtime from validated service configuration.
package database

import (
	"context"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/postgres"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runtimefiles"
	"github.com/Ilia-Shakeri/stackplore/apps/api/postgresmigrations"
)

const (
	primarySchema             = "public"
	maximumConnectionLifetime = 30 * time.Minute
	maximumConnectionIdleTime = 5 * time.Minute
)

// Open starts the sole PostgreSQL runtime used by service and offline operator commands.
func Open(ctx context.Context, settings config.Config) (*postgres.Store, error) {
	if ctx == nil || config.Validate(settings) != nil {
		return nil, postgres.ErrInvalidOptions
	}
	dataDirectory, err := runtimefiles.PrepareDataDirectory(settings.DataDirectory)
	if err != nil {
		return nil, err
	}
	digestKey, digestKeyID, err := runtimefiles.LoadOrCreateAuthDigestKey(dataDirectory)
	if err != nil {
		return nil, err
	}
	return postgres.Open(ctx, postgres.Options{
		ConnectionString:          settings.DatabaseURL.Reveal(),
		Schema:                    primarySchema,
		MinimumConnections:        int32(settings.DatabaseMinConnections),
		MaximumConnections:        int32(settings.DatabaseMaxConnections),
		ConnectTimeout:            settings.DatabaseConnectTimeout,
		QueryTimeout:              settings.DatabaseQueryTimeout,
		MigrationTimeout:          settings.DatabaseMigrationTimeout,
		HealthTimeout:             settings.DatabaseHealthTimeout,
		ShutdownTimeout:           settings.ShutdownTimeout,
		MaximumConnectionLifetime: maximumConnectionLifetime,
		MaximumConnectionIdleTime: maximumConnectionIdleTime,
		AuthDigestKey:             digestKey,
		AuthDigestKeyID:           digestKeyID,
	}, postgresmigrations.FS)
}

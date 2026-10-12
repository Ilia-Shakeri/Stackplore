package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRepositoryErrorMapping(t *testing.T) {
	if err := mapRepositoryWriteError(context.Background(), &pgconn.PgError{Code: "23505"}); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatalf("constraint error = %v; want conflict", err)
	}
	if err := mapRepositoryWriteError(context.Background(), &pgconn.PgError{Code: "08006"}); !errors.Is(err, domain.ErrRepositoryUnavailable) {
		t.Fatalf("connection error = %v; want unavailable", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := mapRepositoryWriteError(canceled, errors.New("query failed")); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v; want canceled cause", err)
	}
}

func TestTransactionRepositoriesLatchFirstFailureAndBoundWrites(t *testing.T) {
	repositories := &transactionRepositories{}
	if err := repositories.beginWrite(context.Background(), 1, domain.ErrRepositoryConflict); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatalf("validation error = %v; want conflict", err)
	}
	if err := repositories.beginWrite(context.Background(), 1, nil); !errors.Is(err, domain.ErrRepositoryConflict) {
		t.Fatalf("latched error = %v; want first conflict", err)
	}

	repositories = &transactionRepositories{transaction: &transaction{}}
	if err := repositories.beginWrite(context.Background(), domain.MaxRepositoryWriteBatch+1, nil); !errors.Is(err, domain.ErrRepositoryInvalidInput) {
		t.Fatalf("oversize error = %v; want invalid input", err)
	}
}

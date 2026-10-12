package postgres

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMaintenanceSessionLossSignalsBeforeRestoreCanContinue(t *testing.T) {
	store, options, admin := openIntegrationStore(t)
	if err := store.Close(); err != nil {
		t.Fatal("close maintenance fixture runtime")
	}
	lock, err := AcquireMaintenanceLock(context.Background(), options.ConnectionString, options.Schema, time.Second)
	if err != nil {
		t.Fatal("acquire maintenance fixture")
	}
	defer lock.Close()
	var pid uint32
	if err := lock.acquire(context.Background()); err != nil {
		t.Fatal("acquire maintenance test gate")
	}
	pid = lock.connection.PgConn().PID()
	lock.release()
	var terminated bool
	if err := admin.QueryRow(context.Background(), `SELECT pg_terminate_backend($1)`, int64(pid)).Scan(&terminated); err != nil || !terminated {
		t.Fatal("terminate maintenance fixture session")
	}
	select {
	case <-lock.OwnershipLost():
	case <-time.After(3 * time.Second):
		t.Fatal("maintenance did not signal lost ownership")
	}
	if _, err := lock.Empty(context.Background(), time.Second); !errors.Is(err, ErrOwnershipUnavailable) {
		t.Fatal("maintenance continued after session loss")
	}
}

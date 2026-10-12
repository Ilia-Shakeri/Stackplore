package postgres

import (
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestAlertCursorIsSourceBoundAndTamperEvident(t *testing.T) {
	store := &Store{authDigestKey: [authDigestKeyBytes]byte{4, 5, 6}}
	alert := domain.Alert{ID: "alert_cursor", UpdatedAt: time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)}
	cursor, err := store.encodeAlertCursor("source_one", alert)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.decodeAlertCursor("source_one", cursor)
	if err != nil || state.ID != string(alert.ID) || state.UpdatedAtUS != alert.UpdatedAt.UnixMicro() {
		t.Fatalf("decodeAlertCursor() = %#v, %v", state, err)
	}
	if _, err := store.decodeAlertCursor("source_two", cursor); err != domain.ErrRepositoryInvalidInput {
		t.Fatalf("cross-source cursor error = %v", err)
	}
	tampered := []byte(cursor)
	if tampered[len(tampered)-1] == 'A' {
		tampered[len(tampered)-1] = 'B'
	} else {
		tampered[len(tampered)-1] = 'A'
	}
	if _, err := store.decodeAlertCursor("source_one", domain.PageCursor(tampered)); err != domain.ErrRepositoryInvalidInput {
		t.Fatalf("tampered cursor error = %v", err)
	}
}

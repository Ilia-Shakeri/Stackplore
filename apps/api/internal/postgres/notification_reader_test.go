package postgres

import (
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestNotificationCursorIsTamperEvident(t *testing.T) {
	store := &Store{authDigestKey: [authDigestKeyBytes]byte{7, 8, 9}}
	notification := domain.Notification{
		ID: "notification_cursor", CreatedAt: time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC),
	}
	cursor, err := store.encodeNotificationCursor(notification)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.decodeNotificationCursor(cursor)
	if err != nil || state.ID != string(notification.ID) || state.CreatedAtUS != notification.CreatedAt.UnixMicro() {
		t.Fatalf("decodeNotificationCursor() = %#v, %v", state, err)
	}
	tampered := []byte(cursor)
	if tampered[len(tampered)-1] == 'A' {
		tampered[len(tampered)-1] = 'B'
	} else {
		tampered[len(tampered)-1] = 'A'
	}
	if _, err := store.decodeNotificationCursor(domain.PageCursor(tampered)); err != domain.ErrRepositoryInvalidInput {
		t.Fatalf("tampered cursor error = %v", err)
	}
}

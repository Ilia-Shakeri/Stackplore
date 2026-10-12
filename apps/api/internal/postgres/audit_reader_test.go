package postgres

import (
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

func TestAuditCursorIsBoundAndTamperEvident(t *testing.T) {
	store := &Store{authDigestKey: [authDigestKeyBytes]byte{1, 2, 3}}
	entry := domain.AuditEntry{
		ID: "audit_cursor", OccurredAt: time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC),
	}
	cursor, err := store.encodeAuditCursor(entry)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.decodeAuditCursor(cursor)
	if err != nil || state.ID != string(entry.ID) || state.OccurredAtUS != entry.OccurredAt.UnixMicro() {
		t.Fatalf("decodeAuditCursor() = %#v, %v", state, err)
	}
	tampered := []byte(cursor)
	tampered[len(tampered)-1] = map[bool]byte{true: 'A', false: 'B'}[tampered[len(tampered)-1] == 'A']
	if _, err := store.decodeAuditCursor(domain.PageCursor(tampered)); err != domain.ErrRepositoryInvalidInput {
		t.Fatalf("tampered cursor error = %v", err)
	}
}

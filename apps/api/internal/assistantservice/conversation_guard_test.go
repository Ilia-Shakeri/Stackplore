package assistantservice

import (
	"context"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
)

func TestExplicitConversationCannotCreateUnknownLedger(t *testing.T) {
	service, repository, _, _ := fixtureService(t, nil)
	input := inputFixture(t, 0)
	input.RequireExistingConversation = true
	if _, err := service.Submit(context.Background(), input); err != assistant.ErrDenied {
		t.Fatal("explicit unknown conversation created a turn")
	}
	if len(repository.runs) != 0 || len(service.entries) != 0 || len(service.conversations) != 0 {
		t.Fatal("rejected explicit conversation retained data or admitted work")
	}
}

func TestRequiredRetainedRunCannotBecomeFreshAdmission(t *testing.T) {
	service, repository, _, _ := fixtureService(t, nil)
	input := inputFixture(t, 0)
	input.Admission.RequireExisting = true
	if _, err := service.Submit(context.Background(), input); err != assistant.ErrConflict {
		t.Fatal("trusted retained-run guard recreated absent metadata")
	}
	if len(repository.runs) != 0 || len(service.entries) != 0 || len(service.conversations) != 0 {
		t.Fatal("rejected retained retry restored question or conversation memory")
	}
}

func TestExplicitConversationContinuesOnlyOwnedLiveLedger(t *testing.T) {
	first := inputFixture(t, 0)
	clock := first.Admission.Until.Add(time.Second)
	service, repository, _, _ := fixtureService(t, func(options *Options, _ *modelFixture) { options.Now = func() time.Time { return clock } })
	if _, err := service.Submit(context.Background(), first); err != nil {
		t.Fatal("create initial conversation")
	}
	next := inputFixture(t, 1)
	next.RequireExistingConversation = true
	if _, err := service.Submit(context.Background(), next); err != nil {
		t.Fatal("owned live conversation did not continue")
	}
	if len(repository.runs) != 2 || service.conversations[first.Admission.ConversationID].turns != 2 {
		t.Fatal("continuation did not consume exactly one turn")
	}
	foreign := inputFixture(t, 2)
	foreign.RequireExistingConversation = true
	foreign.Admission.Actor.UserID = "another_user"
	foreign.Admission.Actor.SessionDigest = [32]byte{2}
	if _, err := service.Submit(context.Background(), foreign); err != assistant.ErrDenied {
		t.Fatal("foreign conversation did not share the missing-conversation denial")
	}
	clock = clock.Add(MemoryLifetime)
	expired := inputFixture(t, 3)
	expired.RequireExistingConversation = true
	if _, err := service.Submit(context.Background(), expired); err != assistant.ErrDenied {
		t.Fatal("expired explicit conversation was recreated")
	}
	if len(repository.runs) != 2 || len(service.entries) != 0 || len(service.conversations) != 0 {
		t.Fatal("expired conversation retained text or admitted another run")
	}
}

func TestExplicitPresenceIsBoundToOriginalFingerprint(t *testing.T) {
	service, repository, _, _ := fixtureService(t, nil)
	input := inputFixture(t, 0)
	if _, err := service.Submit(context.Background(), input); err != nil {
		t.Fatal("initial implicit conversation rejected")
	}
	input.RequireExistingConversation = true
	if _, err := service.Submit(context.Background(), input); err != assistant.ErrConflict {
		t.Fatal("changed explicit-presence material reused a run")
	}
	if len(repository.runs) != 1 || service.conversations[input.Admission.ConversationID].turns != 1 {
		t.Fatal("changed presence admitted work or consumed a turn")
	}
}

func TestMatchingExplicitRetryAfterVolatileLossDoesNotRestoreInput(t *testing.T) {
	service, repository, _, _ := fixtureService(t, nil)
	first := inputFixture(t, 0)
	if _, err := service.Submit(context.Background(), first); err != nil {
		t.Fatal("initial conversation rejected")
	}
	next := inputFixture(t, 1)
	next.RequireExistingConversation = true
	original, err := service.Submit(context.Background(), next)
	if err != nil {
		t.Fatal("explicit continuation rejected")
	}
	service.mu.Lock()
	for _, entry := range service.entries {
		service.eraseLocked(entry)
	}
	clear(service.entries)
	clear(service.conversations)
	service.mu.Unlock()
	repeated, err := service.Submit(context.Background(), next)
	if err != nil || repeated.ID != original.ID || len(repository.runs) != 2 || len(service.entries) != 0 || len(service.conversations) != 0 {
		t.Fatal("matching retained retry restored a lost question or turn ledger")
	}
}

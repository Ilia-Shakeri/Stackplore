package assistantservice

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

type conversation struct {
	owner string
	at    time.Time
	turns int
}

type memoryRun struct {
	input      Input
	run        assistant.Run
	ready      chan struct{}
	readyOnce  sync.Once
	admitting  bool
	revoked    bool
	active     bool
	finishing  bool
	cancel     context.CancelFunc
	expires    time.Time
	answer     *inference.Answer
	citations  []contextbuilder.CitationReference
	evidence   *assistantevidence.Evidence
	operations chan struct{}
}

func (service *Service) stage(input Input) (*memoryRun, bool, error) {
	service.prune()
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed {
		return nil, false, assistant.ErrUnavailable
	}
	if existing := service.entries[input.Admission.ID]; existing != nil {
		if existing.revoked || existing.input.Admission.Actor != input.Admission.Actor || existing.input.Admission.Fingerprint != input.Admission.Fingerprint || existing.input.Admission.ConversationID != input.Admission.ConversationID {
			return nil, false, assistant.ErrConflict
		}
		return existing, false, nil
	}
	c, exists := service.conversations[input.Admission.ConversationID]
	if input.RequireExistingConversation && (!exists || c.owner != input.Admission.Actor.UserID || !service.time().Before(c.at.Add(MemoryLifetime))) {
		return nil, false, assistant.ErrDenied
	}
	if exists && (c.owner != input.Admission.Actor.UserID || c.turns >= MaximumTurns || !service.time().Before(c.at.Add(MemoryLifetime))) {
		return nil, false, assistant.ErrQuota
	}
	ownerEntries, ownerConversations := 0, 0
	for _, entry := range service.entries {
		if entry.input.Admission.Actor.UserID == input.Admission.Actor.UserID {
			ownerEntries++
		}
	}
	for _, existing := range service.conversations {
		if existing.owner == input.Admission.Actor.UserID {
			ownerConversations++
		}
	}
	if !exists && (len(service.conversations) >= MaximumMemoryRuns || ownerConversations >= MaximumOwnerConversations) {
		return nil, false, assistant.ErrQuota
	}
	if ownerEntries >= MaximumOwnerMemoryRuns && !service.evictTerminalLocked(input.Admission.Actor.UserID) {
		return nil, false, assistant.ErrQuota
	}
	if len(service.entries) >= MaximumMemoryRuns && !service.evictTerminalLocked("") {
		return nil, false, assistant.ErrQuota
	}
	if !exists {
		c = conversation{owner: input.Admission.Actor.UserID, at: service.time()}
		service.conversations[input.Admission.ConversationID] = c
	}
	entry := &memoryRun{input: input, ready: make(chan struct{}), admitting: true, expires: c.at.Add(MemoryLifetime), operations: make(chan struct{}, 1)}
	service.entries[input.Admission.ID] = entry
	return entry, true, nil
}

// The conversation ledger outlives an evicted answer so its age and turn cap
// cannot be reset by memory pressure. No queued or active question is evictable.
func (service *Service) evictTerminalLocked(owner string) bool {
	var oldest *memoryRun
	for _, entry := range service.entries {
		if entry.active || entry.admitting || entry.run.Version < 1 || !entry.run.State.Terminal() || owner != "" && entry.input.Admission.Actor.UserID != owner {
			continue
		}
		select {
		case <-entry.ready:
		default:
			continue
		}
		if oldest == nil || entry.run.UpdatedAt.Before(oldest.run.UpdatedAt) || entry.run.UpdatedAt.Equal(oldest.run.UpdatedAt) && entry.run.ID < oldest.run.ID {
			oldest = entry
		}
	}
	if oldest == nil {
		return false
	}
	service.eraseLocked(oldest)
	delete(service.entries, oldest.input.Admission.ID)
	return true
}

func (service *Service) discardStagedLocked(entry *memoryRun) {
	if service.entries[entry.input.Admission.ID] != entry {
		return
	}
	service.eraseLocked(entry)
	delete(service.entries, entry.input.Admission.ID)
	id := entry.input.Admission.ConversationID
	if c := service.conversations[id]; c.turns == 0 {
		delete(service.conversations, id)
	}
}

func (service *Service) rememberTerminalLocked(entry *memoryRun, run assistant.Run) {
	if !service.closed && entry != nil && service.entries[run.ID] == entry && run.State.Terminal() && matchesRun(run, entry.input) && run.Version >= entry.run.Version && run.Fence >= entry.run.Fence {
		entry.run = run
	}
}

func (service *Service) eraseLocked(entry *memoryRun) {
	entry.revoked = true
	entry.input.Question = ""
	entry.answer = nil
	entry.citations = nil
	entry.evidence = nil
	entry.readyOnce.Do(func() { close(entry.ready) })
}

func (service *Service) prune() {
	service.mu.Lock()
	cancels := []context.CancelFunc{}
	at := service.time()
	for id, entry := range service.entries {
		if !at.Before(entry.expires) {
			service.eraseLocked(entry)
			delete(service.entries, id)
			if entry.cancel != nil {
				cancels = append(cancels, entry.cancel)
			}
		}
	}
	for id, c := range service.conversations {
		if !at.Before(c.at.Add(MemoryLifetime)) {
			delete(service.conversations, id)
		}
	}
	service.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

// Invalidate clears private material before signaling any active network task.
// Callers are trusted session/source/policy lifecycle code, not browser routes.
func (service *Service) InvalidateUser(userID string) {
	service.invalidate(func(entry *memoryRun) bool { return entry.input.Admission.Actor.UserID == userID })
}
func (service *Service) InvalidateSource(sourceID string) {
	service.invalidate(func(entry *memoryRun) bool { return entry.input.Admission.SourceID == sourceID })
}

func (service *Service) invalidate(match func(*memoryRun) bool) {
	if service == nil {
		return
	}
	service.mu.Lock()
	cancels := []context.CancelFunc{}
	for _, entry := range service.entries {
		if match(entry) {
			service.eraseLocked(entry)
			if entry.cancel != nil {
				cancels = append(cancels, entry.cancel)
			}
		}
	}
	service.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

// Close fences and releases owned volatile references before cancelling producers.
// Run joins the active producers before it returns; Close itself does not wait.
func (service *Service) Close() {
	if service == nil {
		return
	}
	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		return
	}
	service.closed = true
	clear(service.key[:])
	cancels := make([]context.CancelFunc, 0, len(service.entries))
	for _, entry := range service.entries {
		service.eraseLocked(entry)
		entry.input = Input{}
		if entry.cancel != nil {
			cancels = append(cancels, entry.cancel)
		}
	}
	clear(service.entries)
	clear(service.conversations)
	stopWatching := service.stopWatching
	service.stopWatching = nil
	service.mu.Unlock()
	if stopWatching != nil {
		stopWatching()
	}
	for _, cancel := range cancels {
		cancel()
	}
	service.stop()
}

func (service *Service) Get(ctx context.Context, actor assistant.Actor, id string) (Snapshot, error) {
	if service == nil || ctx == nil || !actor.Valid() || !assistant.Identifier(id) {
		return Snapshot{}, assistant.ErrInvalid
	}
	work, cancel := service.ioContext(ctx)
	defer cancel()
	run, err := service.repository.GetAssistant(work, actor, id, service.time())
	if err != nil {
		return Snapshot{}, ownedError(err)
	}
	service.prune()
	service.mu.Lock()
	entry := service.entries[id]
	if entry == nil || entry.revoked || entry.input.Admission.Actor.UserID != actor.UserID {
		service.mu.Unlock()
		return Snapshot{Run: run, Volatile: false}, nil
	}
	content := run.State == assistant.Completed && (entry.answer != nil || len(entry.citations) > 0)
	seal := entry.evidence
	var evidence assistantevidence.Evidence
	if content && entry.evidence != nil {
		evidence = *entry.evidence
	}
	service.mu.Unlock()
	if content {
		proof, stopProof := service.evidenceContext(ctx)
		err = service.authorizer.AuthorizeEvidence(proof, actor, run, evidence)
		stopProof()
	}
	if err != nil {
		service.invalidateEntry(entry)
		return Snapshot{}, assistant.ErrDenied
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.entries[id] != entry || entry.revoked || !service.time().Before(entry.expires) {
		return Snapshot{Run: run, Volatile: false}, nil
	}
	snapshot := Snapshot{Run: run, Volatile: true}
	if content && entry.evidence == seal && entry.answer != nil && run.State == assistant.Completed {
		snapshot.Answer = cloneAnswer(*entry.answer)
		snapshot.Citations = slices.Clone(entry.citations)
	}
	return snapshot, nil
}

func (service *Service) Cancel(ctx context.Context, actor assistant.Actor, id string) (assistant.Run, error) {
	if service == nil || ctx == nil || !actor.Valid() || !assistant.Identifier(id) {
		return assistant.Run{}, assistant.ErrInvalid
	}
	base, stop := service.ioContext(ctx)
	defer stop()
	work, cancel := context.WithTimeout(base, time.Second)
	defer cancel()
	run, err := service.repository.GetAssistant(work, actor, id, service.time())
	if err != nil {
		return assistant.Run{}, ownedError(err)
	}
	service.mu.Lock()
	entry := service.entries[id]
	owned := entry != nil && entry.input.Admission.Actor.UserID == actor.UserID
	service.mu.Unlock()
	run, err = service.repository.CancelAssistant(work, actor, id, service.time())
	// Durable cancellation wins before the running worker observes context loss.
	// The local fence still clears private state before invoking cancellation.
	if owned {
		if err == nil {
			service.mu.Lock()
			service.rememberTerminalLocked(entry, run)
			service.mu.Unlock()
		}
		service.invalidateEntry(entry)
	}
	if err != nil {
		return assistant.Run{}, ownedError(err)
	}
	return run, nil
}

func (service *Service) invalidateEntry(entry *memoryRun) {
	service.mu.Lock()
	service.eraseLocked(entry)
	cancel := entry.cancel
	service.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

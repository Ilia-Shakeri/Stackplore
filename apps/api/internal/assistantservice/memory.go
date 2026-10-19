package assistantservice

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

type conversation struct {
	owner string
	at    time.Time
	turns int
}

type memoryRun struct {
	input     Input
	run       assistant.Run
	ready     chan struct{}
	readyOnce sync.Once
	revoked   bool
	active    bool
	finishing bool
	cancel    context.CancelFunc
	expires   time.Time
	answer    *inference.Answer
	citations []contextbuilder.CitationReference
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
	if exists && (c.owner != input.Admission.Actor.UserID || c.turns >= MaximumTurns || !service.time().Before(c.at.Add(MemoryLifetime))) {
		return nil, false, assistant.ErrQuota
	}
	if len(service.entries) >= MaximumMemoryRuns || !exists && len(service.conversations) >= MaximumMemoryRuns {
		return nil, false, assistant.ErrQuota
	}
	if !exists {
		c = conversation{owner: input.Admission.Actor.UserID, at: service.time()}
		service.conversations[input.Admission.ConversationID] = c
	}
	entry := &memoryRun{input: input, ready: make(chan struct{}), expires: c.at.Add(MemoryLifetime)}
	service.entries[input.Admission.ID] = entry
	return entry, true, nil
}

func (service *Service) eraseLocked(entry *memoryRun) {
	entry.revoked = true
	entry.input.Question = ""
	entry.answer = nil
	entry.citations = nil
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

func (service *Service) Close() {
	if service == nil {
		return
	}
	service.mu.Lock()
	service.closed = true
	clear(service.key[:])
	service.mu.Unlock()
	service.invalidate(func(*memoryRun) bool { return true })
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
	scope := cloneScope(entry.input.Scope)
	service.mu.Unlock()
	if service.authorizer.Authorize(work, actor, run, scope) != nil {
		service.invalidateEntry(entry)
		return Snapshot{}, assistant.ErrDenied
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.entries[id] != entry || entry.revoked || !service.time().Before(entry.expires) {
		return Snapshot{Run: run, Volatile: false}, nil
	}
	snapshot := Snapshot{Run: run, Volatile: true, Citations: slices.Clone(entry.citations)}
	if entry.answer != nil && run.State == assistant.Completed {
		snapshot.Answer = cloneAnswer(*entry.answer)
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
	service.mu.Unlock()
	run, err = service.repository.CancelAssistant(work, actor, id, service.time())
	// Durable cancellation wins before the running worker observes context loss.
	// The local fence still clears private state before invoking cancellation.
	if entry != nil && entry.input.Admission.Actor.UserID == actor.UserID {
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

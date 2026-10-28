package assistantservice

import (
	"context"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
)

// Each owned run serializes authoritative snapshots and state transitions.
// The context-aware lock owns no goroutine or unbounded queue. Only short
// metadata/repository work uses it; provider work leaves heartbeats unblocked.
func (service *Service) lockRun(ctx context.Context, entry *memoryRun) (func(), error) {
	if ctx == nil || entry == nil || entry.operations == nil {
		return nil, assistant.ErrInvalid
	}
	select {
	case entry.operations <- struct{}{}:
		if ctx.Err() != nil {
			<-entry.operations
			return nil, assistant.ErrUnavailable
		}
		return func() { <-entry.operations }, nil
	case <-ctx.Done():
		return nil, assistant.ErrUnavailable
	}
}

func (service *Service) evidenceContext(ctx context.Context) (context.Context, context.CancelFunc) {
	work, cancel := context.WithTimeout(ctx, assistantevidence.MaximumDuration)
	stop := context.AfterFunc(service.lifetime, cancel)
	return work, func() { stop(); cancel() }
}

// Caller holds the run lock, so a heartbeat cannot replace the exact persisted
// version/lease snapshot while the metadata gate checks it.
func (service *Service) authorizeLocked(ctx context.Context, entry *memoryRun) error {
	service.mu.Lock()
	run, scope, actor, revoked := entry.run, cloneScope(entry.input.Scope), entry.input.Admission.Actor, entry.revoked
	service.mu.Unlock()
	if revoked || ctx.Err() != nil {
		return assistant.ErrDenied
	}
	work, cancel := service.ioContext(ctx)
	defer cancel()
	if service.authorizer.Authorize(work, actor, run, scope) != nil {
		service.invalidateEntry(entry)
		return assistant.ErrDenied
	}
	return nil
}

func (service *Service) authorizeContent(ctx context.Context, entry *memoryRun) error {
	service.mu.Lock()
	run, actor, revoked := entry.run, entry.input.Admission.Actor, entry.revoked
	var evidence assistantevidence.Evidence
	if entry.evidence != nil {
		evidence = *entry.evidence
	}
	service.mu.Unlock()
	if revoked || ctx.Err() != nil {
		return assistant.ErrDenied
	}
	work, cancel := service.evidenceContext(ctx)
	defer cancel()
	if service.authorizer.AuthorizeEvidence(work, actor, run, evidence) != nil {
		service.invalidateEntry(entry)
		return assistant.ErrDenied
	}
	return nil
}

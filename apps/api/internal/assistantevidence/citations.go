package assistantevidence

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

// CitationLocator contains a closed observation kind and exact normalized IDs.
// Transport must map it to its own fixed view; it is never a URL or file path.
type CitationLocator struct {
	Kind       string
	Source     domain.ProviderRef
	ObjectID   string
	ResourceID domain.ResourceID
	ObservedAt time.Time
	Range      domain.TimeRange
}

func (CitationLocator) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected citation locator]")
}
func (CitationLocator) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (reader *Reader) Resolve(ctx context.Context, actor assistant.Actor, run assistant.Run, evidence Evidence, id string) (CitationLocator, error) {
	ctx, stopLifetime, err := reader.boundEvidence(ctx, run, evidence)
	if err != nil {
		return CitationLocator{}, err
	}
	defer stopLifetime()
	work, cancel, err := reader.work(ctx)
	if err != nil {
		return CitationLocator{}, err
	}
	defer cancel()
	if evidence.reader != reader || evidence.actor != actor || !reader.time().Before(evidence.expires) || !canonicalCitation(id) || run.State != assistant.Completed || run.Version <= evidence.run.Version || run.Fence != evidence.run.Fence+1 || !sameRun(run, evidence.run) {
		return CitationLocator{}, ErrDenied
	}
	source, err := reader.validate(actor, run, evidence.scope)
	if err != nil {
		return CitationLocator{}, err
	}
	var found CitationLocator
	for _, reference := range evidence.bundle.Citations() {
		if reference.ID != id {
			continue
		}
		switch reference.Kind {
		case "resource", "health", "metric", "event", "relationship", "log":
		default:
			return CitationLocator{}, ErrDenied
		}
		if reference.Source != evidence.scope.Source || reference.ConfigurationVersion != run.SourceVersion || reference.PolicyVersion != run.PolicyVersion || reference.RestoreGeneration != run.RestoreGeneration || reference.Range != evidence.scope.Range || !assistant.Identifier(reference.ObjectID) {
			return CitationLocator{}, ErrDenied
		}
		found = CitationLocator{Kind: reference.Kind, Source: reference.Source, ObjectID: reference.ObjectID, ResourceID: reference.ResourceID, ObservedAt: reference.ObservedAt, Range: reference.Range}
		break
	}
	if found.Kind == "" {
		return CitationLocator{}, ErrDenied
	}
	if _, err := reader.fence(work, actor, run, evidence.scope, source, evidence.kind, evidence.logConsent); err != nil {
		return CitationLocator{}, err
	}
	if err := reader.verifyLogPolicy(work, actor, run, source, evidence); err != nil {
		return CitationLocator{}, err
	}
	return found, nil
}

// AuthorizeEvidence is mandatory immediately before a consumer dispatches or
// releases previously collected content. Generic scope authorization alone does
// not prove that a log, event or metric family remains readable.
func (reader *Reader) AuthorizeEvidence(ctx context.Context, actor assistant.Actor, run assistant.Run, evidence Evidence) error {
	ctx, stopLifetime, err := reader.boundEvidence(ctx, run, evidence)
	if err != nil {
		return err
	}
	defer stopLifetime()
	work, cancel, err := reader.work(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	if evidence.reader != reader || evidence.actor != actor || !reader.time().Before(evidence.expires) || !sameRun(run, evidence.run) || run.Version < evidence.run.Version {
		return ErrDenied
	}
	switch run.State {
	case assistant.Running:
		if run.Fence != evidence.run.Fence || run.LeaseOwner != evidence.run.LeaseOwner || !run.LeaseUntil.After(reader.time()) || !run.Deadline.After(reader.time()) {
			return ErrDenied
		}
	case assistant.Completed:
		if run.Fence != evidence.run.Fence+1 || run.Version <= evidence.run.Version {
			return ErrDenied
		}
	default:
		return ErrDenied
	}
	source, err := reader.validate(actor, run, evidence.scope)
	if err != nil {
		return err
	}
	_, err = reader.fence(work, actor, run, evidence.scope, source, evidence.kind, evidence.logConsent)
	if err != nil {
		return err
	}
	return reader.verifyLogPolicy(work, actor, run, source, evidence)
}

// Lifetime is bounded before waiting for a provider slot, so a short remaining
// lease or volatile lifetime cannot wait behind a full 25-second read.
func (reader *Reader) boundEvidence(ctx context.Context, run assistant.Run, evidence Evidence) (context.Context, context.CancelFunc, error) {
	if reader == nil || ctx == nil {
		return nil, nil, ErrInvalid
	}
	if evidence.reader != reader || !reader.time().Before(evidence.expires) {
		return nil, nil, ErrDenied
	}
	remaining := evidence.expires.Sub(reader.time())
	if run.State == assistant.Running {
		remaining = min(remaining, run.LeaseUntil.Sub(reader.time()), run.Deadline.Sub(reader.time()))
	}
	if remaining <= 0 {
		return nil, nil, ErrDenied
	}
	work, cancel := context.WithTimeout(ctx, remaining)
	return work, cancel, nil
}

func sameRun(current, original assistant.Run) bool {
	return current.ID == original.ID && current.ConversationID == original.ConversationID && current.OwnerID == original.OwnerID && current.SourceID == original.SourceID && current.ResourceID == original.ResourceID && current.Goal == original.Goal && current.SourceVersion == original.SourceVersion && current.RestoreGeneration == original.RestoreGeneration && current.PolicyVersion == original.PolicyVersion && current.ProfileID == original.ProfileID && current.ProfileVersion == original.ProfileVersion && current.ConfigurationVersion == original.ConfigurationVersion && current.Volatile == original.Volatile && current.Paid == original.Paid && current.CreatedAt == original.CreatedAt && current.From == original.From && current.Until == original.Until && current.Deadline == original.Deadline && current.Attempts == original.Attempts && !current.UsageUncertain
}

func canonicalCitation(value string) bool {
	if len(value) != 67 || !strings.HasPrefix(value, "ev_") || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value[3:])
	return err == nil
}

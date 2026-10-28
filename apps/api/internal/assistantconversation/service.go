package assistantconversation

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantservice"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

type Runtime interface {
	Submit(context.Context, assistantservice.Input) (assistant.Run, error)
	Get(context.Context, assistant.Actor, string) (assistantservice.Snapshot, error)
	Cancel(context.Context, assistant.Actor, string) (assistant.Run, error)
}
type Repository interface {
	GetAssistant(context.Context, assistant.Actor, string, time.Time) (assistant.Run, error)
}
type Resolver interface {
	Resolve(context.Context, assistant.Actor, domain.SourceID, domain.ResourceID, domain.TimeRange) (contextbuilder.Scope, error)
}
type Binding struct {
	ProfileID                                    string
	ProfileVersion, PolicyVersion, ReservedUnits int64
}
type Options struct {
	Runtime    Runtime
	Repository Repository
	Resolver   Resolver
	Binding    Binding
	Key        []byte
	Now        func() time.Time
}

func (Options) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected conversation options]")
}
func (Options) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type Service struct {
	runtime    Runtime
	repository Repository
	resolver   Resolver
	binding    Binding
	now        func() time.Time
	mu         sync.Mutex
	key        [32]byte
	closed     bool
}

func (*Service) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected conversation service]")
}

func New(options Options) (*Service, error) {
	if options.Runtime == nil || options.Repository == nil || options.Resolver == nil || options.Now == nil ||
		!assistant.Identifier(options.Binding.ProfileID) || options.Binding.ProfileVersion < 1 || options.Binding.PolicyVersion < 1 ||
		options.Binding.ReservedUnits < inference.MaximumInputTokens+1 || options.Binding.ReservedUnits > assistant.MaximumReservationUnits ||
		len(options.Key) != 32 || !slices.ContainsFunc(options.Key, func(value byte) bool { return value != 0 }) {
		return nil, assistant.ErrInvalid
	}
	service := &Service{runtime: options.Runtime, repository: options.Repository, resolver: options.Resolver, binding: options.Binding, now: options.Now}
	copy(service.key[:], options.Key)
	return service, nil
}

// Close clears this coordinator's key; the application owns the runtime lifecycle.
func (service *Service) Close() {
	if service == nil {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	service.closed = true
	clear(service.key[:])
}
func (service *Service) open() bool {
	if service == nil {
		return false
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return !service.closed
}

func (service *Service) Get(ctx context.Context, actor assistant.Actor, id string) (assistantservice.Snapshot, error) {
	if !service.open() {
		return assistantservice.Snapshot{}, assistant.ErrUnavailable
	}
	return service.runtime.Get(ctx, actor, id)
}
func (service *Service) Cancel(ctx context.Context, actor assistant.Actor, id string) (assistant.Run, error) {
	if !service.open() {
		return assistant.Run{}, assistant.ErrUnavailable
	}
	return service.runtime.Cancel(ctx, actor, id)
}

func (service *Service) Submit(ctx context.Context, actor assistant.Actor, request Request, nonce string) (assistant.Run, error) {
	if ctx == nil || ctx.Err() != nil || !actor.Valid() || !request.Valid() || !ValidNonce(nonce) {
		return assistant.Run{}, assistant.ErrInvalid
	}
	if service == nil {
		return assistant.Run{}, assistant.ErrUnavailable
	}
	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		return assistant.Run{}, assistant.ErrUnavailable
	}
	key := service.key
	service.mu.Unlock()
	defer clear(key[:])
	derive := func(purpose string) [32]byte {
		mac := hmac.New(sha256.New, key[:])
		for _, part := range []string{purpose, actor.UserID, nonce} {
			_, _ = mac.Write([]byte(part))
			_, _ = mac.Write([]byte{0})
		}
		var sum [32]byte
		copy(sum[:], mac.Sum(nil))
		return sum
	}
	runDigest := derive("conversation-run-v1")
	id := "ar_" + hex.EncodeToString(runDigest[:])
	conversationDigest := derive("conversation-ledger-v1")
	conversation := "ac_" + hex.EncodeToString(conversationDigest[:])
	if request.ConversationID != "" {
		conversation = request.ConversationID
	}
	at := service.now().UTC().Truncate(time.Microsecond)
	if !assistant.Timestamp(at) {
		return assistant.Run{}, assistant.ErrUnavailable
	}
	selected := domain.TimeRange{Start: at.Add(-time.Duration(request.WindowMinutes) * time.Minute), End: at}
	work, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	existing, err := service.repository.GetAssistant(work, actor, id, at)
	if work.Err() != nil {
		return assistant.Run{}, assistant.ErrUnavailable
	}
	requireExisting := err == nil
	if err == nil {
		if existing.ID != id || existing.OwnerID != actor.UserID {
			return assistant.Run{}, assistant.ErrDenied
		}
		if existing.SourceID != string(request.SourceID) || existing.ResourceID != string(request.ResourceID) || existing.Until.Sub(existing.From) != time.Duration(request.WindowMinutes)*time.Minute || (request.ConversationID != "" && existing.ConversationID != request.ConversationID) {
			return assistant.Run{}, assistant.ErrConflict
		}
		if !conversationID.MatchString(existing.ConversationID) || !assistant.Timestamp(existing.From) || !assistant.Timestamp(existing.Until) || existing.Until.After(at) {
			return assistant.Run{}, assistant.ErrUnavailable
		}
		selected = domain.TimeRange{Start: existing.From, End: existing.Until}
		conversation = existing.ConversationID
	} else if err != assistant.ErrDenied {
		return assistant.Run{}, ownedError(err)
	}
	scope, err := service.resolver.Resolve(work, actor, request.SourceID, request.ResourceID, selected)
	if work.Err() != nil {
		return assistant.Run{}, assistant.ErrUnavailable
	}
	if err != nil {
		return assistant.Run{}, ownedError(err)
	}
	if scope.Source.SourceID != request.SourceID || scope.Source.Validate() != nil || len(scope.Resources) != 1 || scope.Resources[0].ID != request.ResourceID || scope.Resources[0].Validate() != nil || !scope.Source.Matches(scope.Resources[0]) || scope.ConfigurationVersion < 1 || scope.PolicyVersion != service.binding.PolicyVersion || scope.RestoreGeneration < 0 || !scope.Range.Start.Equal(selected.Start) || !scope.Range.End.Equal(selected.End) {
		return assistant.Run{}, assistant.ErrUnavailable
	}
	scope.Range = selected
	service.mu.Lock()
	closed := service.closed
	service.mu.Unlock()
	if closed {
		return assistant.Run{}, assistant.ErrUnavailable
	}
	a := assistant.Admission{ID: id, ConversationID: conversation, Actor: actor, IdempotencyDigest: derive("conversation-idempotency-v1"), Goal: assistant.ExplainResource, SourceID: string(request.SourceID), SourceVersion: scope.ConfigurationVersion, ResourceID: string(request.ResourceID), From: selected.Start, Until: selected.End, RestoreGeneration: scope.RestoreGeneration, PolicyVersion: service.binding.PolicyVersion, ProfileID: service.binding.ProfileID, ProfileVersion: service.binding.ProfileVersion, ReservedUnits: service.binding.ReservedUnits, RequireExisting: requireExisting}
	return service.runtime.Submit(work, assistantservice.Input{Admission: a, Scope: scope, Locale: request.Locale, Question: request.Question, RequireExistingConversation: request.ConversationID != ""})
}

func ownedError(err error) error {
	for _, known := range []error{assistant.ErrInvalid, assistant.ErrDenied, assistant.ErrConflict, assistant.ErrQuota} {
		if err == known {
			return known
		}
	}
	return assistant.ErrUnavailable
}

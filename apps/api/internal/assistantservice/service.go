// Package assistantservice coordinates explicitly admitted, read-only local runs.
package assistantservice

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistantevidence"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/inference"
)

const (
	MaximumMemoryRuns         = 18
	MaximumOwnerMemoryRuns    = 8
	MaximumOwnerConversations = 8
	MaximumTurns              = 8
	MemoryLifetime            = 30 * time.Minute
	storageTimeout            = 3 * time.Second
)

// Authorizer separates candidate admission, metadata and sealed content access.
type Authorizer interface {
	AuthorizeAdmission(context.Context, assistant.Actor, assistant.Run, contextbuilder.Scope) error
	Authorize(context.Context, assistant.Actor, assistant.Run, contextbuilder.Scope) error
	AuthorizeEvidence(context.Context, assistant.Actor, assistant.Run, assistantevidence.Evidence) error
}

// EvidenceCollector performs one bounded typed read and respects cancellation.
type EvidenceCollector interface {
	Read(context.Context, assistantevidence.ReadRequest) (assistantevidence.Evidence, error)
}

type Options struct {
	Repository     assistant.RuntimeRepository
	Gateway        *inference.Gateway
	Authorizer     Authorizer
	Collector      EvidenceCollector
	Lifetime       context.Context
	Now            func() time.Time
	ProcessID      string
	FingerprintKey []byte
	Workers        int
	PollInterval   time.Duration
	Heartbeat      time.Duration
	OutputLimit    int
}

func (Options) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected helper options]")
}

type Input struct {
	Admission assistant.Admission
	Scope     contextbuilder.Scope
	Locale    string
	Question  string
}

func (Input) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected helper input]")
}
func (Input) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type Snapshot struct {
	Run       assistant.Run
	Answer    *inference.Answer
	Citations []contextbuilder.CitationReference
	Volatile  bool
}

func (Snapshot) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected helper snapshot]")
}
func (Snapshot) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type Service struct {
	repository    assistant.RuntimeRepository
	gateway       *inference.Gateway
	authorizer    Authorizer
	collector     EvidenceCollector
	lifetime      context.Context
	stop          context.CancelFunc
	stopWatching  func() bool
	now           func() time.Time
	processID     string
	workers       int
	poll          time.Duration
	heartbeat     time.Duration
	output        int
	key           [32]byte
	admissions    chan struct{}
	mu            sync.Mutex
	entries       map[string]*memoryRun
	conversations map[string]conversation
	closed        bool
	running       atomic.Bool
}

func (*Service) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected helper service]")
}

func New(options Options) (*Service, error) {
	if options.Repository == nil || options.Gateway == nil || options.Authorizer == nil || options.Collector == nil || options.Lifetime == nil || options.Now == nil {
		return nil, assistant.ErrInvalid
	}
	if _, ok := options.Repository.(assistant.AdmissionRepository); !ok {
		return nil, assistant.ErrInvalid
	}
	if !assistant.Identifier(options.ProcessID) || len(options.ProcessID) > 112 || len(options.FingerprintKey) != 32 || !slices.ContainsFunc(options.FingerprintKey, func(value byte) bool { return value != 0 }) {
		return nil, assistant.ErrInvalid
	}
	if options.Workers < 1 || options.Workers > 2 || options.PollInterval < 100*time.Millisecond || options.PollInterval > time.Second || options.Heartbeat < time.Second || options.Heartbeat > 10*time.Second {
		return nil, assistant.ErrInvalid
	}
	if options.OutputLimit < 1 || options.OutputLimit > inference.MaximumOutputTokens {
		return nil, assistant.ErrInvalid
	}
	lifetime, stop := context.WithCancel(options.Lifetime)
	service := &Service{repository: options.Repository, gateway: options.Gateway, authorizer: options.Authorizer, collector: options.Collector, lifetime: lifetime, stop: stop, now: options.Now, processID: options.ProcessID, workers: options.Workers, poll: options.PollInterval, heartbeat: options.Heartbeat, output: options.OutputLimit, admissions: make(chan struct{}, 1), entries: map[string]*memoryRun{}, conversations: map[string]conversation{}}
	copy(service.key[:], options.FingerprintKey)
	service.mu.Lock()
	service.stopWatching = context.AfterFunc(lifetime, service.Close)
	service.mu.Unlock()
	return service, nil
}

// Submit accepts only the initial local, single-resource explanation workflow.
// Raw text is staged in memory before the metadata-only transaction can commit.
func (service *Service) Submit(ctx context.Context, input Input) (assistant.Run, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || service.lifetime.Err() != nil {
		return assistant.Run{}, assistant.ErrUnavailable
	}
	admissions, ok := service.repository.(assistant.AdmissionRepository)
	if !ok {
		return assistant.Run{}, assistant.ErrUnavailable
	}
	select {
	case service.admissions <- struct{}{}:
		defer func() { <-service.admissions }()
	default:
		return assistant.Run{}, assistant.ErrQuota
	}
	input.Scope.Resources = slices.Clone(input.Scope.Resources)
	at := service.time()
	a := input.Admission
	a.At, a.Deadline, a.Volatile, a.MemoryOwner = at, at.Add(assistant.MaximumRunDuration), true, service.processID
	a.Fingerprint = [32]byte{1}
	input.Scope.RunID, input.Scope.ObservedAt = domain.RequestID(a.ID), at
	if !a.Valid() || a.Paid || a.Goal != assistant.ExplainResource || len(input.Scope.Resources) != 1 || input.Scope.Source.Validate() != nil || input.Scope.Resources[0].Validate() != nil || !input.Scope.Source.Matches(input.Scope.Resources[0]) || string(input.Scope.Resources[0].ID) != a.ResourceID || string(input.Scope.Source.SourceID) != a.SourceID || input.Scope.ConfigurationVersion != a.SourceVersion || input.Scope.PolicyVersion != a.PolicyVersion || input.Scope.RestoreGeneration != a.RestoreGeneration || input.Scope.Range.Start != a.From || input.Scope.Range.End != a.Until || a.Until.After(at) {
		return assistant.Run{}, assistant.ErrInvalid
	}
	if err := (inference.Request{Locale: input.Locale, Question: input.Question, OutputLimit: service.output}).Validate(); err != nil {
		return assistant.Run{}, assistant.ErrInvalid
	}
	if a.ReservedUnits < service.maximumUnits() {
		return assistant.Run{}, assistant.ErrQuota
	}
	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		return assistant.Run{}, assistant.ErrUnavailable
	}
	key := service.key
	service.mu.Unlock()
	mac := hmac.New(sha256.New, key[:])
	clear(key[:])
	for _, value := range []string{"helper-local-explanation-v1", input.Locale, input.Question, string(input.Scope.Source.Provider), strconv.Itoa(service.output)} {
		_, _ = mac.Write([]byte(value))
		_, _ = mac.Write([]byte{0})
	}
	copy(a.Fingerprint[:], mac.Sum(nil))
	input.Admission = a
	work, cancel := service.ioContext(ctx)
	defer cancel()
	if service.authorizer.AuthorizeAdmission(work, a.Actor, runFrom(a), cloneScope(input.Scope)) != nil {
		return assistant.Run{}, assistant.ErrDenied
	}
	// A retained durable retry never restores a lost question or consumes a turn.
	// The transaction must still find the row after this separate metadata read.
	if existing, lookupErr := service.repository.GetAssistant(work, a.Actor, a.ID, service.time()); lookupErr == nil {
		if existing.ID != a.ID || existing.OwnerID != a.Actor.UserID {
			return assistant.Run{}, assistant.ErrDenied
		}
		a.RequireExisting = true
		outcome, retryErr := admissions.AdmitAssistantWithOutcome(work, a)
		run := outcome.Run
		if retryErr != nil {
			return assistant.Run{}, ownedError(retryErr)
		}
		if outcome.Created || run.ID != a.ID || run.OwnerID != a.Actor.UserID {
			return assistant.Run{}, assistant.ErrConflict
		}
		service.mu.Lock()
		closed := service.closed
		service.mu.Unlock()
		if closed {
			return assistant.Run{}, assistant.ErrUnavailable
		}
		return run, nil
	} else if lookupErr != assistant.ErrDenied {
		return assistant.Run{}, ownedError(lookupErr)
	}
	a.RequireExisting = false
	input.Admission = a
	entry, staged, err := service.stage(input)
	if err != nil {
		return assistant.Run{}, err
	}
	outcome, err := admissions.AdmitAssistantWithOutcome(work, a)
	run := outcome.Run
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed {
		return assistant.Run{}, assistant.ErrUnavailable
	}
	if staged {
		entry.admitting = false
	}
	if err != nil || run.ID != a.ID || run.OwnerID != a.Actor.UserID {
		if staged && service.entries[a.ID] == entry {
			service.discardStagedLocked(entry)
		}
		if err != nil {
			return assistant.Run{}, ownedError(err)
		}
		return assistant.Run{}, assistant.ErrConflict
	}
	if staged && (!outcome.Created || run.State.Terminal() || entry.revoked || service.entries[a.ID] != entry) {
		// A different admission can win after the preflight for any durable state.
		// Reusing its metadata never restores newly supplied private material.
		service.discardStagedLocked(entry)
		return run, nil
	}
	if staged {
		entry.run = run
		c := service.conversations[a.ConversationID]
		c.turns++
		service.conversations[a.ConversationID] = c
	}
	entry.readyOnce.Do(func() { close(entry.ready) })
	return run, nil
}

func runFrom(a assistant.Admission) assistant.Run {
	return assistant.Run{ID: a.ID, ConversationID: a.ConversationID, OwnerID: a.Actor.UserID, SourceID: a.SourceID, ResourceID: a.ResourceID, Goal: a.Goal, State: assistant.Queued, SourceVersion: a.SourceVersion, RestoreGeneration: a.RestoreGeneration, PolicyVersion: a.PolicyVersion, ProfileVersion: a.ProfileVersion, ProfileID: a.ProfileID, Volatile: true, Paid: a.Paid, ReservedUnits: a.ReservedUnits, CreatedAt: a.At, UpdatedAt: a.At, Deadline: a.Deadline, From: a.From, Until: a.Until}
}

func (service *Service) maximumUnits() int64 {
	return int64(inference.MaximumInputTokens + service.output)
}
func (service *Service) time() time.Time { return service.now().UTC().Truncate(time.Microsecond) }

func (service *Service) ioContext(ctx context.Context) (context.Context, context.CancelFunc) {
	work, cancel := context.WithTimeout(ctx, storageTimeout)
	stop := context.AfterFunc(service.lifetime, cancel)
	return work, func() { stop(); cancel() }
}

func ownedError(err error) error {
	for _, allowed := range []error{assistant.ErrInvalid, assistant.ErrDenied, assistant.ErrConflict, assistant.ErrQuota} {
		if err == allowed {
			return allowed
		}
	}
	return assistant.ErrUnavailable
}

func cloneAnswer(answer inference.Answer) *inference.Answer {
	copy := inference.Answer{Sections: slices.Clone(answer.Sections)}
	for index := range copy.Sections {
		copy.Sections[index].Citations = slices.Clone(copy.Sections[index].Citations)
	}
	return &copy
}

func answerBytes(answer inference.Answer) int {
	body, err := json.Marshal(answer)
	if err != nil {
		return inference.MaximumOutputBytes + 1
	}
	defer clear(body)
	return len(body)
}

func cloneScope(scope contextbuilder.Scope) contextbuilder.Scope {
	scope.Resources = slices.Clone(scope.Resources)
	return scope
}

// Package assistantevidence binds typed, bounded evidence reads to current authority.
package assistantevidence

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/assistant"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/contextbuilder"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/logpolicy"
)

const (
	MaximumSources            = 64
	MaximumResources          = 32
	MaximumRecords            = 64
	MaximumConcurrent         = 2
	MaximumMetadataConcurrent = 2
	MaximumMetadataDuration   = 3 * time.Second
	MaximumDuration           = 25 * time.Second
	MaximumAuthorityAge       = 5 * time.Second
	VolatileLifetime          = 30 * time.Minute
)

var (
	ErrInvalid     = errors.New("assistant.evidence_invalid")
	ErrDenied      = errors.New("assistant.evidence_denied")
	ErrUnavailable = errors.New("assistant.evidence_unavailable")
	ErrBudget      = errors.New("assistant.evidence_budget")
	ErrCancelled   = errors.New("assistant.evidence_cancelled")
)

type MetadataGate interface {
	Check(context.Context, assistant.Actor, assistant.Run, time.Time) error
	CheckAdmission(context.Context, assistant.Actor, assistant.Run, time.Time) error
}

type AuthorityRepository interface {
	AuthorizeAssistantAdmission(context.Context, assistant.Actor, assistant.Run, time.Time) error
	AuthorizeAssistantEvidence(context.Context, assistant.Actor, assistant.Run, time.Time) error
}

// RepositoryGate accepts only the use-case-shaped metadata authorization port.
type RepositoryGate struct{ Repository AuthorityRepository }

func (RepositoryGate) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected evidence gate]")
}
func (RepositoryGate) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (gate RepositoryGate) Check(ctx context.Context, actor assistant.Actor, run assistant.Run, at time.Time) error {
	if gate.Repository == nil {
		return ErrUnavailable
	}
	return gate.Repository.AuthorizeAssistantEvidence(ctx, actor, run, at)
}

func (gate RepositoryGate) CheckAdmission(ctx context.Context, actor assistant.Actor, run assistant.Run, at time.Time) error {
	if gate.Repository == nil {
		return ErrUnavailable
	}
	return gate.Repository.AuthorizeAssistantAdmission(ctx, actor, run, at)
}

// ReadPorts deliberately omit provider mutation, streaming and generic query ports.
type ReadPorts interface {
	GetResource(context.Context, domain.ResourceDetailQuery) (domain.ResourceSnapshot, error)
	domain.HealthProvider
	domain.MetricHistoryProvider
	domain.EventHistoryProvider
	domain.TopologyProvider
	domain.CapabilityProvider
}

// LiveScopeAuthority must obtain current permission evidence, not a support enum.
type LiveScopeAuthority interface {
	Check(context.Context, contextbuilder.Scope, assistant.StepKind, bool) error
}

type Source struct {
	Reference            domain.ProviderRef
	ConfigurationVersion int64
	Read                 ReadPorts
	Live                 LiveScopeAuthority
	Logs                 *logpolicy.Reader
}

func (Source) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected evidence source]")
}
func (Source) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type Options struct {
	Gate    MetadataGate
	Sources []Source
	Now     func() time.Time
	Timeout time.Duration
}

func (Options) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected evidence options]")
}
func (Options) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

// ReadRequest is constructed by trusted use cases. No field is a model-selected
// endpoint, file path, tool registration, cursor, metric name or query string.
type ReadRequest struct {
	Actor       assistant.Actor
	Run         assistant.Run
	Scope       contextbuilder.Scope
	Kind        assistant.StepKind
	Destination contextbuilder.Destination
	LogConsent  bool
}

func (ReadRequest) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected evidence read]")
}
func (ReadRequest) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type Reader struct {
	gate          MetadataGate
	sources       map[domain.SourceID]Source
	now           func() time.Time
	timeout       time.Duration
	slots         chan struct{}
	metadataSlots chan struct{}
}

func (Reader) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected evidence reader]")
}
func (Reader) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

// Evidence seals citations to this reader, actor, admitted run and volatile scope.
// Consumers receive copies of references; caller-provided references cannot be
// turned into a sealed citation set.
type Evidence struct {
	reader     *Reader
	actor      assistant.Actor
	run        assistant.Run
	scope      contextbuilder.Scope
	bundle     contextbuilder.Bundle
	kind       assistant.StepKind
	logConsent bool
	logPolicy  [32]byte
	expires    time.Time
}

func (Evidence) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[protected evidence result]")
}
func (Evidence) MarshalJSON() ([]byte, error)           { return []byte("{}"), nil }
func (evidence Evidence) Bundle() contextbuilder.Bundle { return evidence.bundle }
func (evidence Evidence) References() []contextbuilder.CitationReference {
	return evidence.bundle.Citations()
}

func New(options Options) (*Reader, error) {
	if options.Gate == nil || options.Now == nil || len(options.Sources) < 1 || len(options.Sources) > MaximumSources {
		return nil, ErrInvalid
	}
	if options.Timeout == 0 {
		options.Timeout = MaximumDuration
	}
	if options.Timeout < time.Millisecond || options.Timeout > MaximumDuration {
		return nil, ErrInvalid
	}
	sources := make(map[domain.SourceID]Source, len(options.Sources))
	for _, source := range options.Sources {
		if source.Reference.Validate() != nil || source.ConfigurationVersion < 1 || source.Read == nil || source.Live == nil {
			return nil, ErrInvalid
		}
		if _, duplicate := sources[source.Reference.SourceID]; duplicate {
			return nil, ErrInvalid
		}
		if source.Logs != nil {
			if source.Logs.Provider == nil || source.Logs.Settings == nil || source.Logs.Redactor == nil || source.Logs.Redactor.PolicyFingerprint() == [32]byte{} {
				return nil, ErrInvalid
			}
			copy := *source.Logs
			source.Logs = &copy
		}
		sources[source.Reference.SourceID] = source
	}
	return &Reader{gate: options.Gate, sources: sources, now: options.Now, timeout: options.Timeout, slots: make(chan struct{}, MaximumConcurrent), metadataSlots: make(chan struct{}, MaximumMetadataConcurrent)}, nil
}

func (reader *Reader) work(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if reader == nil {
		return nil, nil, ErrInvalid
	}
	return reader.workOn(ctx, reader.slots, reader.timeout)
}

func (reader *Reader) metadataWork(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if reader == nil {
		return nil, nil, ErrInvalid
	}
	return reader.workOn(ctx, reader.metadataSlots, min(reader.timeout, MaximumMetadataDuration))
}

func (reader *Reader) workOn(ctx context.Context, slots chan struct{}, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	if reader == nil || ctx == nil {
		return nil, nil, ErrInvalid
	}
	work, cancel := context.WithTimeout(ctx, timeout)
	select {
	case slots <- struct{}{}:
		if work.Err() != nil {
			<-slots
			cancel()
			return nil, nil, ErrCancelled
		}
		return work, func() { <-slots; cancel() }, nil
	case <-work.Done():
		cancel()
		return nil, nil, ErrCancelled
	}
}

func (reader *Reader) time() time.Time { return reader.now().UTC().Truncate(time.Microsecond) }

func safeError(ctx context.Context, err error) error {
	if ctx != nil && ctx.Err() != nil {
		return ErrCancelled
	}
	for _, denied := range []error{ErrDenied, assistant.ErrDenied, contextbuilder.ErrDenied} {
		if errors.Is(err, denied) {
			return ErrDenied
		}
	}
	var failure *domain.ProviderError
	if errors.As(err, &failure) && failure.Code == domain.ProviderErrorPermissionDenied {
		return ErrDenied
	}
	if errors.Is(err, contextbuilder.ErrBudget) {
		return ErrBudget
	}
	return ErrUnavailable
}

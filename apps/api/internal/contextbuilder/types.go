// Package contextbuilder projects authorized domain observations into bounded evidence.
package contextbuilder

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/logpolicy"
)

// Downstream inference adapters may impose stricter evidence and per-item text caps.
const (
	MaximumRecords      = 256
	MaximumBytes        = 64 << 10
	MaximumLogLines     = 200
	MaximumLogBytes     = 32 << 10
	MaximumLogLineBytes = 4096
	MaximumLogWindow    = 15 * time.Minute
)

var (
	ErrInvalid = errors.New("evidence request is invalid")
	ErrDenied  = errors.New("evidence access is denied")
	ErrBudget  = errors.New("evidence budget exceeded")
)

type Destination string

const (
	Local  Destination = "local"
	Remote Destination = "remote"
)

// Scope is supplied by an authorized use case, never copied from model arguments.
// Projection does not replace current actor, source, or provider authorization.
type Scope struct {
	RunID                domain.RequestID
	Source               domain.ProviderRef
	ConfigurationVersion int64
	PolicyVersion        int64
	RestoreGeneration    int64
	Range                domain.TimeRange
	ObservedAt           time.Time
	Resources            []domain.ResourceRef
}

type Request struct {
	Scope         Scope
	Destination   Destination
	Status        domain.ObservationStatus
	Resources     []domain.ResourceSnapshot
	Health        []domain.HealthSnapshot
	Metrics       []domain.MetricSample
	Events        []domain.InfrastructureEvent
	Relationships []domain.Relationship
	Logs          *logpolicy.Capture
	LogConsent    bool
	// MoreAvailable carries source page/cap bounds; projection does not claim a full estate scan.
	MoreAvailable bool
	RecordLimit   int
	ByteLimit     int
}

func (Request) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("[protected evidence request]"))
}
func (Request) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

type CitationReference struct {
	ID                   string
	Kind                 string
	ObjectID             string
	Source               domain.ProviderRef
	ConfigurationVersion int64
	PolicyVersion        int64
	RestoreGeneration    int64
	ResourceID           domain.ResourceID
	ObservedAt           time.Time
	Range                domain.TimeRange
}

type Bundle struct {
	document   document
	references []CitationReference
}

func (Bundle) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("[protected evidence bundle]"))
}
func (Bundle) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func (bundle Bundle) Citations() []CitationReference {
	return append([]CitationReference(nil), bundle.references...)
}

func (bundle Bundle) RecordCount() int { return len(bundle.document.Records) }

type document struct {
	Version              int                 `json:"version"`
	SourceReference      string              `json:"source_reference"`
	Provider             domain.ProviderKind `json:"provider"`
	ConfigurationVersion int64               `json:"configuration_version"`
	PolicyVersion        int64               `json:"policy_version"`
	RestoreGeneration    int64               `json:"restore_generation"`
	Range                timeRange           `json:"range"`
	Quality              quality             `json:"quality"`
	MoreAvailable        bool                `json:"more_available"`
	Records              []record            `json:"records"`
}

type timeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type quality struct {
	Availability domain.AvailabilityState `json:"availability"`
	Completeness domain.Completeness      `json:"completeness"`
	ObservedAt   time.Time                `json:"observed_at"`
	Synthetic    bool                     `json:"synthetic"`
}

type record struct {
	CitationID       string             `json:"citation_id"`
	Kind             string             `json:"kind"`
	ResourceID       string             `json:"resource_id,omitempty"`
	Quality          quality            `json:"quality"`
	VersionReference string             `json:"version_reference,omitempty"`
	Health           domain.HealthState `json:"health,omitempty"`
	Metric           *metricFact        `json:"metric,omitempty"`
	Event            *eventFact         `json:"event,omitempty"`
	Relationship     *relationshipFact  `json:"relationship,omitempty"`
	Log              *logFact           `json:"log,omitempty"`
}

type metricFact struct {
	Name            domain.MetricName `json:"name"`
	Unit            domain.MetricUnit `json:"unit"`
	Value           float64           `json:"value"`
	SampledAt       time.Time         `json:"sampled_at"`
	IntervalSeconds int64             `json:"interval_seconds"`
}

type eventFact struct {
	Severity   domain.Severity `json:"severity"`
	OccurredAt time.Time       `json:"occurred_at"`
}

type relationshipFact struct {
	From      string                       `json:"from"`
	To        string                       `json:"to"`
	Kind      domain.RelationshipKind      `json:"kind"`
	Direction domain.RelationshipDirection `json:"direction"`
	Evidence  []relationshipEvidence       `json:"evidence"`
}

type relationshipEvidence struct {
	Provenance domain.RelationshipProvenance `json:"provenance"`
	Certainty  domain.EvidenceCertainty      `json:"certainty"`
	Confidence float64                       `json:"confidence"`
}

type logFact struct {
	Timestamp time.Time            `json:"timestamp"`
	Severity  domain.LogSeverity   `json:"severity"`
	Stream    domain.LogStreamKind `json:"stream"`
	Message   string               `json:"message"`
	Redacted  bool                 `json:"redacted"`
	Truncated bool                 `json:"truncated"`
}

type projection struct {
	ctx     context.Context
	request Request
	allowed map[domain.ResourceID]domain.ResourceRef
	bundle  Bundle
	seen    map[string]bool
	maximum int
}

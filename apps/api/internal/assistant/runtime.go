// Package assistant defines bounded, metadata-only investigation runs.
package assistant

import (
	"context"
	"errors"
	"regexp"
	"time"
)

var (
	ErrInvalid     = errors.New("assistant.invalid")
	ErrDenied      = errors.New("assistant.denied")
	ErrConflict    = errors.New("assistant.conflict")
	ErrQuota       = errors.New("assistant.quota")
	ErrUnavailable = errors.New("assistant.unavailable")
)

type State string

const (
	Queued      State = "queued"
	Running     State = "running"
	Completed   State = "completed"
	Failed      State = "failed"
	Cancelled   State = "cancelled"
	Expired     State = "expired"
	Interrupted State = "interrupted"
	Uncertain   State = "uncertain"
)

func (s State) Terminal() bool {
	return s == Completed || s == Failed || s == Cancelled || s == Expired || s == Interrupted || s == Uncertain
}

type Goal string

const (
	ExplainResource     Goal = "explain_resource"
	ExplainAlert        Goal = "explain_alert"
	InvestigateIncident Goal = "investigate_incident"
)

func (g Goal) Valid() bool {
	return g == ExplainResource || g == ExplainAlert || g == InvestigateIncident
}

type StepKind string

const (
	Inventory StepKind = "inventory"
	Health    StepKind = "health"
	Metrics   StepKind = "metrics"
	Events    StepKind = "events"
	Topology  StepKind = "topology"
	Logs      StepKind = "logs"
	Inference StepKind = "inference"
)

func (k StepKind) Valid() bool {
	return k == Inventory || k == Health || k == Metrics || k == Events || k == Topology || k == Logs || k == Inference
}

const (
	MaximumRunDuration            = 120 * time.Second
	MaximumLeaseDuration          = 30 * time.Second
	MinimumLeaseDuration          = time.Second
	MaximumClaims                 = 3
	MaximumStatusVersions         = 64
	MaximumToolCalls              = 12
	MaximumInferenceCalls         = 3
	MaximumReservationUnits int64 = 1_000_000
	MaximumQuotaUnits       int64 = 1_000_000_000
	MetadataRetention             = 7 * 24 * time.Hour
	ReplayRetention               = 24 * time.Hour
	MaximumRetentionBatch         = 1000
)

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:-]{0,127}$`)

func Identifier(value string) bool { return identifier.MatchString(value) }
func Digest(value [32]byte) bool   { return value != [32]byte{} }
func Timestamp(value time.Time) bool {
	_, offset := value.Zone()
	return !value.IsZero() && value.Year() >= 1970 && value.Year() <= 9999 && offset == 0 && value.Equal(value.Truncate(time.Microsecond))
}

// Actor holds a keyed digest reference; raw session identifiers never enter runs.
type Actor struct {
	UserID        string
	SessionDigest [32]byte
	DigestKeyID   string
	AuthRevision  int64
}

func (a Actor) Valid() bool {
	return Identifier(a.UserID) && Identifier(a.DigestKeyID) && Digest(a.SessionDigest) && a.AuthRevision > 0
}

type Settings struct {
	Version           int64
	Enabled           bool
	PaidEnabled       bool
	ProfilePaid       bool
	PolicyVersion     int64
	ProfileID         string
	ProfileVersion    int64
	UserQuota         int64
	InstallationQuota int64
}

func (s Settings) Valid() bool {
	return s.Version > 0 && s.PolicyVersion > 0 && Identifier(s.ProfileID) && s.ProfileVersion > 0 && s.UserQuota > 0 && s.UserQuota <= MaximumQuotaUnits && s.InstallationQuota >= s.UserQuota && s.InstallationQuota <= MaximumQuotaUnits
}

type OperatorAudit struct {
	ID, OperatorID, RequestID string
	At                        time.Time
}

func (a OperatorAudit) Valid() bool {
	return Identifier(a.ID) && Identifier(a.OperatorID) && Identifier(a.RequestID) && Timestamp(a.At)
}

type Admission struct {
	ID, ConversationID             string
	Actor                          Actor
	IdempotencyDigest, Fingerprint [32]byte
	Goal                           Goal
	SourceID                       string
	SourceVersion                  int64
	ResourceID                     string
	From, Until                    time.Time
	RestoreGeneration              int64
	PolicyVersion                  int64
	ProfileID                      string
	ProfileVersion                 int64
	Volatile                       bool
	MemoryOwner                    string
	Paid                           bool
	ReservedUnits                  int64
	At, Deadline                   time.Time
	// RequireExisting is a trusted retry guard, never material or stored metadata.
	RequireExisting bool `json:"-"`
}

func (a Admission) Valid() bool {
	return Identifier(a.ID) && Identifier(a.ConversationID) && a.Actor.Valid() && Digest(a.IdempotencyDigest) && Digest(a.Fingerprint) && a.Goal.Valid() && Identifier(a.SourceID) && a.SourceVersion > 0 && (a.ResourceID == "" || Identifier(a.ResourceID)) && Timestamp(a.From) && Timestamp(a.Until) && !a.Until.Before(a.From) && a.Until.Sub(a.From) <= 24*time.Hour && a.RestoreGeneration >= 0 && a.PolicyVersion > 0 && Identifier(a.ProfileID) && a.ProfileVersion > 0 && (!a.Volatile || Identifier(a.MemoryOwner)) && a.ReservedUnits > 0 && a.ReservedUnits <= MaximumReservationUnits && Timestamp(a.At) && Timestamp(a.Deadline) && a.Deadline.After(a.At) && a.Deadline.Sub(a.At) <= MaximumRunDuration
}

type Run struct {
	ID, ConversationID, OwnerID, SourceID, ResourceID               string
	Goal                                                            Goal
	State                                                           State
	SourceVersion, RestoreGeneration, PolicyVersion, ProfileVersion int64
	ProfileID                                                       string
	Volatile, Paid                                                  bool
	UsageUncertain                                                  bool
	UsageConservative                                               bool
	Version, Fence                                                  int64
	ConfigurationVersion                                            int64
	Attempts                                                        int
	LeaseOwner                                                      string
	LeaseUntil                                                      time.Time
	ToolCalls, InferenceCalls                                       int
	ReservedUnits, UsedUnits                                        int64
	CreatedAt, UpdatedAt, Deadline                                  time.Time
	From, Until                                                     time.Time
}

type Claim struct {
	Worker, ProcessID string
	At                time.Time
	Lease             time.Duration
}
type Lease struct {
	RunID, Worker  string
	Version, Fence int64
	At             time.Time
}

func (l Lease) Valid() bool {
	return Identifier(l.RunID) && Identifier(l.Worker) && l.Version > 0 && l.Fence > 0 && Timestamp(l.At)
}

type Step struct {
	Number                int
	Kind                  StepKind
	State                 string
	Units                 int64
	MaximumUnits          int64
	StartedAt, FinishedAt time.Time
}
type StepStart struct {
	Lease        Lease
	Kind         StepKind
	MaximumUnits int64
}
type StepFinish struct {
	Lease     Lease
	Number    int
	Units     int64
	Uncertain bool
}
type Finish struct {
	Lease Lease
	State State
}
type Event struct {
	Sequence, RunVersion int64
	State                State
	At                   time.Time
}
type UsageSettlement struct {
	RunID         string
	Units         int64
	Conservative  bool
	ReceiptDigest [32]byte
	Audit         OperatorAudit
}
type RestoreSettlement struct {
	Generation    int64
	ExposureUnits int64
	ReceiptDigest [32]byte
	Audit         OperatorAudit
}

// AdmissionResult distinguishes a newly committed run from an idempotent reuse.
type AdmissionResult struct {
	Run     Run
	Created bool
}

// AdmissionRepository returns ownership of a new admission only after commit.
type AdmissionRepository interface {
	AdmitAssistantWithOutcome(context.Context, Admission) (AdmissionResult, error)
}

// RuntimeRepository owns complete transactions; callers perform network I/O only between calls.
type RuntimeRepository interface {
	ConfigureAssistant(context.Context, Settings, OperatorAudit) (Settings, error)
	AdmitAssistant(context.Context, Admission) (Run, error)
	ClaimAssistant(context.Context, Claim) (Run, bool, error)
	RenewAssistantLease(context.Context, Lease, time.Duration) (Run, error)
	BeginAssistantStep(context.Context, StepStart) (Run, Step, error)
	FinishAssistantStep(context.Context, StepFinish) (Run, error)
	FinishAssistant(context.Context, Finish) (Run, error)
	CancelAssistant(context.Context, Actor, string, time.Time) (Run, error)
	GetAssistant(context.Context, Actor, string, time.Time) (Run, error)
	AssistantEvents(context.Context, Actor, string, int64, int, time.Time) ([]Event, error)
	RecoverAssistant(context.Context, time.Time, int) (int, error)
	PruneAssistant(context.Context, time.Time, int) (int, error)
	SettleAssistantUsage(context.Context, UsageSettlement) error
	ResetAssistantQuota(context.Context, OperatorAudit) error
	SettleAssistantRestore(context.Context, RestoreSettlement) error
}

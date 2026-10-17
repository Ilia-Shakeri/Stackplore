package operation

import (
	"context"
	"errors"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const ConfirmationLifetime = 2 * time.Minute
const MaximumActiveExecutions = 64
const MaximumUserExecutions = 4

var (
	ErrDisabled            = errors.New("infrastructure operations are disabled")
	ErrForbidden           = errors.New("infrastructure operation is forbidden")
	ErrInvalid             = errors.New("infrastructure operation is invalid")
	ErrStale               = errors.New("infrastructure resource version changed")
	ErrConfirmation        = errors.New("infrastructure operation confirmation is invalid")
	ErrIdempotencyConflict = errors.New("infrastructure operation idempotency conflict")
	ErrInProgress          = errors.New("infrastructure operation is already in progress")
	ErrUnavailable         = errors.New("infrastructure operation is unavailable")
	ErrProviderFailure     = errors.New("infrastructure provider rejected operation")
	ErrUncertain           = errors.New("infrastructure operation result is uncertain")
	ErrNotFound            = errors.New("infrastructure operation not found")
	ErrAdmission           = errors.New("infrastructure operation capacity reached")
)

type ConfirmationDigest [32]byte
type Confirmation struct {
	ID                    string
	Digest                ConfirmationDigest
	UserID                auth.UserID
	SessionID             auth.SessionID
	Request               domain.OperationRequest
	ParametersFingerprint [32]byte
	CreatedAt, ExpiresAt  time.Time
}

func (c Confirmation) Validate() error {
	if !validToken(c.ID) || allZero(c.Digest[:]) || c.UserID.Validate() != nil || c.SessionID.Validate() != nil || c.Request.Validate() != nil || c.ParametersFingerprint != requestFingerprint(c.Request) || !validTime(c.CreatedAt) || !validTime(c.ExpiresAt) || c.ExpiresAt.Sub(c.CreatedAt) != ConfirmationLifetime {
		return ErrInvalid
	}
	return nil
}

type ConfirmationIssue struct {
	Impact    domain.OperationImpact
	Proof     string
	ExpiresAt time.Time
}

func (ConfirmationIssue) String() string { return "[protected operation preview]" }

type ExecutionState string

const (
	ExecutionInProgress ExecutionState = "in_progress"
	ExecutionSucceeded  ExecutionState = "succeeded"
	ExecutionFailed     ExecutionState = "failed"
	ExecutionUncertain  ExecutionState = "uncertain"
)

type ExecutionRecord struct {
	ID                               string
	IdempotencyDigest                [32]byte
	ConfirmationID                   string
	UserID                           auth.UserID
	SessionID                        auth.SessionID
	Request                          domain.OperationRequest
	RequestID                        domain.RequestID
	State                            ExecutionState
	Result                           *domain.OperationProviderResult
	ReasonCode                       domain.ReasonCode
	CreatedAt, UpdatedAt, LeaseUntil time.Time
}

func (r ExecutionRecord) Validate() error {
	if !validToken(r.ID) || allZero(r.IdempotencyDigest[:]) || !validToken(r.ConfirmationID) || r.UserID.Validate() != nil || r.SessionID.Validate() != nil || r.Request.Validate() != nil || r.RequestID.Validate() != nil || !validTime(r.CreatedAt) || !validTime(r.UpdatedAt) || r.UpdatedAt.Before(r.CreatedAt) || !validTime(r.LeaseUntil) || !r.LeaseUntil.After(r.CreatedAt) || r.LeaseUntil.Sub(r.CreatedAt) > 2*time.Minute {
		return ErrInvalid
	}
	switch r.State {
	case ExecutionInProgress:
		if r.Result != nil || r.ReasonCode != "" || r.UpdatedAt != r.CreatedAt {
			return ErrInvalid
		}
	case ExecutionSucceeded:
		if r.Result == nil || r.Result.ValidateFor(domain.OperationExecution{Request: r.Request, OperationID: r.ID}) != nil || r.ReasonCode != "operation.succeeded" {
			return ErrInvalid
		}
	case ExecutionFailed, ExecutionUncertain:
		if r.Result != nil || r.ReasonCode.Validate() != nil {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

type ExecutionPage struct {
	Executions []ExecutionRecord
	Page       domain.PageInfo
}
type StartExecutionCommand struct {
	Actor                 auth.Principal
	Digest                ConfirmationDigest
	IdempotencyDigest     [32]byte
	Fingerprint           [32]byte
	ExecutionID           string
	Request               domain.OperationRequest
	RequestID             domain.RequestID
	Grant                 auth.RecentGrantUse
	CreatedAt, LeaseUntil time.Time
	AuditIntent           domain.AuditEntry
}

func (c StartExecutionCommand) Validate() error {
	if c.Actor.Validate() != nil || c.Actor.Role != auth.RoleAdministrator || allZero(c.Digest[:]) || allZero(c.IdempotencyDigest[:]) || c.Fingerprint != requestFingerprint(c.Request) || !validToken(c.ExecutionID) || c.Request.Validate() != nil || c.RequestID.Validate() != nil || !validTime(c.CreatedAt) || !validTime(c.LeaseUntil) || !c.LeaseUntil.After(c.CreatedAt) || c.LeaseUntil.Sub(c.CreatedAt) > 2*time.Minute || c.Grant.Actor != c.Actor || c.Grant.Purpose != auth.RecentAuthPurposeOperationExecute || c.Grant.Digest.Validate() != nil || !validTime(c.Grant.ConsumedAt) || !validAudit(c.AuditIntent, c.Actor.UserID, c.Request, c.RequestID, domain.AuditIntent, "operation.requested", c.CreatedAt) {
		return ErrInvalid
	}
	return nil
}

type CompletionCommand struct {
	ExecutionID string
	State       ExecutionState
	Result      *domain.OperationProviderResult
	ReasonCode  domain.ReasonCode
	CompletedAt time.Time
	AuditResult domain.AuditEntry
}

func (c CompletionCommand) ValidateFor(r ExecutionRecord) error {
	if c.ExecutionID != r.ID || r.State != ExecutionInProgress || !validTime(c.CompletedAt) || c.CompletedAt.Before(r.CreatedAt) {
		return ErrInvalid
	}
	outcome := domain.AuditFailure
	switch c.State {
	case ExecutionSucceeded:
		outcome = domain.AuditSuccess
		if c.Result == nil || c.ReasonCode != "operation.succeeded" || c.Result.ValidateFor(domain.OperationExecution{Request: r.Request, OperationID: r.ID}) != nil {
			return ErrInvalid
		}
	case ExecutionUncertain:
		outcome = domain.AuditUncertain
		if c.Result != nil {
			return ErrInvalid
		}
	case ExecutionFailed:
		if c.Result != nil {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	if !validAudit(c.AuditResult, r.UserID, r.Request, r.RequestID, outcome, c.ReasonCode, c.CompletedAt) {
		return ErrInvalid
	}
	return nil
}
func validAudit(a domain.AuditEntry, user auth.UserID, request domain.OperationRequest, id domain.RequestID, outcome domain.AuditOutcome, reason domain.ReasonCode, at time.Time) bool {
	return a.Validate() == nil && a.Actor.Kind == domain.AuditActorUser && a.Actor.ID == string(user) && a.Action == "infrastructure.operation" && a.Target.Kind == "infrastructure.resource" && a.Target.ID == string(request.Resource.ID) && a.Outcome == outcome && a.RequestID == id && a.ReasonCode == reason && a.OccurredAt == at
}

type Repository interface {
	CreateConfirmation(context.Context, auth.Principal, Confirmation) error
	FindExecution(context.Context, auth.Principal, [32]byte, domain.OperationRequest) (ExecutionRecord, bool, error)
	StartExecution(context.Context, StartExecutionCommand) (ExecutionRecord, bool, error)
	CompleteExecution(context.Context, CompletionCommand) (ExecutionRecord, error)
	GetExecution(context.Context, auth.Principal, string) (ExecutionRecord, error)
	ListExecutions(context.Context, auth.Principal, domain.SourceID, domain.PageRequest) (ExecutionPage, error)
	AppendOperationDenial(context.Context, auth.Principal, domain.AuditEntry) error
	RecoverOrphanedOperations(context.Context, time.Time, int) (int, error)
}
type ProviderRegistration struct {
	Source       domain.ProviderRef
	Provider     domain.OperationProvider
	WriteEnabled bool
}

func validTime(v time.Time) bool {
	return !v.IsZero() && v.Location() == time.UTC && v.Nanosecond()%int(time.Microsecond) == 0 && v.Year() >= 1970 && v.Year() <= 9999
}
func validToken(v string) bool { return domain.RequestID(v).Validate() == nil }
func allZero(v []byte) bool {
	for _, b := range v {
		if b != 0 {
			return false
		}
	}
	return true
}

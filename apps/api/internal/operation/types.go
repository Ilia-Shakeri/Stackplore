package operation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

const ConfirmationLifetime = 2 * time.Minute

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
)

type ConfirmationDigest [32]byte

type Confirmation struct {
	ID                    string
	Digest                ConfirmationDigest
	UserID                auth.UserID
	SessionID             auth.SessionID
	Request               domain.OperationRequest
	ParametersFingerprint [32]byte
	CreatedAt             time.Time
	ExpiresAt             time.Time
}

func (confirmation Confirmation) Validate() error {
	if !validToken(confirmation.ID) || allZero(confirmation.Digest[:]) || confirmation.UserID.Validate() != nil ||
		confirmation.SessionID.Validate() != nil || confirmation.Request.Validate() != nil ||
		allZero(confirmation.ParametersFingerprint[:]) || !validTime(confirmation.CreatedAt) ||
		!validTime(confirmation.ExpiresAt) || !confirmation.ExpiresAt.After(confirmation.CreatedAt) ||
		confirmation.ExpiresAt.Sub(confirmation.CreatedAt) > ConfirmationLifetime {
		return ErrInvalid
	}
	return nil
}

type ConfirmationIssue struct {
	Impact    domain.OperationImpact
	Proof     string
	ExpiresAt time.Time
}

type ExecutionState string

const (
	ExecutionInProgress ExecutionState = "in_progress"
	ExecutionSucceeded  ExecutionState = "succeeded"
	ExecutionFailed     ExecutionState = "failed"
	ExecutionUncertain  ExecutionState = "uncertain"
)

type ExecutionRecord struct {
	ID             string
	IdempotencyKey string
	ConfirmationID string
	UserID         auth.UserID
	SessionID      auth.SessionID
	Request        domain.OperationRequest
	RequestID      domain.RequestID
	State          ExecutionState
	Result         *domain.OperationProviderResult
	ReasonCode     domain.ReasonCode
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (record ExecutionRecord) Validate() error {
	if !validToken(record.ID) || !validToken(record.IdempotencyKey) || !validToken(record.ConfirmationID) ||
		record.UserID.Validate() != nil || record.SessionID.Validate() != nil || record.Request.Validate() != nil ||
		record.RequestID.Validate() != nil || !validTime(record.CreatedAt) || !validTime(record.UpdatedAt) ||
		record.UpdatedAt.Before(record.CreatedAt) {
		return ErrInvalid
	}
	switch record.State {
	case ExecutionInProgress:
		if record.Result != nil || record.ReasonCode != "" || record.UpdatedAt != record.CreatedAt {
			return ErrInvalid
		}
	case ExecutionSucceeded:
		if record.Result == nil || record.Result.ValidateFor(domain.OperationExecution{Request: record.Request, IdempotencyKey: record.IdempotencyKey}) != nil ||
			record.ReasonCode != "operation.succeeded" {
			return ErrInvalid
		}
	case ExecutionFailed, ExecutionUncertain:
		if record.Result != nil || record.ReasonCode.Validate() != nil {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

type StartExecutionCommand struct {
	Digest      ConfirmationDigest
	Record      ExecutionRecord
	Fingerprint [32]byte
	ConsumedAt  time.Time
	AuditIntent domain.AuditEntry
}

func (command StartExecutionCommand) Validate() error {
	if allZero(command.Digest[:]) || command.Record.Validate() != nil || command.Record.State != ExecutionInProgress ||
		allZero(command.Fingerprint[:]) || !validTime(command.ConsumedAt) || command.ConsumedAt.Before(command.Record.CreatedAt) ||
		command.AuditIntent.Validate() != nil || command.AuditIntent.Actor.Kind != domain.AuditActorUser ||
		command.AuditIntent.Actor.ID != string(command.Record.UserID) || command.AuditIntent.Action != "infrastructure.operation" ||
		command.AuditIntent.Target.Kind != "infrastructure.resource" || command.AuditIntent.Target.ID != string(command.Record.Request.Resource.ID) ||
		command.AuditIntent.Outcome != domain.AuditIntent || command.AuditIntent.RequestID != command.Record.RequestID ||
		command.AuditIntent.ReasonCode != "operation.requested" || command.AuditIntent.OccurredAt != command.Record.CreatedAt {
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

func (command CompletionCommand) ValidateFor(record ExecutionRecord) error {
	if !validToken(command.ExecutionID) || command.ExecutionID != record.ID || record.State != ExecutionInProgress ||
		!validTime(command.CompletedAt) || command.CompletedAt.Before(record.CreatedAt) || command.AuditResult.Validate() != nil ||
		command.AuditResult.Actor.Kind != domain.AuditActorUser || command.AuditResult.Actor.ID != string(record.UserID) ||
		command.AuditResult.Action != "infrastructure.operation" || command.AuditResult.Target.Kind != "infrastructure.resource" ||
		command.AuditResult.Target.ID != string(record.Request.Resource.ID) || command.AuditResult.RequestID != record.RequestID ||
		command.AuditResult.OccurredAt != command.CompletedAt || command.AuditResult.ReasonCode != command.ReasonCode {
		return ErrInvalid
	}
	switch command.State {
	case ExecutionSucceeded:
		if command.Result == nil || command.Result.ValidateFor(domain.OperationExecution{Request: record.Request, IdempotencyKey: record.IdempotencyKey}) != nil ||
			command.ReasonCode != "operation.succeeded" || command.AuditResult.Outcome != domain.AuditSuccess {
			return ErrInvalid
		}
	case ExecutionFailed:
		if command.Result != nil || command.ReasonCode.Validate() != nil || command.AuditResult.Outcome != domain.AuditFailure {
			return ErrInvalid
		}
	case ExecutionUncertain:
		if command.Result != nil || command.ReasonCode.Validate() != nil || command.AuditResult.Outcome != domain.AuditUncertain {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

type Repository interface {
	CreateConfirmation(context.Context, Confirmation) error
	StartExecution(context.Context, StartExecutionCommand) (ExecutionRecord, bool, error)
	CompleteExecution(context.Context, CompletionCommand) (ExecutionRecord, error)
}

type ProviderRegistration struct {
	Source       domain.ProviderRef
	Provider     domain.OperationProvider
	WriteEnabled bool
}

func validTime(value time.Time) bool {
	return !value.IsZero() && value.Location() == time.UTC && value.Nanosecond()%int(time.Microsecond) == 0 && value.UnixMicro() >= 0
}

func validToken(value string) bool {
	return domain.RequestID(value).Validate() == nil
}

func allZero(value []byte) bool {
	for _, item := range value {
		if item != 0 {
			return false
		}
	}
	return true
}

func errorf(message string) error { return fmt.Errorf("%w: %s", ErrInvalid, message) }

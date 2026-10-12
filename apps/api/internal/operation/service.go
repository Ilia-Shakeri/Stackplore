package operation

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type ServiceOptions struct {
	Repository       Repository
	Providers        []ProviderRegistration
	MutationsEnabled bool
	DigestKey        []byte
	Now              func() time.Time
	ProviderTimeout  time.Duration
}

type Service struct {
	repository       Repository
	providers        map[domain.SourceID]ProviderRegistration
	mutationsEnabled bool
	digestKey        [32]byte
	now              func() time.Time
	providerTimeout  time.Duration
}

func NewService(options ServiceOptions) (*Service, error) {
	if options.Repository == nil || len(options.Providers) == 0 || len(options.DigestKey) != 32 || options.Now == nil ||
		options.ProviderTimeout < time.Second || options.ProviderTimeout > time.Minute {
		return nil, ErrInvalid
	}
	providers := make(map[domain.SourceID]ProviderRegistration, len(options.Providers))
	for _, registration := range options.Providers {
		if registration.Source.Validate() != nil || registration.Provider == nil {
			return nil, ErrInvalid
		}
		if _, exists := providers[registration.Source.SourceID]; exists {
			return nil, ErrInvalid
		}
		providers[registration.Source.SourceID] = registration
	}
	service := &Service{
		repository: options.Repository, providers: providers, mutationsEnabled: options.MutationsEnabled,
		now: options.Now, providerTimeout: options.ProviderTimeout,
	}
	copy(service.digestKey[:], options.DigestKey)
	return service, nil
}

func (service *Service) Preview(
	ctx context.Context,
	principal auth.Principal,
	request domain.OperationRequest,
) (ConfirmationIssue, error) {
	registration, err := service.authorize(ctx, principal, request)
	if err != nil {
		return ConfirmationIssue{}, err
	}
	impact, err := registration.Provider.PreviewOperation(ctx, request)
	if err != nil {
		return ConfirmationIssue{}, mapProviderError(err)
	}
	if impact.Validate() != nil || impact.Request != request {
		return ConfirmationIssue{}, ErrUnavailable
	}
	now, err := service.currentTime()
	if err != nil {
		return ConfirmationIssue{}, err
	}
	proof, err := auth.GenerateOpaqueToken()
	if err != nil {
		return ConfirmationIssue{}, ErrUnavailable
	}
	encoded, err := proof.Encode()
	if err != nil {
		return ConfirmationIssue{}, ErrUnavailable
	}
	digest := service.digestConfirmation(encoded)
	confirmationID, err := secureIdentifier("confirmation_")
	if err != nil {
		return ConfirmationIssue{}, err
	}
	confirmation := Confirmation{
		ID: confirmationID, Digest: digest, UserID: principal.UserID, SessionID: principal.SessionID,
		Request: request, ParametersFingerprint: requestFingerprint(request),
		CreatedAt: now, ExpiresAt: now.Add(ConfirmationLifetime),
	}
	if confirmation.Validate() != nil {
		return ConfirmationIssue{}, ErrUnavailable
	}
	if err := service.repository.CreateConfirmation(ctx, confirmation); err != nil {
		return ConfirmationIssue{}, ErrUnavailable
	}
	return ConfirmationIssue{Impact: impact, Proof: encoded, ExpiresAt: confirmation.ExpiresAt}, nil
}

func (service *Service) Execute(
	ctx context.Context,
	principal auth.Principal,
	request domain.OperationRequest,
	confirmationProof string,
	idempotencyKey string,
	requestID domain.RequestID,
) (ExecutionRecord, error) {
	registration, err := service.authorize(ctx, principal, request)
	if err != nil || !validToken(idempotencyKey) || requestID.Validate() != nil {
		if err != nil {
			return ExecutionRecord{}, err
		}
		return ExecutionRecord{}, ErrInvalid
	}
	proof, err := auth.ParseOpaqueToken(confirmationProof)
	if err != nil {
		return ExecutionRecord{}, ErrConfirmation
	}
	encoded, err := proof.Encode()
	if err != nil {
		return ExecutionRecord{}, ErrConfirmation
	}
	now, err := service.currentTime()
	if err != nil {
		return ExecutionRecord{}, err
	}
	executionID, err := secureIdentifier("operation_")
	if err != nil {
		return ExecutionRecord{}, err
	}
	auditID, err := secureIdentifier("audit_")
	if err != nil {
		return ExecutionRecord{}, err
	}
	record := ExecutionRecord{
		ID: executionID, IdempotencyKey: idempotencyKey, UserID: principal.UserID, SessionID: principal.SessionID,
		Request: request, RequestID: requestID, State: ExecutionInProgress, CreatedAt: now, UpdatedAt: now,
	}
	command := StartExecutionCommand{
		Digest: service.digestConfirmation(encoded), Record: record, Fingerprint: requestFingerprint(request), ConsumedAt: now,
		AuditIntent: operationAudit(auditID, principal.UserID, request, requestID, domain.AuditIntent, "operation.requested", now),
	}
	startedRecord, started, err := service.repository.StartExecution(ctx, command)
	if err != nil {
		if errors.Is(err, ErrConfirmation) || errors.Is(err, ErrIdempotencyConflict) {
			return ExecutionRecord{}, err
		}
		return ExecutionRecord{}, ErrUnavailable
	}
	if !started {
		return replayResult(startedRecord)
	}
	providerContext, cancel := context.WithTimeout(ctx, service.providerTimeout)
	result, providerErr := registration.Provider.ExecuteOperation(providerContext, domain.OperationExecution{
		Request: request, IdempotencyKey: idempotencyKey,
	})
	deadline := errors.Is(providerContext.Err(), context.DeadlineExceeded) || errors.Is(providerErr, context.DeadlineExceeded)
	cancel()
	completedAt, clockErr := service.currentTime()
	if clockErr != nil {
		return ExecutionRecord{}, ErrUncertain
	}
	completion := CompletionCommand{ExecutionID: startedRecord.ID, CompletedAt: completedAt}
	auditResultID, idErr := secureIdentifier("audit_")
	if idErr != nil {
		return ExecutionRecord{}, ErrUncertain
	}
	if providerErr == nil && result.ValidateFor(domain.OperationExecution{Request: request, IdempotencyKey: idempotencyKey}) == nil {
		completion.State, completion.Result, completion.ReasonCode = ExecutionSucceeded, &result, "operation.succeeded"
		completion.AuditResult = operationAudit(auditResultID, principal.UserID, request, requestID, domain.AuditSuccess, completion.ReasonCode, completedAt)
	} else if deadline {
		completion.State, completion.ReasonCode = ExecutionUncertain, "operation.timeout_uncertain"
		completion.AuditResult = operationAudit(auditResultID, principal.UserID, request, requestID, domain.AuditUncertain, completion.ReasonCode, completedAt)
	} else {
		completion.State, completion.ReasonCode = ExecutionFailed, "operation.provider_failed"
		completion.AuditResult = operationAudit(auditResultID, principal.UserID, request, requestID, domain.AuditFailure, completion.ReasonCode, completedAt)
	}
	completed, err := service.repository.CompleteExecution(ctx, completion)
	if err != nil {
		return ExecutionRecord{}, ErrUncertain
	}
	if completion.State == ExecutionSucceeded {
		return completed, nil
	}
	if completion.State == ExecutionUncertain {
		return completed, ErrUncertain
	}
	return completed, ErrProviderFailure
}

func (service *Service) authorize(ctx context.Context, principal auth.Principal, request domain.OperationRequest) (ProviderRegistration, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || principal.Validate() != nil || request.Validate() != nil {
		return ProviderRegistration{}, ErrInvalid
	}
	if !service.mutationsEnabled {
		return ProviderRegistration{}, ErrDisabled
	}
	if principal.Role != auth.RoleAdministrator || request.Source.Provider == domain.ProviderDemo {
		return ProviderRegistration{}, ErrForbidden
	}
	registration, exists := service.providers[request.Source.SourceID]
	if !exists || registration.Source != request.Source || !registration.WriteEnabled {
		return ProviderRegistration{}, ErrDisabled
	}
	return registration, nil
}

func (service *Service) currentTime() (time.Time, error) {
	now := service.now().UTC().Truncate(time.Microsecond)
	if !validTime(now) {
		return time.Time{}, ErrUnavailable
	}
	return now, nil
}

func (service *Service) digestConfirmation(encoded string) ConfirmationDigest {
	mac := hmac.New(sha256.New, service.digestKey[:])
	_, _ = mac.Write([]byte("stackplore/operation-confirmation/v1\x00"))
	_, _ = mac.Write([]byte(encoded))
	var digest ConfirmationDigest
	copy(digest[:], mac.Sum(nil))
	return digest
}

func requestFingerprint(request domain.OperationRequest) [32]byte {
	replicas := ""
	if request.Parameters.Replicas != nil {
		replicas = strconv.Itoa(*request.Parameters.Replicas)
	}
	return sha256.Sum256([]byte(fmt.Sprintf(
		"stackplore-operation-request/v1\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s",
		request.Source.Provider, request.Source.SourceID, request.Resource.ID, request.Action, request.ExpectedVersion, replicas,
	)))
}

func secureIdentifier(prefix string) (string, error) {
	randomBytes := make([]byte, 18)
	defer clear(randomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", ErrUnavailable
	}
	return prefix + base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func operationAudit(
	id string,
	userID auth.UserID,
	request domain.OperationRequest,
	requestID domain.RequestID,
	outcome domain.AuditOutcome,
	reason domain.ReasonCode,
	at time.Time,
) domain.AuditEntry {
	return domain.AuditEntry{
		ID: domain.AuditEntryID(id), OccurredAt: at,
		Actor:  domain.AuditActor{Kind: domain.AuditActorUser, ID: string(userID)},
		Action: "infrastructure.operation", Target: domain.AuditTarget{Kind: "infrastructure.resource", ID: string(request.Resource.ID)},
		Outcome: outcome, RequestID: requestID, ReasonCode: reason,
	}
}

func mapProviderError(err error) error {
	var providerError *domain.ProviderError
	if errors.As(err, &providerError) {
		switch providerError.Code {
		case domain.ProviderErrorInvalidRequest:
			return ErrStale
		case domain.ProviderErrorPermissionDenied, domain.ProviderErrorUnsupported:
			return ErrForbidden
		case domain.ProviderErrorDeadlineExceeded:
			return ErrUncertain
		}
	}
	return ErrUnavailable
}

func replayResult(record ExecutionRecord) (ExecutionRecord, error) {
	if record.Validate() != nil {
		return ExecutionRecord{}, ErrUnavailable
	}
	switch record.State {
	case ExecutionSucceeded:
		return record, nil
	case ExecutionFailed:
		return record, ErrProviderFailure
	case ExecutionUncertain:
		return record, ErrUncertain
	case ExecutionInProgress:
		return record, ErrInProgress
	default:
		return ExecutionRecord{}, ErrUnavailable
	}
}

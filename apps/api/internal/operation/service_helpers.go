package operation

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"strconv"
	"time"
)

func (service *Service) digestConfirmation(encoded string) ConfirmationDigest {
	mac := hmac.New(sha256.New, service.digestKey[:])
	_, _ = mac.Write([]byte("stackplore/operation-confirmation/v1\x00"))
	_, _ = mac.Write([]byte(encoded))
	var digest ConfirmationDigest
	copy(digest[:], mac.Sum(nil))
	return digest
}

func (service *Service) digestIdempotency(value string) [32]byte {
	mac := hmac.New(sha256.New, service.digestKey[:])
	_, _ = mac.Write([]byte("stackplore/operation-idempotency/v1\x00"))
	_, _ = mac.Write([]byte(value))
	var digest [32]byte
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

func RequestFingerprint(request domain.OperationRequest) [32]byte { return requestFingerprint(request) }

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

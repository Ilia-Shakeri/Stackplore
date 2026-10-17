package operation

import (
	"context"
	"errors"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"time"
)

type ServiceOptions struct {
	Repository       Repository
	Providers        []ProviderRegistration
	MutationsEnabled bool
	DigestKey        []byte
	Now              func() time.Time
	ProviderTimeout  time.Duration
	Lifetime         context.Context
}
type Service struct {
	repository       Repository
	providers        map[domain.SourceID]ProviderRegistration
	mutationsEnabled bool
	digestKey        [32]byte
	now              func() time.Time
	providerTimeout  time.Duration
	lifetime         context.Context
}

func NewService(o ServiceOptions) (*Service, error) {
	if o.Repository == nil || len(o.Providers) > 100 || len(o.DigestKey) != 32 || o.Now == nil || o.ProviderTimeout < time.Second || o.ProviderTimeout > 30*time.Second {
		return nil, ErrInvalid
	}
	p := map[domain.SourceID]ProviderRegistration{}
	for _, r := range o.Providers {
		if r.Source.Validate() != nil || r.Source.Provider == domain.ProviderDemo || r.Provider == nil {
			return nil, ErrInvalid
		}
		if _, exists := p[r.Source.SourceID]; exists {
			return nil, ErrInvalid
		}
		p[r.Source.SourceID] = r
	}
	if o.Lifetime == nil {
		o.Lifetime = context.Background()
	}
	s := &Service{repository: o.Repository, providers: p, mutationsEnabled: o.MutationsEnabled, now: o.Now, providerTimeout: o.ProviderTimeout, lifetime: o.Lifetime}
	copy(s.digestKey[:], o.DigestKey)
	return s, nil
}
func (s *Service) Preview(ctx context.Context, actor auth.Principal, request domain.OperationRequest, requestID domain.RequestID) (ConfirmationIssue, error) {
	r, err := s.authorize(ctx, actor, request)
	if err != nil {
		return ConfirmationIssue{}, s.deny(ctx, actor, request.Resource.ID, requestID, err)
	}
	if requestID.Validate() != nil {
		return ConfirmationIssue{}, ErrInvalid
	}
	read, cancel := context.WithTimeout(ctx, min(s.providerTimeout, 5*time.Second))
	impact, err := r.Provider.PreviewOperation(read, request)
	cancel()
	if err != nil {
		return ConfirmationIssue{}, s.deny(ctx, actor, request.Resource.ID, requestID, mapProviderError(err))
	}
	if impact.Validate() != nil || requestFingerprint(impact.Request) != requestFingerprint(request) {
		return ConfirmationIssue{}, s.deny(ctx, actor, request.Resource.ID, requestID, ErrUnavailable)
	}
	at, err := s.currentTime()
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
	id, err := secureIdentifier("confirmation_")
	if err != nil {
		return ConfirmationIssue{}, err
	}
	c := Confirmation{ID: id, Digest: s.digestConfirmation(encoded), UserID: actor.UserID, SessionID: actor.SessionID, Request: request, ParametersFingerprint: requestFingerprint(request), CreatedAt: at, ExpiresAt: at.Add(ConfirmationLifetime)}
	if c.Validate() != nil {
		return ConfirmationIssue{}, ErrUnavailable
	}
	if err = s.repository.CreateConfirmation(ctx, actor, c); err != nil {
		return ConfirmationIssue{}, s.deny(ctx, actor, request.Resource.ID, requestID, err)
	}
	return ConfirmationIssue{Impact: impact, Proof: encoded, ExpiresAt: c.ExpiresAt}, nil
}
func (s *Service) Execute(ctx context.Context, actor auth.Principal, request domain.OperationRequest, confirmationProof, idempotencyKey string, grant auth.RecentGrantUse, requestID domain.RequestID) (ExecutionRecord, error) {
	if s == nil || ctx == nil || actor.Validate() != nil || requestID.Validate() != nil {
		return ExecutionRecord{}, ErrInvalid
	}
	if request.Validate() != nil || !validToken(idempotencyKey) {
		return ExecutionRecord{}, s.deny(ctx, actor, request.Resource.ID, requestID, ErrInvalid)
	}
	if actor.Role != auth.RoleAdministrator {
		return ExecutionRecord{}, s.deny(ctx, actor, request.Resource.ID, requestID, ErrForbidden)
	}
	key := s.digestIdempotency(idempotencyKey)
	prior, exists, err := s.repository.FindExecution(ctx, actor, key, request)
	if err != nil {
		return ExecutionRecord{}, s.deny(ctx, actor, request.Resource.ID, requestID, err)
	}
	if exists {
		return replayResult(prior)
	}
	r, err := s.authorize(ctx, actor, request)
	if err != nil {
		return ExecutionRecord{}, s.deny(ctx, actor, request.Resource.ID, requestID, err)
	}
	proof, err := auth.ParseOpaqueToken(confirmationProof)
	if err != nil {
		return ExecutionRecord{}, s.deny(ctx, actor, request.Resource.ID, requestID, ErrConfirmation)
	}
	encoded, err := proof.Encode()
	if err != nil {
		return ExecutionRecord{}, ErrConfirmation
	}
	at, err := s.currentTime()
	if err != nil {
		return ExecutionRecord{}, err
	}
	id, err := secureIdentifier("operation_")
	if err != nil {
		return ExecutionRecord{}, err
	}
	auditID, err := secureIdentifier("audit_")
	if err != nil {
		return ExecutionRecord{}, err
	}
	cmd := StartExecutionCommand{Actor: actor, Digest: s.digestConfirmation(encoded), IdempotencyDigest: key, Fingerprint: requestFingerprint(request), ExecutionID: id, Request: request, RequestID: requestID, Grant: grant, CreatedAt: at, LeaseUntil: at.Add(s.providerTimeout + 30*time.Second), AuditIntent: operationAudit(auditID, actor.UserID, request, requestID, domain.AuditIntent, "operation.requested", at)}
	if cmd.Validate() != nil {
		return ExecutionRecord{}, s.deny(ctx, actor, request.Resource.ID, requestID, ErrInvalid)
	}
	record, started, err := s.repository.StartExecution(ctx, cmd)
	if err != nil {
		return ExecutionRecord{}, s.deny(ctx, actor, request.Resource.ID, requestID, err)
	}
	if !started {
		return replayResult(record)
	}
	if record.Validate() != nil {
		return ExecutionRecord{}, ErrUncertain
	}
	// Intent is committed. Client cancellation cannot erase the result audit;
	// service shutdown and a fixed deadline still cancel the provider call.
	call, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.providerTimeout)
	stop := context.AfterFunc(s.lifetime, cancel)
	execution := domain.OperationExecution{Request: request, OperationID: record.ID}
	result, providerErr := r.Provider.ExecuteOperation(call, execution)
	callErr := call.Err()
	stop()
	cancel()
	state, reason := ExecutionUncertain, domain.ReasonCode("operation.outcome_uncertain")
	var accepted *domain.OperationProviderResult
	if providerErr == nil && result.ValidateFor(execution) == nil {
		state, reason, accepted = ExecutionSucceeded, "operation.succeeded", &result
	} else if definiteRejection(providerErr) && callErr == nil {
		state, reason = ExecutionFailed, "operation.provider_rejected"
	}
	if state == ExecutionUncertain {
		if reconciler, ok := r.Provider.(domain.OperationReconciler); ok {
			read, stopRead := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			verified, found, readErr := reconciler.ReconcileOperation(read, execution)
			stopRead()
			if readErr == nil && found && verified.ValidateFor(execution) == nil {
				state, reason, accepted = ExecutionSucceeded, "operation.succeeded", &verified
			}
		}
	}
	done, err := s.currentTime()
	if err != nil {
		return record, ErrUncertain
	}
	if done.Before(record.CreatedAt) {
		done = record.CreatedAt
	}
	auditID, err = secureIdentifier("audit_")
	if err != nil {
		return record, ErrUncertain
	}
	outcome := domain.AuditUncertain
	if state == ExecutionSucceeded {
		outcome = domain.AuditSuccess
	} else if state == ExecutionFailed {
		outcome = domain.AuditFailure
	}
	completion := CompletionCommand{ExecutionID: record.ID, State: state, Result: accepted, ReasonCode: reason, CompletedAt: done, AuditResult: operationAudit(auditID, actor.UserID, request, requestID, outcome, reason, done)}
	finish, stopFinish := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	completed, err := s.repository.CompleteExecution(finish, completion)
	stopFinish()
	if err != nil {
		return record, ErrUncertain
	}
	return replayResult(completed)
}
func definiteRejection(err error) bool {
	if err == nil || errors.Is(err, domain.ErrOperationOutcomeUncertain) {
		return false
	}
	var p *domain.ProviderError
	if errors.As(err, &p) {
		return p.Code == domain.ProviderErrorInvalidRequest || p.Code == domain.ProviderErrorPermissionDenied || p.Code == domain.ProviderErrorUnsupported
	}
	return false
}
func (s *Service) authorize(ctx context.Context, actor auth.Principal, request domain.OperationRequest) (ProviderRegistration, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || actor.Validate() != nil || request.Validate() != nil {
		return ProviderRegistration{}, ErrInvalid
	}
	if actor.Role != auth.RoleAdministrator {
		return ProviderRegistration{}, ErrForbidden
	}
	if !s.mutationsEnabled {
		return ProviderRegistration{}, ErrDisabled
	}
	r, ok := s.providers[request.Source.SourceID]
	if !ok || r.Source != request.Source || !r.WriteEnabled {
		return ProviderRegistration{}, ErrDisabled
	}
	return r, nil
}
func (s *Service) Deny(ctx context.Context, actor auth.Principal, target domain.ResourceID, requestID domain.RequestID, reason domain.ReasonCode) error {
	if s == nil || ctx == nil || actor.Validate() != nil || requestID.Validate() != nil || reason.Validate() != nil {
		return ErrInvalid
	}
	if target.Validate() != nil {
		target = ""
	}
	at, err := s.currentTime()
	if err != nil {
		return err
	}
	id, err := secureIdentifier("audit_")
	if err != nil {
		return err
	}
	entry := domain.AuditEntry{ID: domain.AuditEntryID(id), OccurredAt: at, Actor: domain.AuditActor{Kind: domain.AuditActorUser, ID: string(actor.UserID)}, Action: "infrastructure.operation", Target: domain.AuditTarget{Kind: "infrastructure.resource", ID: string(target)}, Outcome: domain.AuditDenied, RequestID: requestID, ReasonCode: reason}
	write, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if s.repository.AppendOperationDenial(write, actor, entry) != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *Service) deny(ctx context.Context, actor auth.Principal, target domain.ResourceID, id domain.RequestID, err error) error {
	if actor.Validate() != nil {
		return err
	}
	reason := domain.ReasonCode("operation.unavailable")
	switch {
	case errors.Is(err, ErrDisabled):
		reason = "operation.disabled"
	case errors.Is(err, ErrForbidden), errors.Is(err, auth.ErrAuthorizationDenied):
		reason = "operation.forbidden"
	case errors.Is(err, ErrStale):
		reason = "operation.version_conflict"
	case errors.Is(err, ErrConfirmation):
		reason = "operation.confirmation_rejected"
	case errors.Is(err, auth.ErrRecentAuthRejected):
		reason = "operation.recent_auth_rejected"
	case errors.Is(err, ErrAdmission):
		reason = "operation.capacity_reached"
	case errors.Is(err, ErrIdempotencyConflict):
		reason = "operation.idempotency_conflict"
	case errors.Is(err, ErrInProgress):
		reason = "operation.target_busy"
	case errors.Is(err, ErrInvalid):
		reason = "operation.invalid"
	}
	if s.Deny(ctx, actor, target, id, reason) != nil {
		return ErrUnavailable
	}
	return err
}
func (s *Service) Get(ctx context.Context, actor auth.Principal, id string) (ExecutionRecord, error) {
	if s == nil || ctx == nil || actor.Validate() != nil || !validToken(id) {
		return ExecutionRecord{}, ErrInvalid
	}
	if actor.Role != auth.RoleAdministrator {
		return ExecutionRecord{}, ErrForbidden
	}
	return s.repository.GetExecution(ctx, actor, id)
}
func (s *Service) List(ctx context.Context, actor auth.Principal, source domain.SourceID, page domain.PageRequest) (ExecutionPage, error) {
	if s == nil || ctx == nil || actor.Validate() != nil || page.Validate() != nil || page.Limit > 100 || source != "" && source.Validate() != nil {
		return ExecutionPage{}, ErrInvalid
	}
	if actor.Role != auth.RoleAdministrator {
		return ExecutionPage{}, ErrForbidden
	}
	return s.repository.ListExecutions(ctx, actor, source, page)
}
func (s *Service) Recover(ctx context.Context) (int, error) {
	at, err := s.currentTime()
	if err != nil {
		return 0, err
	}
	return s.repository.RecoverOrphanedOperations(ctx, at, 100)
}
func (s *Service) RunRecovery(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		_, _ = s.Recover(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Service) currentTime() (time.Time, error) {
	if s == nil {
		return time.Time{}, ErrUnavailable
	}
	at := s.now().UTC().Truncate(time.Microsecond)
	if !validTime(at) {
		return time.Time{}, ErrUnavailable
	}
	return at, nil
}

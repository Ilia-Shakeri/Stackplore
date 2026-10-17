package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/operation"
	"github.com/go-chi/chi/v5"
)

type OperationManager interface {
	Preview(context.Context, auth.Principal, domain.OperationRequest, domain.RequestID) (operation.ConfirmationIssue, error)
	Execute(context.Context, auth.Principal, domain.OperationRequest, string, string, auth.RecentGrantUse, domain.RequestID) (operation.ExecutionRecord, error)
	Get(context.Context, auth.Principal, string) (operation.ExecutionRecord, error)
	List(context.Context, auth.Principal, domain.SourceID, domain.PageRequest) (operation.ExecutionPage, error)
}

type recentMutationPreparer interface {
	PrepareRecentMutation(context.Context, auth.RecentAuthorizationInput) (auth.Authentication, auth.RecentGrantUse, error)
}

type operationParametersJSON struct {
	Replicas *int `json:"replicas,omitempty"`
}
type operationRequestJSON struct {
	SourceID        domain.SourceID          `json:"source_id"`
	ResourceID      domain.ResourceID        `json:"resource_id"`
	Action          domain.OperationAction   `json:"action"`
	ExpectedVersion string                   `json:"expected_version"`
	Parameters      *operationParametersJSON `json:"parameters"`
}
type operationExecuteJSON struct {
	operationRequestJSON
	ConfirmationToken secretInput `json:"confirmation_token"`
}
type operationPreviewJSON struct {
	operationRequestJSON
	TargetName        string            `json:"target_name"`
	EffectCode        domain.ReasonCode `json:"effect_code"`
	RiskCode          domain.ReasonCode `json:"risk_code"`
	CurrentReplicas   *int              `json:"current_replicas,omitempty"`
	ConfirmationToken string            `json:"confirmation_token"`
	ExpiresAt         string            `json:"expires_at"`
}
type operationResultJSON struct {
	PreviousVersion string            `json:"previous_version"`
	CurrentVersion  string            `json:"current_version"`
	EffectCode      domain.ReasonCode `json:"effect_code"`
}
type operationRecordJSON struct {
	operationRequestJSON
	ID         string                   `json:"id"`
	State      operation.ExecutionState `json:"state"`
	ReasonCode domain.ReasonCode        `json:"reason_code,omitempty"`
	CreatedAt  string                   `json:"created_at"`
	UpdatedAt  string                   `json:"updated_at"`
	Result     *operationResultJSON     `json:"result,omitempty"`
}

func operationRequestDTO(request domain.OperationRequest) operationRequestJSON {
	return operationRequestJSON{SourceID: request.Source.SourceID, ResourceID: request.Resource.ID, Action: request.Action, ExpectedVersion: request.ExpectedVersion, Parameters: &operationParametersJSON{Replicas: request.Parameters.Replicas}}
}
func operationRecordDTO(record operation.ExecutionRecord) operationRecordJSON {
	result := operationRecordJSON{operationRequestJSON: operationRequestDTO(record.Request), ID: record.ID, State: record.State, ReasonCode: record.ReasonCode, CreatedAt: authTimestamp(record.CreatedAt), UpdatedAt: authTimestamp(record.UpdatedAt)}
	if record.Result != nil {
		result.Result = &operationResultJSON{PreviousVersion: record.Result.PreviousVersion, CurrentVersion: record.Result.CurrentVersion, EffectCode: record.Result.EffectCode}
	}
	return result
}

func sameOperationRequest(left, right domain.OperationRequest) bool {
	if left.Source != right.Source || left.Resource != right.Resource || left.Action != right.Action || left.ExpectedVersion != right.ExpectedVersion {
		return false
	}
	if left.Parameters.Replicas == nil || right.Parameters.Replicas == nil {
		return left.Parameters.Replicas == nil && right.Parameters.Replicas == nil
	}
	return *left.Parameters.Replicas == *right.Parameters.Replicas
}

func (handler productHTTP) resolveOperation(r *http.Request, input operationRequestJSON) (domain.OperationRequest, error) {
	if input.SourceID.Validate() != nil || input.ResourceID.Validate() != nil || input.Parameters == nil || input.Action.Validate() != nil || domain.ValidateOperationVersion(input.ExpectedVersion) != nil {
		return domain.OperationRequest{}, operation.ErrInvalid
	}
	provider, ok := handler.providers[input.SourceID]
	if !ok {
		return domain.OperationRequest{}, domain.ErrRepositoryNotFound
	}
	snapshot, err := provider.GetResource(r.Context(), domain.ResourceDetailQuery{Source: providerRef(provider), ResourceID: input.ResourceID})
	if err != nil {
		return domain.OperationRequest{}, err
	}
	if snapshot.ValidateFor(domain.ResourceDetailQuery{Source: providerRef(provider), ResourceID: input.ResourceID}) != nil || snapshot.Resource.Synthetic {
		return domain.OperationRequest{}, operation.ErrInvalid
	}
	request := domain.OperationRequest{Source: providerRef(provider), Resource: snapshot.Resource.Ref, ExpectedVersion: input.ExpectedVersion, Action: input.Action, Parameters: domain.OperationParameters{Replicas: input.Parameters.Replicas}}
	if request.Validate() != nil {
		return domain.OperationRequest{}, operation.ErrInvalid
	}
	return request, nil
}

func (handler productHTTP) previewOperation(w http.ResponseWriter, r *http.Request) {
	identity, token, ok := handler.authorizeAdministratorMutation(w, r)
	if !ok {
		return
	}
	if handler.operations == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	var input operationRequestJSON
	if err := decodeJSONBody(r, min(handler.auth.bodyLimit, 16*1024), &input); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	request, err := handler.resolveOperation(r, input)
	if err != nil {
		handler.writeOperationError(w, r, err)
		return
	}
	issue, err := handler.operations.Preview(r.Context(), identity.Principal, request, requestIDForAuth(r))
	if err != nil {
		handler.writeOperationError(w, r, err)
		return
	}
	if issue.Impact.Validate() != nil || !sameOperationRequest(issue.Impact.Request, request) || issue.Proof == "" {
		writeProblem(w, r, unavailableProblem)
		return
	}
	handler.auth.renewSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, struct {
		Data operationPreviewJSON `json:"data"`
	}{operationPreviewJSON{operationRequestJSON: operationRequestDTO(request), TargetName: issue.Impact.DisplayName, EffectCode: issue.Impact.EffectCode, RiskCode: issue.Impact.RiskCode, CurrentReplicas: issue.Impact.CurrentReplicas, ConfirmationToken: issue.Proof, ExpiresAt: authTimestamp(issue.ExpiresAt)}})
}

func (handler productHTTP) executeOperation(w http.ResponseWriter, r *http.Request) {
	if !handler.auth.requireSameOrigin(w, r) || !requireNoQuery(w, r) {
		return
	}
	token, csrf, recent, failure, ok := readAuthenticationMaterial(r, true, true)
	if !ok {
		writeProblem(w, r, failure)
		return
	}
	preparer, ok := handler.auth.service.(recentMutationPreparer)
	if !ok || handler.operations == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	if _, _, ok := handler.authorizeAdministratorMutation(w, r); !ok {
		return
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || domain.RequestID(keys[0]).Validate() != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	key := keys[0]
	var input operationExecuteJSON
	defer func() { clearSecret(input.ConfirmationToken) }()
	if err := decodeJSONBody(r, min(handler.auth.bodyLimit, 16*1024), &input); err != nil {
		writeBodyDecodeError(w, r, err)
		return
	}
	if len(input.ConfirmationToken) == 0 {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	request, err := handler.resolveOperation(r, input.operationRequestJSON)
	if err != nil {
		handler.writeOperationError(w, r, err)
		return
	}
	identity, grant, err := preparer.PrepareRecentMutation(r.Context(), auth.RecentAuthorizationInput{SessionToken: token, CSRFProof: csrf, RecentProof: recent, Purpose: auth.RecentAuthPurposeOperationExecute, RequestID: requestIDForAuth(r)})
	if err != nil {
		handler.auth.writeAuthenticatedError(w, r, token, err)
		return
	}
	record, err := handler.operations.Execute(r.Context(), identity.Principal, request, string(input.ConfirmationToken), key, grant, requestIDForAuth(r))
	if record.Validate() != nil || !sameOperationRequest(record.Request, request) {
		handler.writeOperationError(w, r, err)
		return
	}
	// A durable receipt remains useful after a failed or uncertain provider call.
	if err != nil && !errors.Is(err, operation.ErrUncertain) && !errors.Is(err, operation.ErrInProgress) && !errors.Is(err, operation.ErrProviderFailure) {
		handler.writeOperationError(w, r, err)
		return
	}
	handler.auth.renewSessionCookie(w, r, token)
	status := http.StatusOK
	if record.State == operation.ExecutionInProgress {
		status = http.StatusAccepted
	}
	writeJSON(w, status, struct {
		Data operationRecordJSON `json:"data"`
	}{operationRecordDTO(record)})
}

func (handler productHTTP) getOperation(w http.ResponseWriter, r *http.Request) {
	if !requireNoBodyAndQuery(w, r) {
		return
	}
	identity, ok := handler.authenticateIdentity(w, r)
	if !ok {
		return
	}
	if identity.Principal.Role != auth.RoleAdministrator {
		writeProblem(w, r, forbiddenProblem)
		return
	}
	id := chi.URLParam(r, "operation_id")
	if domain.RequestID(id).Validate() != nil {
		writeProblem(w, r, notFoundProblem)
		return
	}
	if handler.operations == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	record, err := handler.operations.Get(r.Context(), identity.Principal, id)
	if err != nil {
		handler.writeOperationError(w, r, err)
		return
	}
	if record.Validate() != nil || record.ID != id {
		writeProblem(w, r, unavailableProblem)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Data operationRecordJSON `json:"data"`
	}{operationRecordDTO(record)})
}

func (handler productHTTP) listOperations(w http.ResponseWriter, r *http.Request) {
	if !requireNoBody(w, r) {
		return
	}
	identity, ok := handler.authenticateIdentity(w, r)
	if !ok {
		return
	}
	if identity.Principal.Role != auth.RoleAdministrator {
		writeProblem(w, r, forbiddenProblem)
		return
	}
	page, err := parsePageQuery(r, 25)
	source := domain.SourceID(r.URL.Query().Get("source_id"))
	if err != nil || page.Limit > 100 || source != "" && source.Validate() != nil {
		writeProblem(w, r, invalidRequestProblem)
		return
	}
	if handler.operations == nil {
		writeProblem(w, r, unavailableProblem)
		return
	}
	result, err := handler.operations.List(r.Context(), identity.Principal, source, page)
	if err != nil {
		handler.writeOperationError(w, r, err)
		return
	}
	if result.Page.ValidateFor(page) != nil || len(result.Executions) > page.Limit {
		writeProblem(w, r, unavailableProblem)
		return
	}
	items := make([]operationRecordJSON, len(result.Executions))
	seen := map[string]bool{}
	for index, record := range result.Executions {
		if record.Validate() != nil || source != "" && record.Request.Source.SourceID != source || seen[record.ID] {
			writeProblem(w, r, unavailableProblem)
			return
		}
		seen[record.ID] = true
		items[index] = operationRecordDTO(record)
	}
	writeJSON(w, http.StatusOK, struct {
		Data []operationRecordJSON `json:"data"`
		Page pageResponse          `json:"page"`
	}{Data: items, Page: pageJSON(result.Page)})
}

func (handler productHTTP) writeOperationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, operation.ErrInvalid), errors.Is(err, domain.ErrRepositoryInvalidInput):
		writeProblem(w, r, invalidRequestProblem)
	case errors.Is(err, domain.ErrRepositoryNotFound), errors.Is(err, operation.ErrNotFound):
		writeProblem(w, r, notFoundProblem)
	case errors.Is(err, operation.ErrDisabled), errors.Is(err, operation.ErrForbidden), errors.Is(err, auth.ErrAuthorizationDenied):
		writeProblem(w, r, forbiddenProblem)
	case errors.Is(err, auth.ErrRecentAuthRejected):
		handler.auth.writeError(w, r, err)
	case errors.Is(err, operation.ErrStale):
		writeProblem(w, r, problemKind{status: 409, code: "operation.version_conflict", title: "Resource version changed"})
	case errors.Is(err, operation.ErrConfirmation):
		writeProblem(w, r, problemKind{status: 409, code: "operation.confirmation_rejected", title: "Operation confirmation rejected"})
	case errors.Is(err, operation.ErrIdempotencyConflict):
		writeProblem(w, r, problemKind{status: 409, code: "operation.idempotency_conflict", title: "Operation key conflict"})
	case errors.Is(err, operation.ErrAdmission):
		writeProblem(w, r, problemKind{status: 429, code: "operation.capacity_reached", title: "Operation capacity reached"})
	default:
		writeProblem(w, r, unavailableProblem)
	}
}

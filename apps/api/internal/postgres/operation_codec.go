package postgres

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/operation"
	"io"
	"time"
)

type operationRequestPayload struct {
	Provider        domain.ProviderKind     `json:"provider"`
	SourceID        domain.SourceID         `json:"source_id"`
	ResourceID      domain.ResourceID       `json:"resource_id"`
	EnvironmentID   domain.EnvironmentID    `json:"environment_id"`
	Kind            domain.ResourceKind     `json:"kind"`
	Scope           domain.Scope            `json:"scope"`
	NativeID        domain.NativeResourceID `json:"native_id"`
	ExpectedVersion string                  `json:"expected_version"`
	Action          domain.OperationAction  `json:"action"`
	Replicas        *int                    `json:"replicas,omitempty"`
}

func encodeOperationRequest(r domain.OperationRequest) ([]byte, error) {
	if r.Validate() != nil {
		return nil, operation.ErrInvalid
	}
	i := r.Resource.Identity
	return json.Marshal(operationRequestPayload{Provider: r.Source.Provider, SourceID: r.Source.SourceID, ResourceID: r.Resource.ID, EnvironmentID: i.EnvironmentID, Kind: i.Kind, Scope: i.Scope, NativeID: i.NativeID(), ExpectedVersion: r.ExpectedVersion, Action: r.Action, Replicas: r.Parameters.Replicas})
}
func decodeOperationRequest(data []byte) (domain.OperationRequest, error) {
	var p operationRequestPayload
	if len(data) > 8192 || strictOperationJSON(data, &p) != nil {
		return domain.OperationRequest{}, operation.ErrUnavailable
	}
	i, err := domain.NewResourceIdentity(p.Provider, p.SourceID, p.EnvironmentID, p.Kind, p.Scope, p.NativeID)
	if err != nil {
		return domain.OperationRequest{}, operation.ErrUnavailable
	}
	r := domain.OperationRequest{Source: domain.ProviderRef{Provider: p.Provider, SourceID: p.SourceID}, Resource: domain.ResourceRef{ID: p.ResourceID, Identity: i}, ExpectedVersion: p.ExpectedVersion, Action: p.Action, Parameters: domain.OperationParameters{Replicas: p.Replicas}}
	if r.Validate() != nil {
		return domain.OperationRequest{}, operation.ErrUnavailable
	}
	return r, nil
}
func strictOperationJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return operation.ErrUnavailable
	}
	return nil
}

const operationColumns = `id,user_id,session_id,confirmation_id,idempotency_digest,request,request_id,state,result,reason_code,created_at_us,updated_at_us,lease_until_us`

func scanOperation(row rowScanner) (operation.ExecutionRecord, error) {
	var r operation.ExecutionRecord
	var digest, data, result []byte
	var reason sql.NullString
	var created, updated, lease int64
	if err := row.Scan(&r.ID, &r.UserID, &r.SessionID, &r.ConfirmationID, &digest, &data, &r.RequestID, &r.State, &result, &reason, &created, &updated, &lease); err != nil {
		return r, err
	}
	if len(digest) != 32 {
		return r, operation.ErrUnavailable
	}
	copy(r.IdempotencyDigest[:], digest)
	var err error
	r.Request, err = decodeOperationRequest(data)
	if err != nil {
		return r, err
	}
	r.CreatedAt = time.UnixMicro(created).UTC()
	r.UpdatedAt = time.UnixMicro(updated).UTC()
	r.LeaseUntil = time.UnixMicro(lease).UTC()
	r.ReasonCode = domain.ReasonCode(reason.String)
	if len(result) > 0 {
		if len(result) > 4096 {
			return r, operation.ErrUnavailable
		}
		r.Result = new(domain.OperationProviderResult)
		if strictOperationJSON(result, r.Result) != nil {
			return r, operation.ErrUnavailable
		}
	}
	if r.Validate() != nil {
		return r, operation.ErrUnavailable
	}
	return r, nil
}

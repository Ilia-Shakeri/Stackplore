package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/operation"
)

type operationReadFixture struct {
	ProductProvider
	snapshot domain.ResourceSnapshot
	reads    int
}

func (provider *operationReadFixture) Source() domain.InfrastructureSource {
	return domain.InfrastructureSource{ID: "source_cluster", Provider: domain.ProviderKubernetes, Name: "Cluster"}
}
func (provider *operationReadFixture) GetResource(_ context.Context, query domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	provider.reads++
	if query.ResourceID != provider.snapshot.Resource.Ref.ID {
		return domain.ResourceSnapshot{}, domain.ErrRepositoryNotFound
	}
	return provider.snapshot, nil
}

type operationHTTPFixtureManager struct {
	calls   int
	request domain.OperationRequest
	state   operation.ExecutionState
	at      time.Time
}

func (manager *operationHTTPFixtureManager) Preview(_ context.Context, _ auth.Principal, request domain.OperationRequest, requestID domain.RequestID) (operation.ConfirmationIssue, error) {
	manager.calls++
	manager.request = request
	if requestID.Validate() != nil {
		return operation.ConfirmationIssue{}, operation.ErrInvalid
	}
	return operation.ConfirmationIssue{Impact: domain.OperationImpact{Request: request, DisplayName: "API", EffectCode: "operation.kubernetes_restart", RiskCode: "operation.brief_interruption"}, Proof: strings.Repeat("A", 43), ExpiresAt: manager.at.Add(2 * time.Minute)}, nil
}
func (manager *operationHTTPFixtureManager) record() operation.ExecutionRecord {
	r := operation.ExecutionRecord{ID: "operation_saved", IdempotencyDigest: [32]byte{1}, ConfirmationID: "confirmation_one", UserID: "usr_admin", SessionID: "sess_admin", Request: manager.request, RequestID: "request_saved", State: manager.state, CreatedAt: manager.at, UpdatedAt: manager.at, LeaseUntil: manager.at.Add(time.Minute)}
	if r.State != operation.ExecutionInProgress {
		r.ReasonCode = "operation.timeout_uncertain"
		r.UpdatedAt = r.UpdatedAt.Add(time.Second)
	}
	return r
}
func (manager *operationHTTPFixtureManager) Execute(_ context.Context, _ auth.Principal, request domain.OperationRequest, proof, key string, grant auth.RecentGrantUse, _ domain.RequestID) (operation.ExecutionRecord, error) {
	manager.calls++
	manager.request = request
	if grant.Purpose != auth.RecentAuthPurposeOperationExecute || proof != strings.Repeat("A", 43) || key != "operation_browser_one" {
		return operation.ExecutionRecord{}, operation.ErrInvalid
	}
	record := manager.record()
	if record.State == operation.ExecutionUncertain {
		return record, operation.ErrUncertain
	}
	return record, nil
}
func (manager *operationHTTPFixtureManager) Get(context.Context, auth.Principal, string) (operation.ExecutionRecord, error) {
	manager.calls++
	return manager.record(), nil
}
func (manager *operationHTTPFixtureManager) List(_ context.Context, _ auth.Principal, _ domain.SourceID, page domain.PageRequest) (operation.ExecutionPage, error) {
	manager.calls++
	return operation.ExecutionPage{Executions: []operation.ExecutionRecord{manager.record()}, Page: domain.PageInfo{Limit: page.Limit}}, nil
}

type operationAuthentication struct {
	*testAuthentication
	prepare func(context.Context, auth.RecentAuthorizationInput) (auth.Authentication, auth.RecentGrantUse, error)
}

func (authentication operationAuthentication) PrepareRecentMutation(ctx context.Context, input auth.RecentAuthorizationInput) (auth.Authentication, auth.RecentGrantUse, error) {
	return authentication.prepare(ctx, input)
}

func operationFixture(t *testing.T) (http.Handler, *operationHTTPFixtureManager, *operationReadFixture, *auth.Role) {
	t.Helper()
	at := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	role := auth.RoleAdministrator
	identity := func() auth.Authentication {
		return auth.Authentication{Principal: auth.Principal{UserID: "usr_admin", SessionID: "sess_admin", Role: role, AuthRevision: 1}}
	}
	authentication := operationAuthentication{testAuthentication: &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return identity(), nil }, authorizeUnsafeFunc: func(context.Context, string, string) (auth.Authentication, error) { return identity(), nil }}, prepare: func(_ context.Context, input auth.RecentAuthorizationInput) (auth.Authentication, auth.RecentGrantUse, error) {
		if input.Purpose != auth.RecentAuthPurposeOperationExecute {
			return auth.Authentication{}, auth.RecentGrantUse{}, auth.ErrRecentAuthRejected
		}
		return identity(), auth.RecentGrantUse{Actor: identity().Principal, Purpose: input.Purpose}, nil
	}}
	resourceIdentity, err := domain.NewResourceIdentity(domain.ProviderKubernetes, "source_cluster", "env_cluster", "kubernetes.deployment", domain.Scope{Kind: domain.ScopeNamespace, Name: "shop"}, "private-workload-uid")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := domain.NewResourceRef(resourceIdentity)
	if err != nil {
		t.Fatal(err)
	}
	provider := &operationReadFixture{snapshot: domain.ResourceSnapshot{Resource: domain.Resource{Ref: ref, DisplayName: "API", Version: "kv_observed"}, Status: domain.ObservationStatus{State: domain.AvailabilityAvailable, Completeness: domain.CompletenessComplete, ObservedAt: at}}}
	manager := &operationHTTPFixtureManager{at: at, state: operation.ExecutionUncertain, request: domain.OperationRequest{Source: domain.ProviderRef{Provider: domain.ProviderKubernetes, SourceID: "source_cluster"}, Resource: ref, ExpectedVersion: "kv_observed", Action: domain.OperationKubernetesRestart}}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = authentication
		options.ProductProviders = []ProductProvider{provider}
		options.Operations = manager
	})
	return handler, manager, provider, &role
}
func operationPost(path, body string) *http.Request {
	request := withJSONBody(makeRequest(http.MethodPost, path, "198.51.100.10:4000"), body)
	request.Header.Set("Origin", "http://stackplore.test")
	request.Header.Set(csrfHeaderName, strings.Repeat("A", 43))
	request.Header.Set(recentAuthHeaderName, strings.Repeat("A", 43))
	request.Header.Set("Idempotency-Key", "operation_browser_one")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("A", 43)})
	return request
}
func operationBody(provider *operationReadFixture, execute bool) string {
	payload := map[string]any{"source_id": "source_cluster", "resource_id": provider.snapshot.Resource.Ref.ID, "action": "kubernetes.workload_restart", "expected_version": "kv_observed", "parameters": map[string]any{}}
	if execute {
		payload["confirmation_token"] = strings.Repeat("A", 43)
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func TestOperationHTTPResolvesPrivateIdentityAndReturnsSafeUncertainReceipt(t *testing.T) {
	handler, manager, provider, _ := operationFixture(t)
	preview := serve(handler, operationPost("/api/v1/operations/previews", operationBody(provider, false)))
	if preview.Code != 200 || manager.calls != 1 || manager.request.Resource.Identity.NativeID() != "private-workload-uid" {
		t.Fatalf("trusted preview failed status=%d", preview.Code)
	}
	response := serve(handler, operationPost("/api/v1/operations", operationBody(provider, true)))
	if response.Code != 200 || manager.calls != 2 {
		t.Fatalf("uncertain receipt unavailable status=%d", response.Code)
	}
	var receipt struct {
		Data operationRecordJSON `json:"data"`
	}
	if json.Unmarshal(response.Body.Bytes(), &receipt) != nil || receipt.Data.State != operation.ExecutionUncertain || receipt.Data.ID != "operation_saved" {
		t.Fatal("uncertain durable outcome lost")
	}
	for _, forbidden := range []string{"private-workload-uid", "sess_admin", "usr_admin", "operation_browser_one", "confirmation_one", "confirmation_token", "IdempotencyDigest"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatal("operation receipt leaked private material")
		}
	}
	if got := authenticatedProductRequest(handler, "/api/v1/operations/operation_saved"); got.Code != 200 {
		t.Fatal("safe GET receipt recovery failed")
	}
}
func TestOperationHTTPRejectsViewerProofQueryUnknownFieldsAndUnboundedPages(t *testing.T) {
	handler, manager, provider, role := operationFixture(t)
	*role = auth.RoleViewer
	if response := serve(handler, operationPost("/api/v1/operations/previews", operationBody(provider, false))); response.Code != 403 || provider.reads != 0 || manager.calls != 0 {
		t.Fatal("viewer reached operation")
	}
	if response := authenticatedProductRequest(handler, "/api/v1/operations"); response.Code != 403 || manager.calls != 0 {
		t.Fatal("viewer read operation history")
	}
	*role = auth.RoleAdministrator
	for _, path := range []string{"/api/v1/operations?limit=101", "/api/v1/operations?confirmation_token=unsafe", "/api/v1/operations/operation_saved?proof=unsafe"} {
		if response := authenticatedProductRequest(handler, path); response.Code != 400 || manager.calls != 0 {
			t.Fatal("unbounded or protected query accepted")
		}
	}
	body := strings.TrimSuffix(operationBody(provider, false), "}") + `,"endpoint":"https://other"}`
	if response := serve(handler, operationPost("/api/v1/operations/previews", body)); response.Code != 400 || provider.reads != 0 {
		t.Fatal("unknown operation field reached inventory")
	}
	request := operationPost("/api/v1/operations", operationBody(provider, true))
	request.Header.Del(recentAuthHeaderName)
	if response := serve(handler, request); response.Code != 403 || manager.calls != 0 {
		t.Fatal("missing recent proof reached execution")
	}
}
func TestOperationHTTPPendingReceiptUses202AndGETRemains200(t *testing.T) {
	handler, manager, provider, _ := operationFixture(t)
	manager.state = operation.ExecutionInProgress
	if response := serve(handler, operationPost("/api/v1/operations", operationBody(provider, true))); response.Code != 202 {
		t.Fatal("pending operation lacked durable202")
	}
	if response := authenticatedProductRequest(handler, "/api/v1/operations/operation_saved"); response.Code != 200 {
		t.Fatal("pending GET cannot recover receipt")
	}
}

package httpapi

import (
	"context"
	"encoding/json"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
	"net/http"
	"testing"
	"time"
)

type healthHistoryFixture struct {
	ProductProvider
	calls   int
	corrupt bool
}

func (provider *healthHistoryFixture) ObservationTime() time.Time { return demo.ObservationTime() }
func (provider *healthHistoryFixture) GetHealthHistory(_ context.Context, query domain.HealthHistoryQuery) (domain.HealthHistoryResult, error) {
	provider.calls++
	ref := query.Resource.ID
	if provider.corrupt {
		ref = "r1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	status := explorationQueryStatus()
	return domain.HealthHistoryResult{Status: status, Samples: []domain.HealthHistorySample{{ID: "health_one", ResourceID: ref, State: domain.HealthUnknown, Status: status, Synthetic: true}}}, nil
}
func TestHealthHistoryHTTPBindsExactResourceAndRejectsBadRange(t *testing.T) {
	fixture := demo.MustNew()
	capture, err := fixture.CaptureInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	id := capture.Resources[0].Resource.Ref.ID
	provider := &healthHistoryFixture{ProductProvider: fixture}
	handler, _, _ := makeHandler(t, func(options *Options) {
		options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return auth.Authentication{}, nil }}
		options.ProductProviders = []ProductProvider{provider}
	})
	response := authenticatedProductRequest(handler, "/api/v1/health/history?resource_id="+string(id))
	var result healthHistoryResponse
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil || len(result.Items) != 1 || result.Items[0].ResourceID != id {
		t.Fatalf("health history HTTP failed: %d", response.Code)
	}
	for _, query := range []string{"", "?resource_id=" + string(id) + "&from=bad&to=bad", "?resource_id=" + string(id) + "&limit=201", "?resource_id=" + string(id) + "&from=2026-01-01T00:00:00Z"} {
		if r := authenticatedProductRequest(handler, "/api/v1/health/history"+query); r.Code != http.StatusBadRequest {
			t.Fatalf("invalid health query accepted: %d", r.Code)
		}
	}
	provider.corrupt = true
	if r := authenticatedProductRequest(handler, "/api/v1/health/history?resource_id="+string(id)); r.Code != http.StatusServiceUnavailable {
		t.Fatal("mismatched retained health exposed")
	}
}

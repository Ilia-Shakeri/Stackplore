package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/provider/demo"
)

type independentHealthProvider struct {
	ProductProvider
	health      domain.HealthState
	calls       int
	unavailable bool
	denied      bool
}

func (provider *independentHealthProvider) GetResource(ctx context.Context, query domain.ResourceDetailQuery) (domain.ResourceSnapshot, error) {
	snapshot, err := provider.ProductProvider.GetResource(ctx, query)
	snapshot.Health = provider.health
	return snapshot, err
}

func (provider *independentHealthProvider) GetHealth(context.Context, domain.HealthQuery) (domain.HealthResult, error) {
	provider.calls++
	if provider.denied {
		return domain.HealthResult{}, mustPermissionDeniedHealthError()
	}
	if provider.unavailable {
		result, _ := domain.NewProviderError("health.read", domain.ProviderErrorUnavailable, false, domain.ErrRepositoryUnavailable)
		return domain.HealthResult{}, result
	}
	return domain.HealthResult{}, nil
}

func mustPermissionDeniedHealthError() error {
	result, _ := domain.NewProviderError("health.read", domain.ProviderErrorPermissionDenied, false, domain.ErrRepositoryUnavailable)
	return result
}

func TestResourceIdentityDoesNotDependOnOptionalHealthRead(t *testing.T) {
	for _, testcase := range []struct {
		name                string
		health              domain.HealthState
		unavailable, denied bool
	}{
		{"missing_optional_health", "", true, false}, {"empty_optional_health", domain.HealthUnknown, false, false}, {"observed_degraded_health", domain.HealthDegraded, true, false}, {"denied_optional_health", domain.HealthUnknown, false, true},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			fixture := demo.MustNew()
			capture, err := fixture.CaptureInventory(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			resource := capture.Resources[0].Resource
			provider := &independentHealthProvider{ProductProvider: fixture, health: testcase.health, unavailable: testcase.unavailable, denied: testcase.denied}
			handler, _, _ := makeHandler(t, func(options *Options) {
				options.Authentication = &testAuthentication{authenticateFunc: func(context.Context, string) (auth.Authentication, error) { return auth.Authentication{}, nil }}
				options.ProductProviders = []ProductProvider{provider}
			})
			response := authenticatedProductRequest(handler, "/api/v1/resources/"+string(resource.Ref.ID))
			var actual resourceResponse
			expected := testcase.health
			if expected == "" {
				expected = domain.HealthUnknown
			}
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &actual) != nil || actual.ID != resource.Ref.ID || actual.Name != resource.DisplayName || actual.Health != string(expected) {
				t.Fatalf("identity was lost or health invented: status=%d resource=%#v", response.Code, actual)
			}
			if provider.calls != 0 {
				t.Fatal("resource detail performed optional health I/O")
			}
			optional := authenticatedProductRequest(handler, "/api/v1/resources/"+string(resource.Ref.ID)+"/health")
			expectedStatus := http.StatusServiceUnavailable
			if testcase.denied {
				expectedStatus = http.StatusForbidden
			}
			if optional.Code != expectedStatus || provider.calls != 1 {
				t.Fatal("independent health failure did not retain its unavailable/denied state")
			}
		})
	}
}

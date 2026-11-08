package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
)

func TestMetricsReportsCanonicalApplicationVersion(t *testing.T) {
	metrics := NewMetrics(NewReadiness())
	response := httptest.NewRecorder()
	metrics.serveHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	want := fmt.Sprintf("stackplore_build_info{version=%q} 1\n", config.ApplicationVersion)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), want) || strings.Contains(response.Body.String(), `version="0.0.0"`) {
		t.Fatal("metrics did not report the immutable application version")
	}
}

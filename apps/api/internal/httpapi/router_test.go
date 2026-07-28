package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
)

func TestHealth(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	httpapi.NewHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", response.Code, http.StatusOK)
	}

	if contentType := response.Header().Get("Content-Type"); contentType != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q; want %q", contentType, "text/plain; charset=utf-8")
	}

	if cacheControl := response.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Errorf("Cache-Control = %q; want %q", cacheControl, "no-store")
	}

	if contentTypeOptions := response.Header().Get("X-Content-Type-Options"); contentTypeOptions != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q; want %q", contentTypeOptions, "nosniff")
	}

	if body := response.Body.String(); body != "ok\n" {
		t.Errorf("body = %q; want %q", body, "ok\n")
	}
}

func TestHealthRejectsOtherMethods(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	response := httptest.NewRecorder()

	httpapi.NewHandler().ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d; want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestUnknownRoute(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	response := httptest.NewRecorder()

	httpapi.NewHandler().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want %d", response.Code, http.StatusNotFound)
	}
}

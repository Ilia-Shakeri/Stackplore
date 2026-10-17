package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandlerServesAssetsAndOnlyFallsBackForBrowserRoutes(t *testing.T) {
	assets := fstest.MapFS{
		"embed.marker":  {Data: []byte("embed marker")},
		"index.html":    {Data: []byte("<main>app shell</main>")},
		"assets/app.js": {Data: []byte("export {}")},
	}
	handler := Handler(assets)
	for _, test := range []struct {
		name, method, path, accept string
		status                     int
		body                       string
		contentType                string
		cacheControl               string
	}{
		{"root shell", http.MethodGet, "/", "text/html", http.StatusOK, "app shell", "text/html", "no-store"},
		{"deep browser route", http.MethodGet, "/settings/application", "text/html", http.StatusOK, "app shell", "text/html", "no-store"},
		{"bundled asset", http.MethodGet, "/assets/app.js", "*/*", http.StatusOK, "export {}", "javascript", "public, max-age=31536000, immutable"},
		{"unknown asset", http.MethodGet, "/assets/missing.js", "text/html", http.StatusNotFound, "", "", ""},
		{"embed marker", http.MethodGet, "/embed.marker", "*/*", http.StatusNotFound, "", "", ""},
		{"api path", http.MethodGet, "/api/v1/missing", "text/html", http.StatusNotFound, "", "", ""},
		{"health path", http.MethodGet, "/healthz", "text/html", http.StatusNotFound, "", "", ""},
		{"non browser accept", http.MethodGet, "/unknown", "application/json", http.StatusNotFound, "", "", ""},
		{"unsupported method", http.MethodPost, "/", "text/html", http.StatusNotFound, "", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			request.Header.Set("Accept", test.accept)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || test.body != "" && !strings.Contains(response.Body.String(), test.body) {
				t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
			}
			if test.contentType != "" && !strings.Contains(response.Header().Get("Content-Type"), test.contentType) {
				t.Fatalf("content type=%q", response.Header().Get("Content-Type"))
			}
			if response.Code == http.StatusOK && response.Header().Get("Cache-Control") != test.cacheControl {
				t.Fatalf("cache control=%q, want %q", response.Header().Get("Cache-Control"), test.cacheControl)
			}
		})
	}
}

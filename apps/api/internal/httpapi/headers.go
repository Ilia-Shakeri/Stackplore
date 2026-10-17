package httpapi

import (
	"net/http"
	"strings"
)

type fixedHeaderWriter struct {
	http.ResponseWriter
	requestID string
	path      string
	hsts      bool
	wrote     bool
}

func (writer *fixedHeaderWriter) WriteHeader(status int) {
	if writer.wrote {
		return
	}
	writer.wrote = true
	applyFixedHeaders(writer.Header(), writer.path, status, writer.requestID, writer.hsts)
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *fixedHeaderWriter) Write(value []byte) (int, error) {
	if !writer.wrote {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(value)
}

func (writer *fixedHeaderWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func fixedHeadersMiddleware(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writer := &fixedHeaderWriter{
				ResponseWriter: w,
				requestID:      requestIDFromContext(r.Context()),
				path:           r.URL.Path,
				hsts:           hsts,
			}
			next.ServeHTTP(writer, r)
			if !writer.wrote {
				writer.WriteHeader(http.StatusOK)
			}
		})
	}
}

func applyFixedHeaders(header http.Header, path string, status int, requestID string, hsts bool) {
	webAsset := header.Get("X-Stackplore-Web-Asset") == "1" && (status >= 200 && status < 300 || status == http.StatusNotModified)
	header.Del("X-Stackplore-Web-Asset")
	for name := range header {
		normalized := strings.ToLower(name)
		if strings.HasPrefix(normalized, "access-control-") ||
			normalized == "connection" ||
			normalized == "content-length" ||
			normalized == "server" ||
			normalized == "trailer" ||
			normalized == "transfer-encoding" {
			header.Del(name)
		}
	}
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	if webAsset {
		header.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' data:; connect-src 'self'; worker-src 'self'; manifest-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		if strings.HasPrefix(path, "/assets/") {
			header.Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if path == "/offline.html" || path == "/offline.css" || path == "/offline.js" || path == "/brand/stackplore-favicon.svg" {
			header.Set("Cache-Control", "public, max-age=0, must-revalidate")
		} else if path == "/sw.js" || path == "/manifest.webmanifest" {
			header.Set("Cache-Control", "no-cache")
		}
	}
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
	header.Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
	header.Set("X-Permitted-Cross-Domain-Policies", "none")
	header.Set(requestIDHeader, requestID)
	if hsts {
		header.Set("Strict-Transport-Security", "max-age=31536000")
	} else {
		header.Del("Strict-Transport-Security")
	}

	switch {
	case webAsset:
		// The embedded asset handler sets the MIME type from the trusted filename.
	case path == "/metrics" && status >= 200 && status < 300:
		header.Set("Content-Type", metricsContentType)
	case path == "/api/v1/logs/download" && status >= 200 && status < 300:
		header.Set("Content-Type", "text/plain; charset=utf-8")
	case isStreamPath(path) && status >= 200 && status < 300:
		header.Set("Content-Type", "text/event-stream; charset=utf-8")
	case path == "/api/v1" || strings.HasPrefix(path, "/api/v1/"):
		if status >= 400 {
			header.Set("Content-Type", "application/problem+json")
		} else if status == http.StatusNoContent {
			header.Del("Content-Type")
		} else {
			header.Set("Content-Type", "application/json")
		}
	default:
		header.Set("Content-Type", "text/plain; charset=utf-8")
	}
}

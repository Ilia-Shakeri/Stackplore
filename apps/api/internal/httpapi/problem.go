package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
)

type problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

type problemKind struct {
	status int
	code   string
	title  string
}

var (
	invalidRequestProblem     = problemKind{status: http.StatusBadRequest, code: "invalid_request", title: "Invalid request"}
	unauthenticatedProblem    = problemKind{status: http.StatusUnauthorized, code: "unauthenticated", title: "Sign in required"}
	invalidCredentialsProblem = problemKind{status: http.StatusUnauthorized, code: "invalid_credentials", title: "Sign in failed"}
	forbiddenProblem          = problemKind{status: http.StatusForbidden, code: "forbidden", title: "Request not allowed"}
	recentAuthProblem         = problemKind{status: http.StatusForbidden, code: "recent_auth_required", title: "Recent sign in required"}
	notFoundProblem           = problemKind{status: http.StatusNotFound, code: "not_found", title: "Not found"}
	conflictProblem           = problemKind{status: http.StatusConflict, code: "conflict", title: "Request conflict"}
	setupClosedProblem        = problemKind{status: http.StatusConflict, code: "setup_closed", title: "Setup is closed"}
	methodProblem             = problemKind{status: http.StatusMethodNotAllowed, code: "method_not_allowed", title: "Method not allowed"}
	payloadProblem            = problemKind{status: http.StatusRequestEntityTooLarge, code: "payload_too_large", title: "Request too large"}
	mediaTypeProblem          = problemKind{status: http.StatusUnsupportedMediaType, code: "unsupported_media_type", title: "Unsupported media type"}
	rateLimitProblem          = problemKind{status: http.StatusTooManyRequests, code: "rate_limited", title: "Too many requests"}
	internalProblem           = problemKind{status: http.StatusInternalServerError, code: "internal", title: "Request failed"}
	unavailableProblem        = problemKind{status: http.StatusServiceUnavailable, code: "unavailable", title: "Service unavailable"}
	timeoutProblem            = problemKind{status: http.StatusServiceUnavailable, code: "request_timeout", title: "Service unavailable"}
)

func writeProblem(w http.ResponseWriter, r *http.Request, kind problemKind) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(kind.status)
	_ = json.NewEncoder(w).Encode(problem{
		Type:      "urn:stackplore:problem:" + kind.code,
		Title:     kind.title,
		Status:    kind.status,
		Code:      kind.code,
		RequestID: requestIDFromContext(r.Context()),
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writePlain(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

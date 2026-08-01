package httpapi

import "net/http"

const (
	healthBody   = "ok\n"
	readyBody    = "ready\n"
	notReadyBody = "not ready\n"
)

func health(w http.ResponseWriter, r *http.Request) {
	writePlain(w, http.StatusOK, healthBody)
}

func ready(readiness *Readiness) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !readiness.IsReady() {
			writePlain(w, http.StatusServiceUnavailable, notReadyBody)
			return
		}
		writePlain(w, http.StatusOK, readyBody)
	}
}

func closedReadiness(w http.ResponseWriter, r *http.Request) {
	writeProblem(w, r, unauthenticatedProblem)
}

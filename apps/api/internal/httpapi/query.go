package httpapi

import (
	"net/http"
	"net/url"
)

func rejectMalformedQuery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := url.ParseQuery(r.URL.RawQuery); err != nil {
			writeProblem(w, r, invalidRequestProblem)
			return
		}
		next.ServeHTTP(w, r)
	})
}

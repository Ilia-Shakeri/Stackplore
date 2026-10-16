package main

import (
	"context"
	"net/http"
	"strings"
)

// Streams stop when shutdown begins; ordinary requests retain their drain period.
func withStreamShutdown(handler http.Handler, shutdown context.Context) http.Handler {
	if handler == nil {
		handler = http.DefaultServeMux
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/streams/") {
			handler.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithCancel(r.Context())
		stop := context.AfterFunc(shutdown, cancel)
		defer stop()
		defer cancel()
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

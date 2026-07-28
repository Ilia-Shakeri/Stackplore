package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewHandler() http.Handler {
	router := chi.NewRouter()
	router.Get("/healthz", health)

	return router
}

package route

import (
	"net/http"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
)

// Router registers HTTP handlers on a standard library ServeMux.
type Router struct {
	healthHandler *handler.HealthHandler
}

// NewRouter creates a router for the application HTTP handlers.
func NewRouter(healthHandler *handler.HealthHandler) *Router {
	return &Router{healthHandler: healthHandler}
}

// Register attaches application routes to mux.
func (rt *Router) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", rt.healthHandler.Check)
}

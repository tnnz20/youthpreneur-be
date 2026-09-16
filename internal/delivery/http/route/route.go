package route

import (
	"net/http"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
)

type Router struct {
	healthHandler *handler.HealthHandler
}

func NewRouter(healthHandler *handler.HealthHandler) *Router {
	return &Router{healthHandler: healthHandler}
}

func (rt *Router) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", rt.healthHandler.Check)
}

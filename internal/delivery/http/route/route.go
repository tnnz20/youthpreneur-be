package route

import (
	"net/http"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
)

// Router registers HTTP handlers on a standard library ServeMux.
type Router struct {
	healthHandler *handler.HealthHandler
	userHandler   *handler.UserHandler
}

// NewRouter creates a router for the application HTTP handlers.
func NewRouter(healthHandler *handler.HealthHandler, userHandler *handler.UserHandler) *Router {
	return &Router{healthHandler: healthHandler, userHandler: userHandler}
}

// Register attaches application routes to mux.
func (rt *Router) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", rt.healthHandler.Check)

	mux.HandleFunc("POST /users", rt.userHandler.Create)
	mux.HandleFunc("GET /users", rt.userHandler.List)
	mux.HandleFunc("GET /users/{publicID}", rt.userHandler.Get)
	mux.HandleFunc("DELETE /users/{publicID}", rt.userHandler.Delete)
	mux.HandleFunc("PUT /users/{publicID}/profile", rt.userHandler.UpdateProfile)
	mux.HandleFunc("PATCH /users/{publicID}/status", rt.userHandler.UpdateStatus)
	mux.HandleFunc("PUT /users/{publicID}/password", rt.userHandler.ChangePassword)
	mux.HandleFunc("POST /users/{publicID}/password/reset", rt.userHandler.ResetPassword)
}

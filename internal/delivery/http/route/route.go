package route

import (
	"net/http"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
)

// Middleware wraps an HTTP handler with a cross-cutting concern.
type Middleware func(http.Handler) http.Handler

// Dependencies are the handlers and middleware a Router applies. Nil middleware
// is treated as pass-through so tests can register routes without cross-cutting
// concerns.
type Dependencies struct {
	HealthHandler     *handler.HealthHandler
	UserHandler       *handler.UserHandler
	AuthHandler       *handler.AuthHandler
	EnterpriseHandler *handler.EnterpriseHandler

	Authenticate     Middleware
	RequireAdmin     Middleware
	RequireSelf      Middleware
	LoginRateLimit   Middleware
	RefreshRateLimit Middleware
}

// Router registers HTTP handlers on a standard library ServeMux.
type Router struct {
	deps Dependencies
}

// NewRouter creates a router from deps.
func NewRouter(deps Dependencies) *Router {
	return &Router{deps: deps}
}

// Register attaches application routes to mux with the approved public,
// authenticated, and admin policy.
func (rt *Router) Register(mux *http.ServeMux) {
	rt.register(mux, "GET /healthz", nil, rt.deps.HealthHandler.Check)

	rt.register(mux, "POST /users", nil, rt.deps.UserHandler.Create)
	rt.register(mux, "POST /auth/login", []Middleware{rt.deps.LoginRateLimit}, rt.deps.AuthHandler.Login)
	rt.register(mux, "POST /auth/refresh", []Middleware{rt.deps.RefreshRateLimit}, rt.deps.AuthHandler.Refresh)

	rt.register(mux, "POST /auth/logout", []Middleware{rt.deps.Authenticate}, rt.deps.AuthHandler.Logout)
	rt.register(mux, "PUT /users/{publicID}/profile", []Middleware{rt.deps.Authenticate, rt.deps.RequireSelf}, rt.deps.UserHandler.UpdateProfile)
	rt.register(mux, "PUT /users/{publicID}/password", []Middleware{rt.deps.Authenticate, rt.deps.RequireSelf}, rt.deps.UserHandler.ChangePassword)

	authenticated := []Middleware{rt.deps.Authenticate}
	rt.register(mux, "POST /enterprises", authenticated, rt.deps.EnterpriseHandler.Create)
	rt.register(mux, "GET /enterprises", authenticated, rt.deps.EnterpriseHandler.List)
	rt.register(mux, "GET /enterprises/{publicID}", authenticated, rt.deps.EnterpriseHandler.Get)
	rt.register(mux, "PATCH /enterprises/{publicID}", authenticated, rt.deps.EnterpriseHandler.Update)
	rt.register(mux, "DELETE /enterprises/{publicID}", authenticated, rt.deps.EnterpriseHandler.Delete)

	admin := []Middleware{rt.deps.Authenticate, rt.deps.RequireAdmin}
	rt.register(mux, "GET /users", admin, rt.deps.UserHandler.List)
	rt.register(mux, "GET /users/{publicID}", admin, rt.deps.UserHandler.Get)
	rt.register(mux, "DELETE /users/{publicID}", admin, rt.deps.UserHandler.Delete)
	rt.register(mux, "PATCH /users/{publicID}/status", admin, rt.deps.UserHandler.UpdateStatus)
	rt.register(mux, "POST /users/{publicID}/password/reset", admin, rt.deps.UserHandler.ResetPassword)
}

func (rt *Router) register(mux *http.ServeMux, pattern string, chain []Middleware, fn http.HandlerFunc) {
	var h http.Handler = fn
	for i := len(chain) - 1; i >= 0; i-- {
		if chain[i] == nil {
			continue
		}
		h = chain[i](h)
	}

	mux.Handle(pattern, h)
}

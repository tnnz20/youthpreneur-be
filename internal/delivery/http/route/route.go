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
	HealthHandler             *handler.HealthHandler
	UserHandler               *handler.UserHandler
	AuthHandler               *handler.AuthHandler
	EnterpriseHandler         *handler.EnterpriseHandler
	TrainingCatalogHandler    *handler.TrainingCatalogHandler
	TrainingEnrollmentHandler *handler.TrainingEnrollmentHandler
	UploadDir                 string

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
	if rt.deps.UploadDir != "" {
		mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(rt.deps.UploadDir))))
	}

	rt.register(mux, "GET /healthz", nil, rt.deps.HealthHandler.Check)

	rt.register(mux, "POST /users", nil, rt.deps.UserHandler.Create)
	rt.register(mux, "POST /auth/login", []Middleware{rt.deps.LoginRateLimit}, rt.deps.AuthHandler.Login)
	rt.register(mux, "POST /auth/refresh", []Middleware{rt.deps.RefreshRateLimit}, rt.deps.AuthHandler.Refresh)

	rt.register(mux, "POST /auth/logout", []Middleware{rt.deps.Authenticate}, rt.deps.AuthHandler.Logout)
	rt.register(mux, "GET /auth/me", []Middleware{rt.deps.Authenticate}, rt.deps.AuthHandler.Me)
	rt.register(mux, "PUT /users/{publicID}/profile", []Middleware{rt.deps.Authenticate, rt.deps.RequireSelf}, rt.deps.UserHandler.UpdateProfile)
	rt.register(mux, "PUT /users/{publicID}/password", []Middleware{rt.deps.Authenticate, rt.deps.RequireSelf}, rt.deps.UserHandler.ChangePassword)

	rt.register(mux, "GET /enterprises/public", nil, rt.deps.EnterpriseHandler.ListPublic)

	authenticated := []Middleware{rt.deps.Authenticate}
	rt.register(mux, "POST /enterprises", authenticated, rt.deps.EnterpriseHandler.Create)
	rt.register(mux, "GET /enterprises", authenticated, rt.deps.EnterpriseHandler.List)
	rt.register(mux, "GET /enterprises/{publicID}", authenticated, rt.deps.EnterpriseHandler.Get)
	rt.register(mux, "GET /enterprises/{publicID}/audit-logs", authenticated, rt.deps.EnterpriseHandler.ListAuditLogs)
	rt.register(mux, "PATCH /enterprises/{publicID}", authenticated, rt.deps.EnterpriseHandler.Update)
	rt.register(mux, "DELETE /enterprises/{publicID}", authenticated, rt.deps.EnterpriseHandler.Delete)

	admin := []Middleware{rt.deps.Authenticate, rt.deps.RequireAdmin}
	rt.register(mux, "GET /users", admin, rt.deps.UserHandler.List)
	rt.register(mux, "GET /users/{publicID}", admin, rt.deps.UserHandler.Get)
	rt.register(mux, "DELETE /users/{publicID}", admin, rt.deps.UserHandler.Delete)
	rt.register(mux, "PATCH /users/{publicID}/status", admin, rt.deps.UserHandler.UpdateStatus)
	rt.register(mux, "POST /users/{publicID}/password/reset", admin, rt.deps.UserHandler.ResetPassword)

	// Catalog reads are public; catalog writes are admin-only.
	rt.register(mux, "GET /training-catalog", nil, rt.deps.TrainingCatalogHandler.List)
	rt.register(mux, "GET /training-catalog/{publicID}", nil, rt.deps.TrainingCatalogHandler.Get)
	rt.register(mux, "POST /training-catalog", admin, rt.deps.TrainingCatalogHandler.Create)
	rt.register(mux, "POST /training-catalog/upload-thumbnail", admin, rt.deps.TrainingCatalogHandler.UploadThumbnail)
	rt.register(mux, "PATCH /training-catalog/{publicID}", admin, rt.deps.TrainingCatalogHandler.Update)
	rt.register(mux, "PATCH /training-catalog/{publicID}/status", admin, rt.deps.TrainingCatalogHandler.UpdateStatus)
	rt.register(mux, "DELETE /training-catalog/{publicID}", admin, rt.deps.TrainingCatalogHandler.Delete)

	// Enrollment mutations are authenticated and self-scoped; admin history is
	// admin-only.
	rt.register(mux, "POST /training-enrollments", authenticated, rt.deps.TrainingEnrollmentHandler.Create)
	rt.register(mux, "PATCH /training-enrollments/{publicID}/status", admin, rt.deps.TrainingEnrollmentHandler.UpdateStatus)
	rt.register(mux, "DELETE /training-enrollments/{publicID}", authenticated, rt.deps.TrainingEnrollmentHandler.Cancel)
	rt.register(mux, "GET /training-enrollments/my", authenticated, rt.deps.TrainingEnrollmentHandler.ListMine)
	rt.register(mux, "GET /training-enrollments", admin, rt.deps.TrainingEnrollmentHandler.List)
	rt.register(mux, "GET /training-enrollments/catalog/{catalogPublicID}", admin, rt.deps.TrainingEnrollmentHandler.ListByCatalog)
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

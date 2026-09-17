package handler_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/middleware"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/route"
	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/token"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

type routeParser struct {
	claims token.AccessClaims
	err    error
}

func (p routeParser) ParseAccess(string) (token.AccessClaims, error) {
	return p.claims, p.err
}

type routeLookup struct {
	user entity.User
	err  error
}

func (l routeLookup) FindUserByPublicID(context.Context, string) (entity.User, error) {
	return l.user, l.err
}

func newProtectedRouter(uc usecase.UserUseCase, parser routeParser, lookup routeLookup) *http.ServeMux {
	auth := middleware.NewAuthenticator(parser, lookup, slog.Default())
	mux := http.NewServeMux()
	route.NewRouter(route.Dependencies{
		HealthHandler: handler.NewHealthHandler(slog.Default(), usecase.NewHealthUseCase(repository.NewHealthRepository())),
		UserHandler:   handler.NewUserHandler(slog.Default(), uc),
		AuthHandler:   handler.NewAuthHandler(slog.Default(), stubAuthUseCase{}, false),
		Authenticate:  auth.Authenticate,
		RequireAdmin:  auth.RequireAdmin,
		RequireSelf:   auth.RequireSelf,
	}).Register(mux)

	return mux
}

func serveWithCookie(t *testing.T, mux *http.ServeMux, method, target, body, cookie string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, bytes.NewReader([]byte(body)))
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: handler.AccessTokenCookie, Value: cookie})
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec
}

func TestProtectedRouteRejectsMissingToken(t *testing.T) {
	mux := newProtectedRouter(&fakeUserUseCase{}, routeParser{}, routeLookup{user: entity.User{IsActive: true}})

	rec := serveWithCookie(t, mux, http.MethodGet, "/users", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestProtectedRouteRejectsNonAdmin(t *testing.T) {
	parser := routeParser{claims: token.AccessClaims{PublicID: "YTP-000002", Role: entity.RoleMember}}
	lookup := routeLookup{user: entity.User{PublicID: "YTP-000002", Role: entity.RoleMember, IsActive: true}}
	mux := newProtectedRouter(&fakeUserUseCase{}, parser, lookup)

	rec := serveWithCookie(t, mux, http.MethodGet, "/users", "", "good")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestProtectedRouteRejectsStaleAdminClaim(t *testing.T) {
	parser := routeParser{claims: token.AccessClaims{PublicID: "YTP-000009", Role: entity.RoleAdmin}}
	lookup := routeLookup{user: entity.User{PublicID: "YTP-000009", Role: entity.RoleMember, IsActive: true}}
	mux := newProtectedRouter(&fakeUserUseCase{}, parser, lookup)

	rec := serveWithCookie(t, mux, http.MethodGet, "/users", "", "good")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d for demoted admin", rec.Code, http.StatusForbidden)
	}
}

func TestProtectedRouteAllowsAdmin(t *testing.T) {
	parser := routeParser{claims: token.AccessClaims{PublicID: "YTP-000009", Role: entity.RoleAdmin}}
	lookup := routeLookup{user: entity.User{PublicID: "YTP-000009", Role: entity.RoleAdmin, IsActive: true}}
	mux := newProtectedRouter(&fakeUserUseCase{}, parser, lookup)

	rec := serveWithCookie(t, mux, http.MethodGet, "/users", "", "good")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestOwnershipRouteAllowsOwner(t *testing.T) {
	parser := routeParser{claims: token.AccessClaims{PublicID: "YTP-000001", Role: entity.RoleMember}}
	lookup := routeLookup{user: entity.User{PublicID: "YTP-000001", Role: entity.RoleMember, IsActive: true}}
	uc := &fakeUserUseCase{getResult: entity.User{PublicID: "YTP-000001"}}
	mux := newProtectedRouter(uc, parser, lookup)

	rec := serveWithCookie(t, mux, http.MethodPut, "/users/YTP-000001/profile", `{"full_name":"Alice"}`, "good")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestOwnershipRouteRejectsOtherUser(t *testing.T) {
	parser := routeParser{claims: token.AccessClaims{PublicID: "YTP-000002", Role: entity.RoleMember}}
	lookup := routeLookup{user: entity.User{PublicID: "YTP-000002", Role: entity.RoleMember, IsActive: true}}
	mux := newProtectedRouter(&fakeUserUseCase{}, parser, lookup)

	rec := serveWithCookie(t, mux, http.MethodPut, "/users/YTP-000001/profile", `{"full_name":"Alice"}`, "good")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestLogoutRequiresAuthentication(t *testing.T) {
	mux := newProtectedRouter(&fakeUserUseCase{}, routeParser{}, routeLookup{user: entity.User{IsActive: true}})

	rec := serveWithCookie(t, mux, http.MethodPost, "/auth/logout", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestPublicRoutesRemainPublic(t *testing.T) {
	mux := newProtectedRouter(&fakeUserUseCase{}, routeParser{}, routeLookup{})

	rec := serveWithCookie(t, mux, http.MethodPost, "/users", `{
		"email": "alice@example.com",
		"password": "secret123",
		"full_name": "Alice"
	}`, "")

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestLoginRouteAppliesEndpointRateLimit(t *testing.T) {
	auth := middleware.NewAuthenticator(routeParser{}, routeLookup{}, slog.Default())
	authUseCase := &fakeAuthUseCase{loginResult: usecase.LoginResult{AuthTokens: authTokensFixture()}}

	mux := http.NewServeMux()
	route.NewRouter(route.Dependencies{
		HealthHandler:  handler.NewHealthHandler(slog.Default(), usecase.NewHealthUseCase(repository.NewHealthRepository())),
		UserHandler:    handler.NewUserHandler(slog.Default(), &fakeUserUseCase{}),
		AuthHandler:    handler.NewAuthHandler(slog.Default(), authUseCase, false),
		Authenticate:   auth.Authenticate,
		LoginRateLimit: middleware.NewRateLimiter(1, time.Minute).Middleware,
	}).Register(mux)

	body := `{"email":"alice@example.com","password":"secret123"}`
	first := serveWithCookie(t, mux, http.MethodPost, "/auth/login", body, "")
	if first.Code != http.StatusOK {
		t.Fatalf("first login status = %d, want %d", first.Code, http.StatusOK)
	}

	second := serveWithCookie(t, mux, http.MethodPost, "/auth/login", body, "")
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second login status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Error("rate limited login missing Retry-After header")
	}
}

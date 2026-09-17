package middleware_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/middleware"
	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/token"
)

type fakeParser struct {
	claims token.AccessClaims
	err    error
}

func (f fakeParser) ParseAccess(string) (token.AccessClaims, error) {
	return f.claims, f.err
}

type fakeLookup struct {
	user entity.User
	err  error
}

func (f fakeLookup) FindUserByPublicID(_ context.Context, _ string) (entity.User, error) {
	return f.user, f.err
}

func authRequest(t *testing.T, h http.Handler, publicID, cookieValue string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	if publicID != "" {
		req.SetPathValue("publicID", publicID)
	}
	if cookieValue != "" {
		req.AddCookie(&http.Cookie{Name: handler.AccessTokenCookie, Value: cookieValue})
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func TestAuthenticateRejectsBadRequests(t *testing.T) {
	activeUser := entity.User{PublicID: "YTP-000001", Role: entity.RoleMember, IsActive: true}

	cases := []struct {
		name   string
		parser fakeParser
		lookup fakeLookup
		cookie string
	}{
		{
			name:   "missing cookie",
			parser: fakeParser{},
			lookup: fakeLookup{user: activeUser},
		},
		{
			name:   "invalid token",
			parser: fakeParser{err: token.ErrInvalidToken},
			lookup: fakeLookup{user: activeUser},
			cookie: "bad",
		},
		{
			name:   "expired token",
			parser: fakeParser{err: token.ErrInvalidToken},
			lookup: fakeLookup{user: activeUser},
			cookie: "expired",
		},
		{
			name:   "deleted user",
			parser: fakeParser{claims: token.AccessClaims{PublicID: "YTP-000001", Role: entity.RoleMember}},
			lookup: fakeLookup{err: repository.ErrUserNotFound},
			cookie: "good",
		},
		{
			name:   "inactive user",
			parser: fakeParser{claims: token.AccessClaims{PublicID: "YTP-000001", Role: entity.RoleMember}},
			lookup: fakeLookup{user: entity.User{PublicID: "YTP-000001", IsActive: false}},
			cookie: "good",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := middleware.NewAuthenticator(tc.parser, tc.lookup, slog.Default())
			called := false
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })

			rec := authRequest(t, auth.Authenticate(next), "", tc.cookie)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
			if called {
				t.Error("next handler was called for a rejected request")
			}
		})
	}
}

func TestAuthenticateStoresClaims(t *testing.T) {
	claims := token.AccessClaims{UserID: 4, PublicID: "YTP-000004", Role: entity.RoleAdmin}
	auth := middleware.NewAuthenticator(
		fakeParser{claims: claims},
		fakeLookup{user: entity.User{PublicID: "YTP-000004", Role: entity.RoleAdmin, IsActive: true}},
		slog.Default(),
	)

	var got token.AccessClaims
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = middleware.ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})

	rec := authRequest(t, auth.Authenticate(next), "", "good")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got.PublicID != claims.PublicID || got.Role != entity.RoleAdmin {
		t.Errorf("claims = %+v, want %+v", got, claims)
	}
}

func TestRequireAdmin(t *testing.T) {
	cases := []struct {
		name string
		role entity.Role
		want int
	}{
		{name: "admin", role: entity.RoleAdmin, want: http.StatusNoContent},
		{name: "member", role: entity.RoleMember, want: http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := middleware.NewAuthenticator(
				fakeParser{claims: token.AccessClaims{PublicID: "YTP-000001", Role: tc.role}},
				fakeLookup{user: entity.User{PublicID: "YTP-000001", Role: tc.role, IsActive: true}},
				slog.Default(),
			)
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})

			rec := authRequest(t, auth.Authenticate(auth.RequireAdmin(next)), "", "good")

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestRequireAdminRejectsMissingClaims(t *testing.T) {
	auth := middleware.NewAuthenticator(fakeParser{}, fakeLookup{}, slog.Default())
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

	rec := authRequest(t, auth.RequireAdmin(next), "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireSelf(t *testing.T) {
	cases := []struct {
		name     string
		role     entity.Role
		publicID string
		pathID   string
		want     int
	}{
		{name: "owner", role: entity.RoleMember, publicID: "YTP-000001", pathID: "YTP-000001", want: http.StatusNoContent},
		{name: "admin", role: entity.RoleAdmin, publicID: "YTP-000009", pathID: "YTP-000001", want: http.StatusNoContent},
		{name: "other member", role: entity.RoleMember, publicID: "YTP-000002", pathID: "YTP-000001", want: http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := middleware.NewAuthenticator(
				fakeParser{claims: token.AccessClaims{PublicID: tc.publicID, Role: tc.role}},
				fakeLookup{user: entity.User{PublicID: tc.publicID, Role: tc.role, IsActive: true}},
				slog.Default(),
			)
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})

			rec := authRequest(t, auth.Authenticate(auth.RequireSelf(next)), tc.pathID, "good")

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

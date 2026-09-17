package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/model"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

type fakeAuthUseCase struct {
	loginResult usecase.LoginResult
	loginErr    error

	refreshResult usecase.AuthTokens
	refreshErr    error

	logoutErr error

	lastLoginInput   usecase.LoginInput
	lastRefreshToken string
	lastLogoutToken  string
}

func (f *fakeAuthUseCase) Login(_ context.Context, input usecase.LoginInput) (usecase.LoginResult, error) {
	f.lastLoginInput = input

	return f.loginResult, f.loginErr
}

func (f *fakeAuthUseCase) Refresh(_ context.Context, refreshToken string) (usecase.AuthTokens, error) {
	f.lastRefreshToken = refreshToken

	return f.refreshResult, f.refreshErr
}

func (f *fakeAuthUseCase) Logout(_ context.Context, refreshToken string) error {
	f.lastLogoutToken = refreshToken

	return f.logoutErr
}

func newAuthRouter(uc usecase.AuthUseCase, secure bool) *http.ServeMux {
	mux := http.NewServeMux()
	authHandler := handler.NewAuthHandler(slog.Default(), uc, secure)

	mux.HandleFunc("POST /auth/login", authHandler.Login)
	mux.HandleFunc("POST /auth/refresh", authHandler.Refresh)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)

	return mux
}

func serveAuth(t *testing.T, mux *http.ServeMux, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec
}

func cookiesByName(rec *httptest.ResponseRecorder) map[string]*http.Cookie {
	cookies := make(map[string]*http.Cookie)
	for _, cookie := range rec.Result().Cookies() {
		cookies[cookie.Name] = cookie
	}

	return cookies
}

func authTokensFixture() usecase.AuthTokens {
	return usecase.AuthTokens{
		AccessToken:      "access-value",
		RefreshToken:     "refresh-value",
		AccessExpiresAt:  time.Now().Add(15 * time.Minute),
		RefreshExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
}

func TestLoginSetsAuthCookiesAndReturnsUser(t *testing.T) {
	uc := &fakeAuthUseCase{loginResult: usecase.LoginResult{
		AuthTokens: authTokensFixture(),
		User: entity.User{
			PublicID: "YTP-000001",
			Email:    "alice@example.com",
			Password: "$2a$10$super-secret-hash",
			Role:     entity.RoleMember,
			IsActive: true,
		},
	}}

	rec := serveAuth(t, newAuthRouter(uc, false), http.MethodPost, "/auth/login", `{
		"email": "  Alice@Example.com ",
		"password": "secret123"
	}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if uc.lastLoginInput.Email != "  Alice@Example.com " {
		t.Errorf("email = %q, want raw input passed to use case", uc.lastLoginInput.Email)
	}

	cookies := cookiesByName(rec)
	access := cookies[handler.AccessTokenCookie]
	if access == nil {
		t.Fatal("access_token cookie missing")
	}
	if access.Value != "access-value" || !access.HttpOnly {
		t.Errorf("access cookie = %+v, want opaque value and HttpOnly", access)
	}
	if access.SameSite != http.SameSiteLaxMode {
		t.Errorf("access SameSite = %v, want Lax", access.SameSite)
	}
	if access.Path != "/" {
		t.Errorf("access Path = %q, want /", access.Path)
	}
	if access.Secure {
		t.Error("access Secure = true, want false outside production")
	}
	if access.Expires.Before(time.Now()) {
		t.Error("access cookie already expired")
	}

	refresh := cookies[handler.RefreshTokenCookie]
	if refresh == nil {
		t.Fatal("refresh_token cookie missing")
	}
	if refresh.Path != "/auth" {
		t.Errorf("refresh Path = %q, want /auth", refresh.Path)
	}
	if !refresh.HttpOnly {
		t.Error("refresh cookie is not HttpOnly")
	}

	var body model.UserResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.PublicID != "YTP-000001" {
		t.Errorf("public_id = %q, want YTP-000001", body.PublicID)
	}
	if strings.Contains(rec.Body.String(), "super-secret-hash") {
		t.Error("response leaked password hash")
	}
}

func TestLoginMarksCookiesSecureInProduction(t *testing.T) {
	uc := &fakeAuthUseCase{loginResult: usecase.LoginResult{AuthTokens: authTokensFixture()}}

	rec := serveAuth(t, newAuthRouter(uc, true), http.MethodPost, "/auth/login", `{
		"email": "alice@example.com",
		"password": "secret123"
	}`)

	for _, cookie := range cookiesByName(rec) {
		if !cookie.Secure {
			t.Errorf("cookie %s Secure = false, want true in production", cookie.Name)
		}
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	uc := &fakeAuthUseCase{loginErr: usecase.ErrInvalidCredentials}

	rec := serveAuth(t, newAuthRouter(uc, false), http.MethodPost, "/auth/login", `{
		"email": "alice@example.com",
		"password": "wrong"
	}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var body model.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error != "invalid credentials" {
		t.Errorf("error = %q, want invalid credentials", body.Error)
	}
}

func TestLoginRejectsMalformedBody(t *testing.T) {
	rec := serveAuth(t, newAuthRouter(&fakeAuthUseCase{}, false), http.MethodPost, "/auth/login", "{")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRefreshRotatesCookies(t *testing.T) {
	uc := &fakeAuthUseCase{refreshResult: usecase.AuthTokens{
		AccessToken:      "new-access",
		RefreshToken:     "new-refresh",
		AccessExpiresAt:  time.Now().Add(15 * time.Minute),
		RefreshExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}}
	mux := newAuthRouter(uc, false)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: handler.RefreshTokenCookie, Value: "old-refresh"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if uc.lastRefreshToken != "old-refresh" {
		t.Errorf("refresh token = %q, want old-refresh", uc.lastRefreshToken)
	}
	cookies := cookiesByName(rec)
	if cookies[handler.AccessTokenCookie].Value != "new-access" {
		t.Error("access cookie was not rotated")
	}
	if cookies[handler.RefreshTokenCookie].Value != "new-refresh" {
		t.Error("refresh cookie was not rotated")
	}
}

func TestRefreshRejectsMissingCookie(t *testing.T) {
	rec := serveAuth(t, newAuthRouter(&fakeAuthUseCase{}, false), http.MethodPost, "/auth/refresh", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRefreshRejectsInvalidToken(t *testing.T) {
	uc := &fakeAuthUseCase{refreshErr: usecase.ErrInvalidRefreshToken}
	mux := newAuthRouter(uc, false)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: handler.RefreshTokenCookie, Value: "stale"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLogoutRevokesTokenAndClearsCookies(t *testing.T) {
	uc := &fakeAuthUseCase{}
	mux := newAuthRouter(uc, false)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: handler.RefreshTokenCookie, Value: "refresh-value"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if uc.lastLogoutToken != "refresh-value" {
		t.Errorf("logout token = %q, want refresh-value", uc.lastLogoutToken)
	}

	cookies := cookiesByName(rec)
	for _, name := range []string{handler.AccessTokenCookie, handler.RefreshTokenCookie} {
		cookie, ok := cookies[name]
		if !ok {
			t.Fatalf("%s clearing cookie missing", name)
		}
		if cookie.Value != "" || cookie.MaxAge >= 0 {
			t.Errorf("%s cookie = %+v, want cleared", name, cookie)
		}
	}
}

func TestLogoutClearsCookiesEvenWithoutRefreshCookie(t *testing.T) {
	rec := serveAuth(t, newAuthRouter(&fakeAuthUseCase{}, false), http.MethodPost, "/auth/logout", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if len(cookiesByName(rec)) != 2 {
		t.Error("logout did not clear both auth cookies")
	}
}

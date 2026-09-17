package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/middleware"
)

func corsHandler(next *bool) http.Handler {
	return middleware.CORS([]string{"https://app.example.com"})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			*next = true
			w.WriteHeader(http.StatusNoContent)
		}),
	)
}

func TestCORSAllowsConfiguredOriginWithCredentials(t *testing.T) {
	next := false
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()

	corsHandler(&next).ServeHTTP(rec, req)

	if !next {
		t.Fatal("allowed origin did not reach the next handler")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("allow origin = %q, want the request origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("allow credentials = %q, want true", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("vary = %q, want Origin", got)
	}
}

func TestCORSRejectsDisallowedOrigin(t *testing.T) {
	next := false
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()

	corsHandler(&next).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if next {
		t.Error("disallowed origin reached the next handler")
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("disallowed origin received an allow header")
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("vary = %q, want Origin", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	next := false
	req := httptest.NewRequest(http.MethodOptions, "/auth/login", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	corsHandler(&next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if next {
		t.Error("preflight reached the next handler")
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("preflight missing Access-Control-Allow-Methods")
	}
	if rec.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("preflight missing Access-Control-Allow-Headers")
	}
}

func TestCORSAllowsRequestsWithoutOrigin(t *testing.T) {
	next := false
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	corsHandler(&next).ServeHTTP(rec, req)

	if !next {
		t.Fatal("non-browser request did not reach the next handler")
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("non-browser request received an allow origin header")
	}
}

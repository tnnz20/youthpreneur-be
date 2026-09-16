package route_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/route"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

// stubUserUseCase satisfies usecase.UserUseCase so routing tests can register
// the user routes without reaching a use case.
type stubUserUseCase struct {
	usecase.UserUseCase
}

func newTestRouter() *http.ServeMux {
	mux := http.NewServeMux()
	route.NewRouter(
		handler.NewHealthHandler(slog.Default(), usecase.NewHealthUseCase(repository.NewHealthRepository())),
		handler.NewUserHandler(slog.Default(), stubUserUseCase{}),
	).Register(mux)

	return mux
}

func TestHealthz(t *testing.T) {
	mux := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %q, want %q", body["status"], "ok")
	}
}

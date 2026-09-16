package config

import (
	"log/slog"
	"net/http"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/route"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

// Bootstrap wires application dependencies and returns the HTTP route multiplexer.
func Bootstrap(logger *slog.Logger) *http.ServeMux {
	healthRepo := repository.NewHealthRepository()
	healthUsecase := usecase.NewHealthUseCase(healthRepo)
	healthHandler := handler.NewHealthHandler(logger, healthUsecase)

	mux := http.NewServeMux()
	route.NewRouter(healthHandler).Register(mux)

	return mux
}

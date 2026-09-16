package config

import (
	"net/http"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/route"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

func Bootstrap() *http.ServeMux {
	healthRepo := repository.NewHealthRepository()
	healthUsecase := usecase.NewHealthUsecase(healthRepo)
	healthHandler := handler.NewHealthHandler(healthUsecase)

	mux := http.NewServeMux()
	route.NewRouter(healthHandler).Register(mux)

	return mux
}

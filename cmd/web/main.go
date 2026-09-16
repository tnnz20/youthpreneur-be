package main

import (
	"log"
	"net/http"

	"github.com/tnnz20/youthpreneur-be/internal/config"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/route"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

func main() {
	cfg := config.Load()

	healthRepo := repository.NewHealthRepository()
	healthUsecase := usecase.NewHealthUsecase(healthRepo)
	healthHandler := handler.NewHealthHandler(healthUsecase)

	mux := http.NewServeMux()
	route.NewRouter(healthHandler).Register(mux)

	log.Printf("listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, mux); err != nil {
		log.Fatal(err)
	}
}

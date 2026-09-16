package main

import (
	"errors"
	"net/http"
	"os"

	"github.com/tnnz20/youthpreneur-be/internal/config"
)

func main() {
	cfg := config.Load()
	logger := config.NewLogger(cfg.LogLevel)
	mux := config.Bootstrap(logger)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}

	logger.Info("listening", "addr", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

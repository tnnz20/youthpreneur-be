package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/tnnz20/youthpreneur-be/internal/config"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

// run owns the server lifecycle so every exit path returns through the deferred
// cleanup instead of calling os.Exit and skipping it.
func run() error {
	cfg := config.Load()
	logger := config.NewLogger(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mux, db, err := config.Bootstrap(ctx, cfg, logger)
	if err != nil {
		logger.Error("bootstrap failed", "error", err)
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("closing database", "error", err)
		}
	}()

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}

	logger.Info("server starting",
		"environment", cfg.Environment,
		"version", cfg.Version,
		"addr", cfg.Addr,
	)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			return err
		}
	case <-ctx.Done():
		logger.Info("server shutting down", "timeout", cfg.ShutdownTimeout)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			return err
		}
	}

	return nil
}

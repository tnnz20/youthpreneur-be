package config

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/route"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/repository/persistence"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

const postgresPingTimeout = 5 * time.Second

// Bootstrap wires application dependencies and returns the HTTP route
// multiplexer together with the PostgreSQL connection it owns. The caller must
// close the returned database when the server stops.
//
// Bootstrap fails when PostgreSQL cannot be reached, so startup errors surface
// immediately instead of at the first request.
func Bootstrap(ctx context.Context, cfg Config, logger *slog.Logger) (*http.ServeMux, *sql.DB, error) {
	db, err := openPostgres(ctx, cfg.Postgres)
	if err != nil {
		return nil, nil, err
	}

	healthRepo := repository.NewHealthRepository()
	healthUsecase := usecase.NewHealthUseCase(healthRepo)
	healthHandler := handler.NewHealthHandler(logger, healthUsecase)

	userRepo := persistence.NewUserRepository(db)
	userUsecase := usecase.NewUserUseCase(userRepo)
	userHandler := handler.NewUserHandler(logger, userUsecase)

	mux := http.NewServeMux()
	route.NewRouter(healthHandler, userHandler).Register(mux)

	return mux, db, nil
}

func openPostgres(ctx context.Context, cfg PostgresConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, postgresPingTimeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		// Closing after a failed ping is best-effort cleanup.
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

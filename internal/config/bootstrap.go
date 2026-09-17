package config

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/middleware"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/route"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/repository/persistence"
	"github.com/tnnz20/youthpreneur-be/internal/token"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

const postgresPingTimeout = 5 * time.Second

// rateLimitWindow is the fixed window applied to every rate limiter.
const rateLimitWindow = time.Minute

// Bootstrap wires application dependencies and returns the HTTP handler
// together with the PostgreSQL connection it owns. The caller must close the
// returned database when the server stops.
//
// Bootstrap fails when configuration is unsafe or PostgreSQL cannot be reached,
// so startup errors surface immediately instead of at the first request.
func Bootstrap(ctx context.Context, cfg Config, logger *slog.Logger) (http.Handler, *sql.DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}

	db, err := openPostgres(ctx, cfg.Postgres)
	if err != nil {
		return nil, nil, err
	}

	healthRepo := repository.NewHealthRepository()
	healthUsecase := usecase.NewHealthUseCase(healthRepo)
	healthHandler := handler.NewHealthHandler(logger, healthUsecase)

	userRepo := persistence.NewUserRepository(db)
	sessionRepo := persistence.NewRefreshSessionRepository(db)
	userUsecase := usecase.NewUserUseCase(userRepo, sessionRepo)
	userHandler := handler.NewUserHandler(logger, userUsecase)

	tokenService := token.NewService(cfg.Auth.Secret, cfg.Auth.AccessTokenTTL)
	authUsecase := usecase.NewAuthUseCase(
		userRepo,
		sessionRepo,
		tokenService,
		cfg.Auth.AccessTokenTTL,
		cfg.Auth.RefreshTokenTTL,
	)
	authHandler := handler.NewAuthHandler(logger, authUsecase, cfg.SecureCookies)

	authenticator := middleware.NewAuthenticator(tokenService, userRepo, logger)

	mux := http.NewServeMux()
	route.NewRouter(route.Dependencies{
		HealthHandler:    healthHandler,
		UserHandler:      userHandler,
		AuthHandler:      authHandler,
		Authenticate:     authenticator.Authenticate,
		RequireAdmin:     authenticator.RequireAdmin,
		RequireSelf:      authenticator.RequireSelf,
		LoginRateLimit:   middleware.NewRateLimiter(cfg.RateLimit.LoginPerMinute, rateLimitWindow).Middleware,
		RefreshRateLimit: middleware.NewRateLimiter(cfg.RateLimit.RefreshPerMinute, rateLimitWindow).Middleware,
	}).Register(mux)

	generalLimit := middleware.NewRateLimiter(cfg.RateLimit.GeneralPerMinute, rateLimitWindow).Middleware

	// Rate limiting wraps CORS so disallowed-origin requests are also counted
	// and limited. CORS policy itself is unchanged.
	return generalLimit(middleware.CORS(cfg.CORS.AllowedOrigins)(mux)), db, nil
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

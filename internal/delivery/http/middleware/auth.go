// Package middleware provides HTTP middleware for authentication,
// authorization, CORS, and rate limiting.
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/token"
)

// AccessTokenParser validates a raw access token and returns its claims.
type AccessTokenParser interface {
	ParseAccess(raw string) (token.AccessClaims, error)
}

// UserLookup loads the active user behind access token claims.
type UserLookup interface {
	FindUserByPublicID(ctx context.Context, publicID string) (entity.User, error)
}

// Authenticator validates access tokens and enforces role and ownership
// authorization.
type Authenticator struct {
	tokens AccessTokenParser
	users  UserLookup
	logger *slog.Logger
}

// NewAuthenticator creates an authenticator backed by tokens and users.
func NewAuthenticator(tokens AccessTokenParser, users UserLookup, logger *slog.Logger) *Authenticator {
	return &Authenticator{tokens: tokens, users: users, logger: logger}
}

// claimsContextKey is the unexported key for typed auth claims in a request
// context.
type claimsContextKey struct{}

// ClaimsFromContext returns the authenticated access claims stored by
// Authenticate.
func ClaimsFromContext(ctx context.Context) (token.AccessClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(token.AccessClaims)

	return claims, ok
}

// Authenticate rejects requests with a missing, invalid, or expired access
// cookie and requests whose user is inactive or deleted. Valid claims are
// stored in the request context.
func (a *Authenticator) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(handler.AccessTokenCookie)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		claims, err := a.tokens.ParseAccess(cookie.Value)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		user, err := a.users.FindUserByPublicID(r.Context(), claims.PublicID)
		if err != nil {
			if !errors.Is(err, repository.ErrUserNotFound) {
				a.logger.Error("loading authenticated user", "error", err)
			}
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !user.IsActive {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin allows only admin claims through and rejects others with 403.
func (a *Authenticator) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if claims.Role != entity.RoleAdmin {
			writeJSONError(w, http.StatusForbidden, "forbidden")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireSelf allows admins and the user whose public ID matches the route
// publicID, rejecting everyone else with 403.
func (a *Authenticator) RequireSelf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if claims.Role == entity.RoleAdmin || claims.PublicID == r.PathValue("publicID") {
			next.ServeHTTP(w, r)
			return
		}

		writeJSONError(w, http.StatusForbidden, "forbidden")
	})
}

// writeJSONError writes the standard error body used across the API.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	body, err := json.Marshal(map[string]string{"error": message})
	if err != nil {
		return
	}
	_, _ = w.Write(append(body, '\n'))
}

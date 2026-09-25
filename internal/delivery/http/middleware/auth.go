// Package middleware provides HTTP middleware for authentication,
// authorization, CORS, and rate limiting.
package middleware

import (
	"context"
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

// identityContextKey is the unexported key for the current database user in a
// request context.
type identityContextKey struct{}

// ClaimsFromContext returns the authenticated access claims stored by
// Authenticate.
func ClaimsFromContext(ctx context.Context) (token.AccessClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(token.AccessClaims)

	return claims, ok
}

// IdentityFromContext returns the current database user loaded by Authenticate.
// Authorization must use this identity so a stale JWT role claim cannot grant
// access after a role change or demotion.
func IdentityFromContext(ctx context.Context) (entity.User, bool) {
	user, ok := ctx.Value(identityContextKey{}).(entity.User)

	return user, ok
}

// Authenticate rejects requests with a missing, invalid, or expired access
// cookie and requests whose user is inactive or deleted. Valid claims are
// stored in the request context.
func (a *Authenticator) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(handler.AccessTokenCookie)
		if err != nil {
			if a.logger != nil {
				a.logger.Debug("auth rejected: missing access token cookie", "error", err)
			}
			handler.WriteError(a.logger, w, http.StatusUnauthorized, "unauthorized")
			return
		}

		claims, err := a.tokens.ParseAccess(cookie.Value)
		if err != nil {
			if a.logger != nil {
				a.logger.Debug("auth rejected: invalid or expired access token", "error", err)
			}
			handler.WriteError(a.logger, w, http.StatusUnauthorized, "unauthorized")
			return
		}

		user, err := a.users.FindUserByPublicID(r.Context(), claims.PublicID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				if a.logger != nil {
					a.logger.Debug("auth rejected: user not found", "public_id", claims.PublicID)
				}
			} else if a.logger != nil {
				a.logger.Error("loading authenticated user", "error", err)
			}
			handler.WriteError(a.logger, w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !user.IsActive {
			if a.logger != nil {
				a.logger.Debug("auth rejected: user is inactive", "public_id", claims.PublicID)
			}
			handler.WriteError(a.logger, w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey{}, claims)
		ctx = context.WithValue(ctx, identityContextKey{}, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin allows only the current database role admin through and rejects
// others with 403. It uses the identity loaded by Authenticate rather than the
// JWT role claim, so a demoted admin loses access immediately.
func (a *Authenticator) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := IdentityFromContext(r.Context())
		if !ok {
			if a.logger != nil {
				a.logger.Debug("require admin rejected: missing identity in context")
			}
			handler.WriteError(a.logger, w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if identity.Role != entity.RoleAdmin {
			if a.logger != nil {
				a.logger.Debug("require admin rejected: user is not admin", "role", identity.Role, "public_id", identity.PublicID)
			}
			handler.WriteError(a.logger, w, http.StatusForbidden, "forbidden")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireSelf allows admins and the user whose public ID matches the route
// publicID, rejecting everyone else with 403. It uses the current database
// identity, not the JWT claims.
func (a *Authenticator) RequireSelf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := IdentityFromContext(r.Context())
		if !ok {
			if a.logger != nil {
				a.logger.Debug("require self rejected: missing identity in context")
			}
			handler.WriteError(a.logger, w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if identity.Role == entity.RoleAdmin || identity.PublicID == r.PathValue("publicID") {
			next.ServeHTTP(w, r)
			return
		}

		if a.logger != nil {
			a.logger.Debug("require self rejected: user is neither admin nor target user", "role", identity.Role, "actor", identity.PublicID, "target", r.PathValue("publicID"))
		}
		handler.WriteError(a.logger, w, http.StatusForbidden, "forbidden")
	})
}

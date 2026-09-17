package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/model"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

// Auth cookie names and paths. The refresh token is scoped to /auth so it is
// not sent to other endpoints.
const (
	AccessTokenCookie  = "access_token"
	RefreshTokenCookie = "refresh_token"
	accessTokenPath    = "/"
	refreshTokenPath   = "/auth"
)

// AuthHandler serves login, refresh, and logout requests and manages auth
// cookies. Token values are never logged.
type AuthHandler struct {
	logger  *slog.Logger
	useCase usecase.AuthUseCase
	secure  bool
}

// NewAuthHandler creates an auth handler. secureCookies controls the cookie
// Secure flag and must be true only over HTTPS (APP_ENV=production).
func NewAuthHandler(logger *slog.Logger, useCase usecase.AuthUseCase, secureCookies bool) *AuthHandler {
	return &AuthHandler{logger: logger, useCase: useCase, secure: secureCookies}
}

// Login handles POST /auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var request model.LoginRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	result, err := h.useCase.Login(r.Context(), usecase.LoginInput{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.setAuthCookies(w, result.AuthTokens)
	writeJSON(h.logger, w, http.StatusOK, toUserResponse(result.User))
}

// Refresh handles POST /auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(RefreshTokenCookie)
	if err != nil {
		WriteError(h.logger, w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	tokens, err := h.useCase.Refresh(r.Context(), cookie.Value)
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.setAuthCookies(w, tokens)
	w.WriteHeader(http.StatusNoContent)
}

// Logout handles POST /auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var logoutErr error
	if cookie, err := r.Cookie(RefreshTokenCookie); err == nil {
		logoutErr = h.useCase.Logout(r.Context(), cookie.Value)
	}

	h.clearAuthCookies(w)
	if logoutErr != nil {
		h.writeUsecaseError(w, logoutErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) setAuthCookies(w http.ResponseWriter, tokens usecase.AuthTokens) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessTokenCookie,
		Value:    tokens.AccessToken,
		Path:     accessTokenPath,
		Expires:  tokens.AccessExpiresAt,
		MaxAge:   maxAge(tokens.AccessExpiresAt),
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookie,
		Value:    tokens.RefreshToken,
		Path:     refreshTokenPath,
		Expires:  tokens.RefreshExpiresAt,
		MaxAge:   maxAge(tokens.RefreshExpiresAt),
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) clearAuthCookies(w http.ResponseWriter) {
	h.clearCookie(w, AccessTokenCookie, accessTokenPath)
	h.clearCookie(w, RefreshTokenCookie, refreshTokenPath)
}

func (h *AuthHandler) clearCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) writeUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, usecase.ErrBadRequest):
		message := "invalid request"
		var badRequest usecase.BadRequestError
		if errors.As(err, &badRequest) && badRequest.Message != "" {
			message = badRequest.Message
		}
		WriteError(h.logger, w, http.StatusBadRequest, message)
	case errors.Is(err, usecase.ErrInvalidCredentials):
		WriteError(h.logger, w, http.StatusUnauthorized, "invalid credentials")
	case errors.Is(err, usecase.ErrInvalidRefreshToken):
		WriteError(h.logger, w, http.StatusUnauthorized, "invalid refresh token")
	default:
		h.logger.Error("auth request failed", "error", err)
		WriteError(h.logger, w, http.StatusInternalServerError, "internal server error")
	}
}

// maxAge converts an absolute expiry to cookie MaxAge seconds, clamped at zero.
func maxAge(expiresAt time.Time) int {
	seconds := int(time.Until(expiresAt).Seconds())
	if seconds < 0 {
		return 0
	}

	return seconds
}

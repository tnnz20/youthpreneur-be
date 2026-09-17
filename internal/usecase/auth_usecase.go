package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

// ErrInvalidRefreshToken indicates the presented refresh token is unknown,
// revoked, replayed, or expired.
var ErrInvalidRefreshToken = errors.New("usecase: invalid refresh token")

// dummyPasswordHash is a valid bcrypt hash compared against when an email is
// unknown, so login spends the same verification time and does not leak
// account existence through timing.
const dummyPasswordHash = "$2a$10$HoGm57z5fZs2p0tAqJiQke7Napdljhb6qmlnIuCq38AVc3CYI27pS"

// TokenService issues access tokens and opaque refresh tokens.
type TokenService interface {
	// IssueAccess returns a signed access token for user, issued at now.
	IssueAccess(user entity.User, now time.Time) (string, error)
	// GenerateRefresh returns a new opaque refresh token.
	GenerateRefresh() (string, error)
	// HashRefresh returns the stored hash of a raw refresh token.
	HashRefresh(raw string) string
}

// LoginInput carries credentials for email/password login.
type LoginInput struct {
	Email    string
	Password string
}

// AuthTokens are the credentials returned to the delivery layer, which sets
// them as cookies. Raw values never belong in logs.
type AuthTokens struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

// LoginResult is a successful login: tokens plus the authenticated user.
type LoginResult struct {
	AuthTokens
	User entity.User
}

// AuthUseCase implements login, refresh rotation, and logout.
type AuthUseCase interface {
	// Login verifies credentials and starts a refresh session. It returns
	// ErrInvalidCredentials for an unknown email, wrong password, or inactive
	// account so callers cannot enumerate accounts.
	Login(ctx context.Context, input LoginInput) (LoginResult, error)
	// Refresh validates and rotates the presented refresh token. It returns
	// ErrInvalidRefreshToken when the token is unknown, revoked, replayed, or
	// expired, or when its user is gone or inactive.
	Refresh(ctx context.Context, refreshToken string) (AuthTokens, error)
	// Logout revokes the presented refresh token. An empty token is a no-op.
	Logout(ctx context.Context, refreshToken string) error
}

type authUsecase struct {
	users      repository.UserRepository
	sessions   repository.RefreshSessionRepository
	tokens     TokenService
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

// NewAuthUseCase creates an auth use case backed by users, sessions, and
// tokens.
func NewAuthUseCase(
	users repository.UserRepository,
	sessions repository.RefreshSessionRepository,
	tokens TokenService,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) AuthUseCase {
	return authUsecase{
		users:      users,
		sessions:   sessions,
		tokens:     tokens,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        time.Now,
	}
}

// Login verifies credentials and starts a refresh session.
func (u authUsecase) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" || input.Password == "" {
		return LoginResult{}, ErrInvalidCredentials
	}

	user, err := u.users.FindUserByEmail(ctx, email)
	if errors.Is(err, repository.ErrUserNotFound) {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(input.Password))
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf("find user by email: %w", err)
	}
	if !user.IsActive {
		return LoginResult{}, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	tokens, err := u.issueTokens(ctx, user)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{AuthTokens: tokens, User: user}, nil
}

// Refresh validates and rotates the presented refresh token.
func (u authUsecase) Refresh(ctx context.Context, refreshToken string) (AuthTokens, error) {
	if refreshToken == "" {
		return AuthTokens{}, ErrInvalidRefreshToken
	}

	oldHash := u.tokens.HashRefresh(refreshToken)
	now := u.now()

	session, err := u.sessions.FindRefreshSession(ctx, oldHash)
	if errors.Is(err, repository.ErrRefreshSessionNotFound) {
		return AuthTokens{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return AuthTokens{}, fmt.Errorf("find refresh session: %w", err)
	}
	if session.RevokedAt != nil || session.ExpiresAt <= now.Unix() {
		return AuthTokens{}, ErrInvalidRefreshToken
	}

	user, err := u.users.FindUserByID(ctx, session.UserID)
	if errors.Is(err, repository.ErrUserNotFound) {
		return AuthTokens{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return AuthTokens{}, fmt.Errorf("find user by id: %w", err)
	}
	if !user.IsActive {
		return AuthTokens{}, ErrInvalidRefreshToken
	}

	// Issue the access token before rotating so a signing failure does not
	// consume the presented session.
	access, err := u.tokens.IssueAccess(user, now)
	if err != nil {
		return AuthTokens{}, err
	}

	refresh, err := u.tokens.GenerateRefresh()
	if err != nil {
		return AuthTokens{}, err
	}
	refreshExpiresAt := now.Add(u.refreshTTL)

	// Rotate re-checks revoked and expired state atomically, so a concurrent
	// replay loses the race and gets ErrRefreshSessionNotFound.
	_, err = u.sessions.RotateRefreshSession(ctx, oldHash, entity.RefreshSession{
		UserID:    user.ID,
		TokenHash: u.tokens.HashRefresh(refresh),
		ExpiresAt: refreshExpiresAt.Unix(),
		CreatedAt: now.Unix(),
	}, now.Unix())
	if errors.Is(err, repository.ErrRefreshSessionNotFound) {
		return AuthTokens{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return AuthTokens{}, fmt.Errorf("rotate refresh session: %w", err)
	}

	return AuthTokens{
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessExpiresAt:  now.Add(u.accessTTL),
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

// Logout revokes the presented refresh token.
func (u authUsecase) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}

	err := u.sessions.RevokeRefreshSession(ctx, u.tokens.HashRefresh(refreshToken), u.now().Unix())
	if err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}

	return nil
}

func (u authUsecase) issueTokens(ctx context.Context, user entity.User) (AuthTokens, error) {
	now := u.now()

	access, err := u.tokens.IssueAccess(user, now)
	if err != nil {
		return AuthTokens{}, err
	}

	refresh, err := u.tokens.GenerateRefresh()
	if err != nil {
		return AuthTokens{}, err
	}
	refreshExpiresAt := now.Add(u.refreshTTL)

	err = u.sessions.CreateRefreshSession(ctx, entity.RefreshSession{
		UserID:    user.ID,
		TokenHash: u.tokens.HashRefresh(refresh),
		ExpiresAt: refreshExpiresAt.Unix(),
		CreatedAt: now.Unix(),
	})
	if err != nil {
		return AuthTokens{}, fmt.Errorf("create refresh session: %w", err)
	}

	return AuthTokens{
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessExpiresAt:  now.Add(u.accessTTL),
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

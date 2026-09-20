package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
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

// comparePassword is the bcrypt comparison used by Login. It is a package
// variable so tests can observe that password verification happens before the
// inactive-account check without relying on timing.
var comparePassword = bcrypt.CompareHashAndPassword

// TokenService issues access tokens, generates opaque refresh tokens, and seals
// refresh tokens for the rotation grace window.
type TokenService interface {
	// IssueAccess returns a signed access token for user, issued at now.
	IssueAccess(user entity.User, now time.Time) (string, error)
	// GenerateRefresh returns a new opaque refresh token.
	GenerateRefresh() (string, error)
	// HashRefresh returns the stored hash of a raw refresh token.
	HashRefresh(raw string) string
	// EncryptRefresh seals a raw refresh token for grace-window storage.
	EncryptRefresh(raw string) ([]byte, error)
	// DecryptRefresh opens a sealed refresh token produced by EncryptRefresh.
	DecryptRefresh(ciphertext []byte) (string, error)
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
	// expired, or when its user is gone or inactive. A duplicate of a token
	// rotated within the grace window returns the same replacement refresh token
	// with a fresh access token. Replaying a token outside grace revokes only
	// that token's family before failing.
	Refresh(ctx context.Context, refreshToken string) (AuthTokens, error)
	// Logout revokes the presented refresh token. An empty token is a no-op.
	Logout(ctx context.Context, refreshToken string) error
}

// refreshGraceWindow bounds how long a just-rotated refresh token keeps serving
// its replacement to duplicate requests. It only covers client retries of a
// normal rotation; security revocations never open a window.
const refreshGraceWindow = 10 * time.Second

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

// NewAuthUseCaseWithClock creates an auth use case with an injectable clock so
// the grace window can be tested deterministically.
func NewAuthUseCaseWithClock(
	users repository.UserRepository,
	sessions repository.RefreshSessionRepository,
	tokens TokenService,
	accessTTL time.Duration,
	refreshTTL time.Duration,
	now func() time.Time,
) AuthUseCase {
	return authUsecase{
		users:      users,
		sessions:   sessions,
		tokens:     tokens,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        now,
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
		_ = comparePassword([]byte(dummyPasswordHash), []byte(input.Password))
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf("find user by email: %w", err)
	}
	// Verify the password before checking IsActive so unknown emails, wrong
	// passwords, and inactive accounts spend the same bcrypt work and cannot be
	// distinguished by timing.
	if err := comparePassword([]byte(user.Password), []byte(input.Password)); err != nil || !user.IsActive {
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

	// Opportunistic cleanup keeps the table bounded without a scheduled job.
	// A cleanup failure must not block an otherwise valid refresh.
	if err := u.sessions.DeleteExpiredRefreshSessions(ctx, now.Unix()); err != nil {
		slog.DebugContext(ctx, "delete refresh sessions cleanup failed", "error", err)
	}

	session, err := u.sessions.FindRefreshSession(ctx, oldHash)
	if errors.Is(err, repository.ErrRefreshSessionNotFound) {
		return AuthTokens{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return AuthTokens{}, fmt.Errorf("find refresh session: %w", err)
	}

	if session.RevokedAt != nil {
		// A duplicate inside the rotation grace window is a client retry, not a
		// replay: serve the stored replacement (never extend the deadline).
		if session.RevocationReason == entity.ReasonRotated &&
			session.GraceUntil != nil && *session.GraceUntil >= now.Unix() {
			return u.serveGraceWindow(ctx, session, now)
		}

		// Reuse outside grace, or any non-rotation revocation, is a replay.
		// Revoke only the token family so an unrelated login chain survives.
		if err := u.sessions.RevokeRefreshSessionFamily(ctx, session.FamilyID, entity.ReasonReplay, now.Unix()); err != nil {
			slog.DebugContext(ctx, "revoke refresh session family failed", "error", err)
		}
		return AuthTokens{}, ErrInvalidRefreshToken
	}
	if session.ExpiresAt <= now.Unix() {
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
	// Seal the replacement before the write so the grace window can only ever
	// reference ciphertext that was built successfully.
	replacementEnc, err := u.tokens.EncryptRefresh(refresh)
	if err != nil {
		return AuthTokens{}, fmt.Errorf("encrypt replacement refresh token: %w", err)
	}
	refreshExpiresAt := now.Add(u.refreshTTL)

	graceUntil := now.Add(refreshGraceWindow).Unix()

	// Rotate re-checks revoked and expired state atomically, so a concurrent
	// replay loses the race and gets ErrRefreshSessionNotFound.
	_, err = u.sessions.RotateRefreshSession(ctx, oldHash, entity.RefreshSession{
		UserID:              user.ID,
		FamilyID:            session.FamilyID,
		TokenHash:           u.tokens.HashRefresh(refresh),
		ExpiresAt:           refreshExpiresAt.Unix(),
		CreatedAt:           now.Unix(),
		RevocationReason:    entity.ReasonRotated,
		GraceUntil:          &graceUntil,
		ReplacementTokenEnc: replacementEnc,
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

// serveGraceWindow returns the replacement stored during rotation together with
// a freshly issued access token. It never writes to the session, so the grace
// deadline cannot be extended by repeated calls. The user is re-checked so a
// deactivation that raced the grace window still fails closed.
func (u authUsecase) serveGraceWindow(
	ctx context.Context,
	session entity.RefreshSession,
	now time.Time,
) (AuthTokens, error) {
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

	refresh, err := u.tokens.DecryptRefresh(session.ReplacementTokenEnc)
	if err != nil {
		return AuthTokens{}, fmt.Errorf("decrypt replacement refresh token: %w", err)
	}

	access, err := u.tokens.IssueAccess(user, now)
	if err != nil {
		return AuthTokens{}, err
	}

	return AuthTokens{
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessExpiresAt:  now.Add(u.accessTTL),
		RefreshExpiresAt: time.Unix(session.ExpiresAt, 0),
	}, nil
}

// Logout revokes the presented refresh token.
func (u authUsecase) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}

	err := u.sessions.RevokeRefreshSession(ctx, u.tokens.HashRefresh(refreshToken), entity.ReasonLogout, u.now().Unix())
	if err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}

	return nil
}

func (u authUsecase) issueTokens(ctx context.Context, user entity.User) (AuthTokens, error) {
	now := u.now()

	// Opportunistic cleanup; a failure must not block login.
	if err := u.sessions.DeleteExpiredRefreshSessions(ctx, now.Unix()); err != nil {
		slog.DebugContext(ctx, "delete refresh sessions cleanup failed", "error", err)
	}

	access, err := u.tokens.IssueAccess(user, now)
	if err != nil {
		return AuthTokens{}, err
	}

	refresh, err := u.tokens.GenerateRefresh()
	if err != nil {
		return AuthTokens{}, err
	}
	refreshExpiresAt := now.Add(u.refreshTTL)

	familyID, err := newFamilyID()
	if err != nil {
		return AuthTokens{}, fmt.Errorf("generate refresh family id: %w", err)
	}

	err = u.sessions.CreateRefreshSession(ctx, entity.RefreshSession{
		UserID:    user.ID,
		FamilyID:  familyID,
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

// newFamilyID returns a random RFC 4122 version 4 UUID used to group every
// session derived from one login.
func newFamilyID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

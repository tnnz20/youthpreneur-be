package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

type fakeTokenService struct {
	accessSeq  int
	refreshSeq int
	accessErr  error
	refreshErr error
}

func (f *fakeTokenService) IssueAccess(_ entity.User, _ time.Time) (string, error) {
	if f.accessErr != nil {
		return "", f.accessErr
	}
	f.accessSeq++

	return fmt.Sprintf("access-%d", f.accessSeq), nil
}

func (f *fakeTokenService) GenerateRefresh() (string, error) {
	if f.refreshErr != nil {
		return "", f.refreshErr
	}
	f.refreshSeq++

	return fmt.Sprintf("refresh-%d", f.refreshSeq), nil
}

func (f *fakeTokenService) HashRefresh(raw string) string {
	return "hash:" + raw
}

type fakeSessionRepository struct {
	sessions map[string]entity.RefreshSession
	nextID   int

	createErr error
	rotateErr error
	revokeErr error

	lastCreated       entity.RefreshSession
	lastRevokedHash   string
	lastRevokedAt     int64
	lastRotatedOld    string
	lastRotatedNext   entity.RefreshSession
	revokeUserCallFor int
}

func newFakeSessionRepository() *fakeSessionRepository {
	return &fakeSessionRepository{sessions: map[string]entity.RefreshSession{}}
}

func (f *fakeSessionRepository) CreateRefreshSession(_ context.Context, session entity.RefreshSession) error {
	if f.createErr != nil {
		return f.createErr
	}

	f.nextID++
	session.ID = f.nextID
	f.sessions[session.TokenHash] = session
	f.lastCreated = session

	return nil
}

func (f *fakeSessionRepository) FindRefreshSession(_ context.Context, tokenHash string) (entity.RefreshSession, error) {
	session, ok := f.sessions[tokenHash]
	if !ok {
		return entity.RefreshSession{}, repository.ErrRefreshSessionNotFound
	}

	return session, nil
}

func (f *fakeSessionRepository) RotateRefreshSession(
	_ context.Context,
	oldTokenHash string,
	next entity.RefreshSession,
	now int64,
) (entity.RefreshSession, error) {
	if f.rotateErr != nil {
		return entity.RefreshSession{}, f.rotateErr
	}

	session, ok := f.sessions[oldTokenHash]
	if !ok || session.RevokedAt != nil || session.ExpiresAt <= now {
		return entity.RefreshSession{}, repository.ErrRefreshSessionNotFound
	}

	f.lastRotatedOld = oldTokenHash
	f.lastRotatedNext = next

	revokedAt := now
	session.RevokedAt = &revokedAt
	session.ReplacedByHash = next.TokenHash
	f.sessions[oldTokenHash] = session

	created := next
	f.nextID++
	created.ID = f.nextID
	created.UserID = session.UserID
	f.sessions[created.TokenHash] = created

	return session, nil
}

func (f *fakeSessionRepository) RevokeRefreshSession(_ context.Context, tokenHash string, revokedAt int64) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}

	f.lastRevokedHash = tokenHash
	f.lastRevokedAt = revokedAt
	if session, ok := f.sessions[tokenHash]; ok && session.RevokedAt == nil {
		session.RevokedAt = &revokedAt
		f.sessions[tokenHash] = session
	}

	return nil
}

func (f *fakeSessionRepository) RevokeUserRefreshSessions(_ context.Context, userID int, revokedAt int64) error {
	f.revokeUserCallFor = userID
	for hash, session := range f.sessions {
		if session.UserID == userID && session.RevokedAt == nil {
			revoked := revokedAt
			session.RevokedAt = &revoked
			f.sessions[hash] = session
		}
	}

	return nil
}

func hashFor(t *testing.T, password string) string {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash fixture: %v", err)
	}

	return string(hash)
}

func newAuthUseCase(
	t *testing.T,
	users *fakeUserRepository,
	sessions *fakeSessionRepository,
) (usecase.AuthUseCase, *fakeTokenService) {
	t.Helper()

	tokens := &fakeTokenService{}

	return usecase.NewAuthUseCase(users, sessions, tokens, 15*time.Minute, 7*24*time.Hour), tokens
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	active := entity.User{
		ID:       1,
		PublicID: "YTP-000001",
		Email:    "alice@example.com",
		Password: hashFor(t, "correct-horse"),
		Role:     entity.RoleMember,
		IsActive: true,
	}

	cases := []struct {
		name  string
		repo  *fakeUserRepository
		input usecase.LoginInput
	}{
		{
			name:  "unknown email",
			repo:  &fakeUserRepository{},
			input: usecase.LoginInput{Email: "missing@example.com", Password: "correct-horse"},
		},
		{
			name:  "wrong password",
			repo:  &fakeUserRepository{findUser: active},
			input: usecase.LoginInput{Email: "alice@example.com", Password: "wrong-horse"},
		},
		{
			name:  "inactive user",
			repo:  &fakeUserRepository{findUser: entity.User{ID: 1, PublicID: "YTP-000001", Password: hashFor(t, "correct-horse"), Role: entity.RoleMember, IsActive: false}},
			input: usecase.LoginInput{Email: "alice@example.com", Password: "correct-horse"},
		},
		{
			name:  "empty password",
			repo:  &fakeUserRepository{findUser: active},
			input: usecase.LoginInput{Email: "alice@example.com"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, _ := newAuthUseCase(t, tc.repo, newFakeSessionRepository())

			if _, err := uc.Login(context.Background(), tc.input); !errors.Is(err, usecase.ErrInvalidCredentials) {
				t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

func TestLoginIssuesTokensAndStoresHashedRefresh(t *testing.T) {
	repo := &fakeUserRepository{findUser: entity.User{
		ID:       7,
		PublicID: "YTP-000007",
		Email:    "alice@example.com",
		Password: hashFor(t, "correct-horse"),
		Role:     entity.RoleAdmin,
		IsActive: true,
	}}
	sessions := newFakeSessionRepository()
	uc, tokens := newAuthUseCase(t, repo, sessions)

	result, err := uc.Login(context.Background(), usecase.LoginInput{Email: "  Alice@Example.com ", Password: "correct-horse"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if result.AccessToken == "" || result.RefreshToken == "" {
		t.Error("Login() returned empty tokens")
	}
	if result.User.PublicID != "YTP-000007" {
		t.Errorf("user = %+v, want authenticated user", result.User)
	}
	if result.RefreshExpiresAt.Before(result.AccessExpiresAt) {
		t.Error("refresh token expiry is before access token expiry")
	}

	wantHash := tokens.HashRefresh(result.RefreshToken)
	if sessions.lastCreated.TokenHash != wantHash {
		t.Errorf("stored hash = %q, want %q", sessions.lastCreated.TokenHash, wantHash)
	}
	if sessions.lastCreated.UserID != 7 {
		t.Errorf("stored user id = %d, want 7", sessions.lastCreated.UserID)
	}
	if sessions.lastCreated.TokenHash == result.RefreshToken {
		t.Error("raw refresh token was stored instead of its hash")
	}
}

func TestRefreshRotatesSessionAndIssuesNewTokens(t *testing.T) {
	user := entity.User{ID: 3, PublicID: "YTP-000003", Role: entity.RoleMember, IsActive: true}
	repo := &fakeUserRepository{findUser: user}
	sessions := newFakeSessionRepository()
	uc, tokens := newAuthUseCase(t, repo, sessions)

	rawOld := "refresh-old"
	if err := sessions.CreateRefreshSession(context.Background(), entity.RefreshSession{
		UserID:    user.ID,
		TokenHash: tokens.HashRefresh(rawOld),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	result, err := uc.Refresh(context.Background(), rawOld)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		t.Error("Refresh() returned empty tokens")
	}
	if result.RefreshToken == rawOld {
		t.Error("Refresh() did not rotate the refresh token")
	}
	if sessions.lastRotatedOld != tokens.HashRefresh(rawOld) {
		t.Errorf("rotated old hash = %q, want %q", sessions.lastRotatedOld, tokens.HashRefresh(rawOld))
	}
	if sessions.lastRotatedNext.TokenHash != tokens.HashRefresh(result.RefreshToken) {
		t.Errorf("rotated next hash = %q, want hash of returned token", sessions.lastRotatedNext.TokenHash)
	}

	if _, err := uc.Refresh(context.Background(), rawOld); !errors.Is(err, usecase.ErrInvalidRefreshToken) {
		t.Fatalf("replayed Refresh() error = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestRefreshAccessTokenFailureDoesNotRotateSession(t *testing.T) {
	user := entity.User{ID: 3, PublicID: "YTP-000003", Role: entity.RoleMember, IsActive: true}
	repo := &fakeUserRepository{findUser: user}
	sessions := newFakeSessionRepository()
	tokens := &fakeTokenService{accessErr: errors.New("signing failed")}
	uc := usecase.NewAuthUseCase(repo, sessions, tokens, 15*time.Minute, 7*24*time.Hour)

	raw := "refresh-live"
	sessions.sessions[tokens.HashRefresh(raw)] = entity.RefreshSession{
		UserID:    user.ID,
		TokenHash: tokens.HashRefresh(raw),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		CreatedAt: time.Now().Unix(),
	}

	if _, err := uc.Refresh(context.Background(), raw); err == nil {
		t.Fatal("Refresh() error = nil, want access signing error")
	}
	if sessions.lastRotatedOld != "" {
		t.Error("session rotated despite access token signing failure")
	}
}

func TestRefreshRejectsUnusableTokens(t *testing.T) {
	now := time.Now().Unix()

	cases := []struct {
		name     string
		sessions func(tokens *fakeTokenService) *fakeSessionRepository
		raw      string
	}{
		{
			name:     "empty token",
			sessions: func(*fakeTokenService) *fakeSessionRepository { return newFakeSessionRepository() },
			raw:      "",
		},
		{
			name:     "unknown token",
			sessions: func(*fakeTokenService) *fakeSessionRepository { return newFakeSessionRepository() },
			raw:      "refresh-missing",
		},
		{
			name: "revoked token",
			sessions: func(tokens *fakeTokenService) *fakeSessionRepository {
				sessions := newFakeSessionRepository()
				revoked := now - 10
				sessions.sessions[tokens.HashRefresh("refresh-revoked")] = entity.RefreshSession{
					UserID:    1,
					TokenHash: tokens.HashRefresh("refresh-revoked"),
					ExpiresAt: now + 3600,
					CreatedAt: now - 20,
					RevokedAt: &revoked,
				}

				return sessions
			},
			raw: "refresh-revoked",
		},
		{
			name: "expired token",
			sessions: func(tokens *fakeTokenService) *fakeSessionRepository {
				sessions := newFakeSessionRepository()
				sessions.sessions[tokens.HashRefresh("refresh-expired")] = entity.RefreshSession{
					UserID:    1,
					TokenHash: tokens.HashRefresh("refresh-expired"),
					ExpiresAt: now - 1,
					CreatedAt: now - 100,
				}

				return sessions
			},
			raw: "refresh-expired",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeUserRepository{findUser: entity.User{ID: 1, PublicID: "YTP-000001", IsActive: true}}
			tokens := &fakeTokenService{}
			sessions := tc.sessions(tokens)
			uc := usecase.NewAuthUseCase(repo, sessions, tokens, 15*time.Minute, 7*24*time.Hour)

			if _, err := uc.Refresh(context.Background(), tc.raw); !errors.Is(err, usecase.ErrInvalidRefreshToken) {
				t.Fatalf("Refresh() error = %v, want ErrInvalidRefreshToken", err)
			}
		})
	}
}

func TestRefreshRejectsInactiveUserWithoutRotating(t *testing.T) {
	repo := &fakeUserRepository{findUser: entity.User{ID: 5, PublicID: "YTP-000005", IsActive: false}}
	sessions := newFakeSessionRepository()
	tokens := &fakeTokenService{}
	uc := usecase.NewAuthUseCase(repo, sessions, tokens, 15*time.Minute, 7*24*time.Hour)

	raw := "refresh-inactive"
	sessions.sessions[tokens.HashRefresh(raw)] = entity.RefreshSession{
		UserID:    5,
		TokenHash: tokens.HashRefresh(raw),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		CreatedAt: time.Now().Unix(),
	}

	if _, err := uc.Refresh(context.Background(), raw); !errors.Is(err, usecase.ErrInvalidRefreshToken) {
		t.Fatalf("Refresh() error = %v, want ErrInvalidRefreshToken", err)
	}
	if sessions.lastRotatedOld != "" {
		t.Error("inactive user session was rotated")
	}
}

func TestRefreshRejectsDeletedUser(t *testing.T) {
	repo := &fakeUserRepository{}
	sessions := newFakeSessionRepository()
	tokens := &fakeTokenService{}
	uc := usecase.NewAuthUseCase(repo, sessions, tokens, 15*time.Minute, 7*24*time.Hour)

	raw := "refresh-deleted"
	sessions.sessions[tokens.HashRefresh(raw)] = entity.RefreshSession{
		UserID:    9,
		TokenHash: tokens.HashRefresh(raw),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		CreatedAt: time.Now().Unix(),
	}

	if _, err := uc.Refresh(context.Background(), raw); !errors.Is(err, usecase.ErrInvalidRefreshToken) {
		t.Fatalf("Refresh() error = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestLogoutRevokesPresentedToken(t *testing.T) {
	sessions := newFakeSessionRepository()
	uc, tokens := newAuthUseCase(t, &fakeUserRepository{}, sessions)

	if err := uc.Logout(context.Background(), "refresh-abc"); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if sessions.lastRevokedHash != tokens.HashRefresh("refresh-abc") {
		t.Errorf("revoked hash = %q, want %q", sessions.lastRevokedHash, tokens.HashRefresh("refresh-abc"))
	}
	if sessions.lastRevokedAt <= 0 {
		t.Errorf("revoked at = %d, want positive epoch seconds", sessions.lastRevokedAt)
	}
}

func TestLogoutIgnoresEmptyToken(t *testing.T) {
	sessions := newFakeSessionRepository()
	uc, _ := newAuthUseCase(t, &fakeUserRepository{}, sessions)

	if err := uc.Logout(context.Background(), ""); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if sessions.lastRevokedHash != "" {
		t.Error("empty token triggered a revoke")
	}
}

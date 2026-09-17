package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

type stubUserRepository struct {
	repository.UserRepository
	user entity.User
}

func (s stubUserRepository) FindUserByEmail(context.Context, string) (entity.User, error) {
	return s.user, nil
}

type stubRefreshSessionRepository struct {
	repository.RefreshSessionRepository
}

type stubTokenService struct{ TokenService }

// TestInactiveAccountLoginComparesPasswordBeforeRejecting locks the fix for
// inactive-account timing: Login must run the bcrypt comparison before the
// IsActive check. A spy counts comparisons, so moving the active check ahead of
// password verification fails deterministically without timing assertions.
func TestInactiveAccountLoginComparesPasswordBeforeRejecting(t *testing.T) {
	original := comparePassword
	t.Cleanup(func() { comparePassword = original })

	comparisons := 0
	comparePassword = func(hash, password []byte) error {
		comparisons++

		return bcrypt.CompareHashAndPassword(hash, password)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash fixture: %v", err)
	}

	uc := NewAuthUseCase(
		stubUserRepository{user: entity.User{
			ID:       1,
			Email:    "alice@example.com",
			Password: string(hash),
			IsActive: false,
		}},
		stubRefreshSessionRepository{},
		stubTokenService{},
		time.Minute,
		time.Hour,
	)

	_, err = uc.Login(context.Background(), LoginInput{Email: "alice@example.com", Password: "correct-horse"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
	if comparisons != 1 {
		t.Fatalf("bcrypt comparisons = %d, want 1 (password verified before inactive rejection)", comparisons)
	}
}

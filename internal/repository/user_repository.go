package repository

import (
	"context"
	"errors"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
)

// Repository errors are surfaced to usecases so they can map persistence
// failures to domain errors.
var (
	// ErrUserNotFound indicates no active user matches the identifier.
	ErrUserNotFound = errors.New("repository: user not found")
	// ErrDuplicateEmail indicates another user already uses the email.
	ErrDuplicateEmail = errors.New("repository: duplicate email")
	// ErrDuplicatePublicID indicates another user already uses the public id.
	ErrDuplicatePublicID = errors.New("repository: duplicate public id")
)

// UserRepository persists users and their profiles.
//
// Reads exclude soft-deleted users. Callers must pass a non-nil context.
type UserRepository interface {
	// CreateUser inserts user and its profile in one transaction and returns
	// the persisted account.
	CreateUser(ctx context.Context, user entity.User) (entity.User, error)
	// FindUserByPublicID returns the active user matching publicID.
	FindUserByPublicID(ctx context.Context, publicID string) (entity.User, error)
	// FindUsers returns active users ordered by ascending id, filtered by
	// filter.
	FindUsers(ctx context.Context, filter entity.UserFilter) ([]entity.User, error)
	// SoftDeleteUser sets deleted_at for the active user matching publicID.
	// Deleting an unknown or already deleted user returns ErrUserNotFound.
	SoftDeleteUser(ctx context.Context, publicID string, deletedAt int64) error
	// UpdateProfile replaces profile fields and returns the persisted account.
	UpdateProfile(ctx context.Context, publicID string, profile entity.Profile, updatedAt int64) (entity.User, error)
	// UpdateStatus sets the active flag and returns the persisted account.
	UpdateStatus(ctx context.Context, publicID string, isActive bool, updatedAt int64) (entity.User, error)
	// UpdatePassword replaces the password hash of the active user.
	UpdatePassword(ctx context.Context, publicID string, passwordHash string, updatedAt int64) error
	// ChangePassword atomically replaces the password hash only when the stored
	// hash still equals currentHash. A zero-row update returns ErrUserNotFound,
	// which callers map to invalid credentials.
	ChangePassword(ctx context.Context, publicID string, currentHash string, newPasswordHash string, updatedAt int64) error
}

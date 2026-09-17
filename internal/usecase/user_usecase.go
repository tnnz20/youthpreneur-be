package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

const (
	publicIDPrefix   = "YTP-"
	publicIDDigits   = 6
	publicIDAttempts = 5
	// publicIDMax is the exclusive upper bound of the six-digit suffix, so the
	// public ID space holds 10^6 values. It is a lookup handle, not an
	// authentication credential, and must not be treated as secret.
	publicIDMax       = 1000000
	minPasswordLength = 8
	// maxPasswordLength is bcrypt's maximum input size in bytes. Longer inputs
	// are rejected instead of being silently truncated.
	maxPasswordLength = 72
	defaultListLimit  = 20
	maxListLimit      = 100
)

// Usecase errors are returned to the delivery layer for HTTP status mapping.
var (
	// ErrBadRequest indicates request data failed validation. The delivery
	// layer maps it to HTTP 400 and surfaces BadRequestError.Message.
	ErrBadRequest = errors.New("usecase: bad request")
	// ErrUserNotFound indicates the user does not exist or is soft deleted.
	ErrUserNotFound = errors.New("usecase: user not found")
	// ErrEmailTaken indicates the email is already registered.
	ErrEmailTaken = errors.New("usecase: email already registered")
	// ErrInvalidCredentials indicates the supplied current password is wrong.
	ErrInvalidCredentials = errors.New("usecase: invalid credentials")
	// ErrPublicIDGeneration indicates every generated public id collided, which
	// means the six-digit space is effectively exhausted for this deployment.
	ErrPublicIDGeneration = errors.New("usecase: unable to generate unique public id")
)

// BadRequestError carries a client-safe validation message. Error returns only
// Message so the internal sentinel prefix never reaches API clients. It unwraps
// to ErrBadRequest so callers can still match with errors.Is.
type BadRequestError struct {
	Message string
}

// Error returns the client-safe validation message.
func (e BadRequestError) Error() string {
	return e.Message
}

// Unwrap exposes ErrBadRequest for errors.Is checks.
func (e BadRequestError) Unwrap() error {
	return ErrBadRequest
}

func badRequest(message string) error {
	return BadRequestError{Message: message}
}

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// CreateUserInput carries the data needed to create a user.
type CreateUserInput struct {
	Email    string
	Password string
	Profile  entity.Profile
}

// ChangePasswordInput carries the current and replacement passwords.
type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

// FindUsersInput bounds and filters a user listing request.
type FindUsersInput struct {
	District string
	Gender   string
	Cursor   int
	Limit    int
}

// FindUsersResult is one page of users plus the cursor for the following page.
// NextCursor is zero when no further page exists.
type FindUsersResult struct {
	Users      []entity.User
	NextCursor int
}

// UserUseCase implements user and profile operations.
type UserUseCase interface {
	// CreateUser validates input, hashes the password, and registers a new
	// member account. It returns ErrBadRequest on validation failure,
	// ErrEmailTaken when the email is registered, and ErrPublicIDGeneration
	// when no unique public id can be allocated.
	CreateUser(ctx context.Context, input CreateUserInput) (entity.User, error)
	// DeleteUser soft deletes the active user matching publicID.
	// It returns ErrUserNotFound when no active user matches.
	DeleteUser(ctx context.Context, publicID string) error
	// GetUser returns the active user matching publicID.
	// It returns ErrUserNotFound when no active user matches.
	GetUser(ctx context.Context, publicID string) (entity.User, error)
	// UpdateProfile replaces the profile fields of the active user matching
	// publicID. It returns ErrBadRequest for an invalid gender and
	// ErrUserNotFound when no active user matches.
	UpdateProfile(ctx context.Context, publicID string, profile entity.Profile) (entity.User, error)
	// UpdateStatus activates or deactivates the active user matching publicID.
	// It returns ErrUserNotFound when no active user matches.
	UpdateStatus(ctx context.Context, publicID string, isActive bool) (entity.User, error)
	// ChangePassword replaces the password of the active user matching publicID
	// after verifying input.CurrentPassword. It returns ErrBadRequest when the
	// new password is invalid and ErrInvalidCredentials when the current
	// password is wrong or the stored hash changed concurrently.
	ChangePassword(ctx context.Context, publicID string, input ChangePasswordInput) error
	// ResetPassword sets a new password without verifying the current one. It
	// returns ErrBadRequest when the new password is invalid and
	// ErrUserNotFound when no active user matches.
	ResetPassword(ctx context.Context, publicID string, newPassword string) error
	// FindUsers returns one page of active users matching input. It returns
	// ErrBadRequest for an invalid gender.
	FindUsers(ctx context.Context, input FindUsersInput) (FindUsersResult, error)
}

type userUsecase struct {
	repo     repository.UserRepository
	sessions repository.RefreshSessionRepository
	now      func() int64
}

// NewUserUseCase creates a user use case backed by repo and sessions. sessions
// is used to revoke refresh sessions after password and account security
// changes.
func NewUserUseCase(repo repository.UserRepository, sessions repository.RefreshSessionRepository) UserUseCase {
	return userUsecase{
		repo:     repo,
		sessions: sessions,
		now:      func() int64 { return time.Now().Unix() },
	}
}

// CreateUser validates input, assigns a generated public id, and persists a
// new member account.
func (u userUsecase) CreateUser(ctx context.Context, input CreateUserInput) (entity.User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))

	// Never trust a client-supplied role. Self-registration always creates a
	// member; admin provisioning needs a trusted path such as a future
	// authenticated admin endpoint.
	role := entity.RoleMember

	profile := sanitizeProfile(input.Profile)
	if err := validateCreateUser(email, input.Password, profile); err != nil {
		return entity.User{}, err
	}

	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		return entity.User{}, err
	}

	now := u.now()
	// Retry a bounded number of times on collision. When all attempts fail the
	// six-digit space is effectively exhausted and we fail clearly instead of
	// looping forever; see the public ID ceiling note on publicIDMax.
	for range publicIDAttempts {
		publicID, err := generatePublicID()
		if err != nil {
			return entity.User{}, fmt.Errorf("generate public id: %w", err)
		}

		user := entity.User{
			PublicID:  publicID,
			Email:     email,
			Password:  passwordHash,
			Role:      role,
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
			Profile:   &profile,
		}

		created, err := u.repo.CreateUser(ctx, user)
		switch {
		case errors.Is(err, repository.ErrDuplicatePublicID):
			continue
		case errors.Is(err, repository.ErrDuplicateEmail):
			return entity.User{}, ErrEmailTaken
		case err != nil:
			return entity.User{}, fmt.Errorf("create user: %w", err)
		}

		return created, nil
	}

	return entity.User{}, ErrPublicIDGeneration
}

// DeleteUser soft deletes the active user matching publicID.
func (u userUsecase) DeleteUser(ctx context.Context, publicID string) error {
	if err := u.repo.SoftDeleteUser(ctx, publicID, u.now()); err != nil {
		return mapRepositoryError(err)
	}

	return nil
}

// GetUser returns the active user matching publicID.
func (u userUsecase) GetUser(ctx context.Context, publicID string) (entity.User, error) {
	user, err := u.repo.FindUserByPublicID(ctx, publicID)
	if err != nil {
		return entity.User{}, mapRepositoryError(err)
	}

	return user, nil
}

// UpdateProfile replaces the profile fields of the active user matching
// publicID.
func (u userUsecase) UpdateProfile(
	ctx context.Context,
	publicID string,
	profile entity.Profile,
) (entity.User, error) {
	profile = sanitizeProfile(profile)
	if profile.FullName == "" {
		return entity.User{}, badRequest("full_name is required")
	}
	if err := validateGender(profile.Gender); err != nil {
		return entity.User{}, err
	}

	user, err := u.repo.UpdateProfile(ctx, publicID, profile, u.now())
	if err != nil {
		return entity.User{}, mapRepositoryError(err)
	}

	return user, nil
}

// UpdateStatus activates or deactivates the active user matching publicID.
// Deactivating an account revokes its refresh sessions.
func (u userUsecase) UpdateStatus(
	ctx context.Context,
	publicID string,
	isActive bool,
) (entity.User, error) {
	user, err := u.repo.UpdateStatus(ctx, publicID, isActive, u.now())
	if err != nil {
		return entity.User{}, mapRepositoryError(err)
	}

	if !isActive {
		if err := u.revokeUserSessions(ctx, user.ID); err != nil {
			return entity.User{}, err
		}
	}

	return user, nil
}

// ChangePassword replaces the password of the active user matching publicID
// after verifying the current one.
func (u userUsecase) ChangePassword(
	ctx context.Context,
	publicID string,
	input ChangePasswordInput,
) error {
	if err := validatePassword(input.NewPassword); err != nil {
		return err
	}

	user, err := u.repo.FindUserByPublicID(ctx, publicID)
	if err != nil {
		return mapRepositoryError(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.CurrentPassword)); err != nil {
		return ErrInvalidCredentials
	}

	passwordHash, err := hashPassword(input.NewPassword)
	if err != nil {
		return err
	}

	// The current hash predicate makes the write atomic: if another change
	// lands between the read above and this update, zero rows match and the
	// caller gets invalid credentials instead of silently overwriting the
	// newer password.
	err = u.repo.ChangePassword(ctx, publicID, user.Password, passwordHash, u.now())
	if errors.Is(err, repository.ErrUserNotFound) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return mapRepositoryError(err)
	}

	return u.revokeUserSessions(ctx, user.ID)
}

// ResetPassword sets a new password for the active user matching publicID
// without verifying the current one, then revokes that user's refresh sessions.
func (u userUsecase) ResetPassword(ctx context.Context, publicID string, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}

	user, err := u.repo.FindUserByPublicID(ctx, publicID)
	if err != nil {
		return mapRepositoryError(err)
	}

	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := u.repo.UpdatePassword(ctx, publicID, passwordHash, u.now()); err != nil {
		return mapRepositoryError(err)
	}

	return u.revokeUserSessions(ctx, user.ID)
}

// revokeUserSessions revokes every active refresh session for userID. It wraps
// the repository error so callers do not expose session internals.
func (u userUsecase) revokeUserSessions(ctx context.Context, userID int) error {
	if err := u.sessions.RevokeUserRefreshSessions(ctx, userID, u.now()); err != nil {
		return fmt.Errorf("revoke user refresh sessions: %w", err)
	}

	return nil
}

// FindUsers returns one page of active users matching input.
func (u userUsecase) FindUsers(ctx context.Context, input FindUsersInput) (FindUsersResult, error) {
	gender := entity.Gender(strings.TrimSpace(input.Gender))
	if err := validateGender(gender); err != nil {
		return FindUsersResult{}, err
	}

	limit := clampLimit(input.Limit)
	filter := entity.UserFilter{
		District: strings.TrimSpace(input.District),
		Gender:   gender,
		Cursor:   input.Cursor,
		// Fetch one extra row to detect whether a further page exists.
		Limit: limit + 1,
	}

	users, err := u.repo.FindUsers(ctx, filter)
	if err != nil {
		return FindUsersResult{}, fmt.Errorf("find users: %w", err)
	}

	result := FindUsersResult{Users: users}
	if len(users) > limit {
		result.Users = users[:limit]
		result.NextCursor = users[limit-1].ID
	}

	return result, nil
}

func validateCreateUser(email, password string, profile entity.Profile) error {
	if profile.FullName == "" {
		return badRequest("full_name is required")
	}
	if !emailPattern.MatchString(email) {
		return badRequest("invalid email")
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	if err := validateGender(profile.Gender); err != nil {
		return err
	}

	return nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLength {
		return badRequest(fmt.Sprintf("password must be at least %d characters", minPasswordLength))
	}
	if len(password) > maxPasswordLength {
		return badRequest(fmt.Sprintf("password must be at most %d bytes", maxPasswordLength))
	}

	return nil
}

func validateGender(gender entity.Gender) error {
	if gender == "" || gender == entity.GenderMale || gender == entity.GenderFemale {
		return nil
	}

	return badRequest("invalid gender")
}

// sanitizeProfile trims whitespace so empty input is stored as NULL.
func sanitizeProfile(profile entity.Profile) entity.Profile {
	return entity.Profile{
		FullName:  strings.TrimSpace(profile.FullName),
		NIK:       strings.TrimSpace(profile.NIK),
		BirthDate: profile.BirthDate,
		Gender:    entity.Gender(strings.TrimSpace(string(profile.Gender))),
		District:  strings.TrimSpace(profile.District),
		Phone:     strings.TrimSpace(profile.Phone),
		Address:   strings.TrimSpace(profile.Address),
	}
}

func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultListLimit
	case limit > maxListLimit:
		return maxListLimit
	default:
		return limit
	}
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

// generatePublicID returns a YTP- prefixed identifier with six random decimal
// digits drawn from crypto/rand. It is a lookup handle, not an authentication
// credential; callers must not treat it as secret.
func generatePublicID() (string, error) {
	suffix, err := rand.Int(rand.Reader, big.NewInt(publicIDMax))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s%0*d", publicIDPrefix, publicIDDigits, suffix.Int64()), nil
}

func mapRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrUserNotFound):
		return ErrUserNotFound
	case errors.Is(err, repository.ErrDuplicateEmail):
		return ErrEmailTaken
	default:
		return err
	}
}

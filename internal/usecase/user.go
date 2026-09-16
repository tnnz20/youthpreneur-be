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
	publicIDPrefix    = "YTP-"
	publicIDDigits    = 6
	publicIDAttempts  = 5
	publicIDMax       = 1000000
	minPasswordLength = 8
	defaultListLimit  = 20
	maxListLimit      = 100
)

// Usecase errors are returned to the delivery layer for HTTP status mapping.
var (
	// ErrInvalidInput indicates request data failed validation.
	ErrInvalidInput = errors.New("usecase: invalid input")
	// ErrUserNotFound indicates the user does not exist or is soft deleted.
	ErrUserNotFound = errors.New("usecase: user not found")
	// ErrEmailTaken indicates the email is already registered.
	ErrEmailTaken = errors.New("usecase: email already registered")
	// ErrInvalidCredentials indicates the supplied current password is wrong.
	ErrInvalidCredentials = errors.New("usecase: invalid credentials")
	// ErrPublicIDGeneration indicates every generated public id collided.
	ErrPublicIDGeneration = errors.New("usecase: unable to generate unique public id")
)

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// CreateUserInput carries the data needed to create a user.
type CreateUserInput struct {
	Email    string
	Password string
	Role     string
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
	CreateUser(ctx context.Context, input CreateUserInput) (entity.User, error)
	DeleteUser(ctx context.Context, publicID string) error
	GetUser(ctx context.Context, publicID string) (entity.User, error)
	UpdateProfile(ctx context.Context, publicID string, profile entity.Profile) (entity.User, error)
	UpdateStatus(ctx context.Context, publicID string, isActive bool) (entity.User, error)
	ChangePassword(ctx context.Context, publicID string, input ChangePasswordInput) error
	ResetPassword(ctx context.Context, publicID string, newPassword string) error
	FindUsers(ctx context.Context, input FindUsersInput) (FindUsersResult, error)
}

type userUsecase struct {
	repo repository.UserRepository
	now  func() int64
}

// NewUserUseCase creates a user use case backed by repo.
func NewUserUseCase(repo repository.UserRepository) UserUseCase {
	return userUsecase{
		repo: repo,
		now:  func() int64 { return time.Now().Unix() },
	}
}

func (u userUsecase) CreateUser(ctx context.Context, input CreateUserInput) (entity.User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))

	role := entity.Role(strings.TrimSpace(input.Role))
	if role == "" {
		role = entity.RoleMember
	}

	profile := sanitizeProfile(input.Profile)
	if err := validateCreateUser(email, input.Password, role, profile); err != nil {
		return entity.User{}, err
	}

	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		return entity.User{}, err
	}

	now := u.now()
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

func (u userUsecase) DeleteUser(ctx context.Context, publicID string) error {
	if err := u.repo.SoftDeleteUser(ctx, publicID, u.now()); err != nil {
		return mapRepositoryError(err)
	}

	return nil
}

func (u userUsecase) GetUser(ctx context.Context, publicID string) (entity.User, error) {
	user, err := u.repo.FindUserByPublicID(ctx, publicID)
	if err != nil {
		return entity.User{}, mapRepositoryError(err)
	}

	return user, nil
}

func (u userUsecase) UpdateProfile(
	ctx context.Context,
	publicID string,
	profile entity.Profile,
) (entity.User, error) {
	profile = sanitizeProfile(profile)
	if err := validateGender(profile.Gender); err != nil {
		return entity.User{}, err
	}

	user, err := u.repo.UpdateProfile(ctx, publicID, profile, u.now())
	if err != nil {
		return entity.User{}, mapRepositoryError(err)
	}

	return user, nil
}

func (u userUsecase) UpdateStatus(
	ctx context.Context,
	publicID string,
	isActive bool,
) (entity.User, error) {
	user, err := u.repo.UpdateStatus(ctx, publicID, isActive, u.now())
	if err != nil {
		return entity.User{}, mapRepositoryError(err)
	}

	return user, nil
}

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

	if err := u.repo.UpdatePassword(ctx, publicID, passwordHash, u.now()); err != nil {
		return mapRepositoryError(err)
	}

	return nil
}

func (u userUsecase) ResetPassword(ctx context.Context, publicID string, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}

	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := u.repo.UpdatePassword(ctx, publicID, passwordHash, u.now()); err != nil {
		return mapRepositoryError(err)
	}

	return nil
}

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

func validateCreateUser(email, password string, role entity.Role, profile entity.Profile) error {
	if !emailPattern.MatchString(email) {
		return fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	if role != entity.RoleAdmin && role != entity.RoleMember {
		return fmt.Errorf("%w: invalid role", ErrInvalidInput)
	}
	if err := validateGender(profile.Gender); err != nil {
		return err
	}

	return nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLength {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, minPasswordLength)
	}

	return nil
}

func validateGender(gender entity.Gender) error {
	if gender == "" || gender == entity.GenderMale || gender == entity.GenderFemale {
		return nil
	}

	return fmt.Errorf("%w: invalid gender", ErrInvalidInput)
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
// digits drawn from crypto/rand.
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

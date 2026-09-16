package usecase_test

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

var publicIDPattern = regexp.MustCompile(`^YTP-[0-9]{6}$`)

type fakeUserRepository struct {
	createCalls    int
	createFailures int
	createError    error

	findUser entity.User
	findErr  error

	users      []entity.User
	listErr    error
	lastFilter entity.UserFilter

	softDeleteErr       error
	lastDeletedPublicID string
	lastDeletedAt       int64

	updateProfileErr error
	lastProfile      entity.Profile
	lastUpdatedAt    int64

	updateStatusErr error
	lastIsActive    bool

	updatePasswordErr error
	lastPassword      string
	lastPasswordID    string
}

func (f *fakeUserRepository) CreateUser(_ context.Context, user entity.User) (entity.User, error) {
	f.createCalls++
	if f.createCalls <= f.createFailures {
		return entity.User{}, repository.ErrDuplicatePublicID
	}
	if f.createError != nil {
		return entity.User{}, f.createError
	}

	created := user
	created.ID = f.createCalls

	return created, nil
}

func (f *fakeUserRepository) FindUserByPublicID(_ context.Context, _ string) (entity.User, error) {
	if f.findErr != nil {
		return entity.User{}, f.findErr
	}
	if f.findUser.PublicID == "" {
		return entity.User{}, repository.ErrUserNotFound
	}

	return f.findUser, nil
}

func (f *fakeUserRepository) FindUsers(_ context.Context, filter entity.UserFilter) ([]entity.User, error) {
	f.lastFilter = filter
	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.users, nil
}

func (f *fakeUserRepository) SoftDeleteUser(_ context.Context, publicID string, deletedAt int64) error {
	f.lastDeletedPublicID = publicID
	f.lastDeletedAt = deletedAt

	return f.softDeleteErr
}

func (f *fakeUserRepository) UpdateProfile(
	_ context.Context,
	_ string,
	profile entity.Profile,
	updatedAt int64,
) (entity.User, error) {
	f.lastProfile = profile
	f.lastUpdatedAt = updatedAt
	if f.updateProfileErr != nil {
		return entity.User{}, f.updateProfileErr
	}

	return f.findUser, nil
}

func (f *fakeUserRepository) UpdateStatus(
	_ context.Context,
	_ string,
	isActive bool,
	updatedAt int64,
) (entity.User, error) {
	f.lastIsActive = isActive
	f.lastUpdatedAt = updatedAt
	if f.updateStatusErr != nil {
		return entity.User{}, f.updateStatusErr
	}

	return f.findUser, nil
}

func (f *fakeUserRepository) UpdatePassword(
	_ context.Context,
	publicID string,
	passwordHash string,
	updatedAt int64,
) error {
	f.lastPasswordID = publicID
	f.lastPassword = passwordHash
	f.lastUpdatedAt = updatedAt

	return f.updatePasswordErr
}

func validCreateInput() usecase.CreateUserInput {
	return usecase.CreateUserInput{
		Email:    "  Alice@Example.COM ",
		Password: "secret123",
		Profile: entity.Profile{
			FullName: "  Alice  ",
			District: " Bandung ",
			Phone:    " 08123456789 ",
			Address:  "  Jalan Mawar 1  ",
		},
	}
}

func TestCreateUserGeneratesPublicIDHashesPasswordAndNormalizesInput(t *testing.T) {
	repo := &fakeUserRepository{}
	uc := usecase.NewUserUseCase(repo)

	user, err := uc.CreateUser(context.Background(), validCreateInput())
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if !publicIDPattern.MatchString(user.PublicID) {
		t.Errorf("public id = %q, want format YTP- followed by six digits", user.PublicID)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", user.Email, "alice@example.com")
	}
	if user.Role != entity.RoleMember {
		t.Errorf("role = %q, want %q", user.Role, entity.RoleMember)
	}
	if !user.IsActive {
		t.Error("is_active = false, want true")
	}
	if user.CreatedAt <= 0 || user.UpdatedAt != user.CreatedAt {
		t.Errorf("timestamps = (%d, %d), want positive and equal", user.CreatedAt, user.UpdatedAt)
	}
	if user.Password == "" || user.Password == "secret123" {
		t.Errorf("password = %q, want bcrypt hash", user.Password)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("secret123")); err != nil {
		t.Errorf("stored hash does not match password: %v", err)
	}
	if user.Profile == nil || user.Profile.FullName != "Alice" || user.Profile.District != "Bandung" {
		t.Errorf("profile = %+v, want trimmed fields", user.Profile)
	}
	if user.Profile.Phone != "08123456789" || user.Profile.Address != "Jalan Mawar 1" {
		t.Errorf("profile contact = %+v, want trimmed phone and address", user.Profile)
	}
}

func TestCreateUserRetriesOnPublicIDCollision(t *testing.T) {
	repo := &fakeUserRepository{createFailures: 2}
	uc := usecase.NewUserUseCase(repo)

	user, err := uc.CreateUser(context.Background(), validCreateInput())
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if repo.createCalls != 3 {
		t.Errorf("create calls = %d, want 3", repo.createCalls)
	}
	if !publicIDPattern.MatchString(user.PublicID) {
		t.Errorf("public id = %q, want YTP- plus six digits", user.PublicID)
	}
}

func TestCreateUserStopsAfterPublicIDAttempts(t *testing.T) {
	repo := &fakeUserRepository{createFailures: 100}
	uc := usecase.NewUserUseCase(repo)

	_, err := uc.CreateUser(context.Background(), validCreateInput())
	if !errors.Is(err, usecase.ErrPublicIDGeneration) {
		t.Fatalf("CreateUser() error = %v, want ErrPublicIDGeneration", err)
	}
	if repo.createCalls != 5 {
		t.Errorf("create calls = %d, want 5", repo.createCalls)
	}
}

func TestCreateUserMapsDuplicateEmail(t *testing.T) {
	repo := &fakeUserRepository{createError: repository.ErrDuplicateEmail}
	uc := usecase.NewUserUseCase(repo)

	_, err := uc.CreateUser(context.Background(), validCreateInput())
	if !errors.Is(err, usecase.ErrEmailTaken) {
		t.Fatalf("CreateUser() error = %v, want ErrEmailTaken", err)
	}
}

func TestCreateUserValidation(t *testing.T) {
	cases := []struct {
		name  string
		input usecase.CreateUserInput
	}{
		{
			name:  "invalid email",
			input: usecase.CreateUserInput{Email: "not-an-email", Password: "secret123"},
		},
		{
			name:  "short password",
			input: usecase.CreateUserInput{Email: "alice@example.com", Password: "short"},
		},
		{
			name:  "unknown role",
			input: usecase.CreateUserInput{Email: "alice@example.com", Password: "secret123", Role: "owner"},
		},
		{
			name: "unknown gender",
			input: usecase.CreateUserInput{
				Email:    "alice@example.com",
				Password: "secret123",
				Profile:  entity.Profile{Gender: "other"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeUserRepository{}
			uc := usecase.NewUserUseCase(repo)

			if _, err := uc.CreateUser(context.Background(), tc.input); !errors.Is(err, usecase.ErrInvalidInput) {
				t.Fatalf("CreateUser() error = %v, want ErrInvalidInput", err)
			}
			if repo.createCalls != 0 {
				t.Errorf("create calls = %d, want 0", repo.createCalls)
			}
		})
	}
}

func TestFindUsersReturnsNextCursorOnlyWhenMoreRowsExist(t *testing.T) {
	repo := &fakeUserRepository{users: []entity.User{{ID: 1}, {ID: 2}, {ID: 3}}}
	uc := usecase.NewUserUseCase(repo)

	result, err := uc.FindUsers(context.Background(), usecase.FindUsersInput{Limit: 2})
	if err != nil {
		t.Fatalf("FindUsers() error = %v", err)
	}
	if repo.lastFilter.Limit != 3 {
		t.Errorf("repository limit = %d, want 3", repo.lastFilter.Limit)
	}
	if len(result.Users) != 2 {
		t.Fatalf("users = %d, want 2", len(result.Users))
	}
	if result.NextCursor != 2 {
		t.Errorf("next cursor = %d, want 2", result.NextCursor)
	}
}

func TestFindUsersOmitsCursorOnLastPage(t *testing.T) {
	repo := &fakeUserRepository{users: []entity.User{{ID: 1}, {ID: 2}}}
	uc := usecase.NewUserUseCase(repo)

	result, err := uc.FindUsers(context.Background(), usecase.FindUsersInput{Limit: 5})
	if err != nil {
		t.Fatalf("FindUsers() error = %v", err)
	}
	if result.NextCursor != 0 {
		t.Errorf("next cursor = %d, want 0", result.NextCursor)
	}
	if len(result.Users) != 2 {
		t.Errorf("users = %d, want 2", len(result.Users))
	}
}

func TestFindUsersClampsLimit(t *testing.T) {
	repo := &fakeUserRepository{}
	uc := usecase.NewUserUseCase(repo)

	if _, err := uc.FindUsers(context.Background(), usecase.FindUsersInput{Limit: 1000}); err != nil {
		t.Fatalf("FindUsers() error = %v", err)
	}
	if repo.lastFilter.Limit != 101 {
		t.Errorf("clamped limit = %d, want 101", repo.lastFilter.Limit)
	}

	if _, err := uc.FindUsers(context.Background(), usecase.FindUsersInput{}); err != nil {
		t.Fatalf("FindUsers() error = %v", err)
	}
	if repo.lastFilter.Limit != 21 {
		t.Errorf("default limit = %d, want 21", repo.lastFilter.Limit)
	}
}

func TestFindUsersRejectsUnknownGender(t *testing.T) {
	repo := &fakeUserRepository{}
	uc := usecase.NewUserUseCase(repo)

	if _, err := uc.FindUsers(context.Background(), usecase.FindUsersInput{Gender: "other"}); !errors.Is(err, usecase.ErrInvalidInput) {
		t.Fatalf("FindUsers() error = %v, want ErrInvalidInput", err)
	}
}

func TestGetUserMapsNotFound(t *testing.T) {
	repo := &fakeUserRepository{}
	uc := usecase.NewUserUseCase(repo)

	if _, err := uc.GetUser(context.Background(), "YTP-000001"); !errors.Is(err, usecase.ErrUserNotFound) {
		t.Fatalf("GetUser() error = %v, want ErrUserNotFound", err)
	}
}

func TestDeleteUserMapsNotFound(t *testing.T) {
	repo := &fakeUserRepository{softDeleteErr: repository.ErrUserNotFound}
	uc := usecase.NewUserUseCase(repo)

	if err := uc.DeleteUser(context.Background(), "YTP-000001"); !errors.Is(err, usecase.ErrUserNotFound) {
		t.Fatalf("DeleteUser() error = %v, want ErrUserNotFound", err)
	}
	if repo.lastDeletedPublicID != "YTP-000001" {
		t.Errorf("deleted public id = %q, want YTP-000001", repo.lastDeletedPublicID)
	}
	if repo.lastDeletedAt <= 0 {
		t.Errorf("deleted at = %d, want positive epoch seconds", repo.lastDeletedAt)
	}
}

func TestChangePasswordRejectsWrongCurrentPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash fixture: %v", err)
	}

	repo := &fakeUserRepository{findUser: entity.User{PublicID: "YTP-000001", Password: string(hash)}}
	uc := usecase.NewUserUseCase(repo)

	err = uc.ChangePassword(context.Background(), "YTP-000001", usecase.ChangePasswordInput{
		CurrentPassword: "wrong-horse",
		NewPassword:     "new-secret",
	})
	if !errors.Is(err, usecase.ErrInvalidCredentials) {
		t.Fatalf("ChangePassword() error = %v, want ErrInvalidCredentials", err)
	}
	if repo.lastPassword != "" {
		t.Error("password was updated despite invalid credentials")
	}
}

func TestChangePasswordStoresNewHash(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash fixture: %v", err)
	}

	repo := &fakeUserRepository{findUser: entity.User{PublicID: "YTP-000001", Password: string(hash)}}
	uc := usecase.NewUserUseCase(repo)

	err = uc.ChangePassword(context.Background(), "YTP-000001", usecase.ChangePasswordInput{
		CurrentPassword: "correct-horse",
		NewPassword:     "new-secret",
	})
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if repo.lastPasswordID != "YTP-000001" {
		t.Errorf("updated public id = %q, want YTP-000001", repo.lastPasswordID)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(repo.lastPassword), []byte("new-secret")); err != nil {
		t.Errorf("stored hash does not match new password: %v", err)
	}
}

func TestChangePasswordValidatesNewPasswordBeforeLookup(t *testing.T) {
	repo := &fakeUserRepository{findErr: errors.New("should not be read")}
	uc := usecase.NewUserUseCase(repo)

	err := uc.ChangePassword(context.Background(), "YTP-000001", usecase.ChangePasswordInput{
		CurrentPassword: "correct-horse",
		NewPassword:     "short",
	})
	if !errors.Is(err, usecase.ErrInvalidInput) {
		t.Fatalf("ChangePassword() error = %v, want ErrInvalidInput", err)
	}
}

func TestResetPasswordStoresNewHashWithoutCurrentPassword(t *testing.T) {
	repo := &fakeUserRepository{}
	uc := usecase.NewUserUseCase(repo)

	if err := uc.ResetPassword(context.Background(), "YTP-000001", "new-secret"); err != nil {
		t.Fatalf("ResetPassword() error = %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(repo.lastPassword), []byte("new-secret")); err != nil {
		t.Errorf("stored hash does not match new password: %v", err)
	}
}

func TestUpdateStatusRecordsActiveFlag(t *testing.T) {
	repo := &fakeUserRepository{findUser: entity.User{PublicID: "YTP-000001"}}
	uc := usecase.NewUserUseCase(repo)

	if _, err := uc.UpdateStatus(context.Background(), "YTP-000001", false); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if repo.lastIsActive {
		t.Error("last is_active = true, want false")
	}
}

func TestUpdateProfileRejectsUnknownGender(t *testing.T) {
	repo := &fakeUserRepository{}
	uc := usecase.NewUserUseCase(repo)

	if _, err := uc.UpdateProfile(context.Background(), "YTP-000001", entity.Profile{Gender: "other"}); !errors.Is(err, usecase.ErrInvalidInput) {
		t.Fatalf("UpdateProfile() error = %v, want ErrInvalidInput", err)
	}
}

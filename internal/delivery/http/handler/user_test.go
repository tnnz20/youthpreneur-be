package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/middleware"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/route"
	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/model"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

type fakeUserUseCase struct {
	createResult  entity.User
	createErr     error
	getResult     entity.User
	getErr        error
	deleteErr     error
	listResult    usecase.FindUsersResult
	listErr       error
	lastListInput usecase.FindUsersInput

	lastCreateInput usecase.CreateUserInput
}

func (f *fakeUserUseCase) CreateUser(_ context.Context, input usecase.CreateUserInput) (entity.User, error) {
	f.lastCreateInput = input

	return f.createResult, f.createErr
}

func (f *fakeUserUseCase) DeleteUser(_ context.Context, _ string) error {
	return f.deleteErr
}

func (f *fakeUserUseCase) GetUser(_ context.Context, _ string) (entity.User, error) {
	return f.getResult, f.getErr
}

func (f *fakeUserUseCase) UpdateProfile(_ context.Context, _ string, _ entity.Profile) (entity.User, error) {
	return f.getResult, f.getErr
}

func (f *fakeUserUseCase) UpdateStatus(_ context.Context, _ string, _ bool) (entity.User, error) {
	return f.getResult, f.getErr
}

func (f *fakeUserUseCase) ChangePassword(_ context.Context, _ string, _ usecase.ChangePasswordInput) error {
	return nil
}

func (f *fakeUserUseCase) ResetPassword(_ context.Context, _ string, _ string) error {
	return nil
}

func (f *fakeUserUseCase) FindUsers(_ context.Context, input usecase.FindUsersInput) (usecase.FindUsersResult, error) {
	f.lastListInput = input

	return f.listResult, f.listErr
}

func newTestRouter(uc usecase.UserUseCase) *http.ServeMux {
	mux := http.NewServeMux()
	route.NewRouter(route.Dependencies{
		HealthHandler: handler.NewHealthHandler(slog.Default(), usecase.NewHealthUseCase(repository.NewHealthRepository())),
		UserHandler:   handler.NewUserHandler(slog.Default(), uc),
		AuthHandler:   handler.NewAuthHandler(slog.Default(), stubAuthUseCase{}, middleware.IdentityFromContext, false),
		Authenticate:  passthroughMiddleware,
		RequireAdmin:  passthroughMiddleware,
		RequireSelf:   passthroughMiddleware,
	}).Register(mux)

	return mux
}

// stubAuthUseCase satisfies usecase.AuthUseCase for user route tests that do
// not exercise auth handlers.
type stubAuthUseCase struct {
	usecase.AuthUseCase
}

func passthroughMiddleware(next http.Handler) http.Handler {
	return next
}

func serve(t *testing.T, mux *http.ServeMux, method, target string, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}

	req := httptest.NewRequest(method, target, reader)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec
}

func TestHealthzCompatibility(t *testing.T) {
	rec := serve(t, newTestRouter(&fakeUserUseCase{}), http.MethodGet, "/healthz", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Errorf("body = %q, want {\"status\":\"ok\"}", rec.Body.String())
	}
}

func TestCreateUserReturnsCreatedWithoutSecrets(t *testing.T) {
	uc := &fakeUserUseCase{createResult: entity.User{
		ID:        1,
		PublicID:  "YTP-123456",
		Email:     "alice@example.com",
		Password:  "$2a$10$super-secret-hash",
		Role:      entity.RoleMember,
		IsActive:  true,
		CreatedAt: 1700000000,
		UpdatedAt: 1700000000,
		Profile:   &entity.Profile{FullName: "Alice", District: "Bandung"},
	}}

	rec := serve(t, newTestRouter(uc), http.MethodPost, "/users", `{
		"email": "alice@example.com",
		"password": "secret123",
		"full_name": "Alice"
	}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["public_id"] != "YTP-123456" {
		t.Errorf("public_id = %v, want YTP-123456", body["public_id"])
	}
	if _, ok := body["password"]; ok {
		t.Error("response contains password")
	}
	if _, ok := body["password_hash"]; ok {
		t.Error("response contains password_hash")
	}
	if _, ok := body["PasswordHash"]; ok {
		t.Error("response contains PasswordHash")
	}
	if strings.Contains(rec.Body.String(), "super-secret-hash") {
		t.Error("response leaked password hash")
	}
}

func TestCreateUserMapsContactFields(t *testing.T) {
	uc := &fakeUserUseCase{}
	rec := serve(t, newTestRouter(uc), http.MethodPost, "/users", `{
		"email": "alice@example.com",
		"password": "secret123",
		"phone": "08123456789",
		"address": "Jalan Mawar 1"
	}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if uc.lastCreateInput.Profile.Phone != "08123456789" {
		t.Errorf("phone = %q, want 08123456789", uc.lastCreateInput.Profile.Phone)
	}
	if uc.lastCreateInput.Profile.Address != "Jalan Mawar 1" {
		t.Errorf("address = %q, want Jalan Mawar 1", uc.lastCreateInput.Profile.Address)
	}
}

func TestCreateUserRejectsInvalidBirthDate(t *testing.T) {
	rec := serve(t, newTestRouter(&fakeUserUseCase{}), http.MethodPost, "/users", `{
		"email": "alice@example.com",
		"password": "secret123",
		"birth_date": "14-03-1995"
	}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateUserRejectsFutureBirthDate(t *testing.T) {
	rec := serve(t, newTestRouter(&fakeUserUseCase{}), http.MethodPost, "/users", `{
		"email": "alice@example.com",
		"password": "secret123",
		"birth_date": "2999-01-01"
	}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateUserRejectsOversizedBody(t *testing.T) {
	body := `{"email":"alice@example.com","password":"secret123","full_name":"` +
		strings.Repeat("a", (1<<20)+1) + `"}`

	rec := serve(t, newTestRouter(&fakeUserUseCase{}), http.MethodPost, "/users", body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateUserReturnsSafeValidationMessage(t *testing.T) {
	uc := &fakeUserUseCase{createErr: usecase.BadRequestError{Message: "invalid email"}}

	rec := serve(t, newTestRouter(uc), http.MethodPost, "/users", `{
		"email": "not-an-email",
		"password": "secret123"
	}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body model.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error != "invalid email" {
		t.Errorf("error = %q, want %q", body.Error, "invalid email")
	}
	if strings.Contains(body.Error, "usecase:") {
		t.Errorf("error leaks internal sentinel prefix: %q", body.Error)
	}
}

func TestGetUserReturnsBirthDate(t *testing.T) {
	born := time.Date(1995, time.March, 14, 0, 0, 0, 0, time.UTC)
	uc := &fakeUserUseCase{getResult: entity.User{
		PublicID: "YTP-000001",
		Email:    "alice@example.com",
		Profile: &entity.Profile{
			FullName:  "Alice",
			NIK:       "3273010101010001",
			BirthDate: &born,
			Phone:     "08123456789",
			Address:   "Jalan Mawar 1",
		},
	}}

	rec := serve(t, newTestRouter(uc), http.MethodGet, "/users/YTP-000001", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body model.UserResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Profile == nil || body.Profile.BirthDate != "1995-03-14" {
		t.Errorf("birth_date = %+v, want 1995-03-14", body.Profile)
	}
	if body.Profile.NIK != "3273010101010001" {
		t.Errorf("nik = %q, want 3273010101010001", body.Profile.NIK)
	}
	if body.Profile.Phone != "08123456789" || body.Profile.Address != "Jalan Mawar 1" {
		t.Errorf("profile contact = %+v, want phone and address", body.Profile)
	}
}

func TestCreateUserRejectsMalformedBody(t *testing.T) {
	rec := serve(t, newTestRouter(&fakeUserUseCase{}), http.MethodPost, "/users", "{")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetUserReturnsNotFound(t *testing.T) {
	uc := &fakeUserUseCase{getErr: usecase.ErrUserNotFound}

	rec := serve(t, newTestRouter(uc), http.MethodGet, "/users/YTP-000001", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var body model.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error != "user not found" {
		t.Errorf("error = %q, want %q", body.Error, "user not found")
	}
}

func TestGetUserReturnsUser(t *testing.T) {
	uc := &fakeUserUseCase{getResult: entity.User{PublicID: "YTP-000001", Email: "alice@example.com"}}

	rec := serve(t, newTestRouter(uc), http.MethodGet, "/users/YTP-000001", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body model.UserResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.PublicID != "YTP-000001" {
		t.Errorf("public_id = %q, want YTP-000001", body.PublicID)
	}
}

func TestListUsersParsesCursorFiltersAndReturnsNextCursor(t *testing.T) {
	uc := &fakeUserUseCase{listResult: usecase.FindUsersResult{
		Users:      []entity.User{{ID: 11, PublicID: "YTP-000001"}},
		NextCursor: 11,
	}}

	rec := serve(t, newTestRouter(uc), http.MethodGet, "/users?district=Bandung&gender=male&search=ali&cursor=10&limit=2", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	if uc.lastListInput.District != "Bandung" {
		t.Errorf("district = %q, want Bandung", uc.lastListInput.District)
	}
	if uc.lastListInput.Gender != "male" {
		t.Errorf("gender = %q, want male", uc.lastListInput.Gender)
	}
	if uc.lastListInput.Search != "ali" {
		t.Errorf("search = %q, want ali", uc.lastListInput.Search)
	}
	if uc.lastListInput.Cursor != 10 {
		t.Errorf("cursor = %d, want 10", uc.lastListInput.Cursor)
	}
	if uc.lastListInput.Limit != 2 {
		t.Errorf("limit = %d, want 2", uc.lastListInput.Limit)
	}

	var body model.UserListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.NextCursor != "11" {
		t.Errorf("next_cursor = %q, want 11", body.NextCursor)
	}
	if len(body.Users) != 1 {
		t.Fatalf("users = %d, want 1", len(body.Users))
	}
}

func TestListUsersRejectsMalformedCursor(t *testing.T) {
	rec := serve(t, newTestRouter(&fakeUserUseCase{}), http.MethodGet, "/users?cursor=abc", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestListUsersRejectsNonPositiveLimit(t *testing.T) {
	rec := serve(t, newTestRouter(&fakeUserUseCase{}), http.MethodGet, "/users?limit=0", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestDeleteUserReturnsNoContent(t *testing.T) {
	rec := serve(t, newTestRouter(&fakeUserUseCase{}), http.MethodDelete, "/users/YTP-000001", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

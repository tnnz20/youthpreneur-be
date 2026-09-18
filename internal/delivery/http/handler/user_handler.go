package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/model"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

// birthDateFormat is the ISO 8601 date format used for profile birth dates.
const birthDateFormat = "2006-01-02"

// UserHandler serves HTTP requests for user and profile operations.
type UserHandler struct {
	logger  *slog.Logger
	useCase usecase.UserUseCase
}

// NewUserHandler creates a user handler backed by useCase.
func NewUserHandler(logger *slog.Logger, useCase usecase.UserUseCase) *UserHandler {
	return &UserHandler{logger: logger, useCase: useCase}
}

// Create handles POST /users.
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request model.CreateUserRequest
	if !h.decode(w, r, &request) {
		return
	}

	birthDate, err := parseBirthDate(request.BirthDate)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid birth_date")
		return
	}

	user, err := h.useCase.CreateUser(r.Context(), usecase.CreateUserInput{
		Email:    request.Email,
		Password: request.Password,
		Profile: entity.Profile{
			FullName:  request.FullName,
			NIK:       request.NIK,
			BirthDate: birthDate,
			Gender:    entity.Gender(request.Gender),
			District:  request.District,
			Phone:     request.Phone,
			Address:   request.Address,
		},
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, toUserResponse(user))
}

// List handles GET /users and returns active members only.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	cursor, err := parseCursor(query.Get("cursor"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid cursor")
		return
	}

	limit, err := parseLimit(query.Get("limit"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	result, err := h.useCase.FindUsers(r.Context(), usecase.FindUsersInput{
		District: query.Get("district"),
		Gender:   query.Get("gender"),
		Cursor:   cursor,
		Limit:    limit,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	response := model.UserListResponse{Users: toUserResponses(result.Users)}
	if result.NextCursor != 0 {
		response.NextCursor = strconv.Itoa(result.NextCursor)
	}

	h.writeJSON(w, http.StatusOK, response)
}

// Get handles GET /users/{publicID}.
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	user, err := h.useCase.GetUser(r.Context(), r.PathValue("publicID"))
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, toUserResponse(user))
}

// Delete handles DELETE /users/{publicID}.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.useCase.DeleteUser(r.Context(), r.PathValue("publicID")); err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateProfile handles PUT /users/{publicID}/profile.
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var request model.UpdateProfileRequest
	if !h.decode(w, r, &request) {
		return
	}

	birthDate, err := parseBirthDate(request.BirthDate)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid birth_date")
		return
	}

	user, err := h.useCase.UpdateProfile(r.Context(), r.PathValue("publicID"), entity.Profile{
		FullName:  request.FullName,
		NIK:       request.NIK,
		BirthDate: birthDate,
		Gender:    entity.Gender(request.Gender),
		District:  request.District,
		Phone:     request.Phone,
		Address:   request.Address,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, toUserResponse(user))
}

// UpdateStatus handles PATCH /users/{publicID}/status.
func (h *UserHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	var request model.UpdateStatusRequest
	if !h.decode(w, r, &request) {
		return
	}

	if request.IsActive == nil {
		h.writeError(w, http.StatusBadRequest, "is_active is required")
		return
	}

	user, err := h.useCase.UpdateStatus(r.Context(), r.PathValue("publicID"), *request.IsActive)
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, toUserResponse(user))
}

// ChangePassword handles PUT /users/{publicID}/password.
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var request model.ChangePasswordRequest
	if !h.decode(w, r, &request) {
		return
	}

	err := h.useCase.ChangePassword(r.Context(), r.PathValue("publicID"), usecase.ChangePasswordInput{
		CurrentPassword: request.CurrentPassword,
		NewPassword:     request.NewPassword,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ResetPassword handles POST /users/{publicID}/password/reset. It is the admin
// path for setting another user's password.
func (h *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var request model.ResetPasswordRequest
	if !h.decode(w, r, &request) {
		return
	}

	if err := h.useCase.ResetPassword(r.Context(), r.PathValue("publicID"), request.NewPassword); err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) decode(w http.ResponseWriter, r *http.Request, target any) bool {
	return decodeJSON(w, r, target)
}

func (h *UserHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	writeJSON(h.logger, w, status, payload)
}

func (h *UserHandler) writeError(w http.ResponseWriter, status int, message string) {
	WriteError(h.logger, w, status, message)
}

func (h *UserHandler) writeUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, usecase.ErrBadRequest):
		// Surface only the client-safe validation message; the internal
		// sentinel prefix stays in logs and wrapped details.
		message := "invalid request"
		var badRequest usecase.BadRequestError
		if errors.As(err, &badRequest) && badRequest.Message != "" {
			message = badRequest.Message
		}
		h.writeError(w, http.StatusBadRequest, message)
	case errors.Is(err, usecase.ErrUserNotFound):
		h.writeError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, usecase.ErrEmailTaken):
		h.writeError(w, http.StatusConflict, "email already registered")
	case errors.Is(err, usecase.ErrInvalidCredentials):
		h.writeError(w, http.StatusUnauthorized, "invalid credentials")
	default:
		h.logger.Error("user request failed", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func parseCursor(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}

	cursor, err := strconv.Atoi(raw)
	if err != nil || cursor < 0 {
		return 0, errors.New("cursor must be a non-negative integer")
	}

	return cursor, nil
}

func parseBirthDate(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}

	parsed, err := time.Parse(birthDateFormat, raw)
	if err != nil {
		return nil, err
	}
	if parsed.After(time.Now()) {
		return nil, errors.New("birth date must not be in the future")
	}

	return &parsed, nil
}

func parseLimit(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}

	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, errors.New("limit must be a positive integer")
	}

	return limit, nil
}

func toUserResponse(user entity.User) model.UserResponse {
	response := model.UserResponse{
		PublicID:  user.PublicID,
		Email:     user.Email,
		Role:      string(user.Role),
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	if user.Profile != nil {
		profile := &model.ProfileResponse{
			FullName: user.Profile.FullName,
			NIK:      user.Profile.NIK,
			Gender:   string(user.Profile.Gender),
			District: user.Profile.District,
			Phone:    user.Profile.Phone,
			Address:  user.Profile.Address,
		}
		if user.Profile.BirthDate != nil {
			profile.BirthDate = user.Profile.BirthDate.Format(birthDateFormat)
		}
		response.Profile = profile
	}

	return response
}

func toUserResponses(users []entity.User) []model.UserResponse {
	responses := make([]model.UserResponse, 0, len(users))
	for _, user := range users {
		responses = append(responses, toUserResponse(user))
	}

	return responses
}

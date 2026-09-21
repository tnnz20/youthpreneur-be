package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/model"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

// TrainingEnrollmentHandler serves HTTP requests for training enrollment
// operations.
type TrainingEnrollmentHandler struct {
	logger   *slog.Logger
	useCase  usecase.TrainingEnrollmentUseCase
	identity IdentityFunc
}

// NewTrainingEnrollmentHandler creates a training enrollment handler backed by
// useCase and the identity accessor.
func NewTrainingEnrollmentHandler(
	logger *slog.Logger,
	useCase usecase.TrainingEnrollmentUseCase,
	identity IdentityFunc,
) *TrainingEnrollmentHandler {
	return &TrainingEnrollmentHandler{logger: logger, useCase: useCase, identity: identity}
}

// Create handles POST /training-enrollments.
func (h *TrainingEnrollmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request model.CreateTrainingEnrollmentRequest
	if !h.decode(w, r, &request) {
		return
	}

	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	enrollment, err := h.useCase.Enroll(r.Context(), usecase.CreateTrainingEnrollmentInput{
		Actor:           actor,
		CatalogPublicID: request.CatalogPublicID,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, model.ToTrainingEnrollmentResponse(enrollment))
}

// Cancel handles DELETE /training-enrollments/{publicID}.
func (h *TrainingEnrollmentHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	if err := h.useCase.CancelEnrollment(r.Context(), actor, r.PathValue("publicID")); err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateStatus handles PATCH /training-enrollments/{publicID}/status.
func (h *TrainingEnrollmentHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	var request model.UpdateTrainingEnrollmentStatusRequest
	if !h.decode(w, r, &request) {
		return
	}

	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	enrollment, err := h.useCase.UpdateStatus(r.Context(), usecase.UpdateTrainingEnrollmentStatusInput{
		Actor:    actor,
		PublicID: r.PathValue("publicID"),
		Status:   request.Status,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, model.ToTrainingEnrollmentResponse(enrollment))
}

// ListMine handles GET /training-enrollments/my.
func (h *TrainingEnrollmentHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	input, ok := h.listInput(w, r, actor)
	if !ok {
		return
	}

	result, err := h.useCase.FindMyEnrollments(r.Context(), input)
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeList(w, result)
}

// List handles GET /training-enrollments.
func (h *TrainingEnrollmentHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	input, ok := h.listInput(w, r, actor)
	if !ok {
		return
	}

	result, err := h.useCase.FindAllEnrollments(r.Context(), input)
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeList(w, result)
}

// ListByCatalog handles GET /training-enrollments/catalog/{catalogPublicID}.
func (h *TrainingEnrollmentHandler) ListByCatalog(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	cursor, limit, ok := h.paging(w, r)
	if !ok {
		return
	}

	result, err := h.useCase.FindCatalogEnrollments(r.Context(), usecase.FindCatalogEnrollmentsInput{
		Actor:           actor,
		CatalogPublicID: r.PathValue("catalogPublicID"),
		Status:          r.URL.Query().Get("status"),
		Cursor:          cursor,
		Limit:           limit,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeList(w, result)
}

func (h *TrainingEnrollmentHandler) listInput(
	w http.ResponseWriter,
	r *http.Request,
	actor entity.User,
) (usecase.FindTrainingEnrollmentsInput, bool) {
	cursor, limit, ok := h.paging(w, r)
	if !ok {
		return usecase.FindTrainingEnrollmentsInput{}, false
	}

	return usecase.FindTrainingEnrollmentsInput{
		Actor:  actor,
		Status: r.URL.Query().Get("status"),
		Cursor: cursor,
		Limit:  limit,
	}, true
}

func (h *TrainingEnrollmentHandler) paging(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	query := r.URL.Query()

	cursor, err := parseCursor(query.Get("cursor"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid cursor")
		return 0, 0, false
	}

	limit, err := parseLimit(query.Get("limit"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid limit")
		return 0, 0, false
	}

	return cursor, limit, true
}

func (h *TrainingEnrollmentHandler) writeList(w http.ResponseWriter, result usecase.FindTrainingEnrollmentsResult) {
	response := model.TrainingEnrollmentListResponse{Enrollments: model.ToTrainingEnrollmentResponses(result.Enrollments)}
	if result.NextCursor != 0 {
		response.NextCursor = strconv.Itoa(result.NextCursor)
	}

	h.writeJSON(w, http.StatusOK, response)
}

func (h *TrainingEnrollmentHandler) actor(w http.ResponseWriter, r *http.Request) (entity.User, bool) {
	actor, ok := h.identity(r.Context())
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized")
		return entity.User{}, false
	}

	return actor, true
}

func (h *TrainingEnrollmentHandler) decode(w http.ResponseWriter, r *http.Request, target any) bool {
	return decodeJSON(w, r, target)
}

func (h *TrainingEnrollmentHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	writeJSON(h.logger, w, status, payload)
}

func (h *TrainingEnrollmentHandler) writeError(w http.ResponseWriter, status int, message string) {
	WriteError(h.logger, w, status, message)
}

func (h *TrainingEnrollmentHandler) writeUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, usecase.ErrBadRequest):
		message := "invalid request"
		var badRequest usecase.BadRequestError
		if errors.As(err, &badRequest) && badRequest.Message != "" {
			message = badRequest.Message
		}
		h.writeError(w, http.StatusBadRequest, message)
	case errors.Is(err, usecase.ErrForbidden):
		h.writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, usecase.ErrTrainingEnrollmentNotFound):
		h.writeError(w, http.StatusNotFound, "training enrollment not found")
	case errors.Is(err, usecase.ErrTrainingCatalogNotFound):
		h.writeError(w, http.StatusNotFound, "training catalog not found")
	case errors.Is(err, usecase.ErrAlreadyEnrolled):
		h.writeError(w, http.StatusConflict, "already enrolled in this training")
	case errors.Is(err, usecase.ErrTrainingCatalogFull):
		h.writeError(w, http.StatusConflict, "training catalog is full")
	case errors.Is(err, usecase.ErrTrainingCatalogClosed):
		h.writeError(w, http.StatusConflict, "training catalog is not open for enrollment")
	default:
		h.logger.Error("training enrollment request failed", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

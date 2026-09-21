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

// TrainingCatalogHandler serves HTTP requests for training catalog operations.
type TrainingCatalogHandler struct {
	logger   *slog.Logger
	useCase  usecase.TrainingCatalogUseCase
	identity IdentityFunc
}

// NewTrainingCatalogHandler creates a training catalog handler backed by
// useCase and the identity accessor.
func NewTrainingCatalogHandler(
	logger *slog.Logger,
	useCase usecase.TrainingCatalogUseCase,
	identity IdentityFunc,
) *TrainingCatalogHandler {
	return &TrainingCatalogHandler{logger: logger, useCase: useCase, identity: identity}
}

// List handles GET /training-catalog.
func (h *TrainingCatalogHandler) List(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.useCase.FindTrainingCatalogs(r.Context(), usecase.FindTrainingCatalogsInput{
		Category:       query.Get("category"),
		TrainingStatus: query.Get("training_status"),
		TrainingDate:   query.Get("training_date"),
		TrainingPeriod: query.Get("training_period"),
		Cursor:         cursor,
		Limit:          limit,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	response := model.TrainingCatalogListResponse{Catalogs: model.ToTrainingCatalogResponses(result.Catalogs)}
	if result.NextCursor != 0 {
		response.NextCursor = strconv.Itoa(result.NextCursor)
	}

	h.writeJSON(w, http.StatusOK, response)
}

// Get handles GET /training-catalog/{publicID}.
func (h *TrainingCatalogHandler) Get(w http.ResponseWriter, r *http.Request) {
	catalog, err := h.useCase.GetTrainingCatalog(r.Context(), r.PathValue("publicID"))
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, model.ToTrainingCatalogResponse(catalog))
}

// Create handles POST /training-catalog.
func (h *TrainingCatalogHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request model.CreateTrainingCatalogRequest
	if !h.decode(w, r, &request) {
		return
	}

	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	catalog, err := h.useCase.CreateTrainingCatalog(r.Context(), usecase.CreateTrainingCatalogInput{
		Actor:          actor,
		Name:           request.Name,
		Description:    request.Description,
		PicPhone:       request.PicPhone,
		Category:       request.Category,
		TrainingSlots:  request.TrainingSlots,
		TrainingStatus: request.TrainingStatus,
		Link:           request.Link,
		TrainingDate:   request.TrainingDate,
		TrainingPeriod: request.TrainingPeriod,
		Speaker:        request.Speaker,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, model.ToTrainingCatalogResponse(catalog))
}

// Update handles PATCH /training-catalog/{publicID}.
func (h *TrainingCatalogHandler) Update(w http.ResponseWriter, r *http.Request) {
	var request model.UpdateTrainingCatalogRequest
	if !h.decode(w, r, &request) {
		return
	}

	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	catalog, err := h.useCase.UpdateTrainingCatalog(r.Context(), actor, r.PathValue("publicID"), usecase.UpdateTrainingCatalogInput{
		Name:           request.Name,
		Description:    request.Description,
		PicPhone:       request.PicPhone,
		Category:       request.Category,
		TrainingSlots:  request.TrainingSlots,
		TrainingStatus: request.TrainingStatus,
		Link:           request.Link,
		TrainingDate:   request.TrainingDate,
		TrainingPeriod: request.TrainingPeriod,
		Speaker:        request.Speaker,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, model.ToTrainingCatalogResponse(catalog))
}

// UpdateStatus handles PATCH /training-catalog/{publicID}/status.
func (h *TrainingCatalogHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	var request model.UpdateTrainingCatalogStatusRequest
	if !h.decode(w, r, &request) {
		return
	}

	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	catalog, err := h.useCase.UpdateTrainingCatalogStatus(r.Context(), actor, r.PathValue("publicID"), request.TrainingStatus)
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, model.ToTrainingCatalogResponse(catalog))
}

// Delete handles DELETE /training-catalog/{publicID}.
func (h *TrainingCatalogHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	if err := h.useCase.DeleteTrainingCatalog(r.Context(), actor, r.PathValue("publicID")); err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TrainingCatalogHandler) actor(w http.ResponseWriter, r *http.Request) (entity.User, bool) {
	actor, ok := h.identity(r.Context())
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized")
		return entity.User{}, false
	}

	return actor, true
}

func (h *TrainingCatalogHandler) decode(w http.ResponseWriter, r *http.Request, target any) bool {
	return decodeJSON(w, r, target)
}

func (h *TrainingCatalogHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	writeJSON(h.logger, w, status, payload)
}

func (h *TrainingCatalogHandler) writeError(w http.ResponseWriter, status int, message string) {
	WriteError(h.logger, w, status, message)
}

func (h *TrainingCatalogHandler) writeUsecaseError(w http.ResponseWriter, err error) {
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
	case errors.Is(err, usecase.ErrTrainingCatalogNotFound):
		h.writeError(w, http.StatusNotFound, "training catalog not found")
	default:
		h.logger.Error("training catalog request failed", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

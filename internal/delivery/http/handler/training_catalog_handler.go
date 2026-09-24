package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/model"
	"github.com/tnnz20/youthpreneur-be/internal/service"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

// TrainingCatalogHandler serves HTTP requests for training catalog operations.
type TrainingCatalogHandler struct {
	logger        *slog.Logger
	useCase       usecase.TrainingCatalogUseCase
	uploadService service.UploadService
	identity      IdentityFunc
}

// NewTrainingCatalogHandler creates a training catalog handler backed by
// useCase, uploadService, and the identity accessor.
func NewTrainingCatalogHandler(
	logger *slog.Logger,
	useCase usecase.TrainingCatalogUseCase,
	uploadService service.UploadService,
	identity IdentityFunc,
) *TrainingCatalogHandler {
	return &TrainingCatalogHandler{
		logger:        logger,
		useCase:       useCase,
		uploadService: uploadService,
		identity:      identity,
	}
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

	search := query.Get("search")
	if search == "" {
		search = query.Get("q")
	}

	result, err := h.useCase.FindTrainingCatalogs(r.Context(), usecase.FindTrainingCatalogsInput{
		Search:         search,
		Title:          query.Get("title"),
		Mentor:         query.Get("mentor"),
		Category:       query.Get("category"),
		TrainingStatus: query.Get("training_status"),
		StartDate:      query.Get("start_date"),
		Order:          query.Get("order"),
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
		Title:          request.Title,
		Description:    request.Description,
		PicPhone:       request.PicPhone,
		Category:       request.Category,
		MaxSlots:       request.MaxSlots,
		TrainingStatus: request.TrainingStatus,
		Link:           request.Link,
		Address:        request.Address,
		Thumbnail:      request.Thumbnail,
		StartDate:      request.StartDate,
		EndDate:        request.EndDate,
		Mentor:         request.Mentor,
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
		Title:          request.Title,
		Description:    request.Description,
		PicPhone:       request.PicPhone,
		Category:       request.Category,
		MaxSlots:       request.MaxSlots,
		TrainingStatus: request.TrainingStatus,
		Link:           request.Link,
		Address:        request.Address,
		Thumbnail:      request.Thumbnail,
		StartDate:      request.StartDate,
		EndDate:        request.EndDate,
		Mentor:         request.Mentor,
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

// UploadThumbnail handles POST /training-catalog/upload-thumbnail (admin-only).
func (h *TrainingCatalogHandler) UploadThumbnail(w http.ResponseWriter, r *http.Request) {
	_, ok := h.actor(w, r)
	if !ok {
		return
	}

	const maxMultipartMemory = 5 << 20
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		h.writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "thumbnail file is required")
		return
	}
	defer file.Close()

	url, err := h.uploadService.SaveThumbnail(file, header.Size)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidFileType):
			h.writeError(w, http.StatusBadRequest, "invalid file type: only PNG, JPG, and JPEG are allowed")
		case errors.Is(err, service.ErrFileTooLarge):
			h.writeError(w, http.StatusBadRequest, "file too large: maximum size is 5MB")
		default:
			h.logger.Error("thumbnail upload failed", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	h.writeJSON(w, http.StatusCreated, model.UploadThumbnailResponse{ThumbnailURL: url})
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

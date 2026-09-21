package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/model"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

// IdentityFunc loads the authenticated database identity stored by the
// authentication middleware. It is injected so the handler package does not
// import the middleware package that already depends on it.
type IdentityFunc func(ctx context.Context) (entity.User, bool)

// EnterpriseHandler serves HTTP requests for enterprise operations.
type EnterpriseHandler struct {
	logger   *slog.Logger
	useCase  usecase.EnterpriseUseCase
	identity IdentityFunc
}

// NewEnterpriseHandler creates an enterprise handler backed by useCase and the
// identity accessor.
func NewEnterpriseHandler(
	logger *slog.Logger,
	useCase usecase.EnterpriseUseCase,
	identity IdentityFunc,
) *EnterpriseHandler {
	return &EnterpriseHandler{logger: logger, useCase: useCase, identity: identity}
}

// Create handles POST /enterprises.
func (h *EnterpriseHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request model.CreateEnterpriseRequest
	if !h.decode(w, r, &request) {
		return
	}

	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	enterprise, err := h.useCase.CreateEnterprise(r.Context(), usecase.CreateEnterpriseInput{
		Actor:                actor,
		EnterpriseName:       request.EnterpriseName,
		Description:          request.Description,
		Address:              request.Address,
		FocusCommodity:       request.FocusCommodity,
		BusinessSector:       request.BusinessSector,
		LegalStatus:          request.LegalStatus,
		BusinessDigitization: request.BusinessDigitization,
		InterventionNeeds:    request.InterventionNeeds,
		TrainingStatus:       request.TrainingStatus,
		MentoringStatus:      request.MentoringStatus,
		CapitalAccess:        request.CapitalAccess,
		Partnership:          request.Partnership,
		InitialTurnover:      request.InitialTurnover,
		CurrentTurnover:      request.CurrentTurnover,
		District:             request.District,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, model.ToEnterpriseResponse(enterprise))
}

// List handles GET /enterprises.
func (h *EnterpriseHandler) List(w http.ResponseWriter, r *http.Request) {
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

	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	search := query.Get("search")
	if search == "" {
		search = query.Get("q")
	}

	result, err := h.useCase.FindEnterprises(r.Context(), usecase.FindEnterprisesInput{
		Actor:                actor,
		Search:               search,
		District:             query.Get("district"),
		Status:               query.Get("status"),
		BusinessSector:       query.Get("business_sector"),
		LegalStatus:          query.Get("legal_status"),
		BusinessDigitization: query.Get("business_digitization"),
		InterventionNeeds:    query.Get("intervention_needs"),
		TrainingStatus:       query.Get("training_status"),
		MentoringStatus:      query.Get("mentoring_status"),
		CapitalAccess:        query.Get("capital_access"),
		Partnership:          query.Get("partnership"),
		Cursor:               cursor,
		Limit:                limit,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	response := model.EnterpriseListResponse{Enterprises: model.ToEnterpriseResponses(result.Enterprises)}
	if result.NextCursor != 0 {
		response.NextCursor = strconv.Itoa(result.NextCursor)
	}

	h.writeJSON(w, http.StatusOK, response)
}

// ListPublic handles GET /enterprises/public.
func (h *EnterpriseHandler) ListPublic(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.useCase.FindPublicEnterprises(r.Context(), usecase.FindPublicEnterprisesInput{
		Search:            search,
		District:          query.Get("district"),
		InterventionNeeds: query.Get("intervention_needs"),
		BusinessSector:    query.Get("business_sector"),
		Cursor:            cursor,
		Limit:             limit,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	response := model.PublicEnterpriseListResponse{Enterprises: model.ToPublicEnterpriseResponses(result.Enterprises)}
	if result.NextCursor != 0 {
		response.NextCursor = strconv.Itoa(result.NextCursor)
	}

	h.writeJSON(w, http.StatusOK, response)
}

// Get handles GET /enterprises/{publicID}.
func (h *EnterpriseHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	enterprise, err := h.useCase.GetEnterprise(r.Context(), actor, r.PathValue("publicID"))
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, model.ToEnterpriseResponse(enterprise))
}

// ListAuditLogs handles GET /enterprises/{publicID}/audit-logs.
func (h *EnterpriseHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
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

	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	result, err := h.useCase.FindEnterpriseAuditLogs(r.Context(), usecase.FindEnterpriseAuditLogsInput{
		Actor:    actor,
		PublicID: r.PathValue("publicID"),
		Cursor:   cursor,
		Limit:    limit,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	response := model.EnterpriseAuditListResponse{Events: model.ToEnterpriseAuditEventResponses(result.Events)}
	if result.NextCursor != 0 {
		response.NextCursor = strconv.Itoa(result.NextCursor)
	}

	h.writeJSON(w, http.StatusOK, response)
}

// Update handles PATCH /enterprises/{publicID}.
func (h *EnterpriseHandler) Update(w http.ResponseWriter, r *http.Request) {
	var request model.UpdateEnterpriseRequest
	if !h.decode(w, r, &request) {
		return
	}

	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	enterprise, err := h.useCase.UpdateEnterprise(r.Context(), actor, r.PathValue("publicID"), usecase.UpdateEnterpriseInput{
		EnterpriseName:       request.EnterpriseName,
		Description:          request.Description,
		Address:              request.Address,
		FocusCommodity:       request.FocusCommodity,
		DisporaSupport:       request.DisporaSupport,
		BusinessSector:       request.BusinessSector,
		LegalStatus:          request.LegalStatus,
		BusinessDigitization: request.BusinessDigitization,
		InterventionNeeds:    request.InterventionNeeds,
		TrainingStatus:       request.TrainingStatus,
		MentoringStatus:      request.MentoringStatus,
		CapitalAccess:        request.CapitalAccess,
		Partnership:          request.Partnership,
		InitialTurnover:      request.InitialTurnover,
		CurrentTurnover:      request.CurrentTurnover,
		District:             request.District,
		Status:               request.Status,
	})
	if err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, model.ToEnterpriseResponse(enterprise))
}

// Delete handles DELETE /enterprises/{publicID}.
func (h *EnterpriseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}

	if err := h.useCase.DeleteEnterprise(r.Context(), actor, r.PathValue("publicID")); err != nil {
		h.writeUsecaseError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *EnterpriseHandler) actor(w http.ResponseWriter, r *http.Request) (entity.User, bool) {
	actor, ok := h.identity(r.Context())
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized")
		return entity.User{}, false
	}

	return actor, true
}

func (h *EnterpriseHandler) decode(w http.ResponseWriter, r *http.Request, target any) bool {
	return decodeJSON(w, r, target)
}

func (h *EnterpriseHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	writeJSON(h.logger, w, status, payload)
}

func (h *EnterpriseHandler) writeError(w http.ResponseWriter, status int, message string) {
	WriteError(h.logger, w, status, message)
}

func (h *EnterpriseHandler) writeUsecaseError(w http.ResponseWriter, err error) {
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
	case errors.Is(err, usecase.ErrEnterpriseNotFound):
		h.writeError(w, http.StatusNotFound, "enterprise not found")
	default:
		h.logger.Error("enterprise request failed", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

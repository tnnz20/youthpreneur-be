package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/tnnz20/youthpreneur-be/internal/model"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

// HealthHandler serves HTTP health check requests.
type HealthHandler struct {
	logger  *slog.Logger
	useCase usecase.HealthUseCase
}

// NewHealthHandler creates a health handler backed by uc.
func NewHealthHandler(logger *slog.Logger, useCase usecase.HealthUseCase) *HealthHandler {
	return &HealthHandler{logger: logger, useCase: useCase}
}

// Check handles GET /healthz.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(model.HealthResponse{Status: h.useCase.Check().Status}); err != nil {
		h.logger.Error("encoding health response", "error", err)
	}
}

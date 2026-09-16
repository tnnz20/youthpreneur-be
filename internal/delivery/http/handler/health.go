package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/tnnz20/youthpreneur-be/internal/model"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

type HealthHandler struct {
	usecase usecase.HealthUsecase
}

func NewHealthHandler(uc usecase.HealthUsecase) *HealthHandler {
	return &HealthHandler{usecase: uc}
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(model.HealthResponse{Status: h.usecase.Check().Status}); err != nil {
		log.Printf("encoding health response: %v", err)
	}
}

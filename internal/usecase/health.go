package usecase

import (
	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

// HealthUseCase provides health status for HTTP clients.
type HealthUseCase interface {
	Check() entity.Health
}

type healthUsecase struct {
	repo repository.HealthRepository
}

// NewHealthUseCase creates a health use case backed by repo.
func NewHealthUseCase(repo repository.HealthRepository) HealthUseCase {
	return healthUsecase{repo: repo}
}

func (u healthUsecase) Check() entity.Health {
	return entity.Health{Status: u.repo.Status()}
}

package usecase

import (
	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

type HealthUsecase interface {
	Check() entity.Health
}

type healthUsecase struct {
	repo repository.HealthRepository
}

func NewHealthUsecase(repo repository.HealthRepository) HealthUsecase {
	return healthUsecase{repo: repo}
}

func (u healthUsecase) Check() entity.Health {
	return entity.Health{Status: u.repo.Status()}
}

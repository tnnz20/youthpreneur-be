package repository

type HealthRepository interface {
	Status() string
}

type healthRepository struct{}

func NewHealthRepository() HealthRepository {
	return healthRepository{}
}

func (healthRepository) Status() string {
	return "ok"
}

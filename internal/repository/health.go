package repository

// HealthRepository provides application health status.
type HealthRepository interface {
	Status() string
}

type healthRepository struct{}

// NewHealthRepository creates the health repository used by the health check.
func NewHealthRepository() HealthRepository {
	return healthRepository{}
}

func (healthRepository) Status() string {
	return "ok"
}

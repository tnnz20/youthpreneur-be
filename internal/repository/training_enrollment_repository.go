package repository

import (
	"context"
	"errors"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
)

// Training enrollment repository errors are surfaced to usecases so they can
// map persistence failures to domain errors.
var (
	// ErrTrainingEnrollmentNotFound indicates no active enrollment matches the
	// identifier and user scope.
	ErrTrainingEnrollmentNotFound = errors.New("repository: training enrollment not found")
	// ErrDuplicateTrainingEnrollmentPublicID indicates another enrollment
	// already uses the public id.
	ErrDuplicateTrainingEnrollmentPublicID = errors.New("repository: duplicate training enrollment public id")
	// ErrDuplicateTrainingEnrollment indicates the user already has an active
	// enrollment for the catalog.
	ErrDuplicateTrainingEnrollment = errors.New("repository: duplicate active training enrollment")
	// ErrTrainingCatalogClosed indicates the catalog does not accept
	// enrollments in its current status.
	ErrTrainingCatalogClosed = errors.New("repository: training catalog closed for enrollment")
	// ErrTrainingCatalogFull indicates the catalog has reached training_slots.
	ErrTrainingCatalogFull = errors.New("repository: training catalog full")
)

// TrainingEnrollmentRepository persists enrollments.
//
// Reads exclude cancelled enrollments from active checks but include them in
// history. Creation locks the catalog row, checks capacity, and inserts in one
// transaction. userID zero means any user and must only be used for admin
// scope.
type TrainingEnrollmentRepository interface {
	// CreateTrainingEnrollment locks the catalog, rejects a closed or full
	// catalog, rejects an existing active enrollment, and inserts the
	// enrollment in one transaction. It returns the persisted enrollment with
	// its catalog joined.
	CreateTrainingEnrollment(ctx context.Context, enrollment entity.TrainingEnrollment) (entity.TrainingEnrollment, error)
	// CancelTrainingEnrollment soft deletes the active enrollment matching
	// publicID within userID scope.
	CancelTrainingEnrollment(ctx context.Context, publicID string, userID int, deletedAt int64) error
	// FindTrainingEnrollments returns enrollments ordered by ascending id,
	// filtered by filter, with their catalog joined. Cancelled enrollments are
	// included.
	FindTrainingEnrollments(ctx context.Context, filter entity.TrainingEnrollmentFilter) ([]entity.TrainingEnrollment, error)
}

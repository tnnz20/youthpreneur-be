package repository

import (
	"context"
	"errors"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
)

// Training catalog repository errors are surfaced to usecases so they can map
// persistence failures to domain errors.
var (
	// ErrTrainingCatalogNotFound indicates no active catalog matches the
	// identifier.
	ErrTrainingCatalogNotFound = errors.New("repository: training catalog not found")
	// ErrDuplicateTrainingCatalogPublicID indicates another catalog already uses
	// the public id.
	ErrDuplicateTrainingCatalogPublicID = errors.New("repository: duplicate training catalog public id")
)

// TrainingCatalogRepository persists training catalog entries.
//
// Reads exclude soft-deleted catalogs. Every mutation is parameterized and
// context-aware; updates lock the row and return the committed row without a
// post-commit re-read.
type TrainingCatalogRepository interface {
	// CreateTrainingCatalog inserts the catalog and returns the persisted row.
	CreateTrainingCatalog(ctx context.Context, catalog entity.TrainingCatalog) (entity.TrainingCatalog, error)
	// FindTrainingCatalogByPublicID returns the active catalog matching publicID.
	FindTrainingCatalogByPublicID(ctx context.Context, publicID string) (entity.TrainingCatalog, error)
	// FindTrainingCatalogs returns active catalogs ordered by ascending id,
	// filtered by filter.
	FindTrainingCatalogs(ctx context.Context, filter entity.TrainingCatalogFilter) ([]entity.TrainingCatalog, error)
	// UpdateTrainingCatalog locks the active catalog matching publicID, applies
	// the optional update, and returns the committed row.
	UpdateTrainingCatalog(ctx context.Context, publicID string, update entity.TrainingCatalogUpdate) (entity.TrainingCatalog, error)
	// SoftDeleteTrainingCatalog sets deleted_at for the active catalog matching
	// publicID.
	SoftDeleteTrainingCatalog(ctx context.Context, publicID string, deletedAt int64) error
}

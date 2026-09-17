package repository

import (
	"context"
	"errors"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
)

// Enterprise repository errors are surfaced to usecases so they can map
// persistence failures to domain errors.
var (
	// ErrEnterpriseNotFound indicates no active enterprise matches the
	// identifier and owner scope.
	ErrEnterpriseNotFound = errors.New("repository: enterprise not found")
	// ErrDuplicateEnterprisePublicID indicates another enterprise already uses
	// the public id.
	ErrDuplicateEnterprisePublicID = errors.New("repository: duplicate enterprise public id")
)

// EnterpriseRepository persists enterprises and their audit events.
//
// Reads exclude soft-deleted enterprises. ownerID scopes a lookup or mutation
// to one owner; zero means any owner and must only be used for admin scope.
// Every mutation writes its audit event in the same transaction.
type EnterpriseRepository interface {
	// CreateEnterprise inserts the enterprise and its audit event in one
	// transaction and returns the persisted row.
	CreateEnterprise(ctx context.Context, enterprise entity.Enterprise, event entity.EnterpriseAuditEvent) (entity.Enterprise, error)
	// FindEnterpriseByPublicID returns the active enterprise matching publicID
	// within ownerID scope.
	FindEnterpriseByPublicID(ctx context.Context, publicID string, ownerID int) (entity.Enterprise, error)
	// FindEnterprises returns active enterprises ordered by ascending id,
	// filtered by filter.
	FindEnterprises(ctx context.Context, filter entity.EnterpriseFilter) ([]entity.Enterprise, error)
	// UpdateEnterprise locks the active enterprise matching publicID within
	// ownerID scope, applies the optional update, and writes its audit event
	// with the actual changed fields in the same transaction. It returns the
	// committed row without a post-commit re-read.
	UpdateEnterprise(ctx context.Context, publicID string, ownerID int, update entity.EnterpriseUpdate, event entity.EnterpriseAuditEvent) (entity.Enterprise, error)
	// SoftDeleteEnterprise sets deleted_at for the active enterprise matching
	// publicID within ownerID scope and writes its audit event in the same
	// transaction.
	SoftDeleteEnterprise(ctx context.Context, publicID string, ownerID int, deletedAt int64, event entity.EnterpriseAuditEvent) error
}

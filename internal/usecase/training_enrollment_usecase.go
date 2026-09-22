package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

const (
	trainingEnrollmentPublicIDPrefix = "ENR-"
)

// Training enrollment usecase errors returned for HTTP status mapping.
var (
	// ErrTrainingEnrollmentNotFound indicates the enrollment does not exist or
	// is outside the caller's user scope.
	ErrTrainingEnrollmentNotFound = errors.New("usecase: training enrollment not found")
	// ErrAlreadyEnrolled indicates the user already has an active enrollment
	// for the catalog.
	ErrAlreadyEnrolled = errors.New("usecase: already enrolled in training")
	// ErrTrainingCatalogFull indicates the catalog has reached training_slots.
	ErrTrainingCatalogFull = errors.New("usecase: training catalog full")
	// ErrTrainingCatalogClosed indicates the catalog does not accept
	// enrollments in its current status.
	ErrTrainingCatalogClosed = errors.New("usecase: training catalog closed for enrollment")
)

// CreateTrainingEnrollmentInput carries the catalog the authenticated actor
// wants to enroll in. The enrolling user is always the actor.
type CreateTrainingEnrollmentInput struct {
	Actor           entity.User
	CatalogPublicID string
}

// FindTrainingEnrollmentsInput bounds an enrollment history request.
type FindTrainingEnrollmentsInput struct {
	Actor  entity.User
	Status string
	Cursor int
	Limit  int
}

// FindCatalogEnrollmentsInput bounds an admin catalog enrollment history
// request.
type FindCatalogEnrollmentsInput struct {
	Actor           entity.User
	CatalogPublicID string
	Status          string
	Cursor          int
	Limit           int
}

// UpdateTrainingEnrollmentStatusInput holds the fields needed to update an
// enrollment's status.
type UpdateTrainingEnrollmentStatusInput struct {
	Actor    entity.User
	PublicID string
	Status   string
}

// FindTrainingEnrollmentsResult is one page of enrollments plus the cursor for
// the following page. NextCursor is zero when no further page exists.
type FindTrainingEnrollmentsResult struct {
	Enrollments []entity.TrainingEnrollment
	NextCursor  int
}

// TrainingEnrollmentUseCase implements the enrollment lifecycle.
type TrainingEnrollmentUseCase interface {
	// Enroll registers the authenticated actor in a catalog offering. It
	// returns ErrTrainingCatalogNotFound, ErrTrainingCatalogClosed,
	// ErrTrainingCatalogFull, or ErrAlreadyEnrolled when enrollment is not
	// possible.
	Enroll(ctx context.Context, input CreateTrainingEnrollmentInput) (entity.TrainingEnrollment, error)
	// CancelEnrollment cancels the actor's active enrollment matching publicID.
	// Admins may cancel any enrollment.
	CancelEnrollment(ctx context.Context, actor entity.User, publicID string) error
	// UpdateStatus updates the approval status of an enrollment. It is admin-only.
	UpdateStatus(ctx context.Context, input UpdateTrainingEnrollmentStatusInput) (entity.TrainingEnrollment, error)
	// FindMyEnrollments returns one page of the actor's enrollment history,
	// including cancelled enrollments.
	FindMyEnrollments(ctx context.Context, input FindTrainingEnrollmentsInput) (FindTrainingEnrollmentsResult, error)
	// FindAllEnrollments returns one page of every user's enrollment history.
	// It is admin-only.
	FindAllEnrollments(ctx context.Context, input FindTrainingEnrollmentsInput) (FindTrainingEnrollmentsResult, error)
	// FindCatalogEnrollments returns one page of a catalog's enrollment history.
	// It is admin-only.
	FindCatalogEnrollments(ctx context.Context, input FindCatalogEnrollmentsInput) (FindTrainingEnrollmentsResult, error)
}

type trainingEnrollmentUsecase struct {
	repo        repository.TrainingEnrollmentRepository
	catalogRepo repository.TrainingCatalogRepository
	now         func() int64
}

// NewTrainingEnrollmentUseCase creates a training enrollment use case backed by
// repo and catalogRepo.
func NewTrainingEnrollmentUseCase(
	repo repository.TrainingEnrollmentRepository,
	catalogRepo repository.TrainingCatalogRepository,
) TrainingEnrollmentUseCase {
	return trainingEnrollmentUsecase{
		repo:        repo,
		catalogRepo: catalogRepo,
		now:         func() int64 { return time.Now().Unix() },
	}
}

// GenerateTrainingEnrollmentPublicID returns an ENR- prefixed identifier with six random decimal
// digits drawn from crypto/rand.
func GenerateTrainingEnrollmentPublicID() (string, error) {
	suffix, err := rand.Int(rand.Reader, big.NewInt(publicIDMax))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s%0*d", trainingEnrollmentPublicIDPrefix, publicIDDigits, suffix.Int64()), nil
}

// Enroll validates the request and delegates a transactional, capacity-safe
// enrollment to the repository.
func (u trainingEnrollmentUsecase) Enroll(
	ctx context.Context,
	input CreateTrainingEnrollmentInput,
) (entity.TrainingEnrollment, error) {
	if input.Actor.ID == 0 {
		return entity.TrainingEnrollment{}, ErrForbidden
	}

	catalogPublicID := strings.TrimSpace(input.CatalogPublicID)
	if catalogPublicID == "" {
		return entity.TrainingEnrollment{}, badRequest("catalog_public_id is required")
	}

	now := u.now()
	registerDate := dateOnly(now)

	for range publicIDAttempts {
		publicID, err := GenerateTrainingEnrollmentPublicID()
		if err != nil {
			return entity.TrainingEnrollment{}, fmt.Errorf("generate public id: %w", err)
		}

		enrollment := entity.TrainingEnrollment{
			PublicID:     publicID,
			UserID:       input.Actor.ID,
			RegisterDate: &registerDate,
			Status:       entity.TrainingEnrollmentStatusPending,
			CreatedAt:    now,
			UpdatedAt:    now,
			Catalog:      &entity.TrainingCatalog{PublicID: catalogPublicID},
		}

		created, err := u.repo.CreateTrainingEnrollment(ctx, enrollment)
		switch {
		case errors.Is(err, repository.ErrDuplicateTrainingEnrollmentPublicID):
			continue
		case err != nil:
			return entity.TrainingEnrollment{}, mapTrainingEnrollmentRepositoryError(err)
		}

		return created, nil
	}

	return entity.TrainingEnrollment{}, ErrPublicIDGeneration
}

// CancelEnrollment cancels the actor's active enrollment matching publicID.
func (u trainingEnrollmentUsecase) CancelEnrollment(
	ctx context.Context,
	actor entity.User,
	publicID string,
) error {
	if actor.ID == 0 {
		return ErrForbidden
	}

	if err := u.repo.CancelTrainingEnrollment(ctx, publicID, enrollmentScopeUserID(actor), u.now()); err != nil {
		return mapTrainingEnrollmentRepositoryError(err)
	}

	return nil
}

// UpdateStatus updates the approval status of an enrollment.
func (u trainingEnrollmentUsecase) UpdateStatus(
	ctx context.Context,
	input UpdateTrainingEnrollmentStatusInput,
) (entity.TrainingEnrollment, error) {
	if input.Actor.Role != entity.RoleAdmin {
		return entity.TrainingEnrollment{}, ErrForbidden
	}

	publicID := strings.TrimSpace(input.PublicID)
	if publicID == "" {
		return entity.TrainingEnrollment{}, badRequest("public_id is required")
	}

	status := entity.TrainingEnrollmentStatus(strings.TrimSpace(input.Status))
	if !entity.ValidTrainingEnrollmentStatus(status) {
		return entity.TrainingEnrollment{}, badRequest("invalid training enrollment status")
	}

	updated, err := u.repo.UpdateTrainingEnrollmentStatus(ctx, publicID, status, u.now())
	if err != nil {
		return entity.TrainingEnrollment{}, mapTrainingEnrollmentRepositoryError(err)
	}

	return updated, nil
}

// FindMyEnrollments returns one page of the actor's enrollment history.
func (u trainingEnrollmentUsecase) FindMyEnrollments(
	ctx context.Context,
	input FindTrainingEnrollmentsInput,
) (FindTrainingEnrollmentsResult, error) {
	if input.Actor.ID == 0 {
		return FindTrainingEnrollmentsResult{}, ErrForbidden
	}

	status, err := normalizeEnrollmentStatus(input.Status)
	if err != nil {
		return FindTrainingEnrollmentsResult{}, err
	}

	return u.findEnrollments(ctx, entity.TrainingEnrollmentFilter{
		UserID: input.Actor.ID,
		Status: status,
		Cursor: input.Cursor,
		Limit:  input.Limit,
	})
}

// FindAllEnrollments returns one page of every user's enrollment history.
func (u trainingEnrollmentUsecase) FindAllEnrollments(
	ctx context.Context,
	input FindTrainingEnrollmentsInput,
) (FindTrainingEnrollmentsResult, error) {
	if input.Actor.Role != entity.RoleAdmin {
		return FindTrainingEnrollmentsResult{}, ErrForbidden
	}

	status, err := normalizeEnrollmentStatus(input.Status)
	if err != nil {
		return FindTrainingEnrollmentsResult{}, err
	}

	return u.findEnrollments(ctx, entity.TrainingEnrollmentFilter{
		Status: status,
		Cursor: input.Cursor,
		Limit:  input.Limit,
	})
}

// FindCatalogEnrollments returns one page of a catalog's enrollment history.
func (u trainingEnrollmentUsecase) FindCatalogEnrollments(
	ctx context.Context,
	input FindCatalogEnrollmentsInput,
) (FindTrainingEnrollmentsResult, error) {
	if input.Actor.Role != entity.RoleAdmin {
		return FindTrainingEnrollmentsResult{}, ErrForbidden
	}

	catalog, err := u.catalogRepo.FindTrainingCatalogByPublicID(ctx, strings.TrimSpace(input.CatalogPublicID))
	if err != nil {
		return FindTrainingEnrollmentsResult{}, mapTrainingCatalogRepositoryError(err)
	}

	status, err := normalizeEnrollmentStatus(input.Status)
	if err != nil {
		return FindTrainingEnrollmentsResult{}, err
	}

	return u.findEnrollments(ctx, entity.TrainingEnrollmentFilter{
		CatalogID: catalog.ID,
		Status:    status,
		Cursor:    input.Cursor,
		Limit:     input.Limit,
	})
}

func normalizeEnrollmentStatus(raw string) (entity.TrainingEnrollmentStatus, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	status := entity.TrainingEnrollmentStatus(trimmed)
	if !entity.ValidTrainingEnrollmentStatus(status) {
		return "", badRequest("invalid status")
	}

	return status, nil
}

func (u trainingEnrollmentUsecase) findEnrollments(
	ctx context.Context,
	filter entity.TrainingEnrollmentFilter,
) (FindTrainingEnrollmentsResult, error) {
	limit := clampLimit(filter.Limit)
	// Fetch one extra row to detect whether a further page exists.
	filter.Limit = limit + 1

	enrollments, err := u.repo.FindTrainingEnrollments(ctx, filter)
	if err != nil {
		return FindTrainingEnrollmentsResult{}, fmt.Errorf("find training enrollments: %w", err)
	}

	result := FindTrainingEnrollmentsResult{Enrollments: enrollments}
	if len(enrollments) > limit {
		result.Enrollments = enrollments[:limit]
		result.NextCursor = enrollments[limit-1].ID
	}

	return result, nil
}

// enrollmentScopeUserID returns the owner filter for actor: zero for admins,
// who may cancel any enrollment, and the actor's id for members.
func enrollmentScopeUserID(actor entity.User) int {
	if actor.Role == entity.RoleAdmin {
		return 0
	}

	return actor.ID
}

// dateOnly truncates a Unix timestamp to a UTC calendar date.
func dateOnly(unix int64) time.Time {
	utc := time.Unix(unix, 0).UTC()

	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func mapTrainingEnrollmentRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrTrainingEnrollmentNotFound):
		return ErrTrainingEnrollmentNotFound
	case errors.Is(err, repository.ErrDuplicateTrainingEnrollment):
		return ErrAlreadyEnrolled
	case errors.Is(err, repository.ErrTrainingCatalogNotFound):
		return ErrTrainingCatalogNotFound
	case errors.Is(err, repository.ErrTrainingCatalogClosed):
		return ErrTrainingCatalogClosed
	case errors.Is(err, repository.ErrTrainingCatalogFull):
		return ErrTrainingCatalogFull
	default:
		return err
	}
}

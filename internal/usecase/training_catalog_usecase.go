package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

const (
	maxTrainingCatalogNameLength   = 255
	maxTrainingCatalogPhoneLength  = 50
	maxTrainingCatalogCategory     = 100
	maxTrainingCatalogLinkLength   = 255
	maxTrainingCatalogPeriodLength = 100
	// trainingDateFormat is the ISO 8601 date format shared by training dates,
	// periods, and register dates.
	trainingDateFormat = "2006-01-02"
)

// ErrTrainingCatalogNotFound indicates the catalog does not exist or is soft
// deleted.
var ErrTrainingCatalogNotFound = errors.New("usecase: training catalog not found")

// CreateTrainingCatalogInput carries the data needed to create a catalog entry.
// Actor must be an admin; the route and usecase both enforce it.
type CreateTrainingCatalogInput struct {
	Actor          entity.User
	Name           string
	Description    string
	PicPhone       string
	Category       string
	TrainingSlots  *int
	TrainingStatus string
	Link           string
	TrainingDate   string
	TrainingPeriod string
	Speaker        string
}

// UpdateTrainingCatalogInput carries the optional catalog fields to change. A
// nil field keeps its current value.
type UpdateTrainingCatalogInput struct {
	Name           *string
	Description    *string
	PicPhone       *string
	Category       *string
	TrainingSlots  *int
	TrainingStatus *string
	Link           *string
	TrainingDate   *string
	TrainingPeriod *string
	Speaker        *string
}

// FindTrainingCatalogsInput bounds and filters a public catalog listing.
type FindTrainingCatalogsInput struct {
	Category       string
	TrainingStatus string
	TrainingDate   string
	TrainingPeriod string
	Cursor         int
	Limit          int
}

// FindTrainingCatalogsResult is one page of catalogs plus the cursor for the
// following page. NextCursor is zero when no further page exists.
type FindTrainingCatalogsResult struct {
	Catalogs   []entity.TrainingCatalog
	NextCursor int
}

// TrainingCatalogUseCase implements public catalog reads and admin catalog
// writes.
type TrainingCatalogUseCase interface {
	// CreateTrainingCatalog validates input and creates a catalog entry. It is
	// admin-only.
	CreateTrainingCatalog(ctx context.Context, input CreateTrainingCatalogInput) (entity.TrainingCatalog, error)
	// GetTrainingCatalog returns the active catalog matching publicID.
	GetTrainingCatalog(ctx context.Context, publicID string) (entity.TrainingCatalog, error)
	// FindTrainingCatalogs returns one page of active catalogs.
	FindTrainingCatalogs(ctx context.Context, input FindTrainingCatalogsInput) (FindTrainingCatalogsResult, error)
	// UpdateTrainingCatalog changes the permitted fields of the catalog matching
	// publicID. It is admin-only.
	UpdateTrainingCatalog(ctx context.Context, actor entity.User, publicID string, input UpdateTrainingCatalogInput) (entity.TrainingCatalog, error)
	// UpdateTrainingCatalogStatus changes only the training status of the
	// catalog matching publicID. It is admin-only.
	UpdateTrainingCatalogStatus(ctx context.Context, actor entity.User, publicID string, status string) (entity.TrainingCatalog, error)
	// DeleteTrainingCatalog soft deletes the catalog matching publicID. It is
	// admin-only.
	DeleteTrainingCatalog(ctx context.Context, actor entity.User, publicID string) error
}

type trainingCatalogUsecase struct {
	repo repository.TrainingCatalogRepository
	now  func() int64
}

// NewTrainingCatalogUseCase creates a training catalog use case backed by repo.
func NewTrainingCatalogUseCase(repo repository.TrainingCatalogRepository) TrainingCatalogUseCase {
	return trainingCatalogUsecase{
		repo: repo,
		now:  func() int64 { return time.Now().Unix() },
	}
}

// CreateTrainingCatalog validates input, assigns a generated public id, and
// persists the catalog.
func (u trainingCatalogUsecase) CreateTrainingCatalog(
	ctx context.Context,
	input CreateTrainingCatalogInput,
) (entity.TrainingCatalog, error) {
	if input.Actor.Role != entity.RoleAdmin {
		return entity.TrainingCatalog{}, ErrForbidden
	}

	catalog, err := buildTrainingCatalog(input, u.now())
	if err != nil {
		return entity.TrainingCatalog{}, err
	}

	for range publicIDAttempts {
		publicID, err := generatePublicID()
		if err != nil {
			return entity.TrainingCatalog{}, fmt.Errorf("generate public id: %w", err)
		}
		catalog.PublicID = publicID

		created, err := u.repo.CreateTrainingCatalog(ctx, catalog)
		switch {
		case errors.Is(err, repository.ErrDuplicateTrainingCatalogPublicID):
			continue
		case err != nil:
			return entity.TrainingCatalog{}, fmt.Errorf("create training catalog: %w", err)
		}

		return created, nil
	}

	return entity.TrainingCatalog{}, ErrPublicIDGeneration
}

// GetTrainingCatalog returns the active catalog matching publicID.
func (u trainingCatalogUsecase) GetTrainingCatalog(
	ctx context.Context,
	publicID string,
) (entity.TrainingCatalog, error) {
	catalog, err := u.repo.FindTrainingCatalogByPublicID(ctx, publicID)
	if err != nil {
		return entity.TrainingCatalog{}, mapTrainingCatalogRepositoryError(err)
	}

	return catalog, nil
}

// FindTrainingCatalogs returns one page of active catalogs.
func (u trainingCatalogUsecase) FindTrainingCatalogs(
	ctx context.Context,
	input FindTrainingCatalogsInput,
) (FindTrainingCatalogsResult, error) {
	status, err := normalizeProcessStatus("training_status", input.TrainingStatus)
	if err != nil {
		return FindTrainingCatalogsResult{}, err
	}
	date, err := parseTrainingDate("training_date", input.TrainingDate)
	if err != nil {
		return FindTrainingCatalogsResult{}, err
	}

	limit := clampLimit(input.Limit)
	catalogs, err := u.repo.FindTrainingCatalogs(ctx, entity.TrainingCatalogFilter{
		Category:       strings.TrimSpace(input.Category),
		TrainingStatus: status,
		TrainingDate:   date,
		TrainingPeriod: strings.TrimSpace(input.TrainingPeriod),
		Cursor:         input.Cursor,
		// Fetch one extra row to detect whether a further page exists.
		Limit: limit + 1,
	})
	if err != nil {
		return FindTrainingCatalogsResult{}, fmt.Errorf("find training catalogs: %w", err)
	}

	result := FindTrainingCatalogsResult{Catalogs: catalogs}
	if len(catalogs) > limit {
		result.Catalogs = catalogs[:limit]
		result.NextCursor = catalogs[limit-1].ID
	}

	return result, nil
}

// UpdateTrainingCatalog applies validated optional changes to the catalog
// matching publicID.
func (u trainingCatalogUsecase) UpdateTrainingCatalog(
	ctx context.Context,
	actor entity.User,
	publicID string,
	input UpdateTrainingCatalogInput,
) (entity.TrainingCatalog, error) {
	if actor.Role != entity.RoleAdmin {
		return entity.TrainingCatalog{}, ErrForbidden
	}

	update, err := buildTrainingCatalogUpdate(input)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	if isEmptyTrainingCatalogUpdate(update) {
		return entity.TrainingCatalog{}, badRequest("no update fields supplied")
	}
	update.UpdatedAt = u.now()

	result, err := u.repo.UpdateTrainingCatalog(ctx, publicID, update)
	if err != nil {
		return entity.TrainingCatalog{}, mapTrainingCatalogRepositoryError(err)
	}

	return result, nil
}

// UpdateTrainingCatalogStatus changes only the training status of the catalog
// matching publicID.
func (u trainingCatalogUsecase) UpdateTrainingCatalogStatus(
	ctx context.Context,
	actor entity.User,
	publicID string,
	status string,
) (entity.TrainingCatalog, error) {
	if actor.Role != entity.RoleAdmin {
		return entity.TrainingCatalog{}, ErrForbidden
	}

	normalized := entity.ProcessStatus(strings.TrimSpace(status))
	if normalized == "" {
		return entity.TrainingCatalog{}, badRequest("training_status is required")
	}
	if _, ok := validProcessStatuses[normalized]; !ok {
		return entity.TrainingCatalog{}, badRequest("invalid training_status")
	}

	result, err := u.repo.UpdateTrainingCatalog(ctx, publicID, entity.TrainingCatalogUpdate{
		TrainingStatus: &normalized,
		UpdatedAt:      u.now(),
	})
	if err != nil {
		return entity.TrainingCatalog{}, mapTrainingCatalogRepositoryError(err)
	}

	return result, nil
}

// DeleteTrainingCatalog soft deletes the catalog matching publicID.
func (u trainingCatalogUsecase) DeleteTrainingCatalog(
	ctx context.Context,
	actor entity.User,
	publicID string,
) error {
	if actor.Role != entity.RoleAdmin {
		return ErrForbidden
	}

	if err := u.repo.SoftDeleteTrainingCatalog(ctx, publicID, u.now()); err != nil {
		return mapTrainingCatalogRepositoryError(err)
	}

	return nil
}

// buildTrainingCatalog validates and normalizes create input into an entity
// with server-controlled timestamps.
func buildTrainingCatalog(input CreateTrainingCatalogInput, now int64) (entity.TrainingCatalog, error) {
	name, err := normalizeCatalogText("name", input.Name, maxTrainingCatalogNameLength)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	picPhone, err := normalizeCatalogText("pic_phone", input.PicPhone, maxTrainingCatalogPhoneLength)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	category, err := normalizeCatalogText("category", input.Category, maxTrainingCatalogCategory)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	link, err := normalizeCatalogLink(input.Link)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	period, err := normalizeCatalogText("training_period", input.TrainingPeriod, maxTrainingCatalogPeriodLength)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	status, err := normalizeProcessStatus("training_status", input.TrainingStatus)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	slots, err := normalizeTrainingSlots(input.TrainingSlots)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	date, err := parseTrainingDate("training_date", input.TrainingDate)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}

	return entity.TrainingCatalog{
		Name:           name,
		Description:    strings.TrimSpace(input.Description),
		PicPhone:       picPhone,
		Category:       category,
		TrainingSlots:  slots,
		TrainingStatus: status,
		Link:           link,
		TrainingDate:   date,
		TrainingPeriod: period,
		Speaker:        strings.TrimSpace(input.Speaker),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// buildTrainingCatalogUpdate validates and normalizes optional update input.
func buildTrainingCatalogUpdate(input UpdateTrainingCatalogInput) (entity.TrainingCatalogUpdate, error) {
	var update entity.TrainingCatalogUpdate

	if input.Name != nil {
		name, err := normalizeCatalogText("name", *input.Name, maxTrainingCatalogNameLength)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.Name = &name
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		update.Description = &description
	}
	if input.PicPhone != nil {
		picPhone, err := normalizeCatalogText("pic_phone", *input.PicPhone, maxTrainingCatalogPhoneLength)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.PicPhone = &picPhone
	}
	if input.Category != nil {
		category, err := normalizeCatalogText("category", *input.Category, maxTrainingCatalogCategory)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.Category = &category
	}
	if input.TrainingSlots != nil {
		slots, err := normalizeTrainingSlots(input.TrainingSlots)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.TrainingSlots = slots
	}
	if input.TrainingStatus != nil {
		status, err := normalizeProcessStatus("training_status", *input.TrainingStatus)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.TrainingStatus = &status
	}
	if input.Link != nil {
		link, err := normalizeCatalogLink(*input.Link)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.Link = &link
	}
	if input.TrainingDate != nil {
		date, err := parseTrainingDate("training_date", *input.TrainingDate)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.TrainingDate = date
	}
	if input.TrainingPeriod != nil {
		period, err := normalizeCatalogText("training_period", *input.TrainingPeriod, maxTrainingCatalogPeriodLength)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.TrainingPeriod = &period
	}
	if input.Speaker != nil {
		speaker := strings.TrimSpace(*input.Speaker)
		update.Speaker = &speaker
	}

	return update, nil
}

// isEmptyTrainingCatalogUpdate reports whether no update field was supplied.
// UpdatedAt is excluded because the usecase sets it after this check.
func isEmptyTrainingCatalogUpdate(update entity.TrainingCatalogUpdate) bool {
	return update.Name == nil &&
		update.Description == nil &&
		update.PicPhone == nil &&
		update.Category == nil &&
		update.TrainingSlots == nil &&
		update.TrainingStatus == nil &&
		update.Link == nil &&
		update.TrainingDate == nil &&
		update.TrainingPeriod == nil &&
		update.Speaker == nil
}

// normalizeCatalogText trims a nullable text field and enforces its maximum
// length.
func normalizeCatalogText(field, value string, maxLength int) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > maxLength {
		return "", badRequest(fmt.Sprintf("%s must be at most %d characters", field, maxLength))
	}

	return value, nil
}

// normalizeTrainingSlots validates an optional capacity. A nil value means
// unlimited; a supplied value must be positive.
func normalizeTrainingSlots(slots *int) (*int, error) {
	if slots == nil {
		return nil, nil
	}
	if *slots <= 0 {
		return nil, badRequest("training_slots must be positive")
	}

	return slots, nil
}

// normalizeCatalogLink validates an optional http or https URL.
func normalizeCatalogLink(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) > maxTrainingCatalogLinkLength {
		return "", badRequest("link must be at most 255 characters")
	}

	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", badRequest("invalid link")
	}

	return value, nil
}

// parseTrainingDate parses an optional `YYYY-MM-DD` date. An empty value means
// unset.
func parseTrainingDate(field, value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	parsed, err := time.Parse(trainingDateFormat, value)
	if err != nil {
		return nil, badRequest("invalid " + field)
	}

	return &parsed, nil
}

func mapTrainingCatalogRepositoryError(err error) error {
	if errors.Is(err, repository.ErrTrainingCatalogNotFound) {
		return ErrTrainingCatalogNotFound
	}

	return err
}

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
	maxTrainingCatalogTitleLength     = 255
	maxTrainingCatalogPhoneLength     = 50
	maxTrainingCatalogLinkLength      = 255
	maxTrainingCatalogMentorLength    = 255
	maxTrainingCatalogThumbnailLength = 255
	// trainingDateFormat is the ISO 8601 date format shared by training dates.
	trainingDateFormat = "2006-01-02"
)

// ErrTrainingCatalogNotFound indicates the catalog does not exist or is soft
// deleted.
var ErrTrainingCatalogNotFound = errors.New("usecase: training catalog not found")

// CreateTrainingCatalogInput carries the data needed to create a catalog entry.
// Actor must be an admin; the route and usecase both enforce it.
type CreateTrainingCatalogInput struct {
	Actor          entity.User
	Title          string
	Description    string
	PicPhone       string
	Category       string
	MaxSlots       *int
	TrainingStatus string
	Link           string
	Address        string
	Thumbnail      string
	StartDate      string
	EndDate        string
	Mentor         string
}

// UpdateTrainingCatalogInput carries the optional catalog fields to change. A
// nil field keeps its current value.
type UpdateTrainingCatalogInput struct {
	Title          *string
	Description    *string
	PicPhone       *string
	Category       *string
	MaxSlots       *int
	TrainingStatus *string
	Link           *string
	Address        *string
	Thumbnail      *string
	StartDate      *string
	EndDate        *string
	Mentor         *string
}

// FindTrainingCatalogsInput bounds and filters a public catalog listing.
type FindTrainingCatalogsInput struct {
	Search         string
	Title          string
	Mentor         string
	Category       string
	TrainingStatus string
	StartDate      string
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
		publicID, err := GeneratePublicID()
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
	var category entity.TrainingCategory
	trimmedCat := strings.TrimSpace(input.Category)
	if trimmedCat != "" {
		category = entity.TrainingCategory(trimmedCat)
		if !entity.ValidTrainingCategory(category) {
			return FindTrainingCatalogsResult{}, badRequest("invalid category")
		}
	}

	status, err := normalizeProcessStatus("training_status", input.TrainingStatus)
	if err != nil {
		return FindTrainingCatalogsResult{}, err
	}
	date, err := parseTrainingDate("start_date", input.StartDate)
	if err != nil {
		return FindTrainingCatalogsResult{}, err
	}

	limit := clampLimit(input.Limit)
	catalogs, err := u.repo.FindTrainingCatalogs(ctx, entity.TrainingCatalogFilter{
		Search:         strings.TrimSpace(input.Search),
		Title:          strings.TrimSpace(input.Title),
		Mentor:         strings.TrimSpace(input.Mentor),
		Category:       category,
		TrainingStatus: status,
		StartDate:      date,
		Cursor:         input.Cursor,
		Limit:          limit + 1,
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

func buildTrainingCatalog(input CreateTrainingCatalogInput, now int64) (entity.TrainingCatalog, error) {
	title, err := normalizeCatalogText("title", input.Title, maxTrainingCatalogTitleLength)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	picPhone, err := normalizeCatalogText("pic_phone", input.PicPhone, maxTrainingCatalogPhoneLength)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}

	category := entity.TrainingCategory(strings.TrimSpace(input.Category))
	if category != "" && !entity.ValidTrainingCategory(category) {
		return entity.TrainingCatalog{}, badRequest("invalid category")
	}

	link, err := normalizeCatalogLink(input.Link)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	status, err := normalizeProcessStatus("training_status", input.TrainingStatus)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	slots, err := normalizeMaxSlots(input.MaxSlots)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	startDate, err := parseTrainingDate("start_date", input.StartDate)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	endDate, err := parseTrainingDate("end_date", input.EndDate)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	if startDate != nil && endDate != nil && endDate.Before(*startDate) {
		return entity.TrainingCatalog{}, badRequest("end_date must be on or after start_date")
	}
	mentor, err := normalizeCatalogText("mentor", input.Mentor, maxTrainingCatalogMentorLength)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}
	thumbnail, err := normalizeCatalogText("thumbnail", input.Thumbnail, maxTrainingCatalogThumbnailLength)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}

	return entity.TrainingCatalog{
		Title:          title,
		Description:    strings.TrimSpace(input.Description),
		PicPhone:       picPhone,
		Category:       category,
		MaxSlots:       slots,
		TrainingStatus: status,
		Link:           link,
		Address:        strings.TrimSpace(input.Address),
		Thumbnail:      thumbnail,
		StartDate:      startDate,
		EndDate:        endDate,
		Mentor:         mentor,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func buildTrainingCatalogUpdate(input UpdateTrainingCatalogInput) (entity.TrainingCatalogUpdate, error) {
	var update entity.TrainingCatalogUpdate

	if input.Title != nil {
		title, err := normalizeCatalogText("title", *input.Title, maxTrainingCatalogTitleLength)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.Title = &title
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
		catStr := strings.TrimSpace(*input.Category)
		if catStr != "" && !entity.ValidTrainingCategory(entity.TrainingCategory(catStr)) {
			return entity.TrainingCatalogUpdate{}, badRequest("invalid category")
		}
		cat := entity.TrainingCategory(catStr)
		update.Category = &cat
	}
	if input.MaxSlots != nil {
		slots, err := normalizeMaxSlots(input.MaxSlots)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.MaxSlots = slots
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
	if input.Address != nil {
		address := strings.TrimSpace(*input.Address)
		update.Address = &address
	}
	if input.Thumbnail != nil {
		thumbnail, err := normalizeCatalogText("thumbnail", *input.Thumbnail, maxTrainingCatalogThumbnailLength)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.Thumbnail = &thumbnail
	}
	if input.StartDate != nil {
		startDate, err := parseTrainingDate("start_date", *input.StartDate)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.StartDate = startDate
	}
	if input.EndDate != nil {
		endDate, err := parseTrainingDate("end_date", *input.EndDate)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.EndDate = endDate
	}
	if update.StartDate != nil && update.EndDate != nil {
		if update.EndDate.Before(*update.StartDate) {
			return entity.TrainingCatalogUpdate{}, badRequest("end_date must be on or after start_date")
		}
	}
	if input.Mentor != nil {
		mentor, err := normalizeCatalogText("mentor", *input.Mentor, maxTrainingCatalogMentorLength)
		if err != nil {
			return entity.TrainingCatalogUpdate{}, err
		}
		update.Mentor = &mentor
	}

	return update, nil
}

func isEmptyTrainingCatalogUpdate(update entity.TrainingCatalogUpdate) bool {
	return update.Title == nil &&
		update.Description == nil &&
		update.PicPhone == nil &&
		update.Category == nil &&
		update.MaxSlots == nil &&
		update.TrainingStatus == nil &&
		update.Link == nil &&
		update.Address == nil &&
		update.Thumbnail == nil &&
		update.StartDate == nil &&
		update.EndDate == nil &&
		update.Mentor == nil
}

func normalizeCatalogText(field, value string, maxLength int) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > maxLength {
		return "", badRequest(fmt.Sprintf("%s must be at most %d characters", field, maxLength))
	}

	return value, nil
}

func normalizeMaxSlots(slots *int) (*int, error) {
	if slots == nil {
		return nil, nil
	}
	if *slots <= 0 {
		return nil, badRequest("max_slots must be positive")
	}

	return slots, nil
}

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

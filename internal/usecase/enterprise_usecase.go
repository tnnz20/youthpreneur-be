package usecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

const (
	maxEnterpriseNameLength = 255
	maxDistrictLength       = 128
)

// Usecase errors returned by enterprise operations for HTTP status mapping.
var (
	// ErrEnterpriseNotFound indicates the enterprise does not exist, is soft
	// deleted, or is outside the caller's owner scope.
	ErrEnterpriseNotFound = errors.New("usecase: enterprise not found")
	// ErrForbidden indicates the caller is authenticated but not permitted to
	// perform the requested field change.
	ErrForbidden = errors.New("usecase: forbidden")
)

// turnoverPattern enforces a non-negative DECIMAL(15,2): up to thirteen integer
// digits and at most two decimal places.
var turnoverPattern = regexp.MustCompile(`^\d{1,13}(\.\d{1,2})?$`)

var validBusinessSectors = map[entity.BusinessSector]struct{}{
	entity.BusinessSectorKuliner:           {},
	entity.BusinessSectorPerdaganganRitel:  {},
	entity.BusinessSectorAgribisnis:        {},
	entity.BusinessSectorJasaLayananPublik: {},
	entity.BusinessSectorFashionKonveksi:   {},
	entity.BusinessSectorECommerceKreatif:  {},
}

var validEnterpriseStatuses = map[entity.EnterpriseStatus]struct{}{
	entity.EnterpriseStatusActive:   {},
	entity.EnterpriseStatusInactive: {},
}

var validLegalStatuses = map[entity.LegalStatus]struct{}{
	entity.LegalStatusComplete:   {},
	entity.LegalStatusInProgress: {},
	entity.LegalStatusNone:       {},
}

var validBusinessDigitizations = map[entity.BusinessDigitization]struct{}{
	entity.BusinessDigitizationHigh:   {},
	entity.BusinessDigitizationMedium: {},
	entity.BusinessDigitizationLow:    {},
}

var validInterventionNeeds = map[entity.InterventionNeeds]struct{}{
	entity.InterventionNeedsPelatihan:    {},
	entity.InterventionNeedsMentoring:    {},
	entity.InterventionNeedsDigitalisasi: {},
	entity.InterventionNeedsLegalitas:    {},
	entity.InterventionNeedsPermodalan:   {},
	entity.InterventionNeedsKemitraan:    {},
	entity.InterventionNeedsPemasaran:    {},
}

var validProcessStatuses = map[entity.ProcessStatus]struct{}{
	entity.ProcessStatusCompleted: {},
	entity.ProcessStatusOngoing:   {},
	entity.ProcessStatusPlanned:   {},
}

var validGeneralStatuses = map[entity.GeneralStatus]struct{}{
	entity.GeneralStatusYes:        {},
	entity.GeneralStatusNo:         {},
	entity.GeneralStatusInProgress: {},
}

// CreateEnterpriseInput carries the data needed to create an enterprise. Actor
// is the authenticated identity and always owns the created enterprise.
type CreateEnterpriseInput struct {
	Actor                entity.User
	Name                 string
	BusinessSector       string
	LegalStatus          string
	BusinessDigitization string
	InterventionNeeds    string
	TrainingStatus       string
	MentoringStatus      string
	CapitalAccess        string
	Partnership          string
	InitialTurnover      string
	CurrentTurnover      string
	District             string
}

// UpdateEnterpriseInput carries the optional enterprise fields to change. A nil
// field keeps its current value.
type UpdateEnterpriseInput struct {
	Name                 *string
	BusinessSector       *string
	LegalStatus          *string
	BusinessDigitization *string
	InterventionNeeds    *string
	TrainingStatus       *string
	MentoringStatus      *string
	CapitalAccess        *string
	Partnership          *string
	InitialTurnover      *string
	CurrentTurnover      *string
	District             *string
	Status               *string
}

// FindEnterprisesInput bounds and filters an enterprise listing request.
type FindEnterprisesInput struct {
	Actor                entity.User
	District             string
	Status               string
	BusinessSector       string
	LegalStatus          string
	BusinessDigitization string
	InterventionNeeds    string
	TrainingStatus       string
	MentoringStatus      string
	CapitalAccess        string
	Partnership          string
	Cursor               int
	Limit                int
}

// FindEnterprisesResult is one page of enterprises plus the cursor for the
// following page. NextCursor is zero when no further page exists.
type FindEnterprisesResult struct {
	Enterprises []entity.Enterprise
	NextCursor  int
}

// EnterpriseUseCase implements owned enterprise operations.
type EnterpriseUseCase interface {
	// CreateEnterprise validates input and creates an enterprise owned by the
	// authenticated actor.
	CreateEnterprise(ctx context.Context, input CreateEnterpriseInput) (entity.Enterprise, error)
	// GetEnterprise returns the enterprise matching publicID within the
	// actor's owner scope.
	GetEnterprise(ctx context.Context, actor entity.User, publicID string) (entity.Enterprise, error)
	// FindEnterprises returns one page of enterprises within the actor's scope.
	FindEnterprises(ctx context.Context, input FindEnterprisesInput) (FindEnterprisesResult, error)
	// UpdateEnterprise changes the permitted fields of the enterprise matching
	// publicID within the actor's scope. Owners may not change district,
	// status, or the assessment enums; admins may change any mutable field.
	UpdateEnterprise(ctx context.Context, actor entity.User, publicID string, input UpdateEnterpriseInput) (entity.Enterprise, error)
	// DeleteEnterprise soft deletes the enterprise matching publicID within the
	// actor's scope.
	DeleteEnterprise(ctx context.Context, actor entity.User, publicID string) error
}

type enterpriseUsecase struct {
	repo repository.EnterpriseRepository
	now  func() int64
}

// NewEnterpriseUseCase creates an enterprise use case backed by repo.
func NewEnterpriseUseCase(repo repository.EnterpriseRepository) EnterpriseUseCase {
	return enterpriseUsecase{
		repo: repo,
		now:  func() int64 { return time.Now().Unix() },
	}
}

// CreateEnterprise validates input, assigns a generated public id, and
// persists the enterprise with its audit event.
func (u enterpriseUsecase) CreateEnterprise(
	ctx context.Context,
	input CreateEnterpriseInput,
) (entity.Enterprise, error) {
	if input.Actor.ID == 0 {
		return entity.Enterprise{}, ErrForbidden
	}

	name := strings.TrimSpace(input.Name)
	if len(name) > maxEnterpriseNameLength {
		return entity.Enterprise{}, badRequest("name must be at most 255 characters")
	}

	sector := entity.BusinessSector(strings.TrimSpace(input.BusinessSector))
	if err := validateBusinessSector(sector); err != nil {
		return entity.Enterprise{}, err
	}

	legalStatus, err := normalizeLegalStatus(input.LegalStatus)
	if err != nil {
		return entity.Enterprise{}, err
	}
	businessDigitization, err := normalizeBusinessDigitization(input.BusinessDigitization)
	if err != nil {
		return entity.Enterprise{}, err
	}
	interventionNeeds, err := normalizeInterventionNeeds(input.InterventionNeeds)
	if err != nil {
		return entity.Enterprise{}, err
	}
	trainingStatus, err := normalizeProcessStatus("training_status", input.TrainingStatus)
	if err != nil {
		return entity.Enterprise{}, err
	}
	mentoringStatus, err := normalizeProcessStatus("mentoring_status", input.MentoringStatus)
	if err != nil {
		return entity.Enterprise{}, err
	}
	capitalAccess, err := normalizeGeneralStatus("capital_access", input.CapitalAccess)
	if err != nil {
		return entity.Enterprise{}, err
	}
	partnership, err := normalizeGeneralStatus("partnership", input.Partnership)
	if err != nil {
		return entity.Enterprise{}, err
	}

	initialTurnover, err := normalizeTurnover("initial_turnover", input.InitialTurnover)
	if err != nil {
		return entity.Enterprise{}, err
	}
	currentTurnover, err := normalizeTurnover("current_turnover", input.CurrentTurnover)
	if err != nil {
		return entity.Enterprise{}, err
	}

	district := strings.TrimSpace(input.District)
	if len(district) > maxDistrictLength {
		return entity.Enterprise{}, badRequest("district must be at most 128 characters")
	}

	now := u.now()
	status := entity.EnterpriseStatusActive

	// Retry a bounded number of times on collision, matching user creation.
	for range publicIDAttempts {
		publicID, err := GeneratePublicID()
		if err != nil {
			return entity.Enterprise{}, fmt.Errorf("generate public id: %w", err)
		}

		enterprise := entity.Enterprise{
			PublicID:             publicID,
			UserID:               input.Actor.ID,
			Name:                 name,
			BusinessSector:       sector,
			LegalStatus:          legalStatus,
			BusinessDigitization: businessDigitization,
			InterventionNeeds:    interventionNeeds,
			TrainingStatus:       trainingStatus,
			MentoringStatus:      mentoringStatus,
			CapitalAccess:        capitalAccess,
			Partnership:          partnership,
			InitialTurnover:      initialTurnover,
			CurrentTurnover:      currentTurnover,
			District:             district,
			Status:               status,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		event := entity.EnterpriseAuditEvent{
			ActorUserID: input.Actor.ID,
			Action:      entity.AuditActionCreate,
			ChangedFields: map[string]any{
				"name":                  entity.NullableAuditValue(name),
				"business_sector":       string(sector),
				"legal_status":          entity.NullableAuditValue(string(legalStatus)),
				"business_digitization": entity.NullableAuditValue(string(businessDigitization)),
				"intervention_needs":    entity.NullableAuditValue(string(interventionNeeds)),
				"training_status":       entity.NullableAuditValue(string(trainingStatus)),
				"mentoring_status":      entity.NullableAuditValue(string(mentoringStatus)),
				"capital_access":        entity.NullableAuditValue(string(capitalAccess)),
				"partnership":           entity.NullableAuditValue(string(partnership)),
				"initial_turnover":      initialTurnover,
				"current_turnover":      currentTurnover,
				"district":              entity.NullableAuditValue(district),
				"status":                string(status),
			},
			CreatedAt: now,
		}

		created, err := u.repo.CreateEnterprise(ctx, enterprise, event)
		switch {
		case errors.Is(err, repository.ErrDuplicateEnterprisePublicID):
			continue
		case err != nil:
			return entity.Enterprise{}, fmt.Errorf("create enterprise: %w", err)
		}

		return created, nil
	}

	return entity.Enterprise{}, ErrPublicIDGeneration
}

// GetEnterprise returns the enterprise matching publicID within the actor's
// scope.
func (u enterpriseUsecase) GetEnterprise(
	ctx context.Context,
	actor entity.User,
	publicID string,
) (entity.Enterprise, error) {
	enterprise, err := u.repo.FindEnterpriseByPublicID(ctx, publicID, scopeOwnerID(actor))
	if err != nil {
		return entity.Enterprise{}, mapEnterpriseRepositoryError(err)
	}

	return enterprise, nil
}

// FindEnterprises returns one page of enterprises. Members are scoped to their
// own enterprises; admins see every owner.
func (u enterpriseUsecase) FindEnterprises(
	ctx context.Context,
	input FindEnterprisesInput,
) (FindEnterprisesResult, error) {
	filter, err := u.buildFilter(input)
	if err != nil {
		return FindEnterprisesResult{}, err
	}

	enterprises, err := u.repo.FindEnterprises(ctx, filter)
	if err != nil {
		return FindEnterprisesResult{}, fmt.Errorf("find enterprises: %w", err)
	}

	limit := clampLimit(input.Limit)
	result := FindEnterprisesResult{Enterprises: enterprises}
	if len(enterprises) > limit {
		result.Enterprises = enterprises[:limit]
		result.NextCursor = enterprises[limit-1].ID
	}

	return result, nil
}

func (u enterpriseUsecase) buildFilter(input FindEnterprisesInput) (entity.EnterpriseFilter, error) {
	status, err := normalizeEnterpriseStatus(input.Status)
	if err != nil {
		return entity.EnterpriseFilter{}, err
	}
	sector, err := normalizeBusinessSector(input.BusinessSector)
	if err != nil {
		return entity.EnterpriseFilter{}, err
	}
	legalStatus, err := normalizeLegalStatus(input.LegalStatus)
	if err != nil {
		return entity.EnterpriseFilter{}, err
	}
	businessDigitization, err := normalizeBusinessDigitization(input.BusinessDigitization)
	if err != nil {
		return entity.EnterpriseFilter{}, err
	}
	interventionNeeds, err := normalizeInterventionNeeds(input.InterventionNeeds)
	if err != nil {
		return entity.EnterpriseFilter{}, err
	}
	trainingStatus, err := normalizeProcessStatus("training_status", input.TrainingStatus)
	if err != nil {
		return entity.EnterpriseFilter{}, err
	}
	mentoringStatus, err := normalizeProcessStatus("mentoring_status", input.MentoringStatus)
	if err != nil {
		return entity.EnterpriseFilter{}, err
	}
	capitalAccess, err := normalizeGeneralStatus("capital_access", input.CapitalAccess)
	if err != nil {
		return entity.EnterpriseFilter{}, err
	}
	partnership, err := normalizeGeneralStatus("partnership", input.Partnership)
	if err != nil {
		return entity.EnterpriseFilter{}, err
	}

	return entity.EnterpriseFilter{
		District:             strings.TrimSpace(input.District),
		Status:               status,
		BusinessSector:       sector,
		LegalStatus:          legalStatus,
		BusinessDigitization: businessDigitization,
		InterventionNeeds:    interventionNeeds,
		TrainingStatus:       trainingStatus,
		MentoringStatus:      mentoringStatus,
		CapitalAccess:        capitalAccess,
		Partnership:          partnership,
		OwnerID:              scopeOwnerID(input.Actor),
		Cursor:               input.Cursor,
		// Fetch one extra row to detect whether a further page exists.
		Limit: clampLimit(input.Limit) + 1,
	}, nil
}

// UpdateEnterprise validates permitted field changes and delegates the locked,
// transactional mutation to the repository, which derives the actual changed
// fields and records them with the audit event.
func (u enterpriseUsecase) UpdateEnterprise(
	ctx context.Context,
	actor entity.User,
	publicID string,
	input UpdateEnterpriseInput,
) (entity.Enterprise, error) {
	if actor.Role != entity.RoleAdmin && ownerRestrictedChange(input) {
		return entity.Enterprise{}, ErrForbidden
	}

	update := entity.EnterpriseUpdate{UpdatedAt: u.now()}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if len(name) > maxEnterpriseNameLength {
			return entity.Enterprise{}, badRequest("name must be at most 255 characters")
		}
		update.Name = &name
	}

	if input.BusinessSector != nil {
		sector := entity.BusinessSector(strings.TrimSpace(*input.BusinessSector))
		if err := validateBusinessSector(sector); err != nil {
			return entity.Enterprise{}, err
		}
		update.BusinessSector = &sector
	}

	if input.LegalStatus != nil {
		legalStatus, err := normalizeLegalStatus(*input.LegalStatus)
		if err != nil {
			return entity.Enterprise{}, err
		}
		update.LegalStatus = &legalStatus
	}

	if input.BusinessDigitization != nil {
		digitization, err := normalizeBusinessDigitization(*input.BusinessDigitization)
		if err != nil {
			return entity.Enterprise{}, err
		}
		update.BusinessDigitization = &digitization
	}

	if input.InterventionNeeds != nil {
		needs, err := normalizeInterventionNeeds(*input.InterventionNeeds)
		if err != nil {
			return entity.Enterprise{}, err
		}
		update.InterventionNeeds = &needs
	}

	if input.TrainingStatus != nil {
		trainingStatus, err := normalizeProcessStatus("training_status", *input.TrainingStatus)
		if err != nil {
			return entity.Enterprise{}, err
		}
		update.TrainingStatus = &trainingStatus
	}

	if input.MentoringStatus != nil {
		mentoringStatus, err := normalizeProcessStatus("mentoring_status", *input.MentoringStatus)
		if err != nil {
			return entity.Enterprise{}, err
		}
		update.MentoringStatus = &mentoringStatus
	}

	if input.CapitalAccess != nil {
		capitalAccess, err := normalizeGeneralStatus("capital_access", *input.CapitalAccess)
		if err != nil {
			return entity.Enterprise{}, err
		}
		update.CapitalAccess = &capitalAccess
	}

	if input.Partnership != nil {
		partnership, err := normalizeGeneralStatus("partnership", *input.Partnership)
		if err != nil {
			return entity.Enterprise{}, err
		}
		update.Partnership = &partnership
	}

	if input.InitialTurnover != nil {
		turnover, err := normalizeTurnover("initial_turnover", *input.InitialTurnover)
		if err != nil {
			return entity.Enterprise{}, err
		}
		update.InitialTurnover = &turnover
	}

	if input.CurrentTurnover != nil {
		turnover, err := normalizeTurnover("current_turnover", *input.CurrentTurnover)
		if err != nil {
			return entity.Enterprise{}, err
		}
		update.CurrentTurnover = &turnover
	}

	if input.District != nil {
		district := strings.TrimSpace(*input.District)
		if len(district) > maxDistrictLength {
			return entity.Enterprise{}, badRequest("district must be at most 128 characters")
		}
		update.District = &district
	}

	if input.Status != nil {
		status, err := normalizeEnterpriseStatus(*input.Status)
		if err != nil {
			return entity.Enterprise{}, err
		}
		update.Status = &status
	}

	event := entity.EnterpriseAuditEvent{
		ActorUserID: actor.ID,
		Action:      entity.AuditActionUpdate,
		CreatedAt:   update.UpdatedAt,
	}

	result, err := u.repo.UpdateEnterprise(ctx, publicID, scopeOwnerID(actor), update, event)
	if err != nil {
		return entity.Enterprise{}, mapEnterpriseRepositoryError(err)
	}

	return result, nil
}

// DeleteEnterprise soft deletes the enterprise matching publicID within the
// actor's scope and records an audit event.
func (u enterpriseUsecase) DeleteEnterprise(
	ctx context.Context,
	actor entity.User,
	publicID string,
) error {
	ownerID := scopeOwnerID(actor)

	existing, err := u.repo.FindEnterpriseByPublicID(ctx, publicID, ownerID)
	if err != nil {
		return mapEnterpriseRepositoryError(err)
	}

	now := u.now()
	event := entity.EnterpriseAuditEvent{
		EnterpriseID:  existing.ID,
		ActorUserID:   actor.ID,
		Action:        entity.AuditActionDelete,
		ChangedFields: map[string]any{},
		CreatedAt:     now,
	}

	if err := u.repo.SoftDeleteEnterprise(ctx, publicID, ownerID, now, event); err != nil {
		return mapEnterpriseRepositoryError(err)
	}

	return nil
}

// ownerRestrictedChange reports whether a non-admin update targets a field
// reserved for admins.
func ownerRestrictedChange(input UpdateEnterpriseInput) bool {
	return input.LegalStatus != nil ||
		input.BusinessDigitization != nil ||
		input.InterventionNeeds != nil ||
		input.TrainingStatus != nil ||
		input.MentoringStatus != nil ||
		input.CapitalAccess != nil ||
		input.Partnership != nil ||
		input.District != nil ||
		input.Status != nil
}

// scopeOwnerID returns the owner filter for actor: zero for admins, who may
// read and mutate every enterprise, and the actor's id for members.
func scopeOwnerID(actor entity.User) int {
	if actor.Role == entity.RoleAdmin {
		return 0
	}

	return actor.ID
}

func normalizeBusinessSector(value string) (entity.BusinessSector, error) {
	sector := entity.BusinessSector(strings.TrimSpace(value))
	if sector == "" {
		return "", nil
	}
	if err := validateBusinessSector(sector); err != nil {
		return "", err
	}

	return sector, nil
}

func validateBusinessSector(sector entity.BusinessSector) error {
	if _, ok := validBusinessSectors[sector]; !ok {
		return badRequest("invalid business_sector")
	}

	return nil
}

func normalizeEnterpriseStatus(value string) (entity.EnterpriseStatus, error) {
	status := entity.EnterpriseStatus(strings.TrimSpace(value))
	if status != "" {
		if err := validateEnterpriseStatus(status); err != nil {
			return "", err
		}
	}

	return status, nil
}

func validateEnterpriseStatus(status entity.EnterpriseStatus) error {
	if _, ok := validEnterpriseStatuses[status]; !ok {
		return badRequest("invalid status")
	}

	return nil
}

func normalizeLegalStatus(value string) (entity.LegalStatus, error) {
	legalStatus := entity.LegalStatus(strings.TrimSpace(value))
	if legalStatus == "" {
		return "", nil
	}
	if _, ok := validLegalStatuses[legalStatus]; !ok {
		return "", badRequest("invalid legal_status")
	}

	return legalStatus, nil
}

func normalizeBusinessDigitization(value string) (entity.BusinessDigitization, error) {
	digitization := entity.BusinessDigitization(strings.TrimSpace(value))
	if digitization == "" {
		return "", nil
	}
	if _, ok := validBusinessDigitizations[digitization]; !ok {
		return "", badRequest("invalid business_digitization")
	}

	return digitization, nil
}

func normalizeInterventionNeeds(value string) (entity.InterventionNeeds, error) {
	needs := entity.InterventionNeeds(strings.TrimSpace(value))
	if needs == "" {
		return "", nil
	}
	if _, ok := validInterventionNeeds[needs]; !ok {
		return "", badRequest("invalid intervention_needs")
	}

	return needs, nil
}

func normalizeProcessStatus(field, value string) (entity.ProcessStatus, error) {
	status := entity.ProcessStatus(strings.TrimSpace(value))
	if status == "" {
		return "", nil
	}
	if _, ok := validProcessStatuses[status]; !ok {
		return "", badRequest("invalid " + field)
	}

	return status, nil
}

func normalizeGeneralStatus(field, value string) (entity.GeneralStatus, error) {
	status := entity.GeneralStatus(strings.TrimSpace(value))
	if status == "" {
		return "", nil
	}
	if _, ok := validGeneralStatuses[status]; !ok {
		return "", badRequest("invalid " + field)
	}

	return status, nil
}

// normalizeTurnover validates a turnover string and returns its canonical
// DECIMAL(15,2) representation. An empty value means zero.
func normalizeTurnover(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "0.00", nil
	}
	if !turnoverPattern.MatchString(value) {
		return "", badRequest("invalid " + field)
	}

	integerPart, fractionPart, _ := strings.Cut(value, ".")
	for len(fractionPart) < 2 {
		fractionPart += "0"
	}
	integerPart = strings.TrimLeft(integerPart, "0")
	if integerPart == "" {
		integerPart = "0"
	}

	return integerPart + "." + fractionPart, nil
}

func mapEnterpriseRepositoryError(err error) error {
	if errors.Is(err, repository.ErrEnterpriseNotFound) {
		return ErrEnterpriseNotFound
	}

	return err
}

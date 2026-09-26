package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

const (
	enterprisePublicIDPrefix     = "TPN-"
	maxEnterpriseNameLength      = 255
	maxDistrictLength            = 128
	maxCommodityLength           = 255
	maxDisporaSupportLength      = 255
	defaultPublicEnterpriseLimit = 9
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
	EnterpriseName       string
	Description          string
	Address              string
	FocusCommodity       string
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
	EnterpriseName       *string
	Description          *string
	Address              *string
	FocusCommodity       *string
	DisporaSupport       *string
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
	Search               string
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

// FindPublicEnterprisesInput bounds and filters a public enterprise listing request.
type FindPublicEnterprisesInput struct {
	Search            string
	District          string
	InterventionNeeds string
	BusinessSector    string
	Cursor            int
	Limit             int
}

// FindPublicEnterprisesResult is one page of public enterprises plus the cursor
// for the following page.
type FindPublicEnterprisesResult struct {
	Enterprises []entity.PublicEnterprise
	NextCursor  int
}

// FindEnterpriseAuditLogsInput bounds an enterprise audit log request.
type FindEnterpriseAuditLogsInput struct {
	Actor    entity.User
	PublicID string
	Cursor   int
	Limit    int
}

// FindEnterpriseAuditLogsResult is one page of enterprise audit events plus the cursor
// for the following page.
type FindEnterpriseAuditLogsResult struct {
	Events     []entity.EnterpriseAuditEventView
	NextCursor int
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
	// FindPublicEnterprises returns one page of active enterprises ordered newest
	// first for the public showcase.
	FindPublicEnterprises(ctx context.Context, input FindPublicEnterprisesInput) (FindPublicEnterprisesResult, error)
	// FindEnterpriseAuditLogs returns one page of audit events for the enterprise
	// matching publicID within the actor's scope.
	FindEnterpriseAuditLogs(ctx context.Context, input FindEnterpriseAuditLogsInput) (FindEnterpriseAuditLogsResult, error)
	// UpdateEnterprise changes the permitted fields of the enterprise matching
	// publicID within the actor's scope. Owners may not change status, dispora_support,
	// or assessment enums; admins may change any mutable field.
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

// GenerateEnterprisePublicID returns a TPN- prefixed identifier with six random decimal
// digits drawn from crypto/rand.
func GenerateEnterprisePublicID() (string, error) {
	suffix, err := rand.Int(rand.Reader, big.NewInt(publicIDMax))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s%0*d", enterprisePublicIDPrefix, publicIDDigits, suffix.Int64()), nil
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

	name := strings.ToUpper(strings.TrimSpace(input.EnterpriseName))
	if name == "" {
		return entity.Enterprise{}, badRequest("enterprise_name is required")
	}
	if len(name) > maxEnterpriseNameLength {
		return entity.Enterprise{}, badRequest("enterprise_name must be at most 255 characters")
	}

	description := strings.TrimSpace(input.Description)
	address := strings.TrimSpace(input.Address)
	focusCommodity := strings.TrimSpace(input.FocusCommodity)
	if len(focusCommodity) > maxCommodityLength {
		return entity.Enterprise{}, badRequest("focus_commodity must be at most 255 characters")
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
		publicID, err := GenerateEnterprisePublicID()
		if err != nil {
			return entity.Enterprise{}, fmt.Errorf("generate enterprise public id: %w", err)
		}

		enterprise := entity.Enterprise{
			PublicID:             publicID,
			UserID:               input.Actor.ID,
			EnterpriseName:       name,
			Description:          description,
			Address:              address,
			FocusCommodity:       focusCommodity,
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
				"enterprise_name":       name,
				"description":           entity.NullableAuditValue(description),
				"address":               entity.NullableAuditValue(address),
				"focus_commodity":       entity.NullableAuditValue(focusCommodity),
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

// FindPublicEnterprises returns one page of active enterprises ordered newest
// first for the public showcase.
func (u enterpriseUsecase) FindPublicEnterprises(
	ctx context.Context,
	input FindPublicEnterprisesInput,
) (FindPublicEnterprisesResult, error) {
	sector, err := normalizeBusinessSector(input.BusinessSector)
	if err != nil {
		return FindPublicEnterprisesResult{}, err
	}
	interventionNeeds, err := normalizeInterventionNeeds(input.InterventionNeeds)
	if err != nil {
		return FindPublicEnterprisesResult{}, err
	}

	limit := clampPublicEnterpriseLimit(input.Limit)
	filter := entity.PublicEnterpriseFilter{
		Search:            strings.TrimSpace(input.Search),
		District:          strings.TrimSpace(input.District),
		InterventionNeeds: interventionNeeds,
		BusinessSector:    sector,
		Cursor:            input.Cursor,
		Limit:             limit + 1,
	}

	items, err := u.repo.FindPublicEnterprises(ctx, filter)
	if err != nil {
		return FindPublicEnterprisesResult{}, fmt.Errorf("find public enterprises: %w", err)
	}

	result := FindPublicEnterprisesResult{Enterprises: items}
	if len(items) > limit {
		result.Enterprises = items[:limit]
		result.NextCursor = items[limit-1].ID
	}

	return result, nil
}

// FindEnterpriseAuditLogs returns one page of audit events for the enterprise
// matching publicID within the actor's scope.
func (u enterpriseUsecase) FindEnterpriseAuditLogs(
	ctx context.Context,
	input FindEnterpriseAuditLogsInput,
) (FindEnterpriseAuditLogsResult, error) {
	if input.Actor.ID == 0 {
		return FindEnterpriseAuditLogsResult{}, ErrForbidden
	}

	publicID := strings.TrimSpace(input.PublicID)
	if publicID == "" {
		return FindEnterpriseAuditLogsResult{}, ErrEnterpriseNotFound
	}

	limit := clampLimit(input.Limit)
	events, err := u.repo.FindEnterpriseAuditEvents(ctx, entity.EnterpriseAuditFilter{
		PublicID: publicID,
		OwnerID:  scopeOwnerID(input.Actor),
		Cursor:   input.Cursor,
		Limit:    limit + 1,
	})
	if err != nil {
		return FindEnterpriseAuditLogsResult{}, mapEnterpriseRepositoryError(err)
	}

	result := FindEnterpriseAuditLogsResult{Events: events}
	if len(events) > limit {
		result.Events = events[:limit]
		result.NextCursor = events[limit-1].ID
	}

	return result, nil
}

func clampPublicEnterpriseLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultPublicEnterpriseLimit
	case limit > maxListLimit:
		return maxListLimit
	default:
		return limit
	}
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
		Search:               strings.TrimSpace(input.Search),
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

	if input.EnterpriseName != nil {
		name := strings.ToUpper(strings.TrimSpace(*input.EnterpriseName))
		if name == "" {
			return entity.Enterprise{}, badRequest("enterprise_name cannot be empty")
		}
		if len(name) > maxEnterpriseNameLength {
			return entity.Enterprise{}, badRequest("enterprise_name must be at most 255 characters")
		}
		update.EnterpriseName = &name
	}

	if input.Description != nil {
		desc := strings.TrimSpace(*input.Description)
		update.Description = &desc
	}

	if input.Address != nil {
		addr := strings.TrimSpace(*input.Address)
		update.Address = &addr
	}

	if input.FocusCommodity != nil {
		commodity := strings.TrimSpace(*input.FocusCommodity)
		if len(commodity) > maxCommodityLength {
			return entity.Enterprise{}, badRequest("focus_commodity must be at most 255 characters")
		}
		update.FocusCommodity = &commodity
	}

	if input.DisporaSupport != nil {
		support := strings.TrimSpace(*input.DisporaSupport)
		if len(support) > maxDisporaSupportLength {
			return entity.Enterprise{}, badRequest("dispora_support must be at most 255 characters")
		}
		update.DisporaSupport = &support
	}

	if input.BusinessSector != nil {
		sector, err := normalizeBusinessSector(*input.BusinessSector)
		if err != nil {
			return entity.Enterprise{}, err
		}
		if sector == "" {
			return entity.Enterprise{}, badRequest("invalid business_sector")
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
		input.DisporaSupport != nil ||
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

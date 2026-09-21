package usecase_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

var enterprisePublicIDPattern = regexp.MustCompile(`^TPN-[0-9]{6}$`)

type fakeEnterpriseRepository struct {
	createErr      error
	createCalls    int
	lastCreated    entity.Enterprise
	lastCreateEvnt entity.EnterpriseAuditEvent

	findResult       entity.Enterprise
	findErr          error
	lastFindPublicID string
	lastFindOwnerID  int

	listResult []entity.Enterprise
	listErr    error
	lastFilter entity.EnterpriseFilter

	publicResult     []entity.PublicEnterprise
	publicErr        error
	lastPublicFilter entity.PublicEnterpriseFilter

	auditResult     []entity.EnterpriseAuditEventView
	auditErr        error
	lastAuditFilter entity.EnterpriseAuditFilter

	updateErr       error
	updateCalls     int
	updateResult    entity.Enterprise
	lastUpdate      entity.EnterpriseUpdate
	lastUpdateOwner int
	lastUpdateEvent entity.EnterpriseAuditEvent

	deleteErr           error
	lastDeletedPublicID string
	lastDeleteOwnerID   int
	lastDeleteEvent     entity.EnterpriseAuditEvent
}

func (f *fakeEnterpriseRepository) CreateEnterprise(
	_ context.Context,
	enterprise entity.Enterprise,
	event entity.EnterpriseAuditEvent,
) (entity.Enterprise, error) {
	f.createCalls++
	f.lastCreated = enterprise
	f.lastCreateEvnt = event
	if f.createErr != nil {
		return entity.Enterprise{}, f.createErr
	}

	created := enterprise
	created.ID = f.createCalls

	return created, nil
}

func (f *fakeEnterpriseRepository) FindEnterpriseByPublicID(
	_ context.Context,
	publicID string,
	ownerID int,
) (entity.Enterprise, error) {
	f.lastFindPublicID = publicID
	f.lastFindOwnerID = ownerID
	if f.findErr != nil {
		return entity.Enterprise{}, f.findErr
	}
	if f.findResult.PublicID == "" {
		return entity.Enterprise{}, repository.ErrEnterpriseNotFound
	}

	return f.findResult, nil
}

func (f *fakeEnterpriseRepository) FindEnterprises(
	_ context.Context,
	filter entity.EnterpriseFilter,
) ([]entity.Enterprise, error) {
	f.lastFilter = filter

	return f.listResult, f.listErr
}

func (f *fakeEnterpriseRepository) FindPublicEnterprises(
	_ context.Context,
	filter entity.PublicEnterpriseFilter,
) ([]entity.PublicEnterprise, error) {
	f.lastPublicFilter = filter

	return f.publicResult, f.publicErr
}

func (f *fakeEnterpriseRepository) FindEnterpriseAuditEvents(
	_ context.Context,
	filter entity.EnterpriseAuditFilter,
) ([]entity.EnterpriseAuditEventView, error) {
	f.lastAuditFilter = filter

	return f.auditResult, f.auditErr
}

func (f *fakeEnterpriseRepository) UpdateEnterprise(
	_ context.Context,
	_ string,
	ownerID int,
	update entity.EnterpriseUpdate,
	event entity.EnterpriseAuditEvent,
) (entity.Enterprise, error) {
	f.updateCalls++
	f.lastUpdate = update
	f.lastUpdateOwner = ownerID
	f.lastUpdateEvent = event
	if f.updateErr != nil {
		return entity.Enterprise{}, f.updateErr
	}
	if f.updateResult.PublicID != "" {
		return f.updateResult, nil
	}

	return applyFakeEnterpriseUpdate(f.findResult, update), nil
}

func applyFakeEnterpriseUpdate(enterprise entity.Enterprise, update entity.EnterpriseUpdate) entity.Enterprise {
	if update.EnterpriseName != nil {
		enterprise.EnterpriseName = *update.EnterpriseName
	}
	if update.Description != nil {
		enterprise.Description = *update.Description
	}
	if update.Address != nil {
		enterprise.Address = *update.Address
	}
	if update.FocusCommodity != nil {
		enterprise.FocusCommodity = *update.FocusCommodity
	}
	if update.DisporaSupport != nil {
		enterprise.DisporaSupport = *update.DisporaSupport
	}
	if update.BusinessSector != nil {
		enterprise.BusinessSector = *update.BusinessSector
	}
	if update.LegalStatus != nil {
		enterprise.LegalStatus = *update.LegalStatus
	}
	if update.BusinessDigitization != nil {
		enterprise.BusinessDigitization = *update.BusinessDigitization
	}
	if update.InterventionNeeds != nil {
		enterprise.InterventionNeeds = *update.InterventionNeeds
	}
	if update.TrainingStatus != nil {
		enterprise.TrainingStatus = *update.TrainingStatus
	}
	if update.MentoringStatus != nil {
		enterprise.MentoringStatus = *update.MentoringStatus
	}
	if update.CapitalAccess != nil {
		enterprise.CapitalAccess = *update.CapitalAccess
	}
	if update.Partnership != nil {
		enterprise.Partnership = *update.Partnership
	}
	if update.InitialTurnover != nil {
		enterprise.InitialTurnover = *update.InitialTurnover
	}
	if update.CurrentTurnover != nil {
		enterprise.CurrentTurnover = *update.CurrentTurnover
	}
	if update.District != nil {
		enterprise.District = *update.District
	}
	if update.Status != nil {
		enterprise.Status = *update.Status
	}

	enterprise.UpdatedAt = update.UpdatedAt

	return enterprise
}

func (f *fakeEnterpriseRepository) SoftDeleteEnterprise(
	_ context.Context,
	publicID string,
	ownerID int,
	_ int64,
	event entity.EnterpriseAuditEvent,
) error {
	f.lastDeletedPublicID = publicID
	f.lastDeleteOwnerID = ownerID
	f.lastDeleteEvent = event

	return f.deleteErr
}

func stringPtr(value string) *string {
	return &value
}

func memberActor() entity.User {
	return entity.User{ID: 7, PublicID: "YTP-000007", Role: entity.RoleMember}
}

func adminActor() entity.User {
	return entity.User{ID: 9, PublicID: "YTP-000009", Role: entity.RoleAdmin}
}

func TestCreateEnterpriseAssignsOwnerAndNormalizesInput(t *testing.T) {
	repo := &fakeEnterpriseRepository{}
	uc := usecase.NewEnterpriseUseCase(repo)

	created, err := uc.CreateEnterprise(context.Background(), usecase.CreateEnterpriseInput{
		Actor:           memberActor(),
		EnterpriseName:  "  Warung Kopi  ",
		BusinessSector:  "Kuliner",
		InitialTurnover: "1500",
		CurrentTurnover: "",
		District:        " Bandung ",
	})
	if err != nil {
		t.Fatalf("CreateEnterprise() error = %v", err)
	}
	if !enterprisePublicIDPattern.MatchString(created.PublicID) {
		t.Errorf("public id = %q, want TPN- plus six digits", created.PublicID)
	}
	if created.UserID != 7 {
		t.Errorf("owner = %d, want 7", created.UserID)
	}
	if created.EnterpriseName != "Warung Kopi" {
		t.Errorf("enterprise_name = %q, want trimmed", created.EnterpriseName)
	}
	if created.InitialTurnover != "1500.00" || created.CurrentTurnover != "0.00" {
		t.Errorf("turnovers = (%q, %q), want canonical 1500.00 and 0.00", created.InitialTurnover, created.CurrentTurnover)
	}
	if created.District != "Bandung" {
		t.Errorf("district = %q, want trimmed", created.District)
	}
	if created.Status != entity.EnterpriseStatusActive {
		t.Errorf("status = %q, want active", created.Status)
	}
	if repo.lastCreateEvnt.Action != entity.AuditActionCreate {
		t.Errorf("audit action = %q, want create", repo.lastCreateEvnt.Action)
	}
	if repo.lastCreateEvnt.ActorUserID != 7 {
		t.Errorf("audit actor = %d, want 7", repo.lastCreateEvnt.ActorUserID)
	}
	if repo.lastCreateEvnt.ChangedFields["enterprise_name"] != "Warung Kopi" {
		t.Errorf("audit changed fields = %v, want enterprise_name", repo.lastCreateEvnt.ChangedFields)
	}
}

func TestCreateEnterpriseCanonicalizesTurnoverPrecision(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "0.5", want: "0.50"},
		{input: "0.00", want: "0.00"},
		{input: "1500", want: "1500.00"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			repo := &fakeEnterpriseRepository{}
			uc := usecase.NewEnterpriseUseCase(repo)

			created, err := uc.CreateEnterprise(context.Background(), usecase.CreateEnterpriseInput{
				Actor:           memberActor(),
				EnterpriseName:  "Toko",
				BusinessSector:  "Perdagangan Ritel",
				InitialTurnover: tc.input,
			})
			if err != nil {
				t.Fatalf("CreateEnterprise() error = %v", err)
			}
			if created.InitialTurnover != tc.want {
				t.Errorf("initial_turnover = %q, want %q", created.InitialTurnover, tc.want)
			}
		})
	}
}

func TestCreateEnterpriseRejectsZeroActor(t *testing.T) {
	repo := &fakeEnterpriseRepository{}
	uc := usecase.NewEnterpriseUseCase(repo)

	_, err := uc.CreateEnterprise(context.Background(), usecase.CreateEnterpriseInput{
		EnterpriseName: "Toko",
		BusinessSector: "Perdagangan Ritel",
	})
	if !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("CreateEnterprise() error = %v, want ErrForbidden", err)
	}
	if repo.createCalls != 0 {
		t.Errorf("create calls = %d, want 0", repo.createCalls)
	}
}

func TestCreateEnterpriseRequiresNonEmptyName(t *testing.T) {
	repo := &fakeEnterpriseRepository{}
	uc := usecase.NewEnterpriseUseCase(repo)

	_, err := uc.CreateEnterprise(context.Background(), usecase.CreateEnterpriseInput{
		Actor:          memberActor(),
		EnterpriseName: "   ",
		BusinessSector: "Perdagangan Ritel",
	})
	if !errors.Is(err, usecase.ErrBadRequest) {
		t.Fatalf("CreateEnterprise() error = %v, want ErrBadRequest", err)
	}
}

func TestCreateEnterpriseAllowsManyPerOwner(t *testing.T) {
	repo := &fakeEnterpriseRepository{}
	uc := usecase.NewEnterpriseUseCase(repo)

	for range 2 {
		if _, err := uc.CreateEnterprise(context.Background(), usecase.CreateEnterpriseInput{
			Actor:          memberActor(),
			EnterpriseName: "Toko",
			BusinessSector: "Perdagangan Ritel",
		}); err != nil {
			t.Fatalf("CreateEnterprise() error = %v", err)
		}
	}

	if repo.createCalls != 2 {
		t.Fatalf("create calls = %d, want 2", repo.createCalls)
	}
	if repo.lastCreated.UserID != 7 {
		t.Errorf("owner = %d, want 7", repo.lastCreated.UserID)
	}
}

func TestCreateEnterpriseValidation(t *testing.T) {
	cases := []struct {
		name  string
		input usecase.CreateEnterpriseInput
	}{
		{
			name:  "name too long",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), EnterpriseName: strings.Repeat("a", 256), BusinessSector: "Perdagangan Ritel"},
		},
		{
			name:  "unknown sector",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), EnterpriseName: "Toko", BusinessSector: "bogus"},
		},
		{
			name:  "unknown legal status",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), EnterpriseName: "Toko", BusinessSector: "Perdagangan Ritel", LegalStatus: "bogus"},
		},
		{
			name:  "unknown digitization",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), EnterpriseName: "Toko", BusinessSector: "Perdagangan Ritel", BusinessDigitization: "bogus"},
		},
		{
			name:  "unknown intervention needs",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), EnterpriseName: "Toko", BusinessSector: "Perdagangan Ritel", InterventionNeeds: "bogus"},
		},
		{
			name:  "unknown training status",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), EnterpriseName: "Toko", BusinessSector: "Perdagangan Ritel", TrainingStatus: "bogus"},
		},
		{
			name:  "unknown capital access",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), EnterpriseName: "Toko", BusinessSector: "Perdagangan Ritel", CapitalAccess: "bogus"},
		},
		{
			name:  "negative turnover",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), EnterpriseName: "Toko", BusinessSector: "Perdagangan Ritel", InitialTurnover: "-1"},
		},
		{
			name:  "too many decimals",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), EnterpriseName: "Toko", BusinessSector: "Perdagangan Ritel", CurrentTurnover: "1.234"},
		},
		{
			name:  "turnover beyond fifteen digits",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), EnterpriseName: "Toko", BusinessSector: "Perdagangan Ritel", CurrentTurnover: "10000000000000"},
		},
		{
			name: "district too long",
			input: usecase.CreateEnterpriseInput{
				Actor:          memberActor(),
				EnterpriseName: "Toko",
				BusinessSector: "Perdagangan Ritel",
				District:       strings.Repeat("a", 129),
			},
		},
		{
			name: "focus commodity too long",
			input: usecase.CreateEnterpriseInput{
				Actor:          memberActor(),
				EnterpriseName: "Toko",
				BusinessSector: "Perdagangan Ritel",
				FocusCommodity: strings.Repeat("a", 256),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeEnterpriseRepository{}
			uc := usecase.NewEnterpriseUseCase(repo)

			if _, err := uc.CreateEnterprise(context.Background(), tc.input); !errors.Is(err, usecase.ErrBadRequest) {
				t.Fatalf("CreateEnterprise() error = %v, want ErrBadRequest", err)
			}
			if repo.createCalls != 0 {
				t.Errorf("create calls = %d, want 0", repo.createCalls)
			}
		})
	}
}

func TestCreateEnterpriseRetriesPublicIDCollisions(t *testing.T) {
	repo := &fakeEnterpriseRepository{createErr: repository.ErrDuplicateEnterprisePublicID}
	uc := usecase.NewEnterpriseUseCase(repo)

	_, err := uc.CreateEnterprise(context.Background(), usecase.CreateEnterpriseInput{
		Actor:          memberActor(),
		EnterpriseName: "Toko",
		BusinessSector: "Perdagangan Ritel",
	})
	if !errors.Is(err, usecase.ErrPublicIDGeneration) {
		t.Fatalf("CreateEnterprise() error = %v, want ErrPublicIDGeneration", err)
	}
	if repo.createCalls != 5 {
		t.Errorf("create calls = %d, want 5", repo.createCalls)
	}
}

func TestFindEnterprisesScopesMemberToOwner(t *testing.T) {
	repo := &fakeEnterpriseRepository{}
	uc := usecase.NewEnterpriseUseCase(repo)

	if _, err := uc.FindEnterprises(context.Background(), usecase.FindEnterprisesInput{Actor: memberActor()}); err != nil {
		t.Fatalf("FindEnterprises() error = %v", err)
	}
	if repo.lastFilter.OwnerID != 7 {
		t.Errorf("owner filter = %d, want 7", repo.lastFilter.OwnerID)
	}
	if repo.lastFilter.Limit != 21 {
		t.Errorf("limit = %d, want 21", repo.lastFilter.Limit)
	}
}

func TestFindEnterprisesAdminSeesEveryOwner(t *testing.T) {
	repo := &fakeEnterpriseRepository{}
	uc := usecase.NewEnterpriseUseCase(repo)

	if _, err := uc.FindEnterprises(context.Background(), usecase.FindEnterprisesInput{Actor: adminActor()}); err != nil {
		t.Fatalf("FindEnterprises() error = %v", err)
	}
	if repo.lastFilter.OwnerID != 0 {
		t.Errorf("owner filter = %d, want 0 for admin", repo.lastFilter.OwnerID)
	}
}

func TestFindEnterprisesPaginatesWithLimitPlusOne(t *testing.T) {
	repo := &fakeEnterpriseRepository{listResult: []entity.Enterprise{{ID: 1}, {ID: 2}, {ID: 3}}}
	uc := usecase.NewEnterpriseUseCase(repo)

	result, err := uc.FindEnterprises(context.Background(), usecase.FindEnterprisesInput{Actor: memberActor(), Limit: 2})
	if err != nil {
		t.Fatalf("FindEnterprises() error = %v", err)
	}
	if repo.lastFilter.Limit != 3 {
		t.Errorf("repository limit = %d, want 3", repo.lastFilter.Limit)
	}
	if len(result.Enterprises) != 2 {
		t.Fatalf("enterprises = %d, want 2", len(result.Enterprises))
	}
	if result.NextCursor != 2 {
		t.Errorf("next cursor = %d, want 2", result.NextCursor)
	}
}

func TestFindPublicEnterprisesOrdersNewestAndPaginates(t *testing.T) {
	repo := &fakeEnterpriseRepository{
		publicResult: []entity.PublicEnterprise{
			{ID: 30, PublicID: "TPN-000030", EnterpriseName: "Three"},
			{ID: 20, PublicID: "TPN-000020", EnterpriseName: "Two"},
			{ID: 10, PublicID: "TPN-000010", EnterpriseName: "One"},
		},
	}
	uc := usecase.NewEnterpriseUseCase(repo)

	result, err := uc.FindPublicEnterprises(context.Background(), usecase.FindPublicEnterprisesInput{
		Search:            "Three",
		District:          "Bandung",
		InterventionNeeds: "Pelatihan",
		BusinessSector:    "Kuliner",
		Cursor:            50,
		Limit:             2,
	})
	if err != nil {
		t.Fatalf("FindPublicEnterprises() error = %v", err)
	}
	if repo.lastPublicFilter.Search != "Three" {
		t.Errorf("search filter = %q, want Three", repo.lastPublicFilter.Search)
	}
	if repo.lastPublicFilter.District != "Bandung" {
		t.Errorf("district filter = %q, want Bandung", repo.lastPublicFilter.District)
	}
	if repo.lastPublicFilter.InterventionNeeds != entity.InterventionNeedsPelatihan {
		t.Errorf("intervention needs = %v, want Pelatihan", repo.lastPublicFilter.InterventionNeeds)
	}
	if repo.lastPublicFilter.BusinessSector != entity.BusinessSectorKuliner {
		t.Errorf("business sector = %v, want Kuliner", repo.lastPublicFilter.BusinessSector)
	}
	if repo.lastPublicFilter.Cursor != 50 {
		t.Errorf("cursor = %d, want 50", repo.lastPublicFilter.Cursor)
	}
	if repo.lastPublicFilter.Limit != 3 {
		t.Errorf("limit = %d, want 3 (limit+1)", repo.lastPublicFilter.Limit)
	}
	if len(result.Enterprises) != 2 {
		t.Fatalf("enterprises = %d, want 2", len(result.Enterprises))
	}
	if result.NextCursor != 20 {
		t.Errorf("next cursor = %d, want 20", result.NextCursor)
	}
}

func TestFindPublicEnterprisesDefaultsLimitToNine(t *testing.T) {
	repo := &fakeEnterpriseRepository{
		publicResult: []entity.PublicEnterprise{
			{ID: 1, PublicID: "TPN-000001", EnterpriseName: "One"},
		},
	}
	uc := usecase.NewEnterpriseUseCase(repo)

	_, err := uc.FindPublicEnterprises(context.Background(), usecase.FindPublicEnterprisesInput{})
	if err != nil {
		t.Fatalf("FindPublicEnterprises() error = %v", err)
	}
	if repo.lastPublicFilter.Limit != 10 { // 9 + 1 for next page detection
		t.Errorf("public filter limit = %d, want 10 (default 9 + 1)", repo.lastPublicFilter.Limit)
	}
}

func TestFindEnterprisesRejectsInvalidFilters(t *testing.T) {
	cases := []usecase.FindEnterprisesInput{
		{Actor: memberActor(), Status: "bogus"},
		{Actor: memberActor(), BusinessSector: "bogus"},
		{Actor: memberActor(), LegalStatus: "bogus"},
		{Actor: memberActor(), BusinessDigitization: "bogus"},
		{Actor: memberActor(), InterventionNeeds: "bogus"},
		{Actor: memberActor(), TrainingStatus: "bogus"},
		{Actor: memberActor(), MentoringStatus: "bogus"},
		{Actor: memberActor(), CapitalAccess: "bogus"},
		{Actor: memberActor(), Partnership: "bogus"},
	}

	for _, input := range cases {
		repo := &fakeEnterpriseRepository{}
		uc := usecase.NewEnterpriseUseCase(repo)

		if _, err := uc.FindEnterprises(context.Background(), input); !errors.Is(err, usecase.ErrBadRequest) {
			t.Fatalf("FindEnterprises() error = %v, want ErrBadRequest", err)
		}
	}
}

func TestFindEnterprisesPassesEnumFilters(t *testing.T) {
	repo := &fakeEnterpriseRepository{}
	uc := usecase.NewEnterpriseUseCase(repo)

	_, err := uc.FindEnterprises(context.Background(), usecase.FindEnterprisesInput{
		Actor:                memberActor(),
		Search:               "Warung",
		LegalStatus:          "complete",
		BusinessDigitization: "high",
		InterventionNeeds:    "Pelatihan",
		TrainingStatus:       "completed",
		MentoringStatus:      "ongoing",
		CapitalAccess:        "yes",
		Partnership:          "no",
	})
	if err != nil {
		t.Fatalf("FindEnterprises() error = %v", err)
	}
	if repo.lastFilter.Search != "Warung" ||
		repo.lastFilter.LegalStatus != entity.LegalStatusComplete ||
		repo.lastFilter.BusinessDigitization != entity.BusinessDigitizationHigh ||
		repo.lastFilter.InterventionNeeds != entity.InterventionNeedsPelatihan ||
		repo.lastFilter.TrainingStatus != entity.ProcessStatusCompleted ||
		repo.lastFilter.MentoringStatus != entity.ProcessStatusOngoing ||
		repo.lastFilter.CapitalAccess != entity.GeneralStatusYes ||
		repo.lastFilter.Partnership != entity.GeneralStatusNo {
		t.Errorf("filter = %+v, want search and enum filters passed through", repo.lastFilter)
	}
}

func TestGetEnterpriseScopesOwnerAndMapsNotFound(t *testing.T) {
	repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{ID: 3, PublicID: "TPN-000003"}}
	uc := usecase.NewEnterpriseUseCase(repo)

	if _, err := uc.GetEnterprise(context.Background(), memberActor(), "TPN-000003"); err != nil {
		t.Fatalf("GetEnterprise() error = %v", err)
	}
	if repo.lastFindOwnerID != 7 {
		t.Errorf("owner scope = %d, want 7", repo.lastFindOwnerID)
	}

	missing := &fakeEnterpriseRepository{}
	uc = usecase.NewEnterpriseUseCase(missing)
	if _, err := uc.GetEnterprise(context.Background(), memberActor(), "TPN-000404"); !errors.Is(err, usecase.ErrEnterpriseNotFound) {
		t.Fatalf("GetEnterprise() error = %v, want ErrEnterpriseNotFound", err)
	}
}

func TestAdminGetEnterpriseIsUnscoped(t *testing.T) {
	repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{ID: 3, PublicID: "TPN-000003"}}
	uc := usecase.NewEnterpriseUseCase(repo)

	if _, err := uc.GetEnterprise(context.Background(), adminActor(), "TPN-000003"); err != nil {
		t.Fatalf("GetEnterprise() error = %v", err)
	}
	if repo.lastFindOwnerID != 0 {
		t.Errorf("owner scope = %d, want 0 for admin", repo.lastFindOwnerID)
	}
}

func TestUpdateEnterpriseOwnerChangesAllowedFields(t *testing.T) {
	repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{
		ID:              5,
		PublicID:        "TPN-000005",
		UserID:          7,
		EnterpriseName:  "Old",
		BusinessSector:  entity.BusinessSectorPerdaganganRitel,
		District:        "Old District",
		InitialTurnover: "10.00",
		CurrentTurnover: "20.00",
		Status:          entity.EnterpriseStatusActive,
	}}
	uc := usecase.NewEnterpriseUseCase(repo)

	updated, err := uc.UpdateEnterprise(context.Background(), memberActor(), "TPN-000005", usecase.UpdateEnterpriseInput{
		EnterpriseName:  stringPtr("New"),
		District:        stringPtr("New District"),
		Description:     stringPtr("New Description"),
		Address:         stringPtr("New Address"),
		FocusCommodity:  stringPtr("New Focus"),
		CurrentTurnover: stringPtr("30"),
	})
	if err != nil {
		t.Fatalf("UpdateEnterprise() error = %v", err)
	}
	if repo.lastUpdateOwner != 7 {
		t.Errorf("owner scope = %d, want 7", repo.lastUpdateOwner)
	}
	if updated.EnterpriseName != "New" || updated.CurrentTurnover != "30.00" {
		t.Errorf("updated = %+v, want New and 30.00", updated)
	}
	if updated.District != "New District" {
		t.Errorf("district = %q, want New District", updated.District)
	}
	if updated.Description != "New Description" || updated.Address != "New Address" || updated.FocusCommodity != "New Focus" {
		t.Errorf("updated text fields = %+v", updated)
	}
	if updated.InitialTurnover != "10.00" {
		t.Errorf("initial turnover = %q, want untouched 10.00", updated.InitialTurnover)
	}
	if repo.lastUpdate.EnterpriseName == nil || *repo.lastUpdate.EnterpriseName != "New" {
		t.Errorf("update patch enterprise_name = %v, want New", repo.lastUpdate.EnterpriseName)
	}
	if repo.lastUpdate.CurrentTurnover == nil || *repo.lastUpdate.CurrentTurnover != "30.00" {
		t.Errorf("update patch current_turnover = %v, want canonical 30.00", repo.lastUpdate.CurrentTurnover)
	}
	if repo.lastUpdate.InitialTurnover != nil {
		t.Errorf("update patch initial_turnover = %v, want untouched", repo.lastUpdate.InitialTurnover)
	}
	if repo.lastUpdate.BusinessSector != nil {
		t.Errorf("update patch business_sector = %v, want untouched", repo.lastUpdate.BusinessSector)
	}
}

func TestUpdateEnterpriseOwnerCannotChangeStatusOrAssessment(t *testing.T) {
	cases := []usecase.UpdateEnterpriseInput{
		{Status: stringPtr("inactive")},
		{DisporaSupport: stringPtr("Grant 2025")},
		{LegalStatus: stringPtr("complete")},
		{BusinessDigitization: stringPtr("high")},
		{InterventionNeeds: stringPtr("Pelatihan")},
		{TrainingStatus: stringPtr("completed")},
		{MentoringStatus: stringPtr("ongoing")},
		{CapitalAccess: stringPtr("yes")},
		{Partnership: stringPtr("no")},
	}

	for _, input := range cases {
		repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{ID: 5, PublicID: "TPN-000005", UserID: 7}}
		uc := usecase.NewEnterpriseUseCase(repo)

		if _, err := uc.UpdateEnterprise(context.Background(), memberActor(), "TPN-000005", input); !errors.Is(err, usecase.ErrForbidden) {
			t.Fatalf("UpdateEnterprise() error = %v, want ErrForbidden", err)
		}
		if repo.updateCalls != 0 {
			t.Errorf("update calls = %d, want 0", repo.updateCalls)
		}
	}
}

func TestUpdateEnterpriseAdminChangesAnyMutableField(t *testing.T) {
	repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{
		ID:              5,
		PublicID:        "TPN-000005",
		Status:          entity.EnterpriseStatusActive,
		BusinessSector:  entity.BusinessSectorPerdaganganRitel,
		InitialTurnover: "0.00",
		CurrentTurnover: "0.00",
	}}
	uc := usecase.NewEnterpriseUseCase(repo)

	updated, err := uc.UpdateEnterprise(context.Background(), adminActor(), "TPN-000005", usecase.UpdateEnterpriseInput{
		Status:               stringPtr("inactive"),
		District:             stringPtr("Jakarta"),
		DisporaSupport:       stringPtr("Grant 2025"),
		LegalStatus:          stringPtr("complete"),
		BusinessDigitization: stringPtr("high"),
		InterventionNeeds:    stringPtr("Pelatihan"),
		TrainingStatus:       stringPtr("completed"),
		MentoringStatus:      stringPtr("ongoing"),
		CapitalAccess:        stringPtr("yes"),
		Partnership:          stringPtr("no"),
	})
	if err != nil {
		t.Fatalf("UpdateEnterprise() error = %v", err)
	}
	if updated.Status != entity.EnterpriseStatusInactive || updated.District != "Jakarta" || updated.DisporaSupport != "Grant 2025" {
		t.Errorf("updated = %+v, want inactive, Jakarta, and Grant 2025", updated)
	}
	if updated.LegalStatus != entity.LegalStatusComplete ||
		updated.BusinessDigitization != entity.BusinessDigitizationHigh ||
		updated.InterventionNeeds != entity.InterventionNeedsPelatihan ||
		updated.TrainingStatus != entity.ProcessStatusCompleted ||
		updated.MentoringStatus != entity.ProcessStatusOngoing ||
		updated.CapitalAccess != entity.GeneralStatusYes ||
		updated.Partnership != entity.GeneralStatusNo {
		t.Errorf("updated assessment = %+v, want all fields applied", updated)
	}
	if repo.lastUpdateOwner != 0 {
		t.Errorf("owner scope = %d, want 0 for admin", repo.lastUpdateOwner)
	}
}

func TestUpdateEnterpriseForwardsUnchangedValueToRepository(t *testing.T) {
	repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{
		ID:              5,
		PublicID:        "TPN-000005",
		EnterpriseName:  "Old",
		BusinessSector:  entity.BusinessSectorPerdaganganRitel,
		InitialTurnover: "0.00",
		CurrentTurnover: "0.00",
		Status:          entity.EnterpriseStatusActive,
	}}
	uc := usecase.NewEnterpriseUseCase(repo)

	result, err := uc.UpdateEnterprise(context.Background(), memberActor(), "TPN-000005", usecase.UpdateEnterpriseInput{
		EnterpriseName: stringPtr("Old"),
	})
	if err != nil {
		t.Fatalf("UpdateEnterprise() error = %v", err)
	}
	// The repository owns the locked diff and skips the write when nothing
	// changed; the usecase only forwards the requested value.
	if repo.updateCalls != 1 {
		t.Errorf("update calls = %d, want 1", repo.updateCalls)
	}
	if repo.lastUpdate.EnterpriseName == nil || *repo.lastUpdate.EnterpriseName != "Old" {
		t.Errorf("update patch enterprise_name = %v, want Old", repo.lastUpdate.EnterpriseName)
	}
	if result.EnterpriseName != "Old" {
		t.Errorf("enterprise_name = %q, want Old", result.EnterpriseName)
	}
}

func TestUpdateEnterpriseValidation(t *testing.T) {
	repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{ID: 5, PublicID: "TPN-000005", UserID: 7}}
	uc := usecase.NewEnterpriseUseCase(repo)

	_, err := uc.UpdateEnterprise(context.Background(), memberActor(), "TPN-000005", usecase.UpdateEnterpriseInput{
		BusinessSector: stringPtr("bogus"),
	})
	if !errors.Is(err, usecase.ErrBadRequest) {
		t.Fatalf("UpdateEnterprise() error = %v, want ErrBadRequest", err)
	}
}

func TestUpdateEnterpriseMapsNotFound(t *testing.T) {
	repo := &fakeEnterpriseRepository{updateErr: repository.ErrEnterpriseNotFound}
	uc := usecase.NewEnterpriseUseCase(repo)

	_, err := uc.UpdateEnterprise(context.Background(), memberActor(), "TPN-000404", usecase.UpdateEnterpriseInput{})
	if !errors.Is(err, usecase.ErrEnterpriseNotFound) {
		t.Fatalf("UpdateEnterprise() error = %v, want ErrEnterpriseNotFound", err)
	}
}

func TestDeleteEnterpriseRecordsAudit(t *testing.T) {
	repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{ID: 5, PublicID: "TPN-000005", UserID: 7}}
	uc := usecase.NewEnterpriseUseCase(repo)

	if err := uc.DeleteEnterprise(context.Background(), memberActor(), "TPN-000005"); err != nil {
		t.Fatalf("DeleteEnterprise() error = %v", err)
	}
	if repo.lastDeleteOwnerID != 7 {
		t.Errorf("owner scope = %d, want 7", repo.lastDeleteOwnerID)
	}
	if repo.lastDeleteEvent.EnterpriseID != 5 || repo.lastDeleteEvent.Action != entity.AuditActionDelete {
		t.Errorf("audit event = %+v, want delete for enterprise 5", repo.lastDeleteEvent)
	}
}

func TestDeleteEnterpriseMapsNotFound(t *testing.T) {
	repo := &fakeEnterpriseRepository{}
	uc := usecase.NewEnterpriseUseCase(repo)

	if err := uc.DeleteEnterprise(context.Background(), memberActor(), "TPN-000404"); !errors.Is(err, usecase.ErrEnterpriseNotFound) {
		t.Fatalf("DeleteEnterprise() error = %v, want ErrEnterpriseNotFound", err)
	}
}

func TestFindEnterpriseAuditLogs(t *testing.T) {
	t.Run("unauthenticated actor", func(t *testing.T) {
		repo := &fakeEnterpriseRepository{}
		uc := usecase.NewEnterpriseUseCase(repo)

		_, err := uc.FindEnterpriseAuditLogs(context.Background(), usecase.FindEnterpriseAuditLogsInput{
			Actor:    entity.User{ID: 0},
			PublicID: "TPN-000001",
		})
		if !errors.Is(err, usecase.ErrForbidden) {
			t.Errorf("error = %v, want ErrForbidden", err)
		}
	})

	t.Run("empty public id", func(t *testing.T) {
		repo := &fakeEnterpriseRepository{}
		uc := usecase.NewEnterpriseUseCase(repo)

		_, err := uc.FindEnterpriseAuditLogs(context.Background(), usecase.FindEnterpriseAuditLogsInput{
			Actor:    memberActor(),
			PublicID: "   ",
		})
		if !errors.Is(err, usecase.ErrEnterpriseNotFound) {
			t.Errorf("error = %v, want ErrEnterpriseNotFound", err)
		}
	})

	t.Run("member owner scope", func(t *testing.T) {
		repo := &fakeEnterpriseRepository{
			auditResult: []entity.EnterpriseAuditEventView{
				{ID: 10, Action: entity.AuditActionCreate},
			},
		}
		uc := usecase.NewEnterpriseUseCase(repo)

		actor := entity.User{ID: 42, Role: entity.RoleMember}
		res, err := uc.FindEnterpriseAuditLogs(context.Background(), usecase.FindEnterpriseAuditLogsInput{
			Actor:    actor,
			PublicID: "TPN-000001",
			Limit:    20,
		})
		if err != nil {
			t.Fatalf("unexpected error = %v", err)
		}
		if repo.lastAuditFilter.OwnerID != 42 {
			t.Errorf("owner scope = %d, want 42", repo.lastAuditFilter.OwnerID)
		}
		if repo.lastAuditFilter.PublicID != "TPN-000001" {
			t.Errorf("public_id = %q, want TPN-000001", repo.lastAuditFilter.PublicID)
		}
		if len(res.Events) != 1 || res.NextCursor != 0 {
			t.Errorf("result = %+v, want 1 event and no next cursor", res)
		}
	})

	t.Run("admin unscoped", func(t *testing.T) {
		repo := &fakeEnterpriseRepository{
			auditResult: []entity.EnterpriseAuditEventView{
				{ID: 20, Action: entity.AuditActionUpdate},
			},
		}
		uc := usecase.NewEnterpriseUseCase(repo)

		actor := entity.User{ID: 99, Role: entity.RoleAdmin}
		res, err := uc.FindEnterpriseAuditLogs(context.Background(), usecase.FindEnterpriseAuditLogsInput{
			Actor:    actor,
			PublicID: "TPN-000001",
			Limit:    20,
		})
		if err != nil {
			t.Fatalf("unexpected error = %v", err)
		}
		if repo.lastAuditFilter.OwnerID != 0 {
			t.Errorf("owner scope = %d, want 0 for admin", repo.lastAuditFilter.OwnerID)
		}
		if len(res.Events) != 1 {
			t.Errorf("len(Events) = %d, want 1", len(res.Events))
		}
	})

	t.Run("pagination with next cursor", func(t *testing.T) {
		repo := &fakeEnterpriseRepository{
			auditResult: []entity.EnterpriseAuditEventView{
				{ID: 30, Action: entity.AuditActionDelete},
				{ID: 20, Action: entity.AuditActionUpdate},
				{ID: 10, Action: entity.AuditActionCreate}, // extra row
			},
		}
		uc := usecase.NewEnterpriseUseCase(repo)

		res, err := uc.FindEnterpriseAuditLogs(context.Background(), usecase.FindEnterpriseAuditLogsInput{
			Actor:    memberActor(),
			PublicID: "TPN-000001",
			Limit:    2,
		})
		if err != nil {
			t.Fatalf("unexpected error = %v", err)
		}
		if len(res.Events) != 2 {
			t.Fatalf("len(Events) = %d, want 2", len(res.Events))
		}
		if res.NextCursor != 20 {
			t.Errorf("NextCursor = %d, want 20", res.NextCursor)
		}
	})

	t.Run("maps not found error", func(t *testing.T) {
		repo := &fakeEnterpriseRepository{
			auditErr: repository.ErrEnterpriseNotFound,
		}
		uc := usecase.NewEnterpriseUseCase(repo)

		_, err := uc.FindEnterpriseAuditLogs(context.Background(), usecase.FindEnterpriseAuditLogsInput{
			Actor:    memberActor(),
			PublicID: "TPN-000404",
		})
		if !errors.Is(err, usecase.ErrEnterpriseNotFound) {
			t.Errorf("error = %v, want ErrEnterpriseNotFound", err)
		}
	})
}

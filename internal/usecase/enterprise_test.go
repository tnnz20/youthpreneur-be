package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

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

	updateErr       error
	updateCalls     int
	updateResult    entity.Enterprise
	lastUpdate      entity.Enterprise
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

func (f *fakeEnterpriseRepository) UpdateEnterprise(
	_ context.Context,
	_ string,
	ownerID int,
	enterprise entity.Enterprise,
	event entity.EnterpriseAuditEvent,
) (entity.Enterprise, error) {
	f.updateCalls++
	f.lastUpdate = enterprise
	f.lastUpdateOwner = ownerID
	f.lastUpdateEvent = event
	if f.updateErr != nil {
		return entity.Enterprise{}, f.updateErr
	}
	if f.updateResult.PublicID != "" {
		return f.updateResult, nil
	}

	return enterprise, nil
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
		Name:            "  Warung Kopi  ",
		BusinessSector:  "Kuliner",
		InitialTurnover: "1500",
		CurrentTurnover: "",
		District:        " Bandung ",
	})
	if err != nil {
		t.Fatalf("CreateEnterprise() error = %v", err)
	}
	if !publicIDPattern.MatchString(created.PublicID) {
		t.Errorf("public id = %q, want YTP- plus six digits", created.PublicID)
	}
	if created.UserID != 7 {
		t.Errorf("owner = %d, want 7", created.UserID)
	}
	if created.Name != "Warung Kopi" {
		t.Errorf("name = %q, want trimmed", created.Name)
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
	if repo.lastCreateEvnt.ChangedFields["name"] != "Warung Kopi" {
		t.Errorf("audit changed fields = %v, want name", repo.lastCreateEvnt.ChangedFields)
	}
}

func TestCreateEnterpriseAllowsManyPerOwner(t *testing.T) {
	repo := &fakeEnterpriseRepository{}
	uc := usecase.NewEnterpriseUseCase(repo)

	for range 2 {
		if _, err := uc.CreateEnterprise(context.Background(), usecase.CreateEnterpriseInput{
			Actor:          memberActor(),
			Name:           "Toko",
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
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), Name: strings.Repeat("a", 256), BusinessSector: "Perdagangan Ritel"},
		},
		{
			name:  "unknown sector",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), Name: "Toko", BusinessSector: "bogus"},
		},
		{
			name:  "unknown legal status",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), Name: "Toko", BusinessSector: "Perdagangan Ritel", LegalStatus: "bogus"},
		},
		{
			name:  "unknown digitization",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), Name: "Toko", BusinessSector: "Perdagangan Ritel", BusinessDigitization: "bogus"},
		},
		{
			name:  "unknown intervention needs",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), Name: "Toko", BusinessSector: "Perdagangan Ritel", InterventionNeeds: "bogus"},
		},
		{
			name:  "unknown training status",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), Name: "Toko", BusinessSector: "Perdagangan Ritel", TrainingStatus: "bogus"},
		},
		{
			name:  "unknown capital access",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), Name: "Toko", BusinessSector: "Perdagangan Ritel", CapitalAccess: "bogus"},
		},
		{
			name:  "negative turnover",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), Name: "Toko", BusinessSector: "Perdagangan Ritel", InitialTurnover: "-1"},
		},
		{
			name:  "too many decimals",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), Name: "Toko", BusinessSector: "Perdagangan Ritel", CurrentTurnover: "1.234"},
		},
		{
			name:  "turnover beyond fifteen digits",
			input: usecase.CreateEnterpriseInput{Actor: memberActor(), Name: "Toko", BusinessSector: "Perdagangan Ritel", CurrentTurnover: "10000000000000"},
		},
		{
			name: "district too long",
			input: usecase.CreateEnterpriseInput{
				Actor:          memberActor(),
				Name:           "Toko",
				BusinessSector: "Perdagangan Ritel",
				District:       strings.Repeat("a", 129),
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
		Name:           "Toko",
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
	if repo.lastFilter.LegalStatus != entity.LegalStatusComplete ||
		repo.lastFilter.BusinessDigitization != entity.BusinessDigitizationHigh ||
		repo.lastFilter.InterventionNeeds != entity.InterventionNeedsPelatihan ||
		repo.lastFilter.TrainingStatus != entity.ProcessStatusCompleted ||
		repo.lastFilter.MentoringStatus != entity.ProcessStatusOngoing ||
		repo.lastFilter.CapitalAccess != entity.GeneralStatusYes ||
		repo.lastFilter.Partnership != entity.GeneralStatusNo {
		t.Errorf("filter = %+v, want enum filters passed through", repo.lastFilter)
	}
}

func TestGetEnterpriseScopesOwnerAndMapsNotFound(t *testing.T) {
	repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{ID: 3, PublicID: "YTP-000003"}}
	uc := usecase.NewEnterpriseUseCase(repo)

	if _, err := uc.GetEnterprise(context.Background(), memberActor(), "YTP-000003"); err != nil {
		t.Fatalf("GetEnterprise() error = %v", err)
	}
	if repo.lastFindOwnerID != 7 {
		t.Errorf("owner scope = %d, want 7", repo.lastFindOwnerID)
	}

	missing := &fakeEnterpriseRepository{}
	uc = usecase.NewEnterpriseUseCase(missing)
	if _, err := uc.GetEnterprise(context.Background(), memberActor(), "YTP-000404"); !errors.Is(err, usecase.ErrEnterpriseNotFound) {
		t.Fatalf("GetEnterprise() error = %v, want ErrEnterpriseNotFound", err)
	}
}

func TestAdminGetEnterpriseIsUnscoped(t *testing.T) {
	repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{ID: 3, PublicID: "YTP-000003"}}
	uc := usecase.NewEnterpriseUseCase(repo)

	if _, err := uc.GetEnterprise(context.Background(), adminActor(), "YTP-000003"); err != nil {
		t.Fatalf("GetEnterprise() error = %v", err)
	}
	if repo.lastFindOwnerID != 0 {
		t.Errorf("owner scope = %d, want 0 for admin", repo.lastFindOwnerID)
	}
}

func TestUpdateEnterpriseOwnerChangesAllowedFields(t *testing.T) {
	repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{
		ID:              5,
		PublicID:        "YTP-000005",
		UserID:          7,
		Name:            "Old",
		BusinessSector:  entity.BusinessSectorPerdaganganRitel,
		InitialTurnover: "10.00",
		CurrentTurnover: "20.00",
		Status:          entity.EnterpriseStatusActive,
	}}
	uc := usecase.NewEnterpriseUseCase(repo)

	updated, err := uc.UpdateEnterprise(context.Background(), memberActor(), "YTP-000005", usecase.UpdateEnterpriseInput{
		Name:            stringPtr("New"),
		CurrentTurnover: stringPtr("30"),
	})
	if err != nil {
		t.Fatalf("UpdateEnterprise() error = %v", err)
	}
	if repo.lastUpdateOwner != 7 {
		t.Errorf("owner scope = %d, want 7", repo.lastUpdateOwner)
	}
	if updated.Name != "New" || updated.CurrentTurnover != "30.00" {
		t.Errorf("updated = %+v, want New and 30.00", updated)
	}
	if updated.InitialTurnover != "10.00" {
		t.Errorf("initial turnover = %q, want untouched 10.00", updated.InitialTurnover)
	}
	if _, ok := repo.lastUpdateEvent.ChangedFields["name"]; !ok {
		t.Errorf("audit changed fields = %v, want name", repo.lastUpdateEvent.ChangedFields)
	}
	if _, ok := repo.lastUpdateEvent.ChangedFields["current_turnover"]; !ok {
		t.Errorf("audit changed fields = %v, want current_turnover", repo.lastUpdateEvent.ChangedFields)
	}
	if _, ok := repo.lastUpdateEvent.ChangedFields["business_sector"]; ok {
		t.Errorf("audit recorded unchanged business_sector: %v", repo.lastUpdateEvent.ChangedFields)
	}
}

func TestUpdateEnterpriseOwnerCannotChangeStatusOrDistrict(t *testing.T) {
	cases := []usecase.UpdateEnterpriseInput{
		{Status: stringPtr("inactive")},
		{District: stringPtr("Jakarta")},
		{LegalStatus: stringPtr("complete")},
		{BusinessDigitization: stringPtr("high")},
		{InterventionNeeds: stringPtr("Pelatihan")},
		{TrainingStatus: stringPtr("completed")},
		{MentoringStatus: stringPtr("ongoing")},
		{CapitalAccess: stringPtr("yes")},
		{Partnership: stringPtr("no")},
	}

	for _, input := range cases {
		repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{ID: 5, PublicID: "YTP-000005", UserID: 7}}
		uc := usecase.NewEnterpriseUseCase(repo)

		if _, err := uc.UpdateEnterprise(context.Background(), memberActor(), "YTP-000005", input); !errors.Is(err, usecase.ErrForbidden) {
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
		PublicID:        "YTP-000005",
		Status:          entity.EnterpriseStatusActive,
		BusinessSector:  entity.BusinessSectorPerdaganganRitel,
		InitialTurnover: "0.00",
		CurrentTurnover: "0.00",
	}}
	uc := usecase.NewEnterpriseUseCase(repo)

	updated, err := uc.UpdateEnterprise(context.Background(), adminActor(), "YTP-000005", usecase.UpdateEnterpriseInput{
		Status:               stringPtr("inactive"),
		District:             stringPtr("Jakarta"),
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
	if updated.Status != entity.EnterpriseStatusInactive || updated.District != "Jakarta" {
		t.Errorf("updated = %+v, want inactive and Jakarta", updated)
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

func TestUpdateEnterpriseNoChangeSkipsWrite(t *testing.T) {
	repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{
		ID:              5,
		PublicID:        "YTP-000005",
		Name:            "Old",
		BusinessSector:  entity.BusinessSectorPerdaganganRitel,
		InitialTurnover: "0.00",
		CurrentTurnover: "0.00",
		Status:          entity.EnterpriseStatusActive,
	}}
	uc := usecase.NewEnterpriseUseCase(repo)

	result, err := uc.UpdateEnterprise(context.Background(), memberActor(), "YTP-000005", usecase.UpdateEnterpriseInput{
		Name: stringPtr("Old"),
	})
	if err != nil {
		t.Fatalf("UpdateEnterprise() error = %v", err)
	}
	if repo.updateCalls != 0 {
		t.Errorf("update calls = %d, want 0 for no-op", repo.updateCalls)
	}
	if result.Name != "Old" {
		t.Errorf("name = %q, want Old", result.Name)
	}
}

func TestUpdateEnterpriseValidation(t *testing.T) {
	repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{ID: 5, PublicID: "YTP-000005", UserID: 7}}
	uc := usecase.NewEnterpriseUseCase(repo)

	_, err := uc.UpdateEnterprise(context.Background(), memberActor(), "YTP-000005", usecase.UpdateEnterpriseInput{
		BusinessSector: stringPtr("bogus"),
	})
	if !errors.Is(err, usecase.ErrBadRequest) {
		t.Fatalf("UpdateEnterprise() error = %v, want ErrBadRequest", err)
	}
}

func TestUpdateEnterpriseMapsNotFound(t *testing.T) {
	repo := &fakeEnterpriseRepository{}
	uc := usecase.NewEnterpriseUseCase(repo)

	_, err := uc.UpdateEnterprise(context.Background(), memberActor(), "YTP-000404", usecase.UpdateEnterpriseInput{})
	if !errors.Is(err, usecase.ErrEnterpriseNotFound) {
		t.Fatalf("UpdateEnterprise() error = %v, want ErrEnterpriseNotFound", err)
	}
}

func TestDeleteEnterpriseRecordsAudit(t *testing.T) {
	repo := &fakeEnterpriseRepository{findResult: entity.Enterprise{ID: 5, PublicID: "YTP-000005", UserID: 7}}
	uc := usecase.NewEnterpriseUseCase(repo)

	if err := uc.DeleteEnterprise(context.Background(), memberActor(), "YTP-000005"); err != nil {
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

	if err := uc.DeleteEnterprise(context.Background(), memberActor(), "YTP-000404"); !errors.Is(err, usecase.ErrEnterpriseNotFound) {
		t.Fatalf("DeleteEnterprise() error = %v, want ErrEnterpriseNotFound", err)
	}
}

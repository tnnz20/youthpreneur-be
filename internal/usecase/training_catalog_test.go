package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

type fakeTrainingCatalogRepository struct {
	createErr   error
	createCalls int
	lastCreated entity.TrainingCatalog

	findResult       entity.TrainingCatalog
	findErr          error
	lastFindPublicID string

	listResult []entity.TrainingCatalog
	listErr    error
	lastFilter entity.TrainingCatalogFilter

	updateResult       entity.TrainingCatalog
	updateErr          error
	lastUpdatePublicID string
	lastUpdate         entity.TrainingCatalogUpdate

	deleteErr           error
	lastDeletedPublicID string
}

func (f *fakeTrainingCatalogRepository) CreateTrainingCatalog(
	_ context.Context,
	catalog entity.TrainingCatalog,
) (entity.TrainingCatalog, error) {
	f.createCalls++
	f.lastCreated = catalog
	if f.createErr != nil {
		return entity.TrainingCatalog{}, f.createErr
	}

	created := catalog
	created.ID = f.createCalls

	return created, nil
}

func (f *fakeTrainingCatalogRepository) FindTrainingCatalogByPublicID(
	_ context.Context,
	publicID string,
) (entity.TrainingCatalog, error) {
	f.lastFindPublicID = publicID
	if f.findErr != nil {
		return entity.TrainingCatalog{}, f.findErr
	}
	if f.findResult.PublicID == "" {
		return entity.TrainingCatalog{}, repository.ErrTrainingCatalogNotFound
	}

	return f.findResult, nil
}

func (f *fakeTrainingCatalogRepository) FindTrainingCatalogs(
	_ context.Context,
	filter entity.TrainingCatalogFilter,
) ([]entity.TrainingCatalog, error) {
	f.lastFilter = filter

	return f.listResult, f.listErr
}

func (f *fakeTrainingCatalogRepository) UpdateTrainingCatalog(
	_ context.Context,
	publicID string,
	update entity.TrainingCatalogUpdate,
) (entity.TrainingCatalog, error) {
	f.lastUpdatePublicID = publicID
	f.lastUpdate = update
	if f.updateErr != nil {
		return entity.TrainingCatalog{}, f.updateErr
	}

	return f.updateResult, nil
}

func (f *fakeTrainingCatalogRepository) SoftDeleteTrainingCatalog(
	_ context.Context,
	publicID string,
	_ int64,
) error {
	f.lastDeletedPublicID = publicID

	return f.deleteErr
}

func intPtr(value int) *int {
	return &value
}

func TestCreateTrainingCatalogRequiresAdmin(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	_, err := uc.CreateTrainingCatalog(context.Background(), usecase.CreateTrainingCatalogInput{
		Actor: memberActor(),
		Name:  "Kelas",
	})
	if !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("CreateTrainingCatalog() error = %v, want ErrForbidden", err)
	}
	if repo.createCalls != 0 {
		t.Errorf("create calls = %d, want 0", repo.createCalls)
	}
}

func TestCreateTrainingCatalogNormalizesAndGeneratesPublicID(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	created, err := uc.CreateTrainingCatalog(context.Background(), usecase.CreateTrainingCatalogInput{
		Actor:          adminActor(),
		Name:           "  Bisnis Digital  ",
		Category:       " Pemasaran ",
		TrainingSlots:  intPtr(20),
		TrainingStatus: "planned",
		Link:           "https://example.com/training",
		TrainingDate:   "2026-10-01",
		TrainingPeriod: " 09:00-12:00 ",
	})
	if err != nil {
		t.Fatalf("CreateTrainingCatalog() error = %v", err)
	}
	if !publicIDPattern.MatchString(created.PublicID) {
		t.Errorf("public id = %q, want YTP- plus six digits", created.PublicID)
	}
	if repo.lastCreated.Name != "Bisnis Digital" || repo.lastCreated.Category != "Pemasaran" {
		t.Errorf("normalized catalog = %+v, want trimmed name and category", repo.lastCreated)
	}
	if repo.lastCreated.TrainingStatus != entity.ProcessStatusPlanned {
		t.Errorf("status = %q, want planned", repo.lastCreated.TrainingStatus)
	}
	if repo.lastCreated.TrainingSlots == nil || *repo.lastCreated.TrainingSlots != 20 {
		t.Errorf("slots = %v, want 20", repo.lastCreated.TrainingSlots)
	}
	if repo.lastCreated.TrainingDate == nil || repo.lastCreated.TrainingDate.Format("2006-01-02") != "2026-10-01" {
		t.Errorf("training date = %v, want 2026-10-01", repo.lastCreated.TrainingDate)
	}
	if repo.lastCreated.TrainingPeriod != "09:00-12:00" {
		t.Errorf("period = %q, want trimmed", repo.lastCreated.TrainingPeriod)
	}
	if repo.lastCreated.CreatedAt == 0 || repo.lastCreated.UpdatedAt == 0 {
		t.Error("timestamps = 0, want server-controlled values")
	}
}

func TestCreateTrainingCatalogAllowsUnlimitedSlots(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	if _, err := uc.CreateTrainingCatalog(context.Background(), usecase.CreateTrainingCatalogInput{
		Actor: adminActor(),
		Name:  "Kelas",
	}); err != nil {
		t.Fatalf("CreateTrainingCatalog() error = %v", err)
	}
	if repo.lastCreated.TrainingSlots != nil {
		t.Errorf("slots = %v, want nil for unlimited", repo.lastCreated.TrainingSlots)
	}
}

func TestCreateTrainingCatalogValidation(t *testing.T) {
	cases := []struct {
		name  string
		input usecase.CreateTrainingCatalogInput
	}{
		{name: "name too long", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), Name: strings.Repeat("a", 256)}},
		{name: "pic phone too long", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), PicPhone: strings.Repeat("a", 51)}},
		{name: "category too long", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), Category: strings.Repeat("a", 101)}},
		{name: "period too long", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), TrainingPeriod: strings.Repeat("a", 101)}},
		{name: "link too long", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), Link: "https://example.com/" + strings.Repeat("a", 240)}},
		{name: "link not url", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), Link: "not-a-url"}},
		{name: "unknown status", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), TrainingStatus: "bogus"}},
		{name: "zero slots", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), TrainingSlots: intPtr(0)}},
		{name: "negative slots", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), TrainingSlots: intPtr(-2)}},
		{name: "bad date", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), TrainingDate: "01-10-2026"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeTrainingCatalogRepository{}
			uc := usecase.NewTrainingCatalogUseCase(repo)

			if _, err := uc.CreateTrainingCatalog(context.Background(), tc.input); !errors.Is(err, usecase.ErrBadRequest) {
				t.Fatalf("CreateTrainingCatalog() error = %v, want ErrBadRequest", err)
			}
			if repo.createCalls != 0 {
				t.Errorf("create calls = %d, want 0", repo.createCalls)
			}
		})
	}
}

func TestCreateTrainingCatalogRetriesPublicIDCollisions(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{createErr: repository.ErrDuplicateTrainingCatalogPublicID}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	_, err := uc.CreateTrainingCatalog(context.Background(), usecase.CreateTrainingCatalogInput{Actor: adminActor(), Name: "Kelas"})
	if !errors.Is(err, usecase.ErrPublicIDGeneration) {
		t.Fatalf("CreateTrainingCatalog() error = %v, want ErrPublicIDGeneration", err)
	}
	if repo.createCalls != 5 {
		t.Errorf("create calls = %d, want 5", repo.createCalls)
	}
}

func TestFindTrainingCatalogsPaginatesWithLimitPlusOne(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{listResult: []entity.TrainingCatalog{{ID: 1}, {ID: 2}, {ID: 3}}}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	result, err := uc.FindTrainingCatalogs(context.Background(), usecase.FindTrainingCatalogsInput{Limit: 2})
	if err != nil {
		t.Fatalf("FindTrainingCatalogs() error = %v", err)
	}
	if repo.lastFilter.Limit != 3 {
		t.Errorf("repository limit = %d, want 3", repo.lastFilter.Limit)
	}
	if len(result.Catalogs) != 2 || result.NextCursor != 2 {
		t.Errorf("result = %+v, want two catalogs and next cursor 2", result)
	}
}

func TestFindTrainingCatalogsPassesValidatedFilters(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	_, err := uc.FindTrainingCatalogs(context.Background(), usecase.FindTrainingCatalogsInput{
		Category:       " Pemasaran ",
		TrainingStatus: "ongoing",
		TrainingDate:   "2026-10-01",
		TrainingPeriod: " Pagi ",
	})
	if err != nil {
		t.Fatalf("FindTrainingCatalogs() error = %v", err)
	}
	if repo.lastFilter.Category != "Pemasaran" || repo.lastFilter.TrainingPeriod != "Pagi" {
		t.Errorf("string filters = %+v, want trimmed values", repo.lastFilter)
	}
	if repo.lastFilter.TrainingStatus != entity.ProcessStatusOngoing {
		t.Errorf("status filter = %q, want ongoing", repo.lastFilter.TrainingStatus)
	}
	if repo.lastFilter.TrainingDate == nil || repo.lastFilter.TrainingDate.Format("2006-01-02") != "2026-10-01" {
		t.Errorf("date filter = %v, want 2026-10-01", repo.lastFilter.TrainingDate)
	}
}

func TestFindTrainingCatalogsRejectsInvalidFilters(t *testing.T) {
	cases := []usecase.FindTrainingCatalogsInput{
		{TrainingStatus: "bogus"},
		{TrainingDate: "bogus"},
	}

	for _, input := range cases {
		uc := usecase.NewTrainingCatalogUseCase(&fakeTrainingCatalogRepository{})
		if _, err := uc.FindTrainingCatalogs(context.Background(), input); !errors.Is(err, usecase.ErrBadRequest) {
			t.Fatalf("FindTrainingCatalogs(%+v) error = %v, want ErrBadRequest", input, err)
		}
	}
}

func TestGetTrainingCatalogMapsNotFound(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{findResult: entity.TrainingCatalog{ID: 4, PublicID: "YTP-000004"}}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	if _, err := uc.GetTrainingCatalog(context.Background(), "YTP-000004"); err != nil {
		t.Fatalf("GetTrainingCatalog() error = %v", err)
	}

	missing := usecase.NewTrainingCatalogUseCase(&fakeTrainingCatalogRepository{})
	if _, err := missing.GetTrainingCatalog(context.Background(), "YTP-000404"); !errors.Is(err, usecase.ErrTrainingCatalogNotFound) {
		t.Fatalf("GetTrainingCatalog() error = %v, want ErrTrainingCatalogNotFound", err)
	}
}

func TestUpdateTrainingCatalogRequiresAdmin(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	name := "Baru"
	_, err := uc.UpdateTrainingCatalog(context.Background(), memberActor(), "YTP-000004", usecase.UpdateTrainingCatalogInput{Name: &name})
	if !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("UpdateTrainingCatalog() error = %v, want ErrForbidden", err)
	}
	if repo.lastUpdatePublicID != "" {
		t.Error("repository mutated for a non-admin update")
	}
}

func TestUpdateTrainingCatalogAppliesValidatedFields(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{updateResult: entity.TrainingCatalog{ID: 4, PublicID: "YTP-000004"}}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	name := " Baru "
	slots := intPtr(5)
	status := "ongoing"
	link := "https://example.com/x"
	date := "2026-11-02"
	if _, err := uc.UpdateTrainingCatalog(context.Background(), adminActor(), "YTP-000004", usecase.UpdateTrainingCatalogInput{
		Name:           &name,
		TrainingSlots:  slots,
		TrainingStatus: &status,
		Link:           &link,
		TrainingDate:   &date,
	}); err != nil {
		t.Fatalf("UpdateTrainingCatalog() error = %v", err)
	}
	if repo.lastUpdate.Name == nil || *repo.lastUpdate.Name != "Baru" {
		t.Errorf("name = %v, want trimmed Baru", repo.lastUpdate.Name)
	}
	if repo.lastUpdate.TrainingSlots == nil || *repo.lastUpdate.TrainingSlots != 5 {
		t.Errorf("slots = %v, want 5", repo.lastUpdate.TrainingSlots)
	}
	if repo.lastUpdate.TrainingStatus == nil || *repo.lastUpdate.TrainingStatus != entity.ProcessStatusOngoing {
		t.Errorf("status = %v, want ongoing", repo.lastUpdate.TrainingStatus)
	}
	if repo.lastUpdate.TrainingDate == nil || repo.lastUpdate.TrainingDate.Format("2006-01-02") != "2026-11-02" {
		t.Errorf("date = %v, want 2026-11-02", repo.lastUpdate.TrainingDate)
	}
	if repo.lastUpdate.UpdatedAt == 0 {
		t.Error("updated_at = 0, want server timestamp")
	}
}

func TestUpdateTrainingCatalogRejectsEmptyUpdate(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	_, err := uc.UpdateTrainingCatalog(context.Background(), adminActor(), "YTP-000004", usecase.UpdateTrainingCatalogInput{})
	if !errors.Is(err, usecase.ErrBadRequest) {
		t.Fatalf("UpdateTrainingCatalog() error = %v, want ErrBadRequest", err)
	}
	if repo.lastUpdatePublicID != "" {
		t.Errorf("repository updated %q for an empty patch, want no call", repo.lastUpdatePublicID)
	}
}

func TestUpdateTrainingCatalogAllowsExplicitClear(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{updateResult: entity.TrainingCatalog{ID: 4, PublicID: "YTP-000004"}}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	description := ""
	if _, err := uc.UpdateTrainingCatalog(context.Background(), adminActor(), "YTP-000004", usecase.UpdateTrainingCatalogInput{Description: &description}); err != nil {
		t.Fatalf("UpdateTrainingCatalog() error = %v", err)
	}
	if repo.lastUpdate.Description == nil || *repo.lastUpdate.Description != "" {
		t.Errorf("description = %v, want explicit empty clear", repo.lastUpdate.Description)
	}
}

func TestUpdateTrainingCatalogStatusValidatesInput(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	if _, err := uc.UpdateTrainingCatalogStatus(context.Background(), adminActor(), "YTP-000004", ""); !errors.Is(err, usecase.ErrBadRequest) {
		t.Fatalf("empty status error = %v, want ErrBadRequest", err)
	}
	if _, err := uc.UpdateTrainingCatalogStatus(context.Background(), adminActor(), "YTP-000004", "bogus"); !errors.Is(err, usecase.ErrBadRequest) {
		t.Fatalf("bogus status error = %v, want ErrBadRequest", err)
	}
	if _, err := uc.UpdateTrainingCatalogStatus(context.Background(), memberActor(), "YTP-000004", "completed"); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("member status error = %v, want ErrForbidden", err)
	}
}

func TestDeleteTrainingCatalogRequiresAdminAndMapsNotFound(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	if err := uc.DeleteTrainingCatalog(context.Background(), memberActor(), "YTP-000004"); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("member delete error = %v, want ErrForbidden", err)
	}
	if err := uc.DeleteTrainingCatalog(context.Background(), adminActor(), "YTP-000004"); err != nil {
		t.Fatalf("admin delete error = %v", err)
	}
	if repo.lastDeletedPublicID != "YTP-000004" {
		t.Errorf("deleted public id = %q, want YTP-000004", repo.lastDeletedPublicID)
	}

	missing := &fakeTrainingCatalogRepository{deleteErr: repository.ErrTrainingCatalogNotFound}
	if err := usecase.NewTrainingCatalogUseCase(missing).DeleteTrainingCatalog(context.Background(), adminActor(), "YTP-000404"); !errors.Is(err, usecase.ErrTrainingCatalogNotFound) {
		t.Fatalf("missing delete error = %v, want ErrTrainingCatalogNotFound", err)
	}
}

func TestTrainingCatalogDateParsingRejectsFutureIndependentFormat(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	// A valid date far in the future is accepted: catalog dates have no
	// past/future restriction, only the YYYY-MM-DD format.
	future := time.Now().AddDate(5, 0, 0).Format("2006-01-02")
	if _, err := uc.CreateTrainingCatalog(context.Background(), usecase.CreateTrainingCatalogInput{
		Actor:        adminActor(),
		TrainingDate: future,
	}); err != nil {
		t.Fatalf("CreateTrainingCatalog() error = %v, want future date accepted", err)
	}
}

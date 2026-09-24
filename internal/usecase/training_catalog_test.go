package usecase_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

var trainingCatalogPublicIDPattern = regexp.MustCompile(`^TCY-[0-9]{6}$`)

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
		Title: "Kelas",
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
		Title:          "  Bisnis Digital  ",
		Category:       " Wirausaha & Agribisnis ",
		MaxSlots:       intPtr(20),
		TrainingStatus: "planned",
		Link:           "https://example.com/training",
		Address:        " Jl. Pemuda No. 1 ",
		Thumbnail:      " /uploads/thumbnails/sample.png ",
		StartDate:      "2026-10-01",
		EndDate:        "2026-10-05",
		Mentor:         " Pak Budi ",
	})
	if err != nil {
		t.Fatalf("CreateTrainingCatalog() error = %v", err)
	}
	if !trainingCatalogPublicIDPattern.MatchString(created.PublicID) {
		t.Errorf("public id = %q, want TCY- plus six digits", created.PublicID)
	}
	if repo.lastCreated.Title != "Bisnis Digital" || repo.lastCreated.Category != entity.TrainingCategoryWirausahaAgribisnis {
		t.Errorf("normalized catalog = %+v, want trimmed title and category", repo.lastCreated)
	}
	if repo.lastCreated.TrainingStatus != entity.ProcessStatusPlanned {
		t.Errorf("status = %q, want planned", repo.lastCreated.TrainingStatus)
	}
	if repo.lastCreated.MaxSlots == nil || *repo.lastCreated.MaxSlots != 20 {
		t.Errorf("slots = %v, want 20", repo.lastCreated.MaxSlots)
	}
	if repo.lastCreated.StartDate == nil || repo.lastCreated.StartDate.Format("2006-01-02") != "2026-10-01" {
		t.Errorf("start date = %v, want 2026-10-01", repo.lastCreated.StartDate)
	}
	if repo.lastCreated.EndDate == nil || repo.lastCreated.EndDate.Format("2006-01-02") != "2026-10-05" {
		t.Errorf("end date = %v, want 2026-10-05", repo.lastCreated.EndDate)
	}
	if repo.lastCreated.Mentor != "Pak Budi" {
		t.Errorf("mentor = %q, want Pak Budi", repo.lastCreated.Mentor)
	}
	if repo.lastCreated.Address != "Jl. Pemuda No. 1" {
		t.Errorf("address = %q, want Jl. Pemuda No. 1", repo.lastCreated.Address)
	}
	if repo.lastCreated.Thumbnail != "/uploads/thumbnails/sample.png" {
		t.Errorf("thumbnail = %q, want /uploads/thumbnails/sample.png", repo.lastCreated.Thumbnail)
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
		Title: "Kelas",
	}); err != nil {
		t.Fatalf("CreateTrainingCatalog() error = %v", err)
	}
	if repo.lastCreated.MaxSlots != nil {
		t.Errorf("slots = %v, want nil for unlimited", repo.lastCreated.MaxSlots)
	}
}

func TestCreateTrainingCatalogValidation(t *testing.T) {
	cases := []struct {
		name  string
		input usecase.CreateTrainingCatalogInput
	}{
		{name: "title too long", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), Title: strings.Repeat("a", 256)}},
		{name: "pic phone too long", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), PicPhone: strings.Repeat("a", 51)}},
		{name: "invalid category", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), Category: "Bogus Category"}},
		{name: "mentor too long", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), Mentor: strings.Repeat("a", 256)}},
		{name: "thumbnail too long", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), Thumbnail: strings.Repeat("a", 256)}},
		{name: "link too long", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), Link: "https://example.com/" + strings.Repeat("a", 240)}},
		{name: "link not url", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), Link: "not-a-url"}},
		{name: "unknown status", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), TrainingStatus: "bogus"}},
		{name: "zero slots", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), MaxSlots: intPtr(0)}},
		{name: "negative slots", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), MaxSlots: intPtr(-2)}},
		{name: "bad start date", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), StartDate: "01-10-2026"}},
		{name: "bad end date", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), EndDate: "01-10-2026"}},
		{name: "end date before start date", input: usecase.CreateTrainingCatalogInput{Actor: adminActor(), StartDate: "2026-10-05", EndDate: "2026-10-01"}},
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

	_, err := uc.CreateTrainingCatalog(context.Background(), usecase.CreateTrainingCatalogInput{Actor: adminActor(), Title: "Kelas"})
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
		Search:         "  Digital Marketing  ",
		Title:          "  Pemasaran  ",
		Mentor:         "  Budi  ",
		Category:       " Wirausaha & Agribisnis ",
		TrainingStatus: "ongoing",
		StartDate:      "2026-10-01",
	})
	if err != nil {
		t.Fatalf("FindTrainingCatalogs() error = %v", err)
	}
	if repo.lastFilter.Search != "Digital Marketing" {
		t.Errorf("search filter = %q, want Digital Marketing", repo.lastFilter.Search)
	}
	if repo.lastFilter.Title != "Pemasaran" {
		t.Errorf("title filter = %q, want Pemasaran", repo.lastFilter.Title)
	}
	if repo.lastFilter.Mentor != "Budi" {
		t.Errorf("mentor filter = %q, want Budi", repo.lastFilter.Mentor)
	}
	if repo.lastFilter.Category != entity.TrainingCategoryWirausahaAgribisnis {
		t.Errorf("category filter = %q, want Wirausaha & Agribisnis", repo.lastFilter.Category)
	}
	if repo.lastFilter.TrainingStatus != entity.ProcessStatusOngoing {
		t.Errorf("status filter = %q, want ongoing", repo.lastFilter.TrainingStatus)
	}
	if repo.lastFilter.StartDate == nil || repo.lastFilter.StartDate.Format("2006-01-02") != "2026-10-01" {
		t.Errorf("start date filter = %v, want 2026-10-01", repo.lastFilter.StartDate)
	}
	if repo.lastFilter.Order != "asc" {
		t.Errorf("order filter = %q, want default asc", repo.lastFilter.Order)
	}

	_, err = uc.FindTrainingCatalogs(context.Background(), usecase.FindTrainingCatalogsInput{
		Order: "DESC",
	})
	if err != nil {
		t.Fatalf("FindTrainingCatalogs(Order: DESC) error = %v", err)
	}
	if repo.lastFilter.Order != "desc" {
		t.Errorf("order filter = %q, want desc", repo.lastFilter.Order)
	}
}

func TestFindTrainingCatalogsRejectsInvalidFilters(t *testing.T) {
	cases := []usecase.FindTrainingCatalogsInput{
		{Category: "bogus"},
		{TrainingStatus: "bogus"},
		{StartDate: "bogus"},
		{Order: "invalid"},
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

	title := "Baru"
	_, err := uc.UpdateTrainingCatalog(context.Background(), memberActor(), "YTP-000004", usecase.UpdateTrainingCatalogInput{Title: &title})
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

	title := " Baru "
	slots := intPtr(5)
	status := "ongoing"
	category := " Digital & IPTEK "
	link := "https://example.com/x"
	startDate := "2026-11-02"
	endDate := "2026-11-05"
	mentor := " Mentor A "
	address := " Jl. Testing "
	thumbnail := " /uploads/thumbnails/test.png "
	if _, err := uc.UpdateTrainingCatalog(context.Background(), adminActor(), "YTP-000004", usecase.UpdateTrainingCatalogInput{
		Title:          &title,
		MaxSlots:       slots,
		Category:       &category,
		TrainingStatus: &status,
		Link:           &link,
		StartDate:      &startDate,
		EndDate:        &endDate,
		Mentor:         &mentor,
		Address:        &address,
		Thumbnail:      &thumbnail,
	}); err != nil {
		t.Fatalf("UpdateTrainingCatalog() error = %v", err)
	}
	if repo.lastUpdate.Title == nil || *repo.lastUpdate.Title != "Baru" {
		t.Errorf("title = %v, want trimmed Baru", repo.lastUpdate.Title)
	}
	if repo.lastUpdate.Category == nil || *repo.lastUpdate.Category != entity.TrainingCategoryDigitalIPTEK {
		t.Errorf("category = %v, want Digital & IPTEK", repo.lastUpdate.Category)
	}
	if repo.lastUpdate.MaxSlots == nil || *repo.lastUpdate.MaxSlots != 5 {
		t.Errorf("slots = %v, want 5", repo.lastUpdate.MaxSlots)
	}
	if repo.lastUpdate.TrainingStatus == nil || *repo.lastUpdate.TrainingStatus != entity.ProcessStatusOngoing {
		t.Errorf("status = %v, want ongoing", repo.lastUpdate.TrainingStatus)
	}
	if repo.lastUpdate.StartDate == nil || repo.lastUpdate.StartDate.Format("2006-01-02") != "2026-11-02" {
		t.Errorf("start date = %v, want 2026-11-02", repo.lastUpdate.StartDate)
	}
	if repo.lastUpdate.EndDate == nil || repo.lastUpdate.EndDate.Format("2006-01-02") != "2026-11-05" {
		t.Errorf("end date = %v, want 2026-11-05", repo.lastUpdate.EndDate)
	}
	if repo.lastUpdate.Mentor == nil || *repo.lastUpdate.Mentor != "Mentor A" {
		t.Errorf("mentor = %v, want Mentor A", repo.lastUpdate.Mentor)
	}
	if repo.lastUpdate.Address == nil || *repo.lastUpdate.Address != "Jl. Testing" {
		t.Errorf("address = %v, want Jl. Testing", repo.lastUpdate.Address)
	}
	if repo.lastUpdate.Thumbnail == nil || *repo.lastUpdate.Thumbnail != "/uploads/thumbnails/test.png" {
		t.Errorf("thumbnail = %v, want /uploads/thumbnails/test.png", repo.lastUpdate.Thumbnail)
	}
	if repo.lastUpdate.UpdatedAt == 0 {
		t.Error("updated_at = 0, want server timestamp")
	}
}

func TestUpdateTrainingCatalogRejectsEndDateBeforeStartDate(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	startDate := "2026-11-05"
	endDate := "2026-11-02"
	_, err := uc.UpdateTrainingCatalog(context.Background(), adminActor(), "YTP-000004", usecase.UpdateTrainingCatalogInput{
		StartDate: &startDate,
		EndDate:   &endDate,
	})
	if !errors.Is(err, usecase.ErrBadRequest) {
		t.Fatalf("UpdateTrainingCatalog() error = %v, want ErrBadRequest", err)
	}
}

func TestUpdateTrainingCatalogMapsRepositoryInvalidDateRange(t *testing.T) {
	repo := &fakeTrainingCatalogRepository{updateErr: repository.ErrInvalidTrainingCatalogDateRange}
	uc := usecase.NewTrainingCatalogUseCase(repo)

	endDate := "2026-11-01"
	_, err := uc.UpdateTrainingCatalog(context.Background(), adminActor(), "YTP-000004", usecase.UpdateTrainingCatalogInput{
		EndDate: &endDate,
	})
	if !errors.Is(err, usecase.ErrBadRequest) {
		t.Fatalf("UpdateTrainingCatalog() error = %v, want ErrBadRequest", err)
	}
	if !strings.Contains(err.Error(), "end_date must be on or after start_date") {
		t.Errorf("error message = %q, want it to contain 'end_date must be on or after start_date'", err.Error())
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
		Actor:     adminActor(),
		StartDate: future,
	}); err != nil {
		t.Fatalf("CreateTrainingCatalog() error = %v, want future date accepted", err)
	}
}

func TestGenerateTrainingCatalogPublicID(t *testing.T) {
	id, err := usecase.GenerateTrainingCatalogPublicID()
	if err != nil {
		t.Fatalf("GenerateTrainingCatalogPublicID() error = %v", err)
	}
	if !trainingCatalogPublicIDPattern.MatchString(id) {
		t.Errorf("generated public id = %q, want matching %s", id, trainingCatalogPublicIDPattern.String())
	}
}

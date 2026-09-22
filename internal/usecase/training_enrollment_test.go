package usecase_test

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

var trainingEnrollmentPublicIDPattern = regexp.MustCompile(`^ENR-[0-9]{6}$`)

type fakeTrainingEnrollmentRepository struct {
	createResult entity.TrainingEnrollment
	createErr    error
	createCalls  int
	lastCreated  entity.TrainingEnrollment

	cancelErr          error
	cancelCalls        int
	lastCancelPublicID string
	lastCancelUserID   int

	updateStatusResult       entity.TrainingEnrollment
	updateStatusErr          error
	lastUpdateStatusPublicID string
	lastUpdateStatus         entity.TrainingEnrollmentStatus
	lastUpdateStatusAt       int64

	listResult []entity.TrainingEnrollment
	listErr    error
	lastFilter entity.TrainingEnrollmentFilter
}

func (f *fakeTrainingEnrollmentRepository) CreateTrainingEnrollment(
	_ context.Context,
	enrollment entity.TrainingEnrollment,
) (entity.TrainingEnrollment, error) {
	f.createCalls++
	f.lastCreated = enrollment
	if f.createErr != nil {
		return entity.TrainingEnrollment{}, f.createErr
	}

	created := enrollment
	created.ID = f.createCalls

	return created, nil
}

func (f *fakeTrainingEnrollmentRepository) CancelTrainingEnrollment(
	_ context.Context,
	publicID string,
	userID int,
	_ int64,
) error {
	f.cancelCalls++
	f.lastCancelPublicID = publicID
	f.lastCancelUserID = userID

	return f.cancelErr
}

func (f *fakeTrainingEnrollmentRepository) UpdateTrainingEnrollmentStatus(
	_ context.Context,
	publicID string,
	status entity.TrainingEnrollmentStatus,
	now int64,
) (entity.TrainingEnrollment, error) {
	f.lastUpdateStatusPublicID = publicID
	f.lastUpdateStatus = status
	f.lastUpdateStatusAt = now
	if f.updateStatusErr != nil {
		return entity.TrainingEnrollment{}, f.updateStatusErr
	}

	return f.updateStatusResult, nil
}

func (f *fakeTrainingEnrollmentRepository) FindTrainingEnrollments(
	_ context.Context,
	filter entity.TrainingEnrollmentFilter,
) ([]entity.TrainingEnrollment, error) {
	f.lastFilter = filter

	return f.listResult, f.listErr
}

func TestEnrollRejectsMissingIdentity(t *testing.T) {
	repo := &fakeTrainingEnrollmentRepository{}
	uc := usecase.NewTrainingEnrollmentUseCase(repo, &fakeTrainingCatalogRepository{})

	_, err := uc.Enroll(context.Background(), usecase.CreateTrainingEnrollmentInput{CatalogPublicID: "YTP-000004"})
	if !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("Enroll() error = %v, want ErrForbidden", err)
	}
	if repo.createCalls != 0 {
		t.Errorf("create calls = %d, want 0", repo.createCalls)
	}
}

func TestEnrollRequiresCatalogPublicID(t *testing.T) {
	repo := &fakeTrainingEnrollmentRepository{}
	uc := usecase.NewTrainingEnrollmentUseCase(repo, &fakeTrainingCatalogRepository{})

	_, err := uc.Enroll(context.Background(), usecase.CreateTrainingEnrollmentInput{Actor: memberActor()})
	if !errors.Is(err, usecase.ErrBadRequest) {
		t.Fatalf("Enroll() error = %v, want ErrBadRequest", err)
	}
}

func TestEnrollAssignsActorAndRegisterDate(t *testing.T) {
	repo := &fakeTrainingEnrollmentRepository{}
	uc := usecase.NewTrainingEnrollmentUseCase(repo, &fakeTrainingCatalogRepository{})

	created, err := uc.Enroll(context.Background(), usecase.CreateTrainingEnrollmentInput{
		Actor:           memberActor(),
		CatalogPublicID: " YTP-000004 ",
	})
	if err != nil {
		t.Fatalf("Enroll() error = %v", err)
	}
	if created.UserID != 7 {
		t.Errorf("user id = %d, want 7", created.UserID)
	}
	if repo.lastCreated.Catalog == nil || repo.lastCreated.Catalog.PublicID != "YTP-000004" {
		t.Errorf("catalog selector = %+v, want trimmed public id", repo.lastCreated.Catalog)
	}
	if repo.lastCreated.RegisterDate == nil {
		t.Fatal("register date = nil, want server date")
	}
	if repo.lastCreated.RegisterDate.Hour() != 0 || repo.lastCreated.RegisterDate.Minute() != 0 {
		t.Errorf("register date = %v, want date-only", repo.lastCreated.RegisterDate)
	}
	if repo.lastCreated.Status != entity.TrainingEnrollmentStatusPending {
		t.Errorf("status = %q, want pending", repo.lastCreated.Status)
	}
	if !trainingEnrollmentPublicIDPattern.MatchString(repo.lastCreated.PublicID) {
		t.Errorf("public id = %q, want ENR- plus six digits", repo.lastCreated.PublicID)
	}
}

func TestUpdateTrainingEnrollmentStatusRequiresAdmin(t *testing.T) {
	repo := &fakeTrainingEnrollmentRepository{}
	uc := usecase.NewTrainingEnrollmentUseCase(repo, &fakeTrainingCatalogRepository{})

	_, err := uc.UpdateStatus(context.Background(), usecase.UpdateTrainingEnrollmentStatusInput{
		Actor:    memberActor(),
		PublicID: "YTP-000111",
		Status:   "accepted",
	})
	if !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("UpdateStatus() error = %v, want ErrForbidden", err)
	}
}

func TestUpdateTrainingEnrollmentStatusValidation(t *testing.T) {
	repo := &fakeTrainingEnrollmentRepository{}
	uc := usecase.NewTrainingEnrollmentUseCase(repo, &fakeTrainingCatalogRepository{})

	cases := []struct {
		name     string
		publicID string
		status   string
	}{
		{name: "empty public id", publicID: "", status: "accepted"},
		{name: "empty status", publicID: "YTP-000111", status: ""},
		{name: "invalid status", publicID: "YTP-000111", status: "bogus"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.UpdateStatus(context.Background(), usecase.UpdateTrainingEnrollmentStatusInput{
				Actor:    adminActor(),
				PublicID: tc.publicID,
				Status:   tc.status,
			})
			if !errors.Is(err, usecase.ErrBadRequest) {
				t.Fatalf("UpdateStatus() error = %v, want ErrBadRequest", err)
			}
		})
	}
}

func TestUpdateTrainingEnrollmentStatusSuccess(t *testing.T) {
	repo := &fakeTrainingEnrollmentRepository{
		updateStatusResult: entity.TrainingEnrollment{
			PublicID: "YTP-000111",
			Status:   entity.TrainingEnrollmentStatusAccepted,
		},
	}
	uc := usecase.NewTrainingEnrollmentUseCase(repo, &fakeTrainingCatalogRepository{})

	updated, err := uc.UpdateStatus(context.Background(), usecase.UpdateTrainingEnrollmentStatusInput{
		Actor:    adminActor(),
		PublicID: " YTP-000111 ",
		Status:   " accepted ",
	})
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if repo.lastUpdateStatusPublicID != "YTP-000111" {
		t.Errorf("public id = %q, want YTP-000111", repo.lastUpdateStatusPublicID)
	}
	if repo.lastUpdateStatus != entity.TrainingEnrollmentStatusAccepted {
		t.Errorf("status = %q, want accepted", repo.lastUpdateStatus)
	}
	if updated.Status != entity.TrainingEnrollmentStatusAccepted {
		t.Errorf("updated status = %q, want accepted", updated.Status)
	}
}

func TestEnrollMapsRepositoryErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{name: "catalog missing", err: repository.ErrTrainingCatalogNotFound, want: usecase.ErrTrainingCatalogNotFound},
		{name: "catalog closed", err: repository.ErrTrainingCatalogClosed, want: usecase.ErrTrainingCatalogClosed},
		{name: "catalog full", err: repository.ErrTrainingCatalogFull, want: usecase.ErrTrainingCatalogFull},
		{name: "already enrolled", err: repository.ErrDuplicateTrainingEnrollment, want: usecase.ErrAlreadyEnrolled},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeTrainingEnrollmentRepository{createErr: tc.err}
			uc := usecase.NewTrainingEnrollmentUseCase(repo, &fakeTrainingCatalogRepository{})

			_, err := uc.Enroll(context.Background(), usecase.CreateTrainingEnrollmentInput{
				Actor:           memberActor(),
				CatalogPublicID: "YTP-000004",
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("Enroll() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCancelEnrollmentScopesMemberAndAdmin(t *testing.T) {
	repo := &fakeTrainingEnrollmentRepository{}
	uc := usecase.NewTrainingEnrollmentUseCase(repo, &fakeTrainingCatalogRepository{})

	if err := uc.CancelEnrollment(context.Background(), memberActor(), "YTP-000111"); err != nil {
		t.Fatalf("member CancelEnrollment() error = %v", err)
	}
	if repo.lastCancelUserID != 7 {
		t.Errorf("member scope = %d, want 7", repo.lastCancelUserID)
	}

	if err := uc.CancelEnrollment(context.Background(), adminActor(), "YTP-000111"); err != nil {
		t.Fatalf("admin CancelEnrollment() error = %v", err)
	}
	if repo.lastCancelUserID != 0 {
		t.Errorf("admin scope = %d, want 0", repo.lastCancelUserID)
	}
}

func TestCancelEnrollmentMapsNotFoundAndForbidsMissingIdentity(t *testing.T) {
	repo := &fakeTrainingEnrollmentRepository{cancelErr: repository.ErrTrainingEnrollmentNotFound}
	uc := usecase.NewTrainingEnrollmentUseCase(repo, &fakeTrainingCatalogRepository{})

	if err := uc.CancelEnrollment(context.Background(), memberActor(), "YTP-000111"); !errors.Is(err, usecase.ErrTrainingEnrollmentNotFound) {
		t.Fatalf("CancelEnrollment() error = %v, want ErrTrainingEnrollmentNotFound", err)
	}
	if err := uc.CancelEnrollment(context.Background(), entity.User{}, "YTP-000111"); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("zero actor CancelEnrollment() error = %v, want ErrForbidden", err)
	}
}

func TestFindMyEnrollmentsScopesToActor(t *testing.T) {
	repo := &fakeTrainingEnrollmentRepository{listResult: []entity.TrainingEnrollment{{ID: 1}, {ID: 2}, {ID: 3}}}
	uc := usecase.NewTrainingEnrollmentUseCase(repo, &fakeTrainingCatalogRepository{})

	result, err := uc.FindMyEnrollments(context.Background(), usecase.FindTrainingEnrollmentsInput{Actor: memberActor(), Limit: 2})
	if err != nil {
		t.Fatalf("FindMyEnrollments() error = %v", err)
	}
	if repo.lastFilter.UserID != 7 {
		t.Errorf("user filter = %d, want 7", repo.lastFilter.UserID)
	}
	if repo.lastFilter.Limit != 3 {
		t.Errorf("repository limit = %d, want 3", repo.lastFilter.Limit)
	}
	if len(result.Enrollments) != 2 || result.NextCursor != 2 {
		t.Errorf("result = %+v, want two enrollments and next cursor 2", result)
	}
}

func TestFindAllEnrollmentsRequiresAdmin(t *testing.T) {
	repo := &fakeTrainingEnrollmentRepository{}
	uc := usecase.NewTrainingEnrollmentUseCase(repo, &fakeTrainingCatalogRepository{})

	if _, err := uc.FindAllEnrollments(context.Background(), usecase.FindTrainingEnrollmentsInput{Actor: memberActor()}); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("member FindAllEnrollments() error = %v, want ErrForbidden", err)
	}

	if _, err := uc.FindAllEnrollments(context.Background(), usecase.FindTrainingEnrollmentsInput{
		Actor:  adminActor(),
		Search: "  Budi  ",
	}); err != nil {
		t.Fatalf("admin FindAllEnrollments() error = %v", err)
	}
	if repo.lastFilter.UserID != 0 {
		t.Errorf("admin user filter = %d, want 0 for unscoped", repo.lastFilter.UserID)
	}
	if repo.lastFilter.Search != "Budi" {
		t.Errorf("admin search filter = %q, want Budi", repo.lastFilter.Search)
	}
}

func TestFindCatalogEnrollmentsResolvesCatalogAndRequiresAdmin(t *testing.T) {
	repo := &fakeTrainingEnrollmentRepository{}
	catalogRepo := &fakeTrainingCatalogRepository{findResult: entity.TrainingCatalog{ID: 4, PublicID: "TCY-000004"}}
	uc := usecase.NewTrainingEnrollmentUseCase(repo, catalogRepo)

	if _, err := uc.FindCatalogEnrollments(context.Background(), usecase.FindCatalogEnrollmentsInput{
		Actor:           memberActor(),
		CatalogPublicID: "TCY-000004",
	}); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("member FindCatalogEnrollments() error = %v, want ErrForbidden", err)
	}

	if _, err := uc.FindCatalogEnrollments(context.Background(), usecase.FindCatalogEnrollmentsInput{
		Actor:           adminActor(),
		CatalogPublicID: "TCY-000004",
		Search:          "  Siti  ",
		Status:          "accepted",
	}); err != nil {
		t.Fatalf("admin FindCatalogEnrollments() error = %v", err)
	}
	if repo.lastFilter.CatalogID != 4 {
		t.Errorf("catalog filter = %d, want 4", repo.lastFilter.CatalogID)
	}
	if repo.lastFilter.Search != "Siti" {
		t.Errorf("search filter = %q, want Siti", repo.lastFilter.Search)
	}
	if repo.lastFilter.Status != entity.TrainingEnrollmentStatusAccepted {
		t.Errorf("status filter = %q, want accepted", repo.lastFilter.Status)
	}

	if _, err := uc.FindCatalogEnrollments(context.Background(), usecase.FindCatalogEnrollmentsInput{
		Actor:           adminActor(),
		CatalogPublicID: "YTP-000004",
		Status:          "bogus",
	}); !errors.Is(err, usecase.ErrBadRequest) {
		t.Fatalf("invalid status error = %v, want ErrBadRequest", err)
	}
}

func TestFindCatalogEnrollmentsMapsMissingCatalog(t *testing.T) {
	repo := &fakeTrainingEnrollmentRepository{}
	uc := usecase.NewTrainingEnrollmentUseCase(repo, &fakeTrainingCatalogRepository{})

	_, err := uc.FindCatalogEnrollments(context.Background(), usecase.FindCatalogEnrollmentsInput{
		Actor:           adminActor(),
		CatalogPublicID: "YTP-000404",
	})
	if !errors.Is(err, usecase.ErrTrainingCatalogNotFound) {
		t.Fatalf("FindCatalogEnrollments() error = %v, want ErrTrainingCatalogNotFound", err)
	}
}

func TestGenerateTrainingEnrollmentPublicID(t *testing.T) {
	id, err := usecase.GenerateTrainingEnrollmentPublicID()
	if err != nil {
		t.Fatalf("GenerateTrainingEnrollmentPublicID() error = %v", err)
	}
	if !trainingEnrollmentPublicIDPattern.MatchString(id) {
		t.Errorf("generated public id = %q, want matching %s", id, trainingEnrollmentPublicIDPattern.String())
	}
}

package handler_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"

	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/handler"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/middleware"
	"github.com/tnnz20/youthpreneur-be/internal/delivery/http/route"
	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/model"
	"github.com/tnnz20/youthpreneur-be/internal/token"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

type fakeTrainingEnrollmentUseCase struct {
	enrollResult    entity.TrainingEnrollment
	enrollErr       error
	lastEnrollInput usecase.CreateTrainingEnrollmentInput

	cancelErr          error
	lastCancelPublicID string

	myResult    usecase.FindTrainingEnrollmentsResult
	myErr       error
	lastMyInput usecase.FindTrainingEnrollmentsInput

	allResult    usecase.FindTrainingEnrollmentsResult
	allErr       error
	lastAllInput usecase.FindTrainingEnrollmentsInput

	catalogResult    usecase.FindTrainingEnrollmentsResult
	catalogErr       error
	lastCatalogInput usecase.FindCatalogEnrollmentsInput

	updateStatusResult    entity.TrainingEnrollment
	updateStatusErr       error
	lastUpdateStatusInput usecase.UpdateTrainingEnrollmentStatusInput
}

func (f *fakeTrainingEnrollmentUseCase) UpdateStatus(
	_ context.Context,
	input usecase.UpdateTrainingEnrollmentStatusInput,
) (entity.TrainingEnrollment, error) {
	f.lastUpdateStatusInput = input

	return f.updateStatusResult, f.updateStatusErr
}

func (f *fakeTrainingEnrollmentUseCase) Enroll(
	_ context.Context,
	input usecase.CreateTrainingEnrollmentInput,
) (entity.TrainingEnrollment, error) {
	f.lastEnrollInput = input

	return f.enrollResult, f.enrollErr
}

func (f *fakeTrainingEnrollmentUseCase) CancelEnrollment(_ context.Context, _ entity.User, publicID string) error {
	f.lastCancelPublicID = publicID

	return f.cancelErr
}

func (f *fakeTrainingEnrollmentUseCase) FindMyEnrollments(
	_ context.Context,
	input usecase.FindTrainingEnrollmentsInput,
) (usecase.FindTrainingEnrollmentsResult, error) {
	f.lastMyInput = input

	return f.myResult, f.myErr
}

func (f *fakeTrainingEnrollmentUseCase) FindAllEnrollments(
	_ context.Context,
	input usecase.FindTrainingEnrollmentsInput,
) (usecase.FindTrainingEnrollmentsResult, error) {
	f.lastAllInput = input

	return f.allResult, f.allErr
}

func (f *fakeTrainingEnrollmentUseCase) FindCatalogEnrollments(
	_ context.Context,
	input usecase.FindCatalogEnrollmentsInput,
) (usecase.FindTrainingEnrollmentsResult, error) {
	f.lastCatalogInput = input

	return f.catalogResult, f.catalogErr
}

func newTrainingEnrollmentRouter(uc usecase.TrainingEnrollmentUseCase, identity handler.IdentityFunc) *http.ServeMux {
	mux := http.NewServeMux()
	route.NewRouter(route.Dependencies{
		TrainingEnrollmentHandler: handler.NewTrainingEnrollmentHandler(slog.Default(), uc, identity),
		Authenticate:              passthroughMiddleware,
		RequireAdmin:              passthroughMiddleware,
	}).Register(mux)

	return mux
}

func TestCreateTrainingEnrollmentUsesActor(t *testing.T) {
	uc := &fakeTrainingEnrollmentUseCase{enrollResult: entity.TrainingEnrollment{
		PublicID:     "YTP-000111",
		UserPublicID: "YTP-000007",
		Status:       entity.TrainingEnrollmentStatusPending,
		Catalog:      &entity.TrainingCatalog{PublicID: "YTP-000004", Title: "Kelas"},
	}}
	actor := entity.User{ID: 7, PublicID: "YTP-000007", Role: entity.RoleMember}

	rec := serve(t, newTrainingEnrollmentRouter(uc, enterpriseIdentity(actor)),
		http.MethodPost, "/training-enrollments", `{"catalog_public_id":"YTP-000004"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if uc.lastEnrollInput.Actor.ID != 7 || uc.lastEnrollInput.CatalogPublicID != "YTP-000004" {
		t.Errorf("enroll input = %+v, want actor 7 and catalog public id", uc.lastEnrollInput)
	}

	var body model.TrainingEnrollmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Catalog == nil || body.Catalog.PublicID != "YTP-000004" {
		t.Errorf("catalog = %+v, want joined catalog", body.Catalog)
	}
	if body.Status != "pending" {
		t.Errorf("status = %q, want pending", body.Status)
	}
}

func TestUpdateTrainingEnrollmentStatus(t *testing.T) {
	uc := &fakeTrainingEnrollmentUseCase{
		updateStatusResult: entity.TrainingEnrollment{
			PublicID: "YTP-000111",
			Status:   entity.TrainingEnrollmentStatusAccepted,
		},
	}
	actor := entity.User{ID: 9, Role: entity.RoleAdmin}

	rec := serve(t, newTrainingEnrollmentRouter(uc, enterpriseIdentity(actor)),
		http.MethodPatch, "/training-enrollments/YTP-000111/status", `{"status":"accepted"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if uc.lastUpdateStatusInput.PublicID != "YTP-000111" || uc.lastUpdateStatusInput.Status != "accepted" {
		t.Errorf("status input = %+v, want YTP-000111 and accepted", uc.lastUpdateStatusInput)
	}

	var body model.TrainingEnrollmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "accepted" {
		t.Errorf("status = %q, want accepted", body.Status)
	}
}

func TestCreateTrainingEnrollmentErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{usecase.ErrBadRequest, http.StatusBadRequest},
		{usecase.ErrTrainingCatalogNotFound, http.StatusNotFound},
		{usecase.ErrAlreadyEnrolled, http.StatusConflict},
		{usecase.ErrTrainingCatalogFull, http.StatusConflict},
		{usecase.ErrTrainingCatalogClosed, http.StatusConflict},
	}

	for _, tc := range cases {
		uc := &fakeTrainingEnrollmentUseCase{enrollErr: tc.err}
		rec := serve(t, newTrainingEnrollmentRouter(uc, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember})),
			http.MethodPost, "/training-enrollments", `{"catalog_public_id":"YTP-000004"}`)
		if rec.Code != tc.want {
			t.Errorf("error %v status = %d, want %d", tc.err, rec.Code, tc.want)
		}
	}
}

func TestCreateTrainingEnrollmentRequiresIdentity(t *testing.T) {
	identity := func(context.Context) (entity.User, bool) { return entity.User{}, false }

	rec := serve(t, newTrainingEnrollmentRouter(&fakeTrainingEnrollmentUseCase{}, identity),
		http.MethodPost, "/training-enrollments", `{"catalog_public_id":"YTP-000004"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCancelTrainingEnrollmentReturnsNoContent(t *testing.T) {
	uc := &fakeTrainingEnrollmentUseCase{}

	rec := serve(t, newTrainingEnrollmentRouter(uc, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember})),
		http.MethodDelete, "/training-enrollments/YTP-000111", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if uc.lastCancelPublicID != "YTP-000111" {
		t.Errorf("cancelled public id = %q, want YTP-000111", uc.lastCancelPublicID)
	}
}

func TestListMyEnrollmentsParsesPagingAndReturnsCursor(t *testing.T) {
	uc := &fakeTrainingEnrollmentUseCase{myResult: usecase.FindTrainingEnrollmentsResult{
		Enrollments: []entity.TrainingEnrollment{{ID: 5, PublicID: "YTP-000111"}},
		NextCursor:  5,
	}}

	rec := serve(t, newTrainingEnrollmentRouter(uc, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember})),
		http.MethodGet, "/training-enrollments/my?cursor=4&limit=2", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if uc.lastMyInput.Cursor != 4 || uc.lastMyInput.Limit != 2 || uc.lastMyInput.Actor.ID != 7 {
		t.Errorf("my input = %+v, want actor 7 cursor 4 limit 2", uc.lastMyInput)
	}

	var body model.TrainingEnrollmentListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.NextCursor != "5" {
		t.Errorf("next_cursor = %q, want 5", body.NextCursor)
	}
}

func TestListCatalogEnrollmentsPassesCatalogPublicID(t *testing.T) {
	uc := &fakeTrainingEnrollmentUseCase{}

	rec := serve(t, newTrainingEnrollmentRouter(uc, enterpriseIdentity(entity.User{ID: 9, Role: entity.RoleAdmin})),
		http.MethodGet, "/training-enrollments/catalog/YTP-000004?status=accepted", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if uc.lastCatalogInput.CatalogPublicID != "YTP-000004" || uc.lastCatalogInput.Actor.ID != 9 || uc.lastCatalogInput.Status != "accepted" {
		t.Errorf("catalog input = %+v, want catalog public id, actor 9, and status accepted", uc.lastCatalogInput)
	}
}

func TestListEnrollmentsRejectsMalformedPaging(t *testing.T) {
	router := newTrainingEnrollmentRouter(&fakeTrainingEnrollmentUseCase{}, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember}))

	if rec := serve(t, router, http.MethodGet, "/training-enrollments/my?cursor=abc", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if rec := serve(t, router, http.MethodGet, "/training-enrollments/my?limit=0", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad limit status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestTrainingEnrollmentRoutePolicy confirms enrollment routes require
// authentication, member history is self-only, and admin history is admin-only.
func TestTrainingEnrollmentRoutePolicy(t *testing.T) {
	parser := routeParser{claims: token.AccessClaims{PublicID: "YTP-000007", Role: entity.RoleMember}}
	lookup := routeLookup{user: entity.User{ID: 7, PublicID: "YTP-000007", Role: entity.RoleMember, IsActive: true}}
	memberAuth := middleware.NewAuthenticator(parser, lookup, slog.Default())

	newRouter := func(authenticate route.Middleware, requireAdmin route.Middleware) *http.ServeMux {
		mux := http.NewServeMux()
		route.NewRouter(route.Dependencies{
			TrainingEnrollmentHandler: handler.NewTrainingEnrollmentHandler(slog.Default(), &fakeTrainingEnrollmentUseCase{}, middleware.IdentityFromContext),
			Authenticate:              authenticate,
			RequireAdmin:              requireAdmin,
		}).Register(mux)

		return mux
	}

	router := newRouter(memberAuth.Authenticate, memberAuth.RequireAdmin)

	if rec := serveWithCookie(t, router, http.MethodPost, "/training-enrollments", `{"catalog_public_id":"YTP-000004"}`, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated enroll status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if rec := serveWithCookie(t, router, http.MethodGet, "/training-enrollments/my", "", "good"); rec.Code != http.StatusOK {
		t.Fatalf("member my-history status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if rec := serveWithCookie(t, router, http.MethodGet, "/training-enrollments", "", "good"); rec.Code != http.StatusForbidden {
		t.Fatalf("member all-history status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if rec := serveWithCookie(t, router, http.MethodGet, "/training-enrollments/catalog/YTP-000004", "", "good"); rec.Code != http.StatusForbidden {
		t.Fatalf("member catalog-history status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if rec := serveWithCookie(t, router, http.MethodPatch, "/training-enrollments/YTP-000111/status", `{"status":"accepted"}`, "good"); rec.Code != http.StatusForbidden {
		t.Fatalf("member update-status status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

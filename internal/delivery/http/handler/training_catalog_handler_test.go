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
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/token"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

type fakeTrainingCatalogUseCase struct {
	createResult    entity.TrainingCatalog
	createErr       error
	lastCreateInput usecase.CreateTrainingCatalogInput

	getResult       entity.TrainingCatalog
	getErr          error
	lastGetPublicID string

	listResult    usecase.FindTrainingCatalogsResult
	listErr       error
	lastListInput usecase.FindTrainingCatalogsInput

	updateResult       entity.TrainingCatalog
	updateErr          error
	lastUpdatePublicID string
	lastUpdateInput    usecase.UpdateTrainingCatalogInput

	statusResult       entity.TrainingCatalog
	statusErr          error
	lastStatusPublicID string
	lastStatus         string

	deleteErr          error
	lastDeletePublicID string
}

func (f *fakeTrainingCatalogUseCase) CreateTrainingCatalog(
	_ context.Context,
	input usecase.CreateTrainingCatalogInput,
) (entity.TrainingCatalog, error) {
	f.lastCreateInput = input

	return f.createResult, f.createErr
}

func (f *fakeTrainingCatalogUseCase) GetTrainingCatalog(_ context.Context, publicID string) (entity.TrainingCatalog, error) {
	f.lastGetPublicID = publicID

	return f.getResult, f.getErr
}

func (f *fakeTrainingCatalogUseCase) FindTrainingCatalogs(
	_ context.Context,
	input usecase.FindTrainingCatalogsInput,
) (usecase.FindTrainingCatalogsResult, error) {
	f.lastListInput = input

	return f.listResult, f.listErr
}

func (f *fakeTrainingCatalogUseCase) UpdateTrainingCatalog(
	_ context.Context,
	_ entity.User,
	publicID string,
	input usecase.UpdateTrainingCatalogInput,
) (entity.TrainingCatalog, error) {
	f.lastUpdatePublicID = publicID
	f.lastUpdateInput = input

	return f.updateResult, f.updateErr
}

func (f *fakeTrainingCatalogUseCase) UpdateTrainingCatalogStatus(
	_ context.Context,
	_ entity.User,
	publicID string,
	status string,
) (entity.TrainingCatalog, error) {
	f.lastStatusPublicID = publicID
	f.lastStatus = status

	return f.statusResult, f.statusErr
}

func (f *fakeTrainingCatalogUseCase) DeleteTrainingCatalog(_ context.Context, _ entity.User, publicID string) error {
	f.lastDeletePublicID = publicID

	return f.deleteErr
}

func newTrainingCatalogRouter(uc usecase.TrainingCatalogUseCase, identity handler.IdentityFunc) *http.ServeMux {
	mux := http.NewServeMux()
	route.NewRouter(route.Dependencies{
		HealthHandler:          handler.NewHealthHandler(slog.Default(), usecase.NewHealthUseCase(repository.NewHealthRepository())),
		TrainingCatalogHandler: handler.NewTrainingCatalogHandler(slog.Default(), uc, identity),
		Authenticate:           passthroughMiddleware,
		RequireAdmin:           passthroughMiddleware,
	}).Register(mux)

	return mux
}

func TestListTrainingCatalogsIsPublic(t *testing.T) {
	uc := &fakeTrainingCatalogUseCase{listResult: usecase.FindTrainingCatalogsResult{
		Catalogs:   []entity.TrainingCatalog{{ID: 7, PublicID: "YTP-000007"}},
		NextCursor: 7,
	}}
	identity := func(context.Context) (entity.User, bool) { return entity.User{}, false }

	rec := serve(t, newTrainingCatalogRouter(uc, identity),
		http.MethodGet, "/training-catalog?category=Pemasaran&training_status=planned&training_date=2026-10-01&cursor=3&limit=5", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if uc.lastListInput.Category != "Pemasaran" || uc.lastListInput.TrainingStatus != "planned" ||
		uc.lastListInput.TrainingDate != "2026-10-01" || uc.lastListInput.Cursor != 3 || uc.lastListInput.Limit != 5 {
		t.Errorf("list input = %+v, want parsed filters", uc.lastListInput)
	}

	var body model.TrainingCatalogListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.NextCursor != "7" {
		t.Errorf("next_cursor = %q, want 7", body.NextCursor)
	}
}

func TestListTrainingCatalogsRejectsMalformedPaging(t *testing.T) {
	router := newTrainingCatalogRouter(&fakeTrainingCatalogUseCase{}, enterpriseIdentity(entity.User{}))

	if rec := serve(t, router, http.MethodGet, "/training-catalog?cursor=abc", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if rec := serve(t, router, http.MethodGet, "/training-catalog?limit=0", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad limit status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetTrainingCatalogSerializesNullOptionals(t *testing.T) {
	uc := &fakeTrainingCatalogUseCase{getResult: entity.TrainingCatalog{PublicID: "YTP-000007", Name: "Kelas"}}

	rec := serve(t, newTrainingCatalogRouter(uc, enterpriseIdentity(entity.User{})), http.MethodGet, "/training-catalog/YTP-000007", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	for _, field := range []string{"description", "pic_phone", "category", "training_slots", "training_status", "link", "training_date", "training_period", "speaker"} {
		if value, ok := body[field]; !ok || value != nil {
			t.Errorf("%s = %v (present %t), want JSON null", field, value, ok)
		}
	}
}

func TestGetTrainingCatalogNotFoundMaps404(t *testing.T) {
	uc := &fakeTrainingCatalogUseCase{getErr: usecase.ErrTrainingCatalogNotFound}

	rec := serve(t, newTrainingCatalogRouter(uc, enterpriseIdentity(entity.User{})), http.MethodGet, "/training-catalog/YTP-000404", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestCreateTrainingCatalogUsesActorAndReturnsCreated(t *testing.T) {
	uc := &fakeTrainingCatalogUseCase{createResult: entity.TrainingCatalog{PublicID: "YTP-000007", Name: "Kelas"}}
	actor := entity.User{ID: 9, Role: entity.RoleAdmin}

	rec := serve(t, newTrainingCatalogRouter(uc, enterpriseIdentity(actor)),
		http.MethodPost, "/training-catalog", `{"name":"Kelas","training_slots":10,"training_status":"planned"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if uc.lastCreateInput.Actor.ID != 9 {
		t.Errorf("actor = %d, want 9", uc.lastCreateInput.Actor.ID)
	}
	if uc.lastCreateInput.TrainingSlots == nil || *uc.lastCreateInput.TrainingSlots != 10 {
		t.Errorf("slots = %v, want 10", uc.lastCreateInput.TrainingSlots)
	}
}

func TestCreateTrainingCatalogRequiresIdentity(t *testing.T) {
	identity := func(context.Context) (entity.User, bool) { return entity.User{}, false }

	rec := serve(t, newTrainingCatalogRouter(&fakeTrainingCatalogUseCase{}, identity),
		http.MethodPost, "/training-catalog", `{"name":"Kelas"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUpdateTrainingCatalogForbiddenMaps403(t *testing.T) {
	uc := &fakeTrainingCatalogUseCase{updateErr: usecase.ErrForbidden}

	rec := serve(t, newTrainingCatalogRouter(uc, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember})),
		http.MethodPatch, "/training-catalog/YTP-000007", `{"name":"Baru"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestUpdateTrainingCatalogStatusPassesStatus(t *testing.T) {
	uc := &fakeTrainingCatalogUseCase{statusResult: entity.TrainingCatalog{PublicID: "YTP-000007"}}

	rec := serve(t, newTrainingCatalogRouter(uc, enterpriseIdentity(entity.User{ID: 9, Role: entity.RoleAdmin})),
		http.MethodPatch, "/training-catalog/YTP-000007/status", `{"training_status":"completed"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if uc.lastStatusPublicID != "YTP-000007" || uc.lastStatus != "completed" {
		t.Errorf("status call = (%q, %q), want (YTP-000007, completed)", uc.lastStatusPublicID, uc.lastStatus)
	}
}

func TestDeleteTrainingCatalogReturnsNoContent(t *testing.T) {
	uc := &fakeTrainingCatalogUseCase{}

	rec := serve(t, newTrainingCatalogRouter(uc, enterpriseIdentity(entity.User{ID: 9, Role: entity.RoleAdmin})),
		http.MethodDelete, "/training-catalog/YTP-000007", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if uc.lastDeletePublicID != "YTP-000007" {
		t.Errorf("deleted public id = %q, want YTP-000007", uc.lastDeletePublicID)
	}
}

// TestTrainingCatalogRoutePolicy confirms public reads need no cookie while
// catalog writes require the current admin role.
func TestTrainingCatalogRoutePolicy(t *testing.T) {
	memberParser := routeParser{claims: token.AccessClaims{PublicID: "YTP-000007", Role: entity.RoleMember}}
	memberLookup := routeLookup{user: entity.User{ID: 7, PublicID: "YTP-000007", Role: entity.RoleMember, IsActive: true}}
	memberAuth := middleware.NewAuthenticator(memberParser, memberLookup, slog.Default())

	adminParser := routeParser{claims: token.AccessClaims{PublicID: "YTP-000009", Role: entity.RoleAdmin}}
	adminLookup := routeLookup{user: entity.User{ID: 9, PublicID: "YTP-000009", Role: entity.RoleAdmin, IsActive: true}}

	newRouter := func(authenticate route.Middleware, requireAdmin route.Middleware) *http.ServeMux {
		mux := http.NewServeMux()
		route.NewRouter(route.Dependencies{
			TrainingCatalogHandler: handler.NewTrainingCatalogHandler(slog.Default(), &fakeTrainingCatalogUseCase{}, middleware.IdentityFromContext),
			Authenticate:           authenticate,
			RequireAdmin:           requireAdmin,
		}).Register(mux)

		return mux
	}

	public := newRouter(memberAuth.Authenticate, memberAuth.RequireAdmin)
	if rec := serve(t, public, http.MethodGet, "/training-catalog", ""); rec.Code != http.StatusOK {
		t.Fatalf("public list status = %d, want %d", rec.Code, http.StatusOK)
	}

	member := newRouter(memberAuth.Authenticate, memberAuth.RequireAdmin)
	if rec := serveWithCookie(t, member, http.MethodPost, "/training-catalog", `{"name":"Kelas"}`, "good"); rec.Code != http.StatusForbidden {
		t.Fatalf("member create status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	adminAuth := middleware.NewAuthenticator(adminParser, adminLookup, slog.Default())
	admin := newRouter(adminAuth.Authenticate, adminAuth.RequireAdmin)
	if rec := serveWithCookie(t, admin, http.MethodPost, "/training-catalog", `{"name":"Kelas"}`, "good"); rec.Code != http.StatusCreated {
		t.Fatalf("admin create status = %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

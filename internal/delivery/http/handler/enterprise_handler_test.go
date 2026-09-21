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

type fakeEnterpriseUseCase struct {
	createResult    entity.Enterprise
	createErr       error
	lastCreateInput usecase.CreateEnterpriseInput

	listResult    usecase.FindEnterprisesResult
	listErr       error
	lastListInput usecase.FindEnterprisesInput

	publicResult    usecase.FindPublicEnterprisesResult
	publicErr       error
	lastPublicInput usecase.FindPublicEnterprisesInput

	auditResult    usecase.FindEnterpriseAuditLogsResult
	auditErr       error
	lastAuditInput usecase.FindEnterpriseAuditLogsInput

	getResult entity.Enterprise
	getErr    error

	updateResult    entity.Enterprise
	updateErr       error
	lastUpdateInput usecase.UpdateEnterpriseInput

	deleteErr error
}

func (f *fakeEnterpriseUseCase) CreateEnterprise(
	_ context.Context,
	input usecase.CreateEnterpriseInput,
) (entity.Enterprise, error) {
	f.lastCreateInput = input

	return f.createResult, f.createErr
}

func (f *fakeEnterpriseUseCase) GetEnterprise(_ context.Context, _ entity.User, _ string) (entity.Enterprise, error) {
	return f.getResult, f.getErr
}

func (f *fakeEnterpriseUseCase) FindEnterprises(
	_ context.Context,
	input usecase.FindEnterprisesInput,
) (usecase.FindEnterprisesResult, error) {
	f.lastListInput = input

	return f.listResult, f.listErr
}

func (f *fakeEnterpriseUseCase) FindPublicEnterprises(
	_ context.Context,
	input usecase.FindPublicEnterprisesInput,
) (usecase.FindPublicEnterprisesResult, error) {
	f.lastPublicInput = input

	return f.publicResult, f.publicErr
}

func (f *fakeEnterpriseUseCase) FindEnterpriseAuditLogs(
	_ context.Context,
	input usecase.FindEnterpriseAuditLogsInput,
) (usecase.FindEnterpriseAuditLogsResult, error) {
	f.lastAuditInput = input

	return f.auditResult, f.auditErr
}

func (f *fakeEnterpriseUseCase) UpdateEnterprise(
	_ context.Context,
	_ entity.User,
	_ string,
	input usecase.UpdateEnterpriseInput,
) (entity.Enterprise, error) {
	f.lastUpdateInput = input

	return f.updateResult, f.updateErr
}

func (f *fakeEnterpriseUseCase) DeleteEnterprise(_ context.Context, _ entity.User, _ string) error {
	return f.deleteErr
}

func newEnterpriseRouter(uc usecase.EnterpriseUseCase, identity handler.IdentityFunc) *http.ServeMux {
	mux := http.NewServeMux()
	route.NewRouter(route.Dependencies{
		HealthHandler:     handler.NewHealthHandler(slog.Default(), usecase.NewHealthUseCase(repository.NewHealthRepository())),
		EnterpriseHandler: handler.NewEnterpriseHandler(slog.Default(), uc, identity),
		Authenticate:      passthroughMiddleware,
	}).Register(mux)

	return mux
}

func enterpriseIdentity(actor entity.User) handler.IdentityFunc {
	return func(context.Context) (entity.User, bool) {
		return actor, true
	}
}

func TestCreateEnterpriseReturnsCreatedWithNullDistrict(t *testing.T) {
	uc := &fakeEnterpriseUseCase{createResult: entity.Enterprise{
		PublicID:        "TPN-123456",
		EnterpriseName:  "Warung Kopi",
		BusinessSector:  entity.BusinessSectorKuliner,
		InitialTurnover: "1500.00",
		CurrentTurnover: "0.00",
		Status:          entity.EnterpriseStatusActive,
		CreatedAt:       1700000000,
		UpdatedAt:       1700000000,
	}}

	rec := serve(t, newEnterpriseRouter(uc, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember})),
		http.MethodPost, "/enterprises", `{"enterprise_name":"Warung Kopi","business_sector":"Kuliner"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["public_id"] != "TPN-123456" {
		t.Errorf("public_id = %v, want TPN-123456", body["public_id"])
	}
	if district, ok := body["district"]; !ok || district != nil {
		t.Errorf("district = %v (present %t), want JSON null", district, ok)
	}
}

func TestCreateEnterpriseUsesAuthenticatedActor(t *testing.T) {
	uc := &fakeEnterpriseUseCase{}
	actor := entity.User{ID: 7, Role: entity.RoleMember}

	rec := serve(t, newEnterpriseRouter(uc, enterpriseIdentity(actor)),
		http.MethodPost, "/enterprises", `{"enterprise_name":"Toko","business_sector":"Perdagangan Ritel"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if uc.lastCreateInput.Actor.ID != 7 {
		t.Errorf("actor = %d, want 7", uc.lastCreateInput.Actor.ID)
	}
}

func TestCreateEnterpriseRequiresIdentity(t *testing.T) {
	identity := func(context.Context) (entity.User, bool) { return entity.User{}, false }

	rec := serve(t, newEnterpriseRouter(&fakeEnterpriseUseCase{}, identity),
		http.MethodPost, "/enterprises", `{"enterprise_name":"Toko","business_sector":"Perdagangan Ritel"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListEnterprisesParsesFiltersAndReturnsNextCursor(t *testing.T) {
	uc := &fakeEnterpriseUseCase{listResult: usecase.FindEnterprisesResult{
		Enterprises: []entity.Enterprise{{ID: 11, PublicID: "TPN-000011"}},
		NextCursor:  11,
	}}

	rec := serve(t, newEnterpriseRouter(uc, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember})),
		http.MethodGet, "/enterprises?search=Kopi&district=Bandung&status=active&business_sector=Kuliner"+
			"&legal_status=complete&business_digitization=high&intervention_needs=Pelatihan"+
			"&training_status=completed&mentoring_status=ongoing&capital_access=yes&partnership=no"+
			"&cursor=10&limit=2", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if uc.lastListInput.Search != "Kopi" || uc.lastListInput.District != "Bandung" || uc.lastListInput.Status != "active" ||
		uc.lastListInput.BusinessSector != "Kuliner" {
		t.Errorf("filters = %+v, want parsed values", uc.lastListInput)
	}
	if uc.lastListInput.LegalStatus != "complete" || uc.lastListInput.BusinessDigitization != "high" ||
		uc.lastListInput.InterventionNeeds != "Pelatihan" || uc.lastListInput.TrainingStatus != "completed" ||
		uc.lastListInput.MentoringStatus != "ongoing" || uc.lastListInput.CapitalAccess != "yes" ||
		uc.lastListInput.Partnership != "no" {
		t.Errorf("enum filters = %+v, want parsed values", uc.lastListInput)
	}
	if uc.lastListInput.Cursor != 10 || uc.lastListInput.Limit != 2 {
		t.Errorf("cursor/limit = (%d, %d), want (10, 2)", uc.lastListInput.Cursor, uc.lastListInput.Limit)
	}

	var body model.EnterpriseListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.NextCursor != "11" {
		t.Errorf("next_cursor = %q, want 11", body.NextCursor)
	}
}

func TestListPublicEnterprisesParsesFiltersAndDoesNotRequireAuth(t *testing.T) {
	uc := &fakeEnterpriseUseCase{publicResult: usecase.FindPublicEnterprisesResult{
		Enterprises: []entity.PublicEnterprise{{
			ID:                11,
			PublicID:          "TPN-000011",
			EnterpriseName:    "Kopi Mantap",
			OwnerFullName:     "Budi",
			BusinessSector:    entity.BusinessSectorKuliner,
			District:          "Bandung",
			InterventionNeeds: entity.InterventionNeedsPelatihan,
		}},
		NextCursor: 11,
	}}

	router := newEnterpriseRouter(uc, func(context.Context) (entity.User, bool) {
		return entity.User{}, false // no auth identity
	})

	rec := serve(t, router, http.MethodGet,
		"/enterprises/public?search=Kopi&district=Bandung&intervention_needs=Pelatihan&business_sector=Kuliner&cursor=10&limit=2",
		"")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if uc.lastPublicInput.Search != "Kopi" || uc.lastPublicInput.District != "Bandung" ||
		uc.lastPublicInput.InterventionNeeds != "Pelatihan" || uc.lastPublicInput.BusinessSector != "Kuliner" ||
		uc.lastPublicInput.Cursor != 10 || uc.lastPublicInput.Limit != 2 {
		t.Errorf("public filters = %+v, want parsed values", uc.lastPublicInput)
	}

	var body model.PublicEnterpriseListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Enterprises) != 1 || body.Enterprises[0].PublicID != "TPN-000011" {
		t.Errorf("enterprises = %+v, want 1 item with TPN-000011", body.Enterprises)
	}
	if body.NextCursor != "11" {
		t.Errorf("next_cursor = %q, want 11", body.NextCursor)
	}
}

func TestListEnterprisesRejectsMalformedPaging(t *testing.T) {
	router := newEnterpriseRouter(&fakeEnterpriseUseCase{}, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember}))

	if rec := serve(t, router, http.MethodGet, "/enterprises?cursor=abc", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if rec := serve(t, router, http.MethodGet, "/enterprises?limit=0", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad limit status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetEnterpriseReturnsDistrict(t *testing.T) {
	uc := &fakeEnterpriseUseCase{getResult: entity.Enterprise{
		PublicID:       "TPN-000011",
		EnterpriseName: "Toko",
		District:       "Bandung",
	}}

	rec := serve(t, newEnterpriseRouter(uc, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember})),
		http.MethodGet, "/enterprises/TPN-000011", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body model.EnterpriseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.District == nil || *body.District != "Bandung" {
		t.Errorf("district = %v, want Bandung", body.District)
	}
}

func TestGetEnterpriseNotFoundMaps404(t *testing.T) {
	uc := &fakeEnterpriseUseCase{getErr: usecase.ErrEnterpriseNotFound}

	rec := serve(t, newEnterpriseRouter(uc, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember})),
		http.MethodGet, "/enterprises/TPN-000404", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestListEnterpriseAuditLogsSuccess(t *testing.T) {
	uc := &fakeEnterpriseUseCase{
		auditResult: usecase.FindEnterpriseAuditLogsResult{
			Events: []entity.EnterpriseAuditEventView{
				{
					ID:            15,
					ActorPublicID: "YTP-000001",
					ActorEmail:    "admin@example.com",
					ActorName:     "Admin User",
					Action:        entity.AuditActionUpdate,
					ChangedFields: map[string]any{"enterprise_name": "New Name"},
					CreatedAt:     1700000001,
				},
			},
			NextCursor: 15,
		},
	}

	router := newEnterpriseRouter(uc, enterpriseIdentity(entity.User{ID: 1, Role: entity.RoleAdmin}))
	rec := serve(t, router, http.MethodGet, "/enterprises/TPN-000011/audit-logs?cursor=50&limit=10", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if uc.lastAuditInput.PublicID != "TPN-000011" || uc.lastAuditInput.Cursor != 50 || uc.lastAuditInput.Limit != 10 {
		t.Errorf("lastAuditInput = %+v", uc.lastAuditInput)
	}

	var response model.EnterpriseAuditListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(response.Events) != 1 {
		t.Fatalf("got %d events, want 1", len(response.Events))
	}
	if response.NextCursor != "15" {
		t.Errorf("NextCursor = %q, want 15", response.NextCursor)
	}
	event := response.Events[0]
	if event.ID != 15 || event.ActorEmail != "admin@example.com" || event.Action != "update" {
		t.Errorf("unexpected event: %+v", event)
	}
}

func TestListEnterpriseAuditLogsInvalidParams(t *testing.T) {
	router := newEnterpriseRouter(&fakeEnterpriseUseCase{}, enterpriseIdentity(entity.User{ID: 1, Role: entity.RoleAdmin}))

	if rec := serve(t, router, http.MethodGet, "/enterprises/TPN-000011/audit-logs?cursor=xyz", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for invalid cursor", rec.Code)
	}
	if rec := serve(t, router, http.MethodGet, "/enterprises/TPN-000011/audit-logs?limit=abc", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for invalid limit", rec.Code)
	}
}

func TestListEnterpriseAuditLogsNotFound(t *testing.T) {
	uc := &fakeEnterpriseUseCase{auditErr: usecase.ErrEnterpriseNotFound}
	router := newEnterpriseRouter(uc, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember}))

	rec := serve(t, router, http.MethodGet, "/enterprises/TPN-000404/audit-logs", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUpdateEnterpriseForbiddenMaps403(t *testing.T) {
	uc := &fakeEnterpriseUseCase{updateErr: usecase.ErrForbidden}

	rec := serve(t, newEnterpriseRouter(uc, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember})),
		http.MethodPatch, "/enterprises/TPN-000011", `{"status":"inactive"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestUpdateEnterpriseBadRequestMaps400(t *testing.T) {
	uc := &fakeEnterpriseUseCase{updateErr: usecase.BadRequestError{Message: "invalid business_sector"}}

	rec := serve(t, newEnterpriseRouter(uc, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember})),
		http.MethodPatch, "/enterprises/TPN-000011", `{"business_sector":"bogus"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body model.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error != "invalid business_sector" {
		t.Errorf("error = %q, want invalid business_sector", body.Error)
	}
}

func TestDeleteEnterpriseReturnsNoContent(t *testing.T) {
	rec := serve(t, newEnterpriseRouter(&fakeEnterpriseUseCase{}, enterpriseIdentity(entity.User{ID: 7, Role: entity.RoleMember})),
		http.MethodDelete, "/enterprises/TPN-000011", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func newProtectedEnterpriseRouter(uc usecase.EnterpriseUseCase, parser routeParser, lookup routeLookup) *http.ServeMux {
	auth := middleware.NewAuthenticator(parser, lookup, slog.Default())
	mux := http.NewServeMux()
	route.NewRouter(route.Dependencies{
		EnterpriseHandler: handler.NewEnterpriseHandler(slog.Default(), uc, middleware.IdentityFromContext),
		Authenticate:      auth.Authenticate,
	}).Register(mux)

	return mux
}

func TestEnterpriseRoutesRequireAuthentication(t *testing.T) {
	mux := newProtectedEnterpriseRouter(&fakeEnterpriseUseCase{}, routeParser{}, routeLookup{})

	for _, path := range []string{"/enterprises", "/enterprises/TPN-000011/audit-logs"} {
		rec := serveWithCookie(t, mux, http.MethodGet, path, "", "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("path %s status = %d, want %d", path, rec.Code, http.StatusUnauthorized)
		}
	}
}

func TestEnterpriseRoutesAllowAuthenticatedMember(t *testing.T) {
	parser := routeParser{claims: token.AccessClaims{PublicID: "YTP-000007"}}
	lookup := routeLookup{user: entity.User{ID: 7, PublicID: "YTP-000007", Role: entity.RoleMember, IsActive: true}}
	mux := newProtectedEnterpriseRouter(&fakeEnterpriseUseCase{}, parser, lookup)

	rec := serveWithCookie(t, mux, http.MethodGet, "/enterprises", "", "good")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

func TestEnterpriseRepositoryIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; skipping PostgreSQL integration test")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	userRepo := NewUserRepository(db)
	repo := NewEnterpriseRepository(db)

	suffix := time.Now().UnixNano()
	owner := createIntegrationUser(t, ctx, db, userRepo, suffix, "owner")
	other := createIntegrationUser(t, ctx, db, userRepo, suffix+1, "other")
	now := time.Now().Unix()

	publicID := fmt.Sprintf("TPN-%06d", (suffix/1000)%1000000)
	created, err := repo.CreateEnterprise(ctx, entity.Enterprise{
		PublicID:        publicID,
		UserID:          owner.ID,
		EnterpriseName:  "Integration",
		BusinessSector:  entity.BusinessSectorPerdaganganRitel,
		InitialTurnover: "100.00",
		CurrentTurnover: "50.50",
		District:        "Bandung",
		Status:          entity.EnterpriseStatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, entity.EnterpriseAuditEvent{
		ActorUserID:   owner.ID,
		Action:        entity.AuditActionCreate,
		ChangedFields: map[string]any{"enterprise_name": "Integration"},
		CreatedAt:     now,
	})
	if err != nil {
		t.Fatalf("CreateEnterprise() error = %v", err)
	}
	if created.ID == 0 || created.InitialTurnover != "100.00" {
		t.Fatalf("CreateEnterprise() = %+v, want persisted id and turnover", created)
	}

	found, err := repo.FindEnterpriseByPublicID(ctx, publicID, owner.ID)
	if err != nil {
		t.Fatalf("FindEnterpriseByPublicID() error = %v", err)
	}
	if found.EnterpriseName != "Integration" || found.District != "Bandung" || found.Status != entity.EnterpriseStatusActive {
		t.Errorf("FindEnterpriseByPublicID() = %+v, want persisted fields", found)
	}

	if _, err := repo.FindEnterpriseByPublicID(ctx, publicID, other.ID); !errors.Is(err, repository.ErrEnterpriseNotFound) {
		t.Errorf("FindEnterpriseByPublicID() for other owner error = %v, want ErrEnterpriseNotFound", err)
	}

	if _, err := repo.CreateEnterprise(ctx, entity.Enterprise{
		PublicID:        fmt.Sprintf("TPN-%06d", (suffix/1000+1)%1000000),
		UserID:          other.ID,
		EnterpriseName:  "Other",
		BusinessSector:  entity.BusinessSectorJasaLayananPublik,
		InitialTurnover: "0.00",
		CurrentTurnover: "0.00",
		Status:          entity.EnterpriseStatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, entity.EnterpriseAuditEvent{ActorUserID: other.ID, Action: entity.AuditActionCreate, ChangedFields: map[string]any{}, CreatedAt: now}); err != nil {
		t.Fatalf("CreateEnterprise() for other owner error = %v", err)
	}

	list, err := repo.FindEnterprises(ctx, entity.EnterpriseFilter{
		OwnerID:        owner.ID,
		District:       "Bandung",
		Status:         entity.EnterpriseStatusActive,
		BusinessSector: entity.BusinessSectorPerdaganganRitel,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("FindEnterprises() error = %v", err)
	}
	if len(list) != 1 || list[0].PublicID != publicID {
		t.Fatalf("FindEnterprises() = %+v, want only the owner's matching enterprise", list)
	}

	renamed := "Renamed"
	currentTurnover := "75.25"
	inactive := entity.EnterpriseStatusInactive
	updated, err := repo.UpdateEnterprise(ctx, publicID, owner.ID, entity.EnterpriseUpdate{
		EnterpriseName:  &renamed,
		CurrentTurnover: &currentTurnover,
		Status:          &inactive,
		UpdatedAt:       now + 1,
	}, entity.EnterpriseAuditEvent{
		ActorUserID: owner.ID,
		Action:      entity.AuditActionUpdate,
		CreatedAt:   now + 1,
	})
	if err != nil {
		t.Fatalf("UpdateEnterprise() error = %v", err)
	}
	if updated.EnterpriseName != "Renamed" || updated.CurrentTurnover != "75.25" || updated.Status != entity.EnterpriseStatusInactive {
		t.Errorf("UpdateEnterprise() = %+v, want renamed inactive enterprise", updated)
	}
	if updated.BusinessSector != entity.BusinessSectorPerdaganganRitel || updated.InitialTurnover != "100.00" || updated.District != "Bandung" {
		t.Errorf("UpdateEnterprise() corrupted untouched fields = %+v", updated)
	}

	if err := repo.SoftDeleteEnterprise(ctx, publicID, owner.ID, now+2, entity.EnterpriseAuditEvent{
		ActorUserID: owner.ID,
		Action:      entity.AuditActionDelete,
		CreatedAt:   now + 2,
	}); err != nil {
		t.Fatalf("SoftDeleteEnterprise() error = %v", err)
	}

	if _, err := repo.FindEnterpriseByPublicID(ctx, publicID, owner.ID); !errors.Is(err, repository.ErrEnterpriseNotFound) {
		t.Errorf("FindEnterpriseByPublicID() after soft delete error = %v, want ErrEnterpriseNotFound", err)
	}

	var auditCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM enterprise_audit_events eae
		JOIN enterprises e ON e.id = eae.enterprise_id
		WHERE e.public_id = $1`, publicID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit events: %v", err)
	}
	if auditCount != 3 {
		t.Errorf("audit event count = %d, want 3", auditCount)
	}
}

func TestEnterpriseRepositoryPaginationIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	userRepo := NewUserRepository(db)
	repo := NewEnterpriseRepository(db)

	suffix := time.Now().UnixNano()
	owner := createIntegrationUser(t, ctx, db, userRepo, suffix, "paging")
	now := time.Now().Unix()

	var lastPublicID string
	for i := range 3 {
		publicID := fmt.Sprintf("TPN-%06d", (suffix/1000+int64(i)+10)%1000000)
		lastPublicID = publicID
		if _, err := repo.CreateEnterprise(ctx, entity.Enterprise{
			PublicID:        publicID,
			UserID:          owner.ID,
			EnterpriseName:  fmt.Sprintf("Paged %d", i),
			BusinessSector:  entity.BusinessSectorPerdaganganRitel,
			InitialTurnover: "0.00",
			CurrentTurnover: "0.00",
			Status:          entity.EnterpriseStatusActive,
			CreatedAt:       now,
			UpdatedAt:       now,
		}, entity.EnterpriseAuditEvent{ActorUserID: owner.ID, Action: entity.AuditActionCreate, ChangedFields: map[string]any{}, CreatedAt: now}); err != nil {
			t.Fatalf("CreateEnterprise(%d) error = %v", i, err)
		}
	}

	page, err := repo.FindEnterprises(ctx, entity.EnterpriseFilter{OwnerID: owner.ID, Limit: 2})
	if err != nil {
		t.Fatalf("FindEnterprises() error = %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("page size = %d, want 2", len(page))
	}

	next, err := repo.FindEnterprises(ctx, entity.EnterpriseFilter{OwnerID: owner.ID, Cursor: page[1].ID, Limit: 2})
	if err != nil {
		t.Fatalf("FindEnterprises() second page error = %v", err)
	}
	if len(next) != 1 || next[0].PublicID != lastPublicID {
		t.Fatalf("second page = %+v, want remaining %s", next, lastPublicID)
	}
}

func TestApplyEnterpriseUpdateMergesAndReportsChangedFields(t *testing.T) {
	locked := entity.Enterprise{
		ID:              5,
		PublicID:        "TPN-000005",
		UserID:          7,
		EnterpriseName:  "Old",
		BusinessSector:  entity.BusinessSectorKuliner,
		LegalStatus:     entity.LegalStatusComplete,
		InitialTurnover: "100.00",
		CurrentTurnover: "50.50",
		District:        "Bandung",
		Status:          entity.EnterpriseStatusActive,
	}

	name := "New"
	clearedLegal := entity.LegalStatus("")
	currentTurnover := "75.25"
	merged, changed := applyEnterpriseUpdate(locked, entity.EnterpriseUpdate{
		EnterpriseName:  &name,
		LegalStatus:     &clearedLegal,
		CurrentTurnover: &currentTurnover,
		UpdatedAt:       99,
	})

	if merged.EnterpriseName != "New" || merged.CurrentTurnover != "75.25" || merged.LegalStatus != "" {
		t.Errorf("merged = %+v, want applied name, turnover, and cleared legal status", merged)
	}
	if merged.InitialTurnover != "100.00" || merged.BusinessSector != entity.BusinessSectorKuliner || merged.District != "Bandung" || merged.Status != entity.EnterpriseStatusActive {
		t.Errorf("merged = %+v, want untouched fields preserved", merged)
	}
	if merged.UpdatedAt != 99 {
		t.Errorf("updated_at = %d, want 99", merged.UpdatedAt)
	}
	if changed["enterprise_name"] != "New" || changed["current_turnover"] != "75.25" {
		t.Errorf("changed = %v, want enterprise_name and current_turnover", changed)
	}
	if value, ok := changed["legal_status"]; !ok || value != nil {
		t.Errorf("changed legal_status = %v (present %v), want nil", value, ok)
	}
	for _, field := range []string{"business_sector", "initial_turnover", "district", "status"} {
		if _, ok := changed[field]; ok {
			t.Errorf("changed unexpectedly records %s: %v", field, changed)
		}
	}
}

func TestApplyEnterpriseUpdateNoChange(t *testing.T) {
	locked := entity.Enterprise{
		ID:              5,
		PublicID:        "TPN-000005",
		EnterpriseName:  "Old",
		BusinessSector:  entity.BusinessSectorKuliner,
		InitialTurnover: "100.00",
		CurrentTurnover: "50.50",
		Status:          entity.EnterpriseStatusActive,
	}

	name := "Old"
	turnover := "100.00"
	_, changed := applyEnterpriseUpdate(locked, entity.EnterpriseUpdate{
		EnterpriseName:  &name,
		InitialTurnover: &turnover,
		UpdatedAt:       99,
	})

	if len(changed) != 0 {
		t.Errorf("changed = %v, want empty for identical values", changed)
	}
}

func TestEnterpriseTurnoverConstraintsIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	userRepo := NewUserRepository(db)

	suffix := time.Now().UnixNano()
	owner := createIntegrationUser(t, ctx, db, userRepo, suffix, "turnover-check")
	now := time.Now().Unix()

	insert := func(t *testing.T, publicID, initialTurnover, currentTurnover string) error {
		t.Helper()

		_, err := db.ExecContext(ctx, `
			INSERT INTO enterprises (
				public_id, user_id, enterprise_name, business_sector,
				initial_turnover, current_turnover, status, created_at, updated_at
			)
			VALUES ($1, $2, 'Turnover Check', 'Kuliner', $3::numeric, $4::numeric, 'active', $5, $5)`,
			publicID, owner.ID, initialTurnover, currentTurnover, now)

		return err
	}

	base := (suffix / 1000) % 1000000
	if err := insert(t, fmt.Sprintf("TPN-%06d", base), "9999999999999.99", "0.00"); err != nil {
		t.Errorf("insert boundary turnover error = %v, want accepted", err)
	}
	if err := insert(t, fmt.Sprintf("TPN-%06d", base+1), "0.00", "0.00"); err != nil {
		t.Errorf("insert zero turnover error = %v, want accepted", err)
	}

	negative := insert(t, fmt.Sprintf("TPN-%06d", base+2), "-0.01", "0.00")
	if negative == nil {
		t.Errorf("insert negative initial turnover error = nil, want check violation")
	}
	assertEnterpriseTurnoverConstraint(t, negative, "enterprises_initial_turnover_non_negative")

	overflow := insert(t, fmt.Sprintf("TPN-%06d", base+3), "0.00", "10000000000000")
	if overflow == nil {
		t.Errorf("insert overflow current turnover error = nil, want rejection")
	}
	assertEnterprisePgErrorCode(t, overflow, "22003")
}

func TestEnterpriseRepositoryConcurrentUpdatesIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	userRepo := NewUserRepository(db)
	repo := NewEnterpriseRepository(db)

	suffix := time.Now().UnixNano()
	owner := createIntegrationUser(t, ctx, db, userRepo, suffix, "concurrent")
	now := time.Now().Unix()
	publicID := fmt.Sprintf("TPN-%06d", (suffix/1000)%1000000)

	if _, err := repo.CreateEnterprise(ctx, entity.Enterprise{
		PublicID:        publicID,
		UserID:          owner.ID,
		EnterpriseName:  "Start",
		BusinessSector:  entity.BusinessSectorKuliner,
		InitialTurnover: "10.00",
		CurrentTurnover: "20.00",
		District:        "Bandung",
		Status:          entity.EnterpriseStatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, entity.EnterpriseAuditEvent{ActorUserID: owner.ID, Action: entity.AuditActionCreate, ChangedFields: map[string]any{}, CreatedAt: now}); err != nil {
		t.Fatalf("CreateEnterprise() error = %v", err)
	}

	name := "Concurrent Name"
	currentTurnover := "99.99"
	results := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := repo.UpdateEnterprise(ctx, publicID, owner.ID, entity.EnterpriseUpdate{
			EnterpriseName: &name,
			UpdatedAt:      now + 1,
		}, entity.EnterpriseAuditEvent{ActorUserID: owner.ID, Action: entity.AuditActionUpdate, CreatedAt: now + 1})
		results <- err
	}()
	go func() {
		defer wg.Done()
		_, err := repo.UpdateEnterprise(ctx, publicID, owner.ID, entity.EnterpriseUpdate{
			CurrentTurnover: &currentTurnover,
			UpdatedAt:       now + 2,
		}, entity.EnterpriseAuditEvent{ActorUserID: owner.ID, Action: entity.AuditActionUpdate, CreatedAt: now + 2})
		results <- err
	}()
	wg.Wait()
	close(results)

	for err := range results {
		if err != nil {
			t.Fatalf("concurrent UpdateEnterprise() error = %v", err)
		}
	}

	final, err := repo.FindEnterpriseByPublicID(ctx, publicID, owner.ID)
	if err != nil {
		t.Fatalf("FindEnterpriseByPublicID() error = %v", err)
	}
	if final.EnterpriseName != "Concurrent Name" || final.CurrentTurnover != "99.99" {
		t.Errorf("final = %+v, want both concurrent changes preserved", final)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT changed_fields
		FROM enterprise_audit_events eae
		JOIN enterprises e ON e.id = eae.enterprise_id
		WHERE e.public_id = $1 AND eae.action = 'update'
		ORDER BY eae.id`, publicID)
	if err != nil {
		t.Fatalf("query update audits: %v", err)
	}
	defer func() { _ = rows.Close() }()

	seen := map[string]int{}
	for rows.Next() {
		var changed []byte
		if err := rows.Scan(&changed); err != nil {
			t.Fatalf("scan changed_fields: %v", err)
		}

		var fields map[string]any
		if err := json.Unmarshal(changed, &fields); err != nil {
			t.Fatalf("decode changed_fields %s: %v", changed, err)
		}
		if len(fields) != 1 {
			t.Errorf("audit changed_fields = %s, want exactly one field", changed)
		}
		for field := range fields {
			seen[field]++
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate update audits: %v", err)
	}
	if seen["enterprise_name"] != 1 || seen["current_turnover"] != 1 || len(seen) != 2 {
		t.Errorf("audit fields = %v, want one enterprise_name and one current_turnover", seen)
	}
}

func TestEnterpriseRepositoryMutationReturnedNullableAndPrecisionIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	userRepo := NewUserRepository(db)
	repo := NewEnterpriseRepository(db)

	suffix := time.Now().UnixNano()
	owner := createIntegrationUser(t, ctx, db, userRepo, suffix, "nullable")
	now := time.Now().Unix()
	publicID := fmt.Sprintf("TPN-%06d", (suffix/1000)%1000000)

	created, err := repo.CreateEnterprise(ctx, entity.Enterprise{
		PublicID:        publicID,
		UserID:          owner.ID,
		EnterpriseName:  "Nullable Test",
		BusinessSector:  entity.BusinessSectorKuliner,
		InitialTurnover: "0.50",
		CurrentTurnover: "0.00",
		Status:          entity.EnterpriseStatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, entity.EnterpriseAuditEvent{ActorUserID: owner.ID, Action: entity.AuditActionCreate, ChangedFields: map[string]any{}, CreatedAt: now})
	if err != nil {
		t.Fatalf("CreateEnterprise() error = %v", err)
	}
	if created.EnterpriseName != "Nullable Test" || created.LegalStatus != "" || created.District != "" || created.DeletedAt != nil {
		t.Errorf("created nullable fields = %+v", created)
	}
	if created.InitialTurnover != "0.50" || created.CurrentTurnover != "0.00" {
		t.Errorf("created turnovers = (%q, %q), want 0.50 and 0.00", created.InitialTurnover, created.CurrentTurnover)
	}

	newName := "Updated Name"
	boundary := "9999999999999.99"
	updated, err := repo.UpdateEnterprise(ctx, publicID, owner.ID, entity.EnterpriseUpdate{
		EnterpriseName:  &newName,
		InitialTurnover: &boundary,
		UpdatedAt:       now + 1,
	}, entity.EnterpriseAuditEvent{ActorUserID: owner.ID, Action: entity.AuditActionUpdate, CreatedAt: now + 1})
	if err != nil {
		t.Fatalf("UpdateEnterprise() error = %v", err)
	}
	if updated.EnterpriseName != "Updated Name" || updated.LegalStatus != "" || updated.District != "" {
		t.Errorf("updated nullable fields = %+v", updated)
	}
	if updated.InitialTurnover != "9999999999999.99" {
		t.Errorf("updated initial_turnover = %q, want 9999999999999.99", updated.InitialTurnover)
	}
	if updated.CurrentTurnover != "0.00" {
		t.Errorf("updated current_turnover = %q, want untouched 0.00", updated.CurrentTurnover)
	}
}

func assertEnterpriseTurnoverConstraint(t *testing.T, err error, constraint string) {
	t.Helper()

	assertEnterprisePgErrorCode(t, err, "23514")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error = %v, want *pgconn.PgError", err)
	}
	if pgErr.ConstraintName != constraint {
		t.Errorf("constraint = %q, want %q", pgErr.ConstraintName, constraint)
	}
}

func assertEnterprisePgErrorCode(t *testing.T, err error, code string) {
	t.Helper()

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error = %v, want *pgconn.PgError", err)
	}
	if pgErr.Code != code {
		t.Errorf("error code = %q, want %q", pgErr.Code, code)
	}
}

func createIntegrationUser(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	repo repository.UserRepository,
	suffix int64,
	name string,
) entity.User {
	t.Helper()

	user, err := repo.CreateUser(ctx, entity.User{
		PublicID:  fmt.Sprintf("YTP-%06d", suffix%1000000),
		Email:     fmt.Sprintf("%s-%d@example.com", name, suffix),
		Password:  "hash",
		Role:      entity.RoleMember,
		IsActive:  true,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
		Profile:   &entity.Profile{FullName: name},
	})
	if err != nil {
		t.Fatalf("create integration user: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE public_id = $1", user.PublicID) })

	return user
}

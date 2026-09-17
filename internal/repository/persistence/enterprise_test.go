package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

func TestMapEnterpriseInsertErrorTranslatesPublicIDCollision(t *testing.T) {
	err := mapEnterpriseInsertError(&pgconn.PgError{Code: uniqueViolation, ConstraintName: "enterprises_public_id_key"})

	if !errors.Is(err, repository.ErrDuplicateEnterprisePublicID) {
		t.Fatalf("mapEnterpriseInsertError() = %v, want ErrDuplicateEnterprisePublicID", err)
	}
}

func TestMapEnterpriseInsertErrorWrapsOtherFailures(t *testing.T) {
	err := mapEnterpriseInsertError(errors.New("connection reset"))

	if errors.Is(err, repository.ErrDuplicateEnterprisePublicID) {
		t.Fatalf("mapEnterpriseInsertError() = %v, want wrapped generic error", err)
	}
}

// TestEnterpriseRepositoryIntegration exercises real SQL against PostgreSQL. It
// runs only when TEST_POSTGRES_DSN points at a database with migrations
// applied.
func TestEnterpriseRepositoryIntegration(t *testing.T) {
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
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	userRepo := NewUserRepository(db)
	repo := NewEnterpriseRepository(db)

	suffix := time.Now().UnixNano()
	owner := createIntegrationUser(t, ctx, db, userRepo, suffix, "owner")
	other := createIntegrationUser(t, ctx, db, userRepo, suffix+1, "other")
	now := time.Now().Unix()

	publicID := fmt.Sprintf("YTP-%06d", (suffix/1000)%1000000)
	created, err := repo.CreateEnterprise(ctx, entity.Enterprise{
		PublicID:        publicID,
		UserID:          owner.ID,
		Name:            "Integration",
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
		ChangedFields: map[string]any{"name": "Integration"},
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
	if found.Name != "Integration" || found.District != "Bandung" || found.Status != entity.EnterpriseStatusActive {
		t.Errorf("FindEnterpriseByPublicID() = %+v, want persisted fields", found)
	}

	if _, err := repo.FindEnterpriseByPublicID(ctx, publicID, other.ID); !errors.Is(err, repository.ErrEnterpriseNotFound) {
		t.Errorf("FindEnterpriseByPublicID() for other owner error = %v, want ErrEnterpriseNotFound", err)
	}

	if _, err := repo.CreateEnterprise(ctx, entity.Enterprise{
		PublicID:        fmt.Sprintf("YTP-%06d", (suffix/1000+1)%1000000),
		UserID:          other.ID,
		Name:            "Other",
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

	updated, err := repo.UpdateEnterprise(ctx, publicID, owner.ID, entity.Enterprise{
		Name:            "Renamed",
		BusinessSector:  entity.BusinessSectorPerdaganganRitel,
		InitialTurnover: "100.00",
		CurrentTurnover: "75.25",
		District:        "Bandung",
		Status:          entity.EnterpriseStatusInactive,
		UpdatedAt:       now + 1,
	}, entity.EnterpriseAuditEvent{
		ActorUserID:   owner.ID,
		Action:        entity.AuditActionUpdate,
		ChangedFields: map[string]any{"name": "Renamed", "status": "inactive"},
		CreatedAt:     now + 1,
	})
	if err != nil {
		t.Fatalf("UpdateEnterprise() error = %v", err)
	}
	if updated.Name != "Renamed" || updated.CurrentTurnover != "75.25" || updated.Status != entity.EnterpriseStatusInactive {
		t.Errorf("UpdateEnterprise() = %+v, want renamed inactive enterprise", updated)
	}

	if _, err := repo.UpdateEnterprise(ctx, publicID, other.ID, entity.Enterprise{
		Name:            "Hijacked",
		BusinessSector:  entity.BusinessSectorPerdaganganRitel,
		InitialTurnover: "0.00",
		CurrentTurnover: "0.00",
		Status:          entity.EnterpriseStatusActive,
		UpdatedAt:       now + 2,
	}, entity.EnterpriseAuditEvent{ActorUserID: other.ID, Action: entity.AuditActionUpdate, ChangedFields: map[string]any{}, CreatedAt: now + 2}); !errors.Is(err, repository.ErrEnterpriseNotFound) {
		t.Errorf("UpdateEnterprise() for other owner error = %v, want ErrEnterpriseNotFound", err)
	}

	if err := repo.SoftDeleteEnterprise(ctx, publicID, owner.ID, now+3, entity.EnterpriseAuditEvent{
		ActorUserID:   owner.ID,
		Action:        entity.AuditActionDelete,
		ChangedFields: map[string]any{},
		CreatedAt:     now + 3,
	}); err != nil {
		t.Fatalf("SoftDeleteEnterprise() error = %v", err)
	}

	if _, err := repo.FindEnterpriseByPublicID(ctx, publicID, owner.ID); !errors.Is(err, repository.ErrEnterpriseNotFound) {
		t.Errorf("FindEnterpriseByPublicID() after delete error = %v, want ErrEnterpriseNotFound", err)
	}
	if err := repo.SoftDeleteEnterprise(ctx, publicID, owner.ID, now+4, entity.EnterpriseAuditEvent{ActorUserID: owner.ID, Action: entity.AuditActionDelete, ChangedFields: map[string]any{}, CreatedAt: now + 4}); !errors.Is(err, repository.ErrEnterpriseNotFound) {
		t.Errorf("second SoftDeleteEnterprise() error = %v, want ErrEnterpriseNotFound", err)
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
		publicID := fmt.Sprintf("YTP-%06d", (suffix/1000+int64(i)+10)%1000000)
		lastPublicID = publicID
		if _, err := repo.CreateEnterprise(ctx, entity.Enterprise{
			PublicID:        publicID,
			UserID:          owner.ID,
			Name:            fmt.Sprintf("Paged %d", i),
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

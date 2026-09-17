package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

func TestMapTrainingCatalogInsertErrorTranslatesPublicIDCollision(t *testing.T) {
	err := mapTrainingCatalogInsertError(&pgconn.PgError{Code: uniqueViolation, ConstraintName: "training_catalog_public_id_key"})

	if !errors.Is(err, repository.ErrDuplicateTrainingCatalogPublicID) {
		t.Fatalf("mapTrainingCatalogInsertError() = %v, want ErrDuplicateTrainingCatalogPublicID", err)
	}
}

func TestMapTrainingCatalogInsertErrorWrapsOtherFailures(t *testing.T) {
	err := mapTrainingCatalogInsertError(errors.New("connection reset"))

	if errors.Is(err, repository.ErrDuplicateTrainingCatalogPublicID) {
		t.Fatalf("mapTrainingCatalogInsertError() = %v, want wrapped generic error", err)
	}
}

func TestApplyTrainingCatalogUpdateMergesOptionalFields(t *testing.T) {
	slots := 10
	locked := entity.TrainingCatalog{
		ID:             3,
		PublicID:       "YTP-000003",
		Name:           "Old",
		Category:       "Lama",
		TrainingSlots:  &slots,
		TrainingStatus: entity.ProcessStatusPlanned,
	}

	name := "New"
	category := ""
	newStatus := entity.ProcessStatusCompleted
	merged := applyTrainingCatalogUpdate(locked, entity.TrainingCatalogUpdate{
		Name:           &name,
		Category:       &category,
		TrainingStatus: &newStatus,
		UpdatedAt:      99,
	})

	if merged.Name != "New" || merged.Category != "" || merged.TrainingStatus != entity.ProcessStatusCompleted {
		t.Errorf("merged = %+v, want applied name, cleared category, completed status", merged)
	}
	if merged.TrainingSlots == nil || *merged.TrainingSlots != 10 {
		t.Errorf("slots = %v, want untouched 10", merged.TrainingSlots)
	}
	if merged.UpdatedAt != 99 {
		t.Errorf("updated_at = %d, want 99", merged.UpdatedAt)
	}
	if merged.PublicID != "YTP-000003" {
		t.Errorf("public id = %q, want untouched", merged.PublicID)
	}
}

// TestTrainingCatalogScanSharedAcrossJoinedEnrollment locks the catalog column
// mapping so joined enrollment reads and direct catalog reads cannot drift.
func TestTrainingCatalogScanSharedAcrossJoinedEnrollment(t *testing.T) {
	trainingDay := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	fillCatalog := func(dest []any) {
		*(dest[0].(*int)) = 3
		*(dest[1].(*string)) = "YTP-000003"
		*(dest[2].(*sql.NullString)) = sql.NullString{String: "Kelas", Valid: true}
		*(dest[3].(*sql.NullString)) = sql.NullString{String: "Deskripsi", Valid: true}
		*(dest[4].(*sql.NullString)) = sql.NullString{String: "0812", Valid: true}
		*(dest[5].(*sql.NullString)) = sql.NullString{String: "Pemasaran", Valid: true}
		*(dest[6].(*sql.NullInt64)) = sql.NullInt64{Int64: 20, Valid: true}
		*(dest[7].(*sql.NullString)) = sql.NullString{String: "planned", Valid: true}
		*(dest[8].(*sql.NullString)) = sql.NullString{String: "https://example.com", Valid: true}
		*(dest[9].(*sql.NullTime)) = sql.NullTime{Time: trainingDay, Valid: true}
		*(dest[10].(*sql.NullString)) = sql.NullString{String: "09:00-12:00", Valid: true}
		*(dest[11].(*sql.NullString)) = sql.NullString{String: "Budi", Valid: true}
		*(dest[12].(*int64)) = 100
		*(dest[13].(*int64)) = 200
		*(dest[14].(*sql.NullInt64)) = sql.NullInt64{Int64: 300, Valid: true}
	}

	direct, err := scanTrainingCatalog(func(dest ...any) error {
		fillCatalog(dest)

		return nil
	})
	if err != nil {
		t.Fatalf("scanTrainingCatalog() error = %v", err)
	}

	joined, err := scanTrainingEnrollmentJoined(func(dest ...any) error {
		*(dest[0].(*int)) = 7
		*(dest[1].(*string)) = "YTP-000007"
		*(dest[2].(*int)) = 9
		*(dest[3].(*string)) = "YTP-000009"
		*(dest[4].(*int)) = 3
		*(dest[5].(*sql.NullTime)) = sql.NullTime{Time: trainingDay, Valid: true}
		*(dest[6].(*int64)) = 400
		*(dest[7].(*int64)) = 400
		*(dest[8].(*sql.NullInt64)) = sql.NullInt64{}
		fillCatalog(dest[9:])

		return nil
	})
	if err != nil {
		t.Fatalf("scanTrainingEnrollmentJoined() error = %v", err)
	}

	if joined.Catalog == nil {
		t.Fatal("joined catalog = nil, want shared row mapping")
	}
	if !reflect.DeepEqual(direct, *joined.Catalog) {
		t.Errorf("joined catalog = %+v, want same mapping as direct %+v", *joined.Catalog, direct)
	}
}

// TestTrainingCatalogRepositoryIntegration exercises real SQL against
// PostgreSQL. It runs only when TEST_POSTGRES_DSN points at a database with
// migrations applied.
func TestTrainingCatalogRepositoryIntegration(t *testing.T) {
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
	repo := NewTrainingCatalogRepository(db)
	suffix := time.Now().UnixNano()
	now := time.Now().Unix()
	publicID := fmt.Sprintf("YTP-%06d", (suffix/1000)%1000000)
	trainingDate := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM training_catalog WHERE public_id = $1", publicID)
	})

	slots := 2
	created, err := repo.CreateTrainingCatalog(ctx, entity.TrainingCatalog{
		PublicID:       publicID,
		Name:           "Bisnis Digital",
		Category:       "Pemasaran",
		TrainingSlots:  &slots,
		TrainingStatus: entity.ProcessStatusPlanned,
		TrainingDate:   &trainingDate,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		t.Fatalf("CreateTrainingCatalog() error = %v", err)
	}
	if created.ID == 0 || created.TrainingSlots == nil || *created.TrainingSlots != 2 {
		t.Fatalf("CreateTrainingCatalog() = %+v, want persisted id and slots", created)
	}
	if created.TrainingDate == nil || created.TrainingDate.Format("2006-01-02") != "2026-10-01" {
		t.Errorf("training date = %v, want 2026-10-01", created.TrainingDate)
	}

	found, err := repo.FindTrainingCatalogByPublicID(ctx, publicID)
	if err != nil {
		t.Fatalf("FindTrainingCatalogByPublicID() error = %v", err)
	}
	if found.Name != "Bisnis Digital" || found.Category != "Pemasaran" || found.TrainingStatus != entity.ProcessStatusPlanned {
		t.Errorf("found = %+v, want persisted fields", found)
	}

	list, err := repo.FindTrainingCatalogs(ctx, entity.TrainingCatalogFilter{
		Category:       "Pemasaran",
		TrainingStatus: entity.ProcessStatusPlanned,
		TrainingDate:   &trainingDate,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("FindTrainingCatalogs() error = %v", err)
	}
	if len(list) != 1 || list[0].PublicID != publicID {
		t.Fatalf("FindTrainingCatalogs() = %+v, want only the matching catalog", list)
	}

	newName := "Bisnis Digital Lanjutan"
	completed := entity.ProcessStatusCompleted
	updated, err := repo.UpdateTrainingCatalog(ctx, publicID, entity.TrainingCatalogUpdate{
		Name:           &newName,
		TrainingStatus: &completed,
		UpdatedAt:      now + 1,
	})
	if err != nil {
		t.Fatalf("UpdateTrainingCatalog() error = %v", err)
	}
	if updated.Name != "Bisnis Digital Lanjutan" || updated.TrainingStatus != entity.ProcessStatusCompleted {
		t.Errorf("updated = %+v, want renamed completed catalog", updated)
	}
	if updated.Category != "Pemasaran" || updated.TrainingSlots == nil || *updated.TrainingSlots != 2 {
		t.Errorf("updated = %+v, want untouched category and slots", updated)
	}

	if err := repo.SoftDeleteTrainingCatalog(ctx, publicID, now+2); err != nil {
		t.Fatalf("SoftDeleteTrainingCatalog() error = %v", err)
	}
	if _, err := repo.FindTrainingCatalogByPublicID(ctx, publicID); !errors.Is(err, repository.ErrTrainingCatalogNotFound) {
		t.Errorf("FindTrainingCatalogByPublicID() after delete error = %v, want ErrTrainingCatalogNotFound", err)
	}
	if err := repo.SoftDeleteTrainingCatalog(ctx, publicID, now+3); !errors.Is(err, repository.ErrTrainingCatalogNotFound) {
		t.Errorf("second SoftDeleteTrainingCatalog() error = %v, want ErrTrainingCatalogNotFound", err)
	}
}

func TestTrainingCatalogSlotsConstraintIntegration(t *testing.T) {
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
	suffix := time.Now().UnixNano()
	now := time.Now().Unix()
	publicID := fmt.Sprintf("YTP-%06d", (suffix/1000)%1000000)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM training_catalog WHERE public_id = $1", publicID)
	})

	_, err = db.ExecContext(ctx, `
		INSERT INTO training_catalog (public_id, name, training_slots, created_at, updated_at)
		VALUES ($1, 'Bad', 0, $2, $2)`,
		publicID, now)
	if err == nil {
		t.Fatal("insert zero slots error = nil, want check violation")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "training_catalog_training_slots_positive" {
		t.Errorf("constraint = %v, want training_catalog_training_slots_positive", err)
	}
}

// TestTrainingEnrollmentRepositoryIntegration covers capacity, duplicate
// rejection, cancellation, re-enrollment, and history in one database.
func TestTrainingEnrollmentRepositoryIntegration(t *testing.T) {
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
	catalogRepo := NewTrainingCatalogRepository(db)
	repo := NewTrainingEnrollmentRepository(db)

	suffix := time.Now().UnixNano()
	first := createIntegrationUser(t, ctx, db, userRepo, suffix, "enroll-a")
	second := createIntegrationUser(t, ctx, db, userRepo, suffix+1, "enroll-b")
	now := time.Now().Unix()
	registerDate := time.Unix(now, 0).UTC()

	slots := 1
	catalog := createIntegrationCatalog(t, ctx, db, catalogRepo, suffix, slots, entity.ProcessStatusPlanned)

	firstEnrollment := createIntegrationEnrollment(t, ctx, repo, first, catalog, suffix, registerDate, now)
	if firstEnrollment.UserPublicID != first.PublicID {
		t.Errorf("user public id = %q, want %q", firstEnrollment.UserPublicID, first.PublicID)
	}
	if firstEnrollment.Catalog == nil || firstEnrollment.Catalog.PublicID != catalog.PublicID {
		t.Errorf("catalog = %+v, want joined catalog %s", firstEnrollment.Catalog, catalog.PublicID)
	}

	// Same user cannot enroll twice while the first enrollment is active.
	if _, err := repo.CreateTrainingEnrollment(ctx, entity.TrainingEnrollment{
		PublicID:     fmt.Sprintf("YTP-%06d", (suffix/1000+1)%1000000),
		UserID:       first.ID,
		RegisterDate: &registerDate,
		CreatedAt:    now,
		UpdatedAt:    now,
		Catalog:      &entity.TrainingCatalog{PublicID: catalog.PublicID},
	}); !errors.Is(err, repository.ErrDuplicateTrainingEnrollment) {
		t.Fatalf("duplicate enrollment error = %v, want ErrDuplicateTrainingEnrollment", err)
	}

	// Capacity is full for a different user.
	if _, err := repo.CreateTrainingEnrollment(ctx, entity.TrainingEnrollment{
		PublicID:     fmt.Sprintf("YTP-%06d", (suffix/1000+2)%1000000),
		UserID:       second.ID,
		RegisterDate: &registerDate,
		CreatedAt:    now,
		UpdatedAt:    now,
		Catalog:      &entity.TrainingCatalog{PublicID: catalog.PublicID},
	}); !errors.Is(err, repository.ErrTrainingCatalogFull) {
		t.Fatalf("full catalog error = %v, want ErrTrainingCatalogFull", err)
	}

	// Cancel, then capacity frees up and the same user may re-enroll.
	if err := repo.CancelTrainingEnrollment(ctx, firstEnrollment.PublicID, first.ID, now+1); err != nil {
		t.Fatalf("CancelTrainingEnrollment() error = %v", err)
	}
	if err := repo.CancelTrainingEnrollment(ctx, firstEnrollment.PublicID, first.ID, now+2); !errors.Is(err, repository.ErrTrainingEnrollmentNotFound) {
		t.Fatalf("second CancelTrainingEnrollment() error = %v, want ErrTrainingEnrollmentNotFound", err)
	}

	secondEnrollment := createIntegrationEnrollment(t, ctx, repo, second, catalog, suffix+3000, registerDate, now+3)

	history, err := repo.FindTrainingEnrollments(ctx, entity.TrainingEnrollmentFilter{UserID: first.ID, Limit: 10})
	if err != nil {
		t.Fatalf("FindTrainingEnrollments() error = %v", err)
	}
	if len(history) != 1 || history[0].PublicID != firstEnrollment.PublicID {
		t.Fatalf("member history = %+v, want the cancelled enrollment only", history)
	}
	if history[0].DeletedAt == nil {
		t.Error("cancelled enrollment deleted_at = nil, want preserved cancellation")
	}
	if history[0].Catalog == nil {
		t.Error("cancelled enrollment catalog = nil, want joined history")
	}

	all, err := repo.FindTrainingEnrollments(ctx, entity.TrainingEnrollmentFilter{UserID: second.ID, Limit: 10})
	if err != nil {
		t.Fatalf("FindTrainingEnrollments(second) error = %v", err)
	}
	if len(all) != 1 || all[0].PublicID != secondEnrollment.PublicID {
		t.Fatalf("second history = %+v, want one active enrollment", all)
	}
}

func TestTrainingEnrollmentCatalogStatusIntegration(t *testing.T) {
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
	catalogRepo := NewTrainingCatalogRepository(db)
	repo := NewTrainingEnrollmentRepository(db)

	suffix := time.Now().UnixNano()
	member := createIntegrationUser(t, ctx, db, userRepo, suffix, "open")
	now := time.Now().Unix()
	registerDate := time.Unix(now, 0).UTC()

	completedCatalog := createIntegrationCatalog(t, ctx, db, catalogRepo, suffix, 0, entity.ProcessStatusCompleted)
	if _, err := repo.CreateTrainingEnrollment(ctx, entity.TrainingEnrollment{
		PublicID:     fmt.Sprintf("YTP-%06d", (suffix/1000+1)%1000000),
		UserID:       member.ID,
		RegisterDate: &registerDate,
		CreatedAt:    now,
		UpdatedAt:    now,
		Catalog:      &entity.TrainingCatalog{PublicID: completedCatalog.PublicID},
	}); !errors.Is(err, repository.ErrTrainingCatalogClosed) {
		t.Fatalf("completed catalog error = %v, want ErrTrainingCatalogClosed", err)
	}

	ongoingCatalog := createIntegrationCatalog(t, ctx, db, catalogRepo, suffix+1000, 0, entity.ProcessStatusOngoing)
	if _, err := repo.CreateTrainingEnrollment(ctx, entity.TrainingEnrollment{
		PublicID:     fmt.Sprintf("YTP-%06d", (suffix/1000+2)%1000000),
		UserID:       member.ID,
		RegisterDate: &registerDate,
		CreatedAt:    now,
		UpdatedAt:    now,
		Catalog:      &entity.TrainingCatalog{PublicID: ongoingCatalog.PublicID},
	}); err != nil {
		t.Fatalf("ongoing enrollment error = %v, want accepted", err)
	}
}

func TestTrainingEnrollmentCapacityConcurrencyIntegration(t *testing.T) {
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
	catalogRepo := NewTrainingCatalogRepository(db)
	repo := NewTrainingEnrollmentRepository(db)

	suffix := time.Now().UnixNano()
	first := createIntegrationUser(t, ctx, db, userRepo, suffix, "race-a")
	second := createIntegrationUser(t, ctx, db, userRepo, suffix+1, "race-b")
	now := time.Now().Unix()
	registerDate := time.Unix(now, 0).UTC()
	catalog := createIntegrationCatalog(t, ctx, db, catalogRepo, suffix, 1, entity.ProcessStatusPlanned)

	users := []entity.User{first, second}
	results := make(chan error, len(users))
	var wg sync.WaitGroup

	for i, user := range users {
		wg.Add(1)
		go func(index int, user entity.User) {
			defer wg.Done()
			_, err := repo.CreateTrainingEnrollment(ctx, entity.TrainingEnrollment{
				PublicID:     fmt.Sprintf("YTP-%06d", (suffix/1000+int64(index)+1)%1000000),
				UserID:       user.ID,
				RegisterDate: &registerDate,
				CreatedAt:    now,
				UpdatedAt:    now,
				Catalog:      &entity.TrainingCatalog{PublicID: catalog.PublicID},
			})
			results <- err
		}(i, user)
	}
	wg.Wait()
	close(results)

	var success, full int
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, repository.ErrTrainingCatalogFull):
			full++
		default:
			t.Fatalf("concurrent enrollment error = %v, want nil or full", err)
		}
	}
	if success != 1 || full != 1 {
		t.Fatalf("success = %d, full = %d, want exactly one active slot", success, full)
	}

	var active int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM training_enrollments
		WHERE training_catalog_id = $1 AND deleted_at IS NULL`,
		catalog.ID,
	).Scan(&active); err != nil {
		t.Fatalf("count active enrollments: %v", err)
	}
	if active != 1 {
		t.Fatalf("active enrollments = %d, want 1", active)
	}
}

func createIntegrationCatalog(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	repo repository.TrainingCatalogRepository,
	suffix int64,
	slots int,
	status entity.ProcessStatus,
) entity.TrainingCatalog {
	t.Helper()

	publicID := fmt.Sprintf("YTP-%06d", (suffix/1000)%1000000)
	now := time.Now().Unix()
	options := entity.TrainingCatalog{
		PublicID:       publicID,
		Name:           "Integration Training",
		TrainingStatus: status,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if slots > 0 {
		options.TrainingSlots = &slots
	}

	catalog, err := repo.CreateTrainingCatalog(ctx, options)
	if err != nil {
		t.Fatalf("create integration catalog: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM training_enrollments WHERE training_catalog_id = $1", catalog.ID)
		_, _ = db.ExecContext(ctx, "DELETE FROM training_catalog WHERE id = $1", catalog.ID)
	})

	return catalog
}

func createIntegrationEnrollment(
	t *testing.T,
	ctx context.Context,
	repo repository.TrainingEnrollmentRepository,
	user entity.User,
	catalog entity.TrainingCatalog,
	suffix int64,
	registerDate time.Time,
	now int64,
) entity.TrainingEnrollment {
	t.Helper()

	enrollment, err := repo.CreateTrainingEnrollment(ctx, entity.TrainingEnrollment{
		PublicID:     fmt.Sprintf("YTP-%06d", (suffix/1000)%1000000),
		UserID:       user.ID,
		RegisterDate: &registerDate,
		CreatedAt:    now,
		UpdatedAt:    now,
		Catalog:      &entity.TrainingCatalog{PublicID: catalog.PublicID},
	})
	if err != nil {
		t.Fatalf("create integration enrollment: %v", err)
	}

	return enrollment
}

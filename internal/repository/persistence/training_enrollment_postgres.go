package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

const trainingEnrollmentColumns = `
	e.id, e.public_id, e.user_id, u.public_id, e.training_catalog_id,
	e.register_date, e.status, e.created_at, e.updated_at, e.deleted_at`

// findTrainingEnrollmentsQuery joins the catalog and enrolling user so history
// stays readable. Cancelled enrollments remain visible; callers exclude them
// from active capacity counts separately.
const findTrainingEnrollmentsQuery = `
	SELECT ` + trainingEnrollmentColumns + `, ` + trainingCatalogColumns + `
	FROM training_enrollments e
	JOIN training_catalog c ON c.id = e.training_catalog_id
	JOIN users u ON u.id = e.user_id
	WHERE ($1::int = 0 OR e.user_id = $1)
	  AND ($2::int = 0 OR e.training_catalog_id = $2)
	  AND ($3 = '' OR e.status = NULLIF($3, '')::training_enrollment_status_enum)
	  AND ($4::int = 0 OR e.id > $4)
	ORDER BY e.id ASC
	LIMIT $5`

type trainingEnrollmentRepository struct {
	db *sql.DB
}

// NewTrainingEnrollmentRepository creates a PostgreSQL-backed training
// enrollment repository.
func NewTrainingEnrollmentRepository(db *sql.DB) repository.TrainingEnrollmentRepository {
	return trainingEnrollmentRepository{db: db}
}

func (r trainingEnrollmentRepository) CreateTrainingEnrollment(
	ctx context.Context,
	enrollment entity.TrainingEnrollment,
) (entity.TrainingEnrollment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.TrainingEnrollment{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	catalog, err := scanTrainingCatalog(tx.QueryRowContext(ctx, `
		SELECT `+trainingCatalogColumns+`
		FROM training_catalog c
		WHERE c.public_id = $1 AND c.deleted_at IS NULL
		FOR UPDATE`,
		enrollment.Catalog.PublicID,
	).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.TrainingEnrollment{}, repository.ErrTrainingCatalogNotFound
	}
	if err != nil {
		return entity.TrainingEnrollment{}, fmt.Errorf("lock training catalog: %w", err)
	}
	if !catalog.IsOpenForEnrollment() {
		return entity.TrainingEnrollment{}, repository.ErrTrainingCatalogClosed
	}

	var existing int
	err = tx.QueryRowContext(ctx, `
		SELECT 1
		FROM training_enrollments
		WHERE training_catalog_id = $1 AND user_id = $2 AND deleted_at IS NULL
		LIMIT 1`,
		catalog.ID,
		enrollment.UserID,
	).Scan(&existing)
	switch {
	case err == nil:
		return entity.TrainingEnrollment{}, repository.ErrDuplicateTrainingEnrollment
	case !errors.Is(err, sql.ErrNoRows):
		return entity.TrainingEnrollment{}, fmt.Errorf("check active enrollment: %w", err)
	}

	if catalog.MaxSlots != nil {
		var active int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM training_enrollments
			WHERE training_catalog_id = $1 AND deleted_at IS NULL AND status != 'rejected'`,
			catalog.ID,
		).Scan(&active); err != nil {
			return entity.TrainingEnrollment{}, fmt.Errorf("count active enrollments: %w", err)
		}
		if active >= *catalog.MaxSlots {
			return entity.TrainingEnrollment{}, repository.ErrTrainingCatalogFull
		}
	}

	status := enrollment.Status
	if status == "" {
		status = entity.TrainingEnrollmentStatusPending
	}

	var id int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO training_enrollments (
			public_id, user_id, training_catalog_id, register_date, status,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4::date, $5::training_enrollment_status_enum, $6, $7)
		RETURNING id`,
		enrollment.PublicID,
		enrollment.UserID,
		catalog.ID,
		nullTime(enrollment.RegisterDate),
		string(status),
		enrollment.CreatedAt,
		enrollment.UpdatedAt,
	).Scan(&id)
	if err != nil {
		return entity.TrainingEnrollment{}, mapTrainingEnrollmentInsertError(err)
	}

	created, err := scanTrainingEnrollmentJoined(tx.QueryRowContext(ctx, `
		SELECT `+trainingEnrollmentColumns+`, `+trainingCatalogColumns+`
		FROM training_enrollments e
		JOIN training_catalog c ON c.id = e.training_catalog_id
		JOIN users u ON u.id = e.user_id
		WHERE e.id = $1`,
		id,
	).Scan)
	if err != nil {
		return entity.TrainingEnrollment{}, fmt.Errorf("find created enrollment: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return entity.TrainingEnrollment{}, fmt.Errorf("commit transaction: %w", err)
	}

	return created, nil
}

func (r trainingEnrollmentRepository) UpdateTrainingEnrollmentStatus(
	ctx context.Context,
	publicID string,
	status entity.TrainingEnrollmentStatus,
	updatedAt int64,
) (entity.TrainingEnrollment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.TrainingEnrollment{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		enrollmentID int
		catalogID    int
		oldStatus    string
		maxSlots     sql.NullInt64
	)
	err = tx.QueryRowContext(ctx, `
		SELECT e.id, e.training_catalog_id, e.status, c.max_slots
		FROM training_enrollments e
		JOIN training_catalog c ON c.id = e.training_catalog_id
		WHERE e.public_id = $1 AND e.deleted_at IS NULL
		FOR UPDATE OF c, e`,
		publicID,
	).Scan(&enrollmentID, &catalogID, &oldStatus, &maxSlots)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.TrainingEnrollment{}, repository.ErrTrainingEnrollmentNotFound
	}
	if err != nil {
		return entity.TrainingEnrollment{}, fmt.Errorf("lock enrollment and catalog: %w", err)
	}

	if status == entity.TrainingEnrollmentStatusAccepted && oldStatus != string(entity.TrainingEnrollmentStatusAccepted) {
		if maxSlots.Valid {
			var active int
			if err := tx.QueryRowContext(ctx, `
				SELECT COUNT(*)
				FROM training_enrollments
				WHERE training_catalog_id = $1 AND deleted_at IS NULL AND status = 'accepted'`,
				catalogID,
			).Scan(&active); err != nil {
				return entity.TrainingEnrollment{}, fmt.Errorf("count accepted enrollments: %w", err)
			}
			if active >= int(maxSlots.Int64) {
				return entity.TrainingEnrollment{}, repository.ErrTrainingCatalogFull
			}
		}
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE training_enrollments
		SET status = $2::training_enrollment_status_enum, updated_at = $3
		WHERE public_id = $1 AND deleted_at IS NULL`,
		publicID,
		string(status),
		updatedAt,
	)
	if err != nil {
		return entity.TrainingEnrollment{}, fmt.Errorf("update training enrollment status: %w", err)
	}
	if err := requireTrainingEnrollmentAffected(result); err != nil {
		return entity.TrainingEnrollment{}, err
	}

	updated, err := scanTrainingEnrollmentJoined(tx.QueryRowContext(ctx, `
		SELECT `+trainingEnrollmentColumns+`, `+trainingCatalogColumns+`
		FROM training_enrollments e
		JOIN training_catalog c ON c.id = e.training_catalog_id
		JOIN users u ON u.id = e.user_id
		WHERE e.id = $1`,
		enrollmentID,
	).Scan)
	if err != nil {
		return entity.TrainingEnrollment{}, fmt.Errorf("find updated enrollment: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return entity.TrainingEnrollment{}, fmt.Errorf("commit transaction: %w", err)
	}

	return updated, nil
}

func (r trainingEnrollmentRepository) CancelTrainingEnrollment(
	ctx context.Context,
	publicID string,
	userID int,
	deletedAt int64,
) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE training_enrollments
		SET deleted_at = $3, updated_at = $3
		WHERE public_id = $1 AND deleted_at IS NULL
		  AND ($2::int = 0 OR user_id = $2)`,
		publicID,
		userID,
		deletedAt,
	)
	if err != nil {
		return fmt.Errorf("cancel training enrollment: %w", err)
	}
	if err := requireTrainingEnrollmentAffected(result); err != nil {
		return err
	}

	return nil
}

func (r trainingEnrollmentRepository) FindTrainingEnrollments(
	ctx context.Context,
	filter entity.TrainingEnrollmentFilter,
) ([]entity.TrainingEnrollment, error) {
	rows, err := r.db.QueryContext(ctx, findTrainingEnrollmentsQuery,
		filter.UserID,
		filter.CatalogID,
		string(filter.Status),
		filter.Cursor,
		filter.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query training enrollments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	enrollments := []entity.TrainingEnrollment{}
	for rows.Next() {
		enrollment, err := scanTrainingEnrollmentJoined(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan training enrollment: %w", err)
		}

		enrollments = append(enrollments, enrollment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate training enrollments: %w", err)
	}

	return enrollments, nil
}

func scanTrainingEnrollmentJoined(scan func(dest ...any) error) (entity.TrainingEnrollment, error) {
	var (
		enrollment  entity.TrainingEnrollment
		registerDay sql.NullTime
		status      string
		deletedAt   sql.NullInt64
		catalogRow  trainingCatalogRow
	)

	dest := []any{
		&enrollment.ID,
		&enrollment.PublicID,
		&enrollment.UserID,
		&enrollment.UserPublicID,
		&enrollment.TrainingCatalogID,
		&registerDay,
		&status,
		&enrollment.CreatedAt,
		&enrollment.UpdatedAt,
		&deletedAt,
	}
	dest = append(dest, catalogRow.scanDest()...)

	if err := scan(dest...); err != nil {
		return entity.TrainingEnrollment{}, err
	}

	if registerDay.Valid {
		enrollment.RegisterDate = &registerDay.Time
	}
	enrollment.Status = entity.TrainingEnrollmentStatus(status)
	if deletedAt.Valid {
		enrollment.DeletedAt = &deletedAt.Int64
	}
	catalog := catalogRow.toCatalog()
	enrollment.Catalog = &catalog

	return enrollment, nil
}

func mapTrainingEnrollmentInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		switch {
		case strings.Contains(pgErr.ConstraintName, "active_unique"):
			return repository.ErrDuplicateTrainingEnrollment
		case strings.Contains(pgErr.ConstraintName, "public_id"):
			return repository.ErrDuplicateTrainingEnrollmentPublicID
		}
	}

	return fmt.Errorf("insert training enrollment: %w", err)
}

func requireTrainingEnrollmentAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return repository.ErrTrainingEnrollmentNotFound
	}

	return nil
}

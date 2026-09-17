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

const trainingCatalogColumns = `
	c.id, c.public_id, c.name, c.description, c.pic_phone, c.category,
	c.training_slots, c.training_status, c.link, c.training_date,
	c.training_period, c.speaker, c.created_at, c.updated_at, c.deleted_at`

// findTrainingCatalogsQuery applies optional filters. The enum parameter is
// wrapped in NULLIF so an empty filter binds NULL instead of failing an enum
// cast.
const findTrainingCatalogsQuery = `
	SELECT ` + trainingCatalogColumns + `
	FROM training_catalog c
	WHERE c.deleted_at IS NULL
	  AND ($1 = '' OR c.category = $1)
	  AND ($2 = '' OR c.training_status = NULLIF($2, '')::process_status_enum)
	  AND ($3::date IS NULL OR c.training_date = $3::date)
	  AND ($4 = '' OR c.training_period = $4)
	  AND ($5::int = 0 OR c.id > $5)
	ORDER BY c.id ASC
	LIMIT $6`

type trainingCatalogRepository struct {
	db *sql.DB
}

// NewTrainingCatalogRepository creates a PostgreSQL-backed training catalog
// repository.
func NewTrainingCatalogRepository(db *sql.DB) repository.TrainingCatalogRepository {
	return trainingCatalogRepository{db: db}
}

func (r trainingCatalogRepository) CreateTrainingCatalog(
	ctx context.Context,
	catalog entity.TrainingCatalog,
) (entity.TrainingCatalog, error) {
	created, err := scanTrainingCatalog(r.db.QueryRowContext(ctx, `
		INSERT INTO training_catalog AS c (
			public_id, name, description, pic_phone, category, training_slots,
			training_status, link, training_date, training_period, speaker,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6,
			NULLIF($7, '')::process_status_enum, $8, $9::date, $10, $11, $12, $13)
		RETURNING `+trainingCatalogColumns,
		catalog.PublicID,
		nullString(catalog.Name),
		nullString(catalog.Description),
		nullString(catalog.PicPhone),
		nullString(catalog.Category),
		nullInt(catalog.TrainingSlots),
		string(catalog.TrainingStatus),
		nullString(catalog.Link),
		nullTime(catalog.TrainingDate),
		nullString(catalog.TrainingPeriod),
		nullString(catalog.Speaker),
		catalog.CreatedAt,
		catalog.UpdatedAt,
	).Scan)
	if err != nil {
		return entity.TrainingCatalog{}, mapTrainingCatalogInsertError(err)
	}

	return created, nil
}

func (r trainingCatalogRepository) FindTrainingCatalogByPublicID(
	ctx context.Context,
	publicID string,
) (entity.TrainingCatalog, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+trainingCatalogColumns+`
		FROM training_catalog c
		WHERE c.public_id = $1 AND c.deleted_at IS NULL`,
		publicID,
	)

	catalog, err := scanTrainingCatalog(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.TrainingCatalog{}, repository.ErrTrainingCatalogNotFound
	}
	if err != nil {
		return entity.TrainingCatalog{}, fmt.Errorf("find training catalog: %w", err)
	}

	return catalog, nil
}

func (r trainingCatalogRepository) FindTrainingCatalogs(
	ctx context.Context,
	filter entity.TrainingCatalogFilter,
) ([]entity.TrainingCatalog, error) {
	rows, err := r.db.QueryContext(ctx, findTrainingCatalogsQuery,
		filter.Category,
		string(filter.TrainingStatus),
		nullTime(filter.TrainingDate),
		filter.TrainingPeriod,
		filter.Cursor,
		filter.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query training catalogs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	catalogs := []entity.TrainingCatalog{}
	for rows.Next() {
		catalog, err := scanTrainingCatalog(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan training catalog: %w", err)
		}

		catalogs = append(catalogs, catalog)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate training catalogs: %w", err)
	}

	return catalogs, nil
}

func (r trainingCatalogRepository) UpdateTrainingCatalog(
	ctx context.Context,
	publicID string,
	update entity.TrainingCatalogUpdate,
) (entity.TrainingCatalog, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.TrainingCatalog{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	locked, err := scanTrainingCatalog(tx.QueryRowContext(ctx, `
		SELECT `+trainingCatalogColumns+`
		FROM training_catalog c
		WHERE c.public_id = $1 AND c.deleted_at IS NULL
		FOR UPDATE`,
		publicID,
	).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.TrainingCatalog{}, repository.ErrTrainingCatalogNotFound
	}
	if err != nil {
		return entity.TrainingCatalog{}, fmt.Errorf("lock training catalog: %w", err)
	}

	merged := applyTrainingCatalogUpdate(locked, update)

	updated, err := scanTrainingCatalog(tx.QueryRowContext(ctx, `
		UPDATE training_catalog c
		SET name = $2,
		    description = $3,
		    pic_phone = $4,
		    category = $5,
		    training_slots = $6,
		    training_status = NULLIF($7, '')::process_status_enum,
		    link = $8,
		    training_date = $9::date,
		    training_period = $10,
		    speaker = $11,
		    updated_at = $12
		WHERE c.public_id = $1 AND c.deleted_at IS NULL
		RETURNING `+trainingCatalogColumns,
		publicID,
		nullString(merged.Name),
		nullString(merged.Description),
		nullString(merged.PicPhone),
		nullString(merged.Category),
		nullInt(merged.TrainingSlots),
		string(merged.TrainingStatus),
		nullString(merged.Link),
		nullTime(merged.TrainingDate),
		nullString(merged.TrainingPeriod),
		nullString(merged.Speaker),
		merged.UpdatedAt,
	).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.TrainingCatalog{}, repository.ErrTrainingCatalogNotFound
	}
	if err != nil {
		return entity.TrainingCatalog{}, fmt.Errorf("update training catalog: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return entity.TrainingCatalog{}, fmt.Errorf("commit transaction: %w", err)
	}

	return updated, nil
}

func (r trainingCatalogRepository) SoftDeleteTrainingCatalog(
	ctx context.Context,
	publicID string,
	deletedAt int64,
) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE training_catalog
		SET deleted_at = $2, updated_at = $2
		WHERE public_id = $1 AND deleted_at IS NULL`,
		publicID,
		deletedAt,
	)
	if err != nil {
		return fmt.Errorf("soft delete training catalog: %w", err)
	}
	if err := requireTrainingCatalogAffected(result); err != nil {
		return err
	}

	return nil
}

// applyTrainingCatalogUpdate merges an optional field update into the locked
// row. A nil pointer keeps the current value and an empty string clears a
// nullable string.
func applyTrainingCatalogUpdate(
	locked entity.TrainingCatalog,
	update entity.TrainingCatalogUpdate,
) entity.TrainingCatalog {
	merged := locked
	merged.UpdatedAt = update.UpdatedAt

	if update.Name != nil {
		merged.Name = *update.Name
	}
	if update.Description != nil {
		merged.Description = *update.Description
	}
	if update.PicPhone != nil {
		merged.PicPhone = *update.PicPhone
	}
	if update.Category != nil {
		merged.Category = *update.Category
	}
	if update.TrainingSlots != nil {
		merged.TrainingSlots = update.TrainingSlots
	}
	if update.TrainingStatus != nil {
		merged.TrainingStatus = *update.TrainingStatus
	}
	if update.Link != nil {
		merged.Link = *update.Link
	}
	if update.TrainingDate != nil {
		merged.TrainingDate = update.TrainingDate
	}
	if update.TrainingPeriod != nil {
		merged.TrainingPeriod = *update.TrainingPeriod
	}
	if update.Speaker != nil {
		merged.Speaker = *update.Speaker
	}

	return merged
}

// scanTrainingCatalog maps one catalog row. scan is sql.Row.Scan or
// sql.Rows.Scan.
func scanTrainingCatalog(scan func(dest ...any) error) (entity.TrainingCatalog, error) {
	var (
		catalog                               entity.TrainingCatalog
		name, description, picPhone, category sql.NullString
		trainingSlots                         sql.NullInt64
		trainingStatus, link                  sql.NullString
		trainingDate                          sql.NullTime
		trainingPeriod, speaker               sql.NullString
		deletedAt                             sql.NullInt64
	)

	err := scan(
		&catalog.ID,
		&catalog.PublicID,
		&name,
		&description,
		&picPhone,
		&category,
		&trainingSlots,
		&trainingStatus,
		&link,
		&trainingDate,
		&trainingPeriod,
		&speaker,
		&catalog.CreatedAt,
		&catalog.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return entity.TrainingCatalog{}, err
	}

	catalog.Name = name.String
	catalog.Description = description.String
	catalog.PicPhone = picPhone.String
	catalog.Category = category.String
	if trainingSlots.Valid {
		slots := int(trainingSlots.Int64)
		catalog.TrainingSlots = &slots
	}
	catalog.TrainingStatus = entity.ProcessStatus(trainingStatus.String)
	catalog.Link = link.String
	if trainingDate.Valid {
		catalog.TrainingDate = &trainingDate.Time
	}
	catalog.TrainingPeriod = trainingPeriod.String
	catalog.Speaker = speaker.String
	if deletedAt.Valid {
		catalog.DeletedAt = &deletedAt.Int64
	}

	return catalog, nil
}

// mapTrainingCatalogInsertError translates a unique public id violation into a
// repository sentinel error so the usecase can retry generation.
func mapTrainingCatalogInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation &&
		strings.Contains(pgErr.ConstraintName, "public_id") {
		return repository.ErrDuplicateTrainingCatalogPublicID
	}

	return fmt.Errorf("insert training catalog: %w", err)
}

// requireTrainingCatalogAffected turns an update with no matching active row
// into ErrTrainingCatalogNotFound.
func requireTrainingCatalogAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return repository.ErrTrainingCatalogNotFound
	}

	return nil
}

// nullInt returns nil for a nil pointer so an unset capacity is stored as NULL.
func nullInt(value *int) any {
	if value == nil {
		return nil
	}

	return *value
}

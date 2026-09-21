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
	c.id, c.public_id, c.title, c.description, c.pic_phone, c.category,
	c.max_slots, c.training_status, c.link, c.address, c.thumbnail, c.start_date,
	c.end_date, c.mentor, c.created_at, c.updated_at, c.deleted_at, c.registered_count`

// findTrainingCatalogsQuery applies optional filters.
const findTrainingCatalogsQuery = `
	SELECT ` + trainingCatalogColumns + `
	FROM training_catalog c
	WHERE c.deleted_at IS NULL
	  AND ($1 = '' OR (COALESCE(c.title, '') ILIKE '%' || $1 || '%' OR COALESCE(c.mentor, '') ILIKE '%' || $1 || '%'))
	  AND ($2 = '' OR COALESCE(c.title, '') ILIKE '%' || $2 || '%')
	  AND ($3 = '' OR COALESCE(c.mentor, '') ILIKE '%' || $3 || '%')
	  AND ($4 = '' OR c.category = NULLIF($4, '')::training_category_enum)
	  AND ($5 = '' OR c.training_status = NULLIF($5, '')::process_status_enum)
	  AND ($6::date IS NULL OR c.start_date = $6::date)
	  AND ($7::int = 0 OR c.id > $7)
	ORDER BY c.id ASC
	LIMIT $8`

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
			public_id, title, description, pic_phone, category, max_slots,
			training_status, link, address, thumbnail, start_date, end_date, mentor,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::training_category_enum, $6,
			NULLIF($7, '')::process_status_enum, $8, $9, $10, $11::date, $12::date, $13, $14, $15)
		RETURNING `+trainingCatalogColumns,
		catalog.PublicID,
		nullString(catalog.Title),
		nullString(catalog.Description),
		nullString(catalog.PicPhone),
		nullString(string(catalog.Category)),
		nullInt(catalog.MaxSlots),
		string(catalog.TrainingStatus),
		nullString(catalog.Link),
		nullString(catalog.Address),
		nullString(catalog.Thumbnail),
		nullTime(catalog.StartDate),
		nullTime(catalog.EndDate),
		nullString(catalog.Mentor),
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
		filter.Search,
		filter.Title,
		filter.Mentor,
		string(filter.Category),
		string(filter.TrainingStatus),
		nullTime(filter.StartDate),
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
	if merged.StartDate != nil && merged.EndDate != nil && merged.EndDate.Before(*merged.StartDate) {
		return entity.TrainingCatalog{}, repository.ErrInvalidTrainingCatalogDateRange
	}

	updated, err := scanTrainingCatalog(tx.QueryRowContext(ctx, `
		UPDATE training_catalog c
		SET title = $2,
		    description = $3,
		    pic_phone = $4,
		    category = NULLIF($5, '')::training_category_enum,
		    max_slots = $6,
		    training_status = NULLIF($7, '')::process_status_enum,
		    link = $8,
		    address = $9,
		    thumbnail = $10,
		    start_date = $11::date,
		    end_date = $12::date,
		    mentor = $13,
		    updated_at = $14
		WHERE c.public_id = $1 AND c.deleted_at IS NULL
		RETURNING `+trainingCatalogColumns,
		publicID,
		nullString(merged.Title),
		nullString(merged.Description),
		nullString(merged.PicPhone),
		nullString(string(merged.Category)),
		nullInt(merged.MaxSlots),
		string(merged.TrainingStatus),
		nullString(merged.Link),
		nullString(merged.Address),
		nullString(merged.Thumbnail),
		nullTime(merged.StartDate),
		nullTime(merged.EndDate),
		nullString(merged.Mentor),
		merged.UpdatedAt,
	).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.TrainingCatalog{}, repository.ErrTrainingCatalogNotFound
	}
	if err != nil {
		return entity.TrainingCatalog{}, fmt.Errorf("update training catalog: %w", err)
	}

	updated.RegisteredCount = locked.RegisteredCount

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

func applyTrainingCatalogUpdate(
	locked entity.TrainingCatalog,
	update entity.TrainingCatalogUpdate,
) entity.TrainingCatalog {
	merged := locked
	merged.UpdatedAt = update.UpdatedAt

	if update.Title != nil {
		merged.Title = *update.Title
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
	if update.MaxSlots != nil {
		merged.MaxSlots = update.MaxSlots
	}
	if update.TrainingStatus != nil {
		merged.TrainingStatus = *update.TrainingStatus
	}
	if update.Link != nil {
		merged.Link = *update.Link
	}
	if update.Address != nil {
		merged.Address = *update.Address
	}
	if update.Thumbnail != nil {
		merged.Thumbnail = *update.Thumbnail
	}
	if update.StartDate != nil {
		merged.StartDate = update.StartDate
	}
	if update.EndDate != nil {
		merged.EndDate = update.EndDate
	}
	if update.Mentor != nil {
		merged.Mentor = *update.Mentor
	}

	return merged
}

type trainingCatalogRow struct {
	catalog                      entity.TrainingCatalog
	title, description, picPhone sql.NullString
	category                     sql.NullString
	maxSlots                     sql.NullInt64
	trainingStatus, link         sql.NullString
	address, thumbnail           sql.NullString
	startDate, endDate           sql.NullTime
	mentor                       sql.NullString
	deletedAt                    sql.NullInt64
	registeredCount              sql.NullInt64
}

func (row *trainingCatalogRow) scanDest() []any {
	return []any{
		&row.catalog.ID,
		&row.catalog.PublicID,
		&row.title,
		&row.description,
		&row.picPhone,
		&row.category,
		&row.maxSlots,
		&row.trainingStatus,
		&row.link,
		&row.address,
		&row.thumbnail,
		&row.startDate,
		&row.endDate,
		&row.mentor,
		&row.catalog.CreatedAt,
		&row.catalog.UpdatedAt,
		&row.deletedAt,
		&row.registeredCount,
	}
}

func (row *trainingCatalogRow) toCatalog() entity.TrainingCatalog {
	catalog := row.catalog
	catalog.Title = row.title.String
	catalog.Description = row.description.String
	catalog.PicPhone = row.picPhone.String
	catalog.Category = entity.TrainingCategory(row.category.String)
	if row.maxSlots.Valid {
		slots := int(row.maxSlots.Int64)
		catalog.MaxSlots = &slots
	}
	catalog.TrainingStatus = entity.ProcessStatus(row.trainingStatus.String)
	catalog.Link = row.link.String
	catalog.Address = row.address.String
	catalog.Thumbnail = row.thumbnail.String
	if row.startDate.Valid {
		catalog.StartDate = &row.startDate.Time
	}
	if row.endDate.Valid {
		catalog.EndDate = &row.endDate.Time
	}
	catalog.Mentor = row.mentor.String
	if row.deletedAt.Valid {
		catalog.DeletedAt = &row.deletedAt.Int64
	}
	if row.registeredCount.Valid {
		catalog.RegisteredCount = int(row.registeredCount.Int64)
	}

	return catalog
}

func scanTrainingCatalog(scan func(dest ...any) error) (entity.TrainingCatalog, error) {
	var row trainingCatalogRow
	if err := scan(row.scanDest()...); err != nil {
		return entity.TrainingCatalog{}, err
	}

	return row.toCatalog(), nil
}

func mapTrainingCatalogInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation &&
		strings.Contains(pgErr.ConstraintName, "public_id") {
		return repository.ErrDuplicateTrainingCatalogPublicID
	}

	return fmt.Errorf("insert training catalog: %w", err)
}

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

func nullInt(value *int) any {
	if value == nil {
		return nil
	}

	return *value
}

package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

const enterpriseColumns = `
	e.id, e.public_id, e.user_id, e.name, e.business_sector,
	e.legal_status, e.business_digitization, e.intervention_needs,
	e.training_status, e.mentoring_status, e.capital_access, e.partnership,
	e.initial_turnover, e.current_turnover, e.district, e.status,
	e.created_at, e.updated_at, e.deleted_at`

// findEnterprisesQuery applies the owner scope plus optional enum and district
// filters. Enum parameters are wrapped in NULLIF so an empty filter binds NULL
// instead of failing an enum cast.
const findEnterprisesQuery = `
	SELECT ` + enterpriseColumns + `
	FROM enterprises e
	WHERE e.deleted_at IS NULL
	  AND ($1::int = 0 OR e.user_id = $1)
	  AND ($2 = '' OR e.district = $2)
	  AND ($3 = '' OR e.status = NULLIF($3, '')::enterprise_status_enum)
	  AND ($4 = '' OR e.business_sector = NULLIF($4, '')::business_sector_enum)
	  AND ($5 = '' OR e.legal_status = NULLIF($5, '')::legal_status_enum)
	  AND ($6 = '' OR e.business_digitization = NULLIF($6, '')::business_digitization_enum)
	  AND ($7 = '' OR e.intervention_needs = NULLIF($7, '')::intervention_needs_enum)
	  AND ($8 = '' OR e.training_status = NULLIF($8, '')::process_status_enum)
	  AND ($9 = '' OR e.mentoring_status = NULLIF($9, '')::process_status_enum)
	  AND ($10 = '' OR e.capital_access = NULLIF($10, '')::general_status_enum)
	  AND ($11 = '' OR e.partnership = NULLIF($11, '')::general_status_enum)
	  AND ($12::int = 0 OR e.id > $12)
	ORDER BY e.id ASC
	LIMIT $13`

type enterpriseRepository struct {
	db *sql.DB
}

// NewEnterpriseRepository creates a PostgreSQL-backed enterprise repository.
func NewEnterpriseRepository(db *sql.DB) repository.EnterpriseRepository {
	return enterpriseRepository{db: db}
}

func (r enterpriseRepository) CreateEnterprise(
	ctx context.Context,
	enterprise entity.Enterprise,
	event entity.EnterpriseAuditEvent,
) (entity.Enterprise, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Enterprise{}, fmt.Errorf("begin transaction: %w", err)
	}
	// Rollback after a successful commit reports ErrTxDone, which is expected.
	defer func() { _ = tx.Rollback() }()

	created, err := scanEnterprise(tx.QueryRowContext(ctx, `
		INSERT INTO enterprises AS e (
			public_id, user_id, name, business_sector, legal_status,
			business_digitization, intervention_needs, training_status,
			mentoring_status, capital_access, partnership,
			initial_turnover, current_turnover, district, status,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4::text::business_sector_enum,
			NULLIF($5, '')::legal_status_enum,
			NULLIF($6, '')::business_digitization_enum,
			NULLIF($7, '')::intervention_needs_enum,
			NULLIF($8, '')::process_status_enum,
			NULLIF($9, '')::process_status_enum,
			NULLIF($10, '')::general_status_enum,
			NULLIF($11, '')::general_status_enum,
			$12::text::numeric, $13::text::numeric, $14,
			$15::text::enterprise_status_enum, $16, $17)
		RETURNING `+enterpriseColumns,
		enterprise.PublicID,
		enterprise.UserID,
		nullString(enterprise.Name),
		string(enterprise.BusinessSector),
		string(enterprise.LegalStatus),
		string(enterprise.BusinessDigitization),
		string(enterprise.InterventionNeeds),
		string(enterprise.TrainingStatus),
		string(enterprise.MentoringStatus),
		string(enterprise.CapitalAccess),
		string(enterprise.Partnership),
		enterprise.InitialTurnover,
		enterprise.CurrentTurnover,
		nullString(enterprise.District),
		string(enterprise.Status),
		enterprise.CreatedAt,
		enterprise.UpdatedAt,
	).Scan)
	if err != nil {
		return entity.Enterprise{}, mapEnterpriseInsertError(err)
	}

	if err := insertEnterpriseAuditEvent(ctx, tx, created.ID, event); err != nil {
		return entity.Enterprise{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Enterprise{}, fmt.Errorf("commit transaction: %w", err)
	}

	return created, nil
}

func (r enterpriseRepository) FindEnterpriseByPublicID(
	ctx context.Context,
	publicID string,
	ownerID int,
) (entity.Enterprise, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+enterpriseColumns+`
		FROM enterprises e
		WHERE e.public_id = $1 AND e.deleted_at IS NULL
		  AND ($2::int = 0 OR e.user_id = $2)`,
		publicID,
		ownerID,
	)

	enterprise, err := scanEnterprise(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Enterprise{}, repository.ErrEnterpriseNotFound
	}
	if err != nil {
		return entity.Enterprise{}, fmt.Errorf("find enterprise: %w", err)
	}

	return enterprise, nil
}

func (r enterpriseRepository) FindEnterprises(
	ctx context.Context,
	filter entity.EnterpriseFilter,
) ([]entity.Enterprise, error) {
	rows, err := r.db.QueryContext(ctx, findEnterprisesQuery,
		filter.OwnerID,
		filter.District,
		string(filter.Status),
		string(filter.BusinessSector),
		string(filter.LegalStatus),
		string(filter.BusinessDigitization),
		string(filter.InterventionNeeds),
		string(filter.TrainingStatus),
		string(filter.MentoringStatus),
		string(filter.CapitalAccess),
		string(filter.Partnership),
		filter.Cursor,
		filter.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query enterprises: %w", err)
	}
	defer func() { _ = rows.Close() }()

	enterprises := []entity.Enterprise{}
	for rows.Next() {
		enterprise, err := scanEnterprise(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan enterprise: %w", err)
		}

		enterprises = append(enterprises, enterprise)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enterprises: %w", err)
	}

	return enterprises, nil
}

func (r enterpriseRepository) UpdateEnterprise(
	ctx context.Context,
	publicID string,
	ownerID int,
	update entity.EnterpriseUpdate,
	event entity.EnterpriseAuditEvent,
) (entity.Enterprise, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Enterprise{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	locked, err := scanEnterprise(tx.QueryRowContext(ctx, `
		SELECT `+enterpriseColumns+`
		FROM enterprises e
		WHERE e.public_id = $1 AND e.deleted_at IS NULL
		  AND ($2::int = 0 OR e.user_id = $2)
		FOR UPDATE`,
		publicID,
		ownerID,
	).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Enterprise{}, repository.ErrEnterpriseNotFound
	}
	if err != nil {
		return entity.Enterprise{}, fmt.Errorf("lock enterprise: %w", err)
	}

	merged, changed := applyEnterpriseUpdate(locked, update)
	if len(changed) == 0 {
		return locked, nil
	}

	updated, err := scanEnterprise(tx.QueryRowContext(ctx, `
		UPDATE enterprises e
		SET name = $3,
		    business_sector = $4::text::business_sector_enum,
		    legal_status = NULLIF($5, '')::legal_status_enum,
		    business_digitization = NULLIF($6, '')::business_digitization_enum,
		    intervention_needs = NULLIF($7, '')::intervention_needs_enum,
		    training_status = NULLIF($8, '')::process_status_enum,
		    mentoring_status = NULLIF($9, '')::process_status_enum,
		    capital_access = NULLIF($10, '')::general_status_enum,
		    partnership = NULLIF($11, '')::general_status_enum,
		    initial_turnover = $12::text::numeric,
		    current_turnover = $13::text::numeric,
		    district = $14,
		    status = $15::text::enterprise_status_enum,
		    updated_at = $16
		WHERE e.public_id = $1 AND e.deleted_at IS NULL
		  AND ($2::int = 0 OR e.user_id = $2)
		RETURNING `+enterpriseColumns,
		publicID,
		ownerID,
		nullString(merged.Name),
		string(merged.BusinessSector),
		string(merged.LegalStatus),
		string(merged.BusinessDigitization),
		string(merged.InterventionNeeds),
		string(merged.TrainingStatus),
		string(merged.MentoringStatus),
		string(merged.CapitalAccess),
		string(merged.Partnership),
		merged.InitialTurnover,
		merged.CurrentTurnover,
		nullString(merged.District),
		string(merged.Status),
		merged.UpdatedAt,
	).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Enterprise{}, repository.ErrEnterpriseNotFound
	}
	if err != nil {
		return entity.Enterprise{}, fmt.Errorf("update enterprise: %w", err)
	}

	event.ChangedFields = changed
	if err := insertEnterpriseAuditEvent(ctx, tx, updated.ID, event); err != nil {
		return entity.Enterprise{}, err
	}

	if err := tx.Commit(); err != nil {
		return entity.Enterprise{}, fmt.Errorf("commit transaction: %w", err)
	}

	return updated, nil
}

func (r enterpriseRepository) SoftDeleteEnterprise(
	ctx context.Context,
	publicID string,
	ownerID int,
	deletedAt int64,
	event entity.EnterpriseAuditEvent,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var id int
	err = tx.QueryRowContext(ctx, `
		UPDATE enterprises e
		SET deleted_at = $3, updated_at = $3
		WHERE e.public_id = $1 AND e.deleted_at IS NULL
		  AND ($2::int = 0 OR e.user_id = $2)
		RETURNING e.id`,
		publicID,
		ownerID,
		deletedAt,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.ErrEnterpriseNotFound
	}
	if err != nil {
		return fmt.Errorf("soft delete enterprise: %w", err)
	}

	if err := insertEnterpriseAuditEvent(ctx, tx, id, event); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// applyEnterpriseUpdate merges an optional field update into the locked row and
// reports the fields that actually changed for the audit event.
func applyEnterpriseUpdate(locked entity.Enterprise, update entity.EnterpriseUpdate) (entity.Enterprise, map[string]any) {
	merged := locked
	merged.UpdatedAt = update.UpdatedAt
	changed := map[string]any{}

	if update.Name != nil {
		if *update.Name != locked.Name {
			changed["name"] = entity.NullableAuditValue(*update.Name)
		}
		merged.Name = *update.Name
	}
	if update.BusinessSector != nil {
		if *update.BusinessSector != locked.BusinessSector {
			changed["business_sector"] = string(*update.BusinessSector)
		}
		merged.BusinessSector = *update.BusinessSector
	}
	if update.LegalStatus != nil {
		if *update.LegalStatus != locked.LegalStatus {
			changed["legal_status"] = entity.NullableAuditValue(string(*update.LegalStatus))
		}
		merged.LegalStatus = *update.LegalStatus
	}
	if update.BusinessDigitization != nil {
		if *update.BusinessDigitization != locked.BusinessDigitization {
			changed["business_digitization"] = entity.NullableAuditValue(string(*update.BusinessDigitization))
		}
		merged.BusinessDigitization = *update.BusinessDigitization
	}
	if update.InterventionNeeds != nil {
		if *update.InterventionNeeds != locked.InterventionNeeds {
			changed["intervention_needs"] = entity.NullableAuditValue(string(*update.InterventionNeeds))
		}
		merged.InterventionNeeds = *update.InterventionNeeds
	}
	if update.TrainingStatus != nil {
		if *update.TrainingStatus != locked.TrainingStatus {
			changed["training_status"] = entity.NullableAuditValue(string(*update.TrainingStatus))
		}
		merged.TrainingStatus = *update.TrainingStatus
	}
	if update.MentoringStatus != nil {
		if *update.MentoringStatus != locked.MentoringStatus {
			changed["mentoring_status"] = entity.NullableAuditValue(string(*update.MentoringStatus))
		}
		merged.MentoringStatus = *update.MentoringStatus
	}
	if update.CapitalAccess != nil {
		if *update.CapitalAccess != locked.CapitalAccess {
			changed["capital_access"] = entity.NullableAuditValue(string(*update.CapitalAccess))
		}
		merged.CapitalAccess = *update.CapitalAccess
	}
	if update.Partnership != nil {
		if *update.Partnership != locked.Partnership {
			changed["partnership"] = entity.NullableAuditValue(string(*update.Partnership))
		}
		merged.Partnership = *update.Partnership
	}
	if update.InitialTurnover != nil {
		if *update.InitialTurnover != locked.InitialTurnover {
			changed["initial_turnover"] = *update.InitialTurnover
		}
		merged.InitialTurnover = *update.InitialTurnover
	}
	if update.CurrentTurnover != nil {
		if *update.CurrentTurnover != locked.CurrentTurnover {
			changed["current_turnover"] = *update.CurrentTurnover
		}
		merged.CurrentTurnover = *update.CurrentTurnover
	}
	if update.District != nil {
		if *update.District != locked.District {
			changed["district"] = entity.NullableAuditValue(*update.District)
		}
		merged.District = *update.District
	}
	if update.Status != nil {
		if *update.Status != locked.Status {
			changed["status"] = string(*update.Status)
		}
		merged.Status = *update.Status
	}

	return merged, changed
}

// insertEnterpriseAuditEvent appends one audit row using the caller's
// transaction so the mutation and its audit event commit or roll back together.
func insertEnterpriseAuditEvent(
	ctx context.Context,
	tx *sql.Tx,
	enterpriseID int,
	event entity.EnterpriseAuditEvent,
) error {
	changedFields := event.ChangedFields
	if changedFields == nil {
		changedFields = map[string]any{}
	}

	encoded, err := json.Marshal(changedFields)
	if err != nil {
		return fmt.Errorf("encode audit changed fields: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO enterprise_audit_events (
			enterprise_id, actor_user_id, action, changed_fields, created_at
		)
		VALUES ($1, $2, $3, $4::text::jsonb, $5)`,
		enterpriseID,
		event.ActorUserID,
		string(event.Action),
		string(encoded),
		event.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert enterprise audit event: %w", err)
	}

	return nil
}

// scanEnterprise maps one enterprise row. scan is sql.Row.Scan or
// sql.Rows.Scan.
func scanEnterprise(scan func(dest ...any) error) (entity.Enterprise, error) {
	var (
		enterprise                       entity.Enterprise
		businessSector, status           string
		name                             sql.NullString
		legalStatus                      sql.NullString
		businessDigitization             sql.NullString
		interventionNeeds                sql.NullString
		trainingStatus, mentoringStatus  sql.NullString
		capitalAccess, partnership       sql.NullString
		initialTurnover, currentTurnover sql.NullString
		district                         sql.NullString
		deletedAt                        sql.NullInt64
	)

	err := scan(
		&enterprise.ID,
		&enterprise.PublicID,
		&enterprise.UserID,
		&name,
		&businessSector,
		&legalStatus,
		&businessDigitization,
		&interventionNeeds,
		&trainingStatus,
		&mentoringStatus,
		&capitalAccess,
		&partnership,
		&initialTurnover,
		&currentTurnover,
		&district,
		&status,
		&enterprise.CreatedAt,
		&enterprise.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return entity.Enterprise{}, err
	}

	enterprise.Name = name.String
	enterprise.BusinessSector = entity.BusinessSector(businessSector)
	enterprise.LegalStatus = entity.LegalStatus(legalStatus.String)
	enterprise.BusinessDigitization = entity.BusinessDigitization(businessDigitization.String)
	enterprise.InterventionNeeds = entity.InterventionNeeds(interventionNeeds.String)
	enterprise.TrainingStatus = entity.ProcessStatus(trainingStatus.String)
	enterprise.MentoringStatus = entity.ProcessStatus(mentoringStatus.String)
	enterprise.CapitalAccess = entity.GeneralStatus(capitalAccess.String)
	enterprise.Partnership = entity.GeneralStatus(partnership.String)
	enterprise.InitialTurnover = initialTurnover.String
	enterprise.CurrentTurnover = currentTurnover.String
	enterprise.District = district.String
	enterprise.Status = entity.EnterpriseStatus(status)
	if deletedAt.Valid {
		enterprise.DeletedAt = &deletedAt.Int64
	}

	return enterprise, nil
}

// mapEnterpriseInsertError translates a unique public id violation into a
// repository sentinel error so the usecase can retry generation.
func mapEnterpriseInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation &&
		strings.Contains(pgErr.ConstraintName, "public_id") {
		return repository.ErrDuplicateEnterprisePublicID
	}

	return fmt.Errorf("insert enterprise: %w", err)
}

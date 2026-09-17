package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

// uniqueViolation is the PostgreSQL SQLSTATE code for a unique constraint
// violation.
const uniqueViolation = "23505"

const userColumns = `
	u.id, u.public_id, u.email, u.password, u.role, u.is_active,
	u.created_at, u.updated_at, u.deleted_at,
	p.id, p.full_name, p.nik, p.birth_date, p.gender, p.district, p.phone, p.address,
	p.created_at, p.updated_at, p.deleted_at`

const findUsersQuery = `
	SELECT ` + userColumns + `
	FROM users u
	LEFT JOIN user_profiles p ON p.user_id = u.id
	WHERE u.deleted_at IS NULL
	  AND ($1 = '' OR p.district = $1)
	  AND ($2 = '' OR p.gender = $2)
	  AND ($3::int = 0 OR u.id > $3)
	ORDER BY u.id ASC
	LIMIT $4`

type userRepository struct {
	db *sql.DB
}

// NewUserRepository creates a PostgreSQL-backed user repository.
func NewUserRepository(db *sql.DB) repository.UserRepository {
	return userRepository{db: db}
}

func (r userRepository) CreateUser(ctx context.Context, user entity.User) (entity.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.User{}, fmt.Errorf("begin transaction: %w", err)
	}
	// Rollback after a successful commit reports ErrTxDone, which is expected.
	defer func() { _ = tx.Rollback() }()

	var id int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (public_id, email, password, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		user.PublicID,
		user.Email,
		user.Password,
		string(user.Role),
		user.IsActive,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(&id)
	if err != nil {
		return entity.User{}, mapInsertError(err)
	}

	profile := entity.Profile{}
	if user.Profile != nil {
		profile = *user.Profile
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_profiles (user_id, full_name, nik, birth_date, gender, district, phone, address, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		id,
		profile.FullName,
		nullString(profile.NIK),
		nullTime(profile.BirthDate),
		nullString(string(profile.Gender)),
		nullString(profile.District),
		nullString(profile.Phone),
		nullString(profile.Address),
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return entity.User{}, fmt.Errorf("insert user profile: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return entity.User{}, fmt.Errorf("commit transaction: %w", err)
	}

	return r.FindUserByPublicID(ctx, user.PublicID)
}

func (r userRepository) FindUserByPublicID(ctx context.Context, publicID string) (entity.User, error) {
	query := `
		SELECT ` + userColumns + `
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.public_id = $1 AND u.deleted_at IS NULL`

	user, err := scanUser(r.db.QueryRowContext(ctx, query, publicID).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.User{}, repository.ErrUserNotFound
	}
	if err != nil {
		return entity.User{}, fmt.Errorf("find user by public id: %w", err)
	}

	return user, nil
}

func (r userRepository) FindUsers(ctx context.Context, filter entity.UserFilter) ([]entity.User, error) {
	rows, err := r.db.QueryContext(ctx, findUsersQuery,
		filter.District,
		string(filter.Gender),
		filter.Cursor,
		filter.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	users := []entity.User{}
	for rows.Next() {
		user, err := scanUser(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}

		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}

func (r userRepository) SoftDeleteUser(ctx context.Context, publicID string, deletedAt int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE users
		SET deleted_at = $2, updated_at = $2
		WHERE public_id = $1 AND deleted_at IS NULL`,
		publicID,
		deletedAt,
	)
	if err != nil {
		return fmt.Errorf("soft delete user: %w", err)
	}
	if err := requireAffected(result); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE user_profiles
		SET deleted_at = $2, updated_at = $2
		FROM users
		WHERE user_profiles.user_id = users.id
		  AND users.public_id = $1`,
		publicID,
		deletedAt,
	); err != nil {
		return fmt.Errorf("soft delete user profile: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r userRepository) UpdateProfile(
	ctx context.Context,
	publicID string,
	profile entity.Profile,
	updatedAt int64,
) (entity.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.User{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE user_profiles
		SET full_name = $2,
		    nik = $3,
		    birth_date = $4,
		    gender = $5,
		    district = $6,
		    phone = $7,
		    address = $8,
		    updated_at = $9
		FROM users
		WHERE user_profiles.user_id = users.id
		  AND users.public_id = $1
		  AND users.deleted_at IS NULL`,
		publicID,
		profile.FullName,
		nullString(profile.NIK),
		nullTime(profile.BirthDate),
		nullString(string(profile.Gender)),
		nullString(profile.District),
		nullString(profile.Phone),
		nullString(profile.Address),
		updatedAt,
	)
	if err != nil {
		return entity.User{}, fmt.Errorf("update user profile: %w", err)
	}
	if err := requireAffected(result); err != nil {
		return entity.User{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET updated_at = $2
		WHERE public_id = $1 AND deleted_at IS NULL`,
		publicID,
		updatedAt,
	); err != nil {
		return entity.User{}, fmt.Errorf("touch user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return entity.User{}, fmt.Errorf("commit transaction: %w", err)
	}

	return r.FindUserByPublicID(ctx, publicID)
}

func (r userRepository) UpdateStatus(
	ctx context.Context,
	publicID string,
	isActive bool,
	updatedAt int64,
) (entity.User, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET is_active = $2, updated_at = $3
		WHERE public_id = $1 AND deleted_at IS NULL`,
		publicID,
		isActive,
		updatedAt,
	)
	if err != nil {
		return entity.User{}, fmt.Errorf("update user status: %w", err)
	}
	if err := requireAffected(result); err != nil {
		return entity.User{}, err
	}

	return r.FindUserByPublicID(ctx, publicID)
}

func (r userRepository) UpdatePassword(
	ctx context.Context,
	publicID string,
	passwordHash string,
	updatedAt int64,
) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET password = $2, updated_at = $3
		WHERE public_id = $1 AND deleted_at IS NULL`,
		publicID,
		passwordHash,
		updatedAt,
	)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}

	return requireAffected(result)
}

func (r userRepository) ChangePassword(
	ctx context.Context,
	publicID string,
	currentHash string,
	newPasswordHash string,
	updatedAt int64,
) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET password = $3, updated_at = $4
		WHERE public_id = $1 AND password = $2 AND deleted_at IS NULL`,
		publicID,
		currentHash,
		newPasswordHash,
		updatedAt,
	)
	if err != nil {
		return fmt.Errorf("change user password: %w", err)
	}

	return requireAffected(result)
}

// scanUser maps one joined user/profile row. scan is sql.Row.Scan or
// sql.Rows.Scan.
func scanUser(scan func(dest ...any) error) (entity.User, error) {
	var (
		user                                                 entity.User
		role                                                 string
		deletedAt                                            sql.NullInt64
		profileID                                            sql.NullInt64
		fullName, nik, gender, district, phone, address      sql.NullString
		birthDate                                            sql.NullTime
		profileCreatedAt, profileUpdatedAt, profileDeletedAt sql.NullInt64
	)

	err := scan(
		&user.ID,
		&user.PublicID,
		&user.Email,
		&user.Password,
		&role,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
		&deletedAt,
		&profileID,
		&fullName,
		&nik,
		&birthDate,
		&gender,
		&district,
		&phone,
		&address,
		&profileCreatedAt,
		&profileUpdatedAt,
		&profileDeletedAt,
	)
	if err != nil {
		return entity.User{}, err
	}

	user.Role = entity.Role(role)
	if deletedAt.Valid {
		user.DeletedAt = &deletedAt.Int64
	}
	if profileID.Valid {
		profile := entity.Profile{
			FullName: fullName.String,
			NIK:      nik.String,
			Gender:   entity.Gender(gender.String),
			District: district.String,
			Phone:    phone.String,
			Address:  address.String,
		}
		if birthDate.Valid {
			profile.BirthDate = &birthDate.Time
		}
		user.Profile = &profile
	}

	return user, nil
}

// mapInsertError translates unique constraint violations into repository
// sentinel errors so usecases can react to duplicates.
func mapInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		switch {
		case strings.Contains(pgErr.ConstraintName, "email"):
			return repository.ErrDuplicateEmail
		case strings.Contains(pgErr.ConstraintName, "public_id"):
			return repository.ErrDuplicatePublicID
		}
	}

	return fmt.Errorf("insert user: %w", err)
}

// requireAffected turns an update with no matching active row into
// ErrUserNotFound.
func requireAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return repository.ErrUserNotFound
	}

	return nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}

	return *value
}

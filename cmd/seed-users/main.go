// Command seed-users seeds member users and their enterprises from a CSV file directly into PostgreSQL.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/tnnz20/youthpreneur-be/internal/config"
	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/repository/persistence"
	"github.com/tnnz20/youthpreneur-be/internal/sshtunnel"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

const (
	defaultCSVPath   = "db/seeds/external/rekap_wirausaha_dan_user.csv"
	publicIDAttempts = 5
)

type userRepo interface {
	CreateUser(ctx context.Context, user entity.User) (entity.User, error)
	FindUserByEmail(ctx context.Context, email string) (entity.User, error)
}

type enterpriseRepo interface {
	CreateEnterprise(ctx context.Context, enterprise entity.Enterprise, event entity.EnterpriseAuditEvent) (entity.Enterprise, error)
}

func main() {
	filePath := flag.String("file", defaultCSVPath, "Path to user & enterprise CSV file")
	useSSH := flag.Bool("ssh", false, "Use SSH tunnel to connect to PostgreSQL")
	flag.Parse()

	if err := run(context.Background(), *filePath, *useSSH, os.Stdout, time.Now); err != nil {
		fmt.Fprintln(os.Stderr, "seed-users error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, filePath string, useSSH bool, out io.Writer, now func() time.Time) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open csv file %q: %w", filePath, err)
	}
	defer func() { _ = file.Close() }()

	records, err := parseUserEnterpriseCSV(file)
	if err != nil {
		return fmt.Errorf("parse csv: %w", err)
	}

	if len(records) == 0 {
		return errors.New("csv file contains no records to seed")
	}

	postgresCfg, closer, err := sshtunnel.ResolvePostgres(ctx, useSSH)
	if err != nil {
		return err
	}
	defer func() { _ = closer.Close() }()

	db, err := config.OpenPostgres(ctx, postgresCfg)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer func() { _ = db.Close() }()

	uRepo := persistence.NewUserRepository(db)
	eRepo := persistence.NewEnterpriseRepository(db)

	return seedUserEnterprises(ctx, uRepo, eRepo, records, out, now)
}

func seedUserEnterprises(
	ctx context.Context,
	uRepo userRepo,
	eRepo enterpriseRepo,
	records []UserEnterpriseRecord,
	out io.Writer,
	now func() time.Time,
) error {
	currentTime := now().Unix()
	seededCount := 0
	skippedCount := 0

	for i, rec := range records {
		existingUser, err := uRepo.FindUserByEmail(ctx, rec.Email)
		if err == nil {
			fmt.Fprintf(out, "[%d/%d] Skipped existing user: %s (public_id=%s)\n",
				i+1, len(records), existingUser.Email, existingUser.PublicID)
			skippedCount++
			continue
		}
		if !errors.Is(err, repository.ErrUserNotFound) {
			return fmt.Errorf("check existing user %q: %w", rec.Email, err)
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(rec.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password for %q: %w", rec.Email, err)
		}

		var createdUser entity.User
		userCreated := false

		for range publicIDAttempts {
			userPublicID, err := usecase.GeneratePublicID()
			if err != nil {
				return fmt.Errorf("generate user public id: %w", err)
			}

			profile := entity.Profile{
				FullName: rec.FullName,
				District: rec.District,
				Address:  rec.Address,
			}

			userToCreate := entity.User{
				PublicID:  userPublicID,
				Email:     rec.Email,
				Password:  string(passwordHash),
				Role:      entity.RoleMember,
				IsActive:  true,
				CreatedAt: currentTime,
				UpdatedAt: currentTime,
				Profile:   &profile,
			}

			user, err := uRepo.CreateUser(ctx, userToCreate)
			switch {
			case errors.Is(err, repository.ErrDuplicatePublicID):
				continue
			case errors.Is(err, repository.ErrDuplicateEmail):
				fmt.Fprintf(out, "[%d/%d] Skipped duplicate email during creation: %s\n", i+1, len(records), rec.Email)
				userCreated = false
				break
			case err != nil:
				return fmt.Errorf("create user %q: %w", rec.Email, err)
			default:
				createdUser = user
				userCreated = true
			}
			break
		}

		if !userCreated {
			skippedCount++
			continue
		}

		enterpriseCreated := false
		var createdEnterprise entity.Enterprise

		for range publicIDAttempts {
			enterprisePublicID, err := usecase.GenerateEnterprisePublicID()
			if err != nil {
				return fmt.Errorf("generate enterprise public id: %w", err)
			}

			enterpriseToCreate := entity.Enterprise{
				PublicID:             enterprisePublicID,
				UserID:               createdUser.ID,
				EnterpriseName:       rec.EnterpriseName,
				Description:          rec.Description,
				Address:              rec.Address,
				BusinessSector:       rec.BusinessSector,
				LegalStatus:          rec.LegalStatus,
				BusinessDigitization: rec.BusinessDigitization,
				InterventionNeeds:    rec.InterventionNeeds,
				TrainingStatus:       rec.TrainingStatus,
				MentoringStatus:      rec.MentoringStatus,
				CapitalAccess:        rec.CapitalAccess,
				Partnership:          rec.Partnership,
				InitialTurnover:      rec.InitialTurnover,
				CurrentTurnover:      rec.CurrentTurnover,
				District:             rec.District,
				Status:               rec.Status,
				CreatedAt:            currentTime,
				UpdatedAt:            currentTime,
			}

			auditEvent := entity.EnterpriseAuditEvent{
				ActorUserID: createdUser.ID,
				Action:      entity.AuditActionCreate,
				ChangedFields: map[string]any{
					"enterprise_name":       rec.EnterpriseName,
					"description":           entity.NullableAuditValue(rec.Description),
					"address":               entity.NullableAuditValue(rec.Address),
					"business_sector":       string(rec.BusinessSector),
					"legal_status":          entity.NullableAuditValue(string(rec.LegalStatus)),
					"business_digitization": entity.NullableAuditValue(string(rec.BusinessDigitization)),
					"intervention_needs":    entity.NullableAuditValue(string(rec.InterventionNeeds)),
					"training_status":       entity.NullableAuditValue(string(rec.TrainingStatus)),
					"mentoring_status":      entity.NullableAuditValue(string(rec.MentoringStatus)),
					"capital_access":        entity.NullableAuditValue(string(rec.CapitalAccess)),
					"partnership":           entity.NullableAuditValue(string(rec.Partnership)),
					"initial_turnover":      rec.InitialTurnover,
					"current_turnover":      rec.CurrentTurnover,
					"district":              entity.NullableAuditValue(rec.District),
					"status":                string(rec.Status),
				},
				CreatedAt: currentTime,
			}

			ent, err := eRepo.CreateEnterprise(ctx, enterpriseToCreate, auditEvent)
			if errors.Is(err, repository.ErrDuplicateEnterprisePublicID) {
				continue
			}
			if err != nil {
				return fmt.Errorf("create enterprise for %q: %w", rec.Email, err)
			}

			createdEnterprise = ent
			enterpriseCreated = true
			break
		}

		if !enterpriseCreated {
			return fmt.Errorf("failed to allocate enterprise public id for %q: attempts exhausted", rec.EnterpriseName)
		}

		fmt.Fprintf(out, "[%d/%d] Seeded: user=%s (%s), enterprise=%s (%s)\n",
			i+1, len(records), createdUser.PublicID, createdUser.Email, createdEnterprise.PublicID, createdEnterprise.EnterpriseName)
		seededCount++
	}

	fmt.Fprintf(out, "Seeding complete: %d seeded, %d skipped, %d total.\n", seededCount, skippedCount, len(records))
	return nil
}

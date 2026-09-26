// Command seed-catalog seeds training catalog offerings from a CSV file directly into PostgreSQL.
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/tnnz20/youthpreneur-be/internal/config"
	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/repository/persistence"
	"github.com/tnnz20/youthpreneur-be/internal/sshtunnel"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

const (
	defaultCSVPath   = "db/seeds/training_catalogs.csv"
	publicIDAttempts = 5
	dateFormat       = "2006-01-02"
)

type catalogCreator interface {
	CreateTrainingCatalog(ctx context.Context, catalog entity.TrainingCatalog) (entity.TrainingCatalog, error)
}

func main() {
	filePath := flag.String("file", defaultCSVPath, "Path to training catalog CSV file")
	useSSH := flag.Bool("ssh", false, "Use SSH tunnel to connect to PostgreSQL")
	flag.Parse()

	if err := run(context.Background(), *filePath, *useSSH, os.Stdout, time.Now); err != nil {
		fmt.Fprintln(os.Stderr, "seed-catalog error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, filePath string, useSSH bool, out io.Writer, now func() time.Time) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open csv file %q: %w", filePath, err)
	}
	defer func() { _ = file.Close() }()

	catalogs, err := parseCSV(file, now)
	if err != nil {
		return fmt.Errorf("parse csv: %w", err)
	}

	if len(catalogs) == 0 {
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

	repo := persistence.NewTrainingCatalogRepository(db)
	return seedCatalogs(ctx, repo, catalogs, out)
}

func parseCSV(r io.Reader, now func() time.Time) ([]entity.TrainingCatalog, error) {
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 2 {
		return nil, errors.New("csv must contain header and at least one row")
	}

	header := records[0]
	colIndex := make(map[string]int, len(header))
	for i, h := range header {
		colIndex[strings.TrimSpace(strings.ToLower(h))] = i
	}

	requiredCols := []string{"title", "category", "training_status"}
	for _, col := range requiredCols {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	currentTime := now().Unix()
	catalogs := make([]entity.TrainingCatalog, 0, len(records)-1)

	getCol := func(row []string, colName string) string {
		idx, ok := colIndex[colName]
		if !ok || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	for lineNum, row := range records[1:] {
		title := getCol(row, "title")
		if title == "" {
			return nil, fmt.Errorf("row %d: title is required", lineNum+2)
		}

		catStr := getCol(row, "category")
		cat := entity.TrainingCategory(catStr)
		if !entity.ValidTrainingCategory(cat) {
			return nil, fmt.Errorf("row %d: invalid category %q", lineNum+2, catStr)
		}

		statusStr := getCol(row, "training_status")
		status := entity.ProcessStatus(statusStr)
		if status != entity.ProcessStatusPlanned && status != entity.ProcessStatusOngoing && status != entity.ProcessStatusCompleted {
			return nil, fmt.Errorf("row %d: invalid training_status %q", lineNum+2, statusStr)
		}

		var maxSlots *int
		if slotsStr := getCol(row, "max_slots"); slotsStr != "" {
			s, err := strconv.Atoi(slotsStr)
			if err != nil || s <= 0 {
				return nil, fmt.Errorf("row %d: invalid max_slots %q (must be positive integer)", lineNum+2, slotsStr)
			}
			maxSlots = &s
		}

		var startDate *time.Time
		if dStr := getCol(row, "start_date"); dStr != "" {
			d, err := time.Parse(dateFormat, dStr)
			if err != nil {
				return nil, fmt.Errorf("row %d: invalid start_date %q", lineNum+2, dStr)
			}
			startDate = &d
		}

		var endDate *time.Time
		if dStr := getCol(row, "end_date"); dStr != "" {
			d, err := time.Parse(dateFormat, dStr)
			if err != nil {
				return nil, fmt.Errorf("row %d: invalid end_date %q", lineNum+2, dStr)
			}
			endDate = &d
		}

		if startDate != nil && endDate != nil && endDate.Before(*startDate) {
			return nil, fmt.Errorf("row %d: end_date must be on or after start_date", lineNum+2)
		}

		catalogs = append(catalogs, entity.TrainingCatalog{
			Title:          title,
			Description:    getCol(row, "description"),
			PicPhone:       getCol(row, "pic_phone"),
			Category:       cat,
			MaxSlots:       maxSlots,
			TrainingStatus: status,
			Link:           getCol(row, "link"),
			Address:        getCol(row, "address"),
			Thumbnail:      getCol(row, "thumbnail"),
			StartDate:      startDate,
			EndDate:        endDate,
			Mentor:         getCol(row, "mentor"),
			CreatedAt:      currentTime,
			UpdatedAt:      currentTime,
		})
	}

	return catalogs, nil
}

func seedCatalogs(ctx context.Context, repo catalogCreator, catalogs []entity.TrainingCatalog, out io.Writer) error {
	for i, cat := range catalogs {
		seeded := false
		for range publicIDAttempts {
			publicID, err := usecase.GenerateTrainingCatalogPublicID()
			if err != nil {
				return fmt.Errorf("generate catalog public id: %w", err)
			}
			cat.PublicID = publicID

			created, err := repo.CreateTrainingCatalog(ctx, cat)
			if errors.Is(err, repository.ErrDuplicateTrainingCatalogPublicID) {
				continue
			}
			if err != nil {
				return fmt.Errorf("insert catalog %q: %w", cat.Title, err)
			}

			fmt.Fprintf(out, "[%d/%d] Seeded: public_id=%s, title=%q, category=%q, status=%s\n",
				i+1, len(catalogs), created.PublicID, created.Title, created.Category, created.TrainingStatus)
			seeded = true
			break
		}
		if !seeded {
			return fmt.Errorf("failed to allocate public id for %q: attempts exhausted", cat.Title)
		}
	}

	fmt.Fprintf(out, "Successfully seeded %d training catalogs.\n", len(catalogs))
	return nil
}

package main

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

var publicIDPattern = regexp.MustCompile(`^TCY-[0-9]{6}$`)

type fakeCatalogCreator struct {
	calls        int
	failures     int
	err          error
	seeded       []entity.TrainingCatalog
}

func (f *fakeCatalogCreator) CreateTrainingCatalog(_ context.Context, cat entity.TrainingCatalog) (entity.TrainingCatalog, error) {
	f.calls++
	if f.calls <= f.failures {
		return entity.TrainingCatalog{}, repository.ErrDuplicateTrainingCatalogPublicID
	}
	if f.err != nil {
		return entity.TrainingCatalog{}, f.err
	}

	created := cat
	created.ID = f.calls
	f.seeded = append(f.seeded, created)

	return created, nil
}

func fixedNow() time.Time {
	return time.Unix(1_700_000_000, 0)
}

func TestParseCSVValid(t *testing.T) {
	csvData := `title,description,pic_phone,category,max_slots,training_status,link,address,start_date,end_date,mentor
Digital Marketing,Pelatihan SEO,081234,Digital & IPTEK,30,ongoing,https://example.com,Bandung,2026-10-01,2026-10-05,Budi
Kriya Bambu,Workshop Bambu,081235,Kriya & Kreativitas,,planned,,Jakarta,,,Siti
`
	catalogs, err := parseCSV(strings.NewReader(csvData), fixedNow)
	if err != nil {
		t.Fatalf("parseCSV() error = %v", err)
	}

	if len(catalogs) != 2 {
		t.Fatalf("catalogs count = %d, want 2", len(catalogs))
	}

	c1 := catalogs[0]
	if c1.Title != "Digital Marketing" || c1.Category != entity.TrainingCategoryDigitalIPTEK || c1.TrainingStatus != entity.ProcessStatusOngoing {
		t.Errorf("c1 = %+v, want Digital Marketing / Digital & IPTEK / ongoing", c1)
	}
	if c1.MaxSlots == nil || *c1.MaxSlots != 30 {
		t.Errorf("c1.MaxSlots = %v, want 30", c1.MaxSlots)
	}
	if c1.StartDate == nil || c1.StartDate.Format("2006-01-02") != "2026-10-01" {
		t.Errorf("c1.StartDate = %v, want 2026-10-01", c1.StartDate)
	}
	if c1.CreatedAt != fixedNow().Unix() {
		t.Errorf("c1.CreatedAt = %d, want %d", c1.CreatedAt, fixedNow().Unix())
	}

	c2 := catalogs[1]
	if c2.Title != "Kriya Bambu" || c2.Category != entity.TrainingCategoryKriyaKreativitas || c2.TrainingStatus != entity.ProcessStatusPlanned {
		t.Errorf("c2 = %+v, want Kriya Bambu / Kriya & Kreativitas / planned", c2)
	}
	if c2.MaxSlots != nil {
		t.Errorf("c2.MaxSlots = %v, want nil", c2.MaxSlots)
	}
	if c2.StartDate != nil {
		t.Errorf("c2.StartDate = %v, want nil", c2.StartDate)
	}
}

func TestParseCSVValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		csv  string
	}{
		{name: "empty csv", csv: ""},
		{name: "header only", csv: "title,category,training_status\n"},
		{name: "missing required header", csv: "title,max_slots\nTest,20\n"},
		{name: "empty title", csv: "title,category,training_status\n,Digital & IPTEK,planned\n"},
		{name: "invalid category", csv: "title,category,training_status\nTest,Bogus,planned\n"},
		{name: "invalid status", csv: "title,category,training_status\nTest,Digital & IPTEK,unknown\n"},
		{name: "invalid slots", csv: "title,category,training_status,max_slots\nTest,Digital & IPTEK,planned,-5\n"},
		{name: "invalid date", csv: "title,category,training_status,start_date\nTest,Digital & IPTEK,planned,not-a-date\n"},
		{name: "end date before start date", csv: "title,category,training_status,start_date,end_date\nTest,Digital & IPTEK,planned,2026-10-05,2026-10-01\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseCSV(strings.NewReader(tc.csv), fixedNow)
			if err == nil {
				t.Fatalf("parseCSV(%s) error = nil, want validation error", tc.name)
			}
		})
	}
}

func TestSeedCatalogsGeneratesPublicIDAndLogs(t *testing.T) {
	repo := &fakeCatalogCreator{failures: 1}
	catalogs := []entity.TrainingCatalog{
		{
			Title:          "Digital Marketing",
			Category:       entity.TrainingCategoryDigitalIPTEK,
			TrainingStatus: entity.ProcessStatusPlanned,
		},
		{
			Title:          "Pertanian Modern",
			Category:       entity.TrainingCategoryWirausahaAgribisnis,
			TrainingStatus: entity.ProcessStatusOngoing,
		},
	}

	var out bytes.Buffer
	err := seedCatalogs(context.Background(), repo, catalogs, &out)
	if err != nil {
		t.Fatalf("seedCatalogs() error = %v", err)
	}

	if len(repo.seeded) != 2 {
		t.Fatalf("seeded count = %d, want 2", len(repo.seeded))
	}
	for _, c := range repo.seeded {
		if !publicIDPattern.MatchString(c.PublicID) {
			t.Errorf("public id = %q, want TCY-DDDDDD pattern", c.PublicID)
		}
	}

	output := out.String()
	if !strings.Contains(output, "Digital Marketing") || !strings.Contains(output, "Pertanian Modern") {
		t.Errorf("output missing titles: %q", output)
	}
	if !strings.Contains(output, "Successfully seeded 2 training catalogs") {
		t.Errorf("output missing success summary: %q", output)
	}
}

func TestSeedCatalogsWrapsDatabaseError(t *testing.T) {
	repo := &fakeCatalogCreator{err: errors.New("db connection lost")}
	catalogs := []entity.TrainingCatalog{
		{Title: "Digital Marketing", Category: entity.TrainingCategoryDigitalIPTEK, TrainingStatus: entity.ProcessStatusPlanned},
	}

	var out bytes.Buffer
	err := seedCatalogs(context.Background(), repo, catalogs, &out)
	if err == nil || !strings.Contains(err.Error(), "db connection lost") {
		t.Fatalf("seedCatalogs() error = %v, want wrapped db connection lost", err)
	}
}

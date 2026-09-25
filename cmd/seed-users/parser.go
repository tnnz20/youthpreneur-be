package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

const (
	defaultFallbackPassword = "Youthpreneur123!"
	minPasswordLen          = 8
	maxPasswordLen          = 72
)

// UserEnterpriseRecord holds parsed and normalized row data for seeding.
type UserEnterpriseRecord struct {
	FullName             string
	Email                string
	Password             string
	EnterpriseName       string
	BusinessSector       entity.BusinessSector
	Description          string
	Address              string
	LegalStatus          entity.LegalStatus
	BusinessDigitization entity.BusinessDigitization
	InterventionNeeds    entity.InterventionNeeds
	TrainingStatus       entity.ProcessStatus
	MentoringStatus      entity.ProcessStatus
	CapitalAccess        entity.GeneralStatus
	Partnership          entity.GeneralStatus
	InitialTurnover      string
	CurrentTurnover      string
	District             string
	Status               entity.EnterpriseStatus
}

func parseUserEnterpriseCSV(r io.Reader) ([]UserEnterpriseRecord, error) {
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}

	if len(records) < 2 {
		return nil, errors.New("csv must contain header and at least one row")
	}

	header := records[0]
	colIndex := make(map[string]int, len(header))
	for i, h := range header {
		colIndex[strings.TrimSpace(strings.ToLower(h))] = i
	}

	requiredCols := []string{"full_name", "email", "enterprise_name"}
	for _, col := range requiredCols {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	getCol := func(row []string, colName string) string {
		idx, ok := colIndex[colName]
		if !ok || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	parsed := make([]UserEnterpriseRecord, 0, len(records)-1)
	seenEmails := make(map[string]int, len(records)-1)

	for i, row := range records[1:] {
		rowNum := i + 2

		fullName := getCol(row, "full_name")
		if fullName == "" {
			return nil, fmt.Errorf("row %d: full_name is required", rowNum)
		}

		email := usecase.NormalizeEmail(getCol(row, "email"))
		if email == "" {
			return nil, fmt.Errorf("row %d: email is required", rowNum)
		}
		if err := usecase.ValidateEmail(email); err != nil {
			return nil, fmt.Errorf("row %d: invalid email %q: %w", rowNum, email, err)
		}
		if prevRow, seen := seenEmails[email]; seen {
			return nil, fmt.Errorf("row %d: duplicate email %q (already on row %d)", rowNum, email, prevRow)
		}
		seenEmails[email] = rowNum

		rawPassword := getCol(row, "password")
		password := normalizePassword(rawPassword)

		enterpriseName := getCol(row, "enterprise_name")
		if enterpriseName == "" {
			return nil, fmt.Errorf("row %d: enterprise_name is required", rowNum)
		}

		initialTurnover, err := normalizeTurnover(getCol(row, "initial_turnover"))
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid initial_turnover: %w", rowNum, err)
		}

		currentTurnover, err := normalizeTurnover(getCol(row, "current_turnover"))
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid current_turnover: %w", rowNum, err)
		}

		record := UserEnterpriseRecord{
			FullName:             fullName,
			Email:                email,
			Password:             password,
			EnterpriseName:       enterpriseName,
			BusinessSector:       normalizeBusinessSector(getCol(row, "business_sector")),
			Description:          getCol(row, "description"),
			Address:              getCol(row, "address"),
			LegalStatus:          normalizeLegalStatus(getCol(row, "legal_status")),
			BusinessDigitization: normalizeBusinessDigitization(getCol(row, "business_digitization")),
			InterventionNeeds:    normalizeInterventionNeeds(getCol(row, "intervention_needs")),
			TrainingStatus:       normalizeProcessStatus(getCol(row, "training_status")),
			MentoringStatus:      normalizeProcessStatus(getCol(row, "mentoring_status")),
			CapitalAccess:        normalizeGeneralStatus(getCol(row, "capital_access")),
			Partnership:          normalizeGeneralStatus(getCol(row, "partnership")),
			InitialTurnover:      initialTurnover,
			CurrentTurnover:      currentTurnover,
			District:             getCol(row, "district"),
			Status:               normalizeEnterpriseStatus(getCol(row, "status")),
		}

		parsed = append(parsed, record)
	}

	return parsed, nil
}

func normalizePassword(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) < minPasswordLen || len(raw) > maxPasswordLen || strings.HasPrefix(raw, "#") {
		return defaultFallbackPassword
	}
	return raw
}

func normalizeBusinessSector(raw string) entity.BusinessSector {
	raw = strings.TrimSpace(raw)
	switch raw {
	case string(entity.BusinessSectorKuliner):
		return entity.BusinessSectorKuliner
	case string(entity.BusinessSectorPerdaganganRitel):
		return entity.BusinessSectorPerdaganganRitel
	case string(entity.BusinessSectorAgribisnis):
		return entity.BusinessSectorAgribisnis
	case string(entity.BusinessSectorJasaLayananPublik):
		return entity.BusinessSectorJasaLayananPublik
	case string(entity.BusinessSectorFashionKonveksi):
		return entity.BusinessSectorFashionKonveksi
	case string(entity.BusinessSectorECommerceKreatif):
		return entity.BusinessSectorECommerceKreatif
	default:
		return entity.BusinessSectorPerdaganganRitel
	}
}

func normalizeLegalStatus(raw string) entity.LegalStatus {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(entity.LegalStatusComplete):
		return entity.LegalStatusComplete
	case string(entity.LegalStatusInProgress):
		return entity.LegalStatusInProgress
	default:
		return entity.LegalStatusNone
	}
}

func normalizeBusinessDigitization(raw string) entity.BusinessDigitization {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(entity.BusinessDigitizationHigh):
		return entity.BusinessDigitizationHigh
	case string(entity.BusinessDigitizationMedium):
		return entity.BusinessDigitizationMedium
	default:
		return entity.BusinessDigitizationLow
	}
}

func normalizeInterventionNeeds(raw string) entity.InterventionNeeds {
	switch strings.TrimSpace(raw) {
	case string(entity.InterventionNeedsPelatihan):
		return entity.InterventionNeedsPelatihan
	case string(entity.InterventionNeedsMentoring):
		return entity.InterventionNeedsMentoring
	case string(entity.InterventionNeedsDigitalisasi):
		return entity.InterventionNeedsDigitalisasi
	case string(entity.InterventionNeedsLegalitas):
		return entity.InterventionNeedsLegalitas
	case string(entity.InterventionNeedsPermodalan):
		return entity.InterventionNeedsPermodalan
	case string(entity.InterventionNeedsKemitraan):
		return entity.InterventionNeedsKemitraan
	case string(entity.InterventionNeedsPemasaran):
		return entity.InterventionNeedsPemasaran
	default:
		return entity.InterventionNeeds("")
	}
}

func normalizeProcessStatus(raw string) entity.ProcessStatus {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(entity.ProcessStatusCompleted):
		return entity.ProcessStatusCompleted
	case string(entity.ProcessStatusOngoing):
		return entity.ProcessStatusOngoing
	default:
		return entity.ProcessStatusPlanned
	}
}

func normalizeGeneralStatus(raw string) entity.GeneralStatus {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(entity.GeneralStatusYes):
		return entity.GeneralStatusYes
	case string(entity.GeneralStatusInProgress):
		return entity.GeneralStatusInProgress
	default:
		return entity.GeneralStatusNo
	}
}

func normalizeEnterpriseStatus(raw string) entity.EnterpriseStatus {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(entity.EnterpriseStatusInactive):
		return entity.EnterpriseStatusInactive
	default:
		return entity.EnterpriseStatusActive
	}
}

func normalizeTurnover(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "0", nil
	}
	cleaned := strings.ReplaceAll(trimmed, " ", "")

	val, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return "", fmt.Errorf("invalid numeric turnover %q", trimmed)
	}
	if val < 0 {
		return "", fmt.Errorf("turnover cannot be negative: %s", cleaned)
	}
	if val >= 10000000000000 {
		return "", fmt.Errorf("turnover out of allowed bounds: %s", cleaned)
	}

	return cleaned, nil
}

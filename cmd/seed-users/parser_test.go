package main

import (
	"os"
	"strings"
	"testing"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
)

func TestParseUserEnterpriseCSV_Valid(t *testing.T) {
	csvData := `full_name,email,password,enterprise_name,business_sector,description,address,legal_status,business_digitization,intervention_needs,training_status,mentoring_status,capital_access,partnership,initial_turnover,current_turnover,district,status
Jane Doe,jane@example.com,secret123,Jane Bakery,Kuliner,Delicious bakery,Jl. Mawar No. 1,complete,high,Pelatihan,completed,ongoing,yes,in_progress,1000000,2000000,Binuang,active
John Smith,john@example.com,#NAME?,Smith Store,data tidak lengkap,,Jl. Melati,invalid,none,Unknown,invalid,invalid,data tidak lengkap,no,,,Tapin Utara,inactive
`

	records, err := parseUserEnterpriseCSV(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r1 := records[0]
	if r1.FullName != "Jane Doe" || r1.Email != "jane@example.com" || r1.Password != "secret123" {
		t.Errorf("unexpected r1 user data: %+v", r1)
	}
	if r1.EnterpriseName != "Jane Bakery" || r1.BusinessSector != entity.BusinessSectorKuliner {
		t.Errorf("unexpected r1 enterprise data: %+v", r1)
	}
	if r1.LegalStatus != entity.LegalStatusComplete || r1.BusinessDigitization != entity.BusinessDigitizationHigh {
		t.Errorf("unexpected r1 status data: %+v", r1)
	}
	if r1.InterventionNeeds != entity.InterventionNeedsPelatihan {
		t.Errorf("unexpected r1 intervention: %s", r1.InterventionNeeds)
	}
	if r1.CapitalAccess != entity.GeneralStatusYes || r1.Partnership != entity.GeneralStatusInProgress {
		t.Errorf("unexpected r1 general status: %+v", r1)
	}
	if r1.InitialTurnover != "1000000" || r1.CurrentTurnover != "2000000" {
		t.Errorf("unexpected r1 turnover: %s, %s", r1.InitialTurnover, r1.CurrentTurnover)
	}
	if r1.Status != entity.EnterpriseStatusActive {
		t.Errorf("unexpected r1 status: %s", r1.Status)
	}

	r2 := records[1]
	if r2.Password != defaultFallbackPassword {
		t.Errorf("expected fallback password for #NAME?, got %q", r2.Password)
	}
	if r2.BusinessSector != entity.BusinessSectorPerdaganganRitel {
		t.Errorf("expected fallback sector 'Perdagangan Ritel', got %q", r2.BusinessSector)
	}
	if r2.LegalStatus != entity.LegalStatusNone || r2.BusinessDigitization != entity.BusinessDigitizationLow {
		t.Errorf("expected default legal and digitization, got %s, %s", r2.LegalStatus, r2.BusinessDigitization)
	}
	if r2.InterventionNeeds != "" {
		t.Errorf("expected empty intervention needs, got %q", r2.InterventionNeeds)
	}
	if r2.TrainingStatus != entity.ProcessStatusPlanned || r2.MentoringStatus != entity.ProcessStatusPlanned {
		t.Errorf("expected planned process status, got %s, %s", r2.TrainingStatus, r2.MentoringStatus)
	}
	if r2.CapitalAccess != entity.GeneralStatusNo || r2.Partnership != entity.GeneralStatusNo {
		t.Errorf("expected 'no' general status, got %s, %s", r2.CapitalAccess, r2.Partnership)
	}
	if r2.InitialTurnover != "0" || r2.CurrentTurnover != "0" {
		t.Errorf("expected '0' turnover, got %s, %s", r2.InitialTurnover, r2.CurrentTurnover)
	}
	if r2.Status != entity.EnterpriseStatusInactive {
		t.Errorf("expected inactive status, got %s", r2.Status)
	}
}

func TestParseUserEnterpriseCSV_Errors(t *testing.T) {
	tests := []struct {
		name    string
		csv     string
		wantErr string
	}{
		{
			name:    "empty file",
			csv:     "",
			wantErr: "csv must contain header and at least one row",
		},
		{
			name:    "missing required column",
			csv:     "full_name,email\nJane,jane@example.com\n",
			wantErr: "missing required column: enterprise_name",
		},
		{
			name:    "missing full_name",
			csv:     "full_name,email,enterprise_name\n,jane@example.com,Jane Bakery\n",
			wantErr: "row 2: full_name is required",
		},
		{
			name:    "invalid email",
			csv:     "full_name,email,enterprise_name\nJane,not-an-email,Jane Bakery\n",
			wantErr: "row 2: invalid email",
		},
		{
			name:    "duplicate email",
			csv:     "full_name,email,enterprise_name\nJane,jane@example.com,Bakery 1\nJanet,jane@example.com,Bakery 2\n",
			wantErr: "duplicate email",
		},
		{
			name:    "missing enterprise_name",
			csv:     "full_name,email,enterprise_name\nJane,jane@example.com,\n",
			wantErr: "row 2: enterprise_name is required",
		},
		{
			name:    "negative turnover",
			csv:     "full_name,email,enterprise_name,initial_turnover\nJane,jane@example.com,Bakery,-500\n",
			wantErr: "turnover cannot be negative",
		},
		{
			name:    "turnover out of bounds",
			csv:     "full_name,email,enterprise_name,initial_turnover\nJane,jane@example.com,Bakery,10000000000000\n",
			wantErr: "turnover out of allowed bounds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseUserEnterpriseCSV(strings.NewReader(tt.csv))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseUserEnterpriseCSV_ActualExternalFile(t *testing.T) {
	const externalPath = "../../db/seeds/external/rekap_wirausaha_dan_user.csv"
	file, err := os.Open(externalPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("external file not found, skipping integration test")
		}
		t.Fatalf("failed to open external file: %v", err)
	}
	defer file.Close()

	records, err := parseUserEnterpriseCSV(file)
	if err != nil {
		t.Fatalf("failed to parse actual external CSV: %v", err)
	}

	if len(records) != 931 {
		t.Errorf("expected 931 records, got %d", len(records))
	}
}

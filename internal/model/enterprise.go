package model

// CreateEnterpriseRequest is the JSON body for creating an enterprise. The
// owner and initial status are server-controlled and never read from the body.
type CreateEnterpriseRequest struct {
	Name                 string `json:"name"`
	BusinessSector       string `json:"business_sector"`
	LegalStatus          string `json:"legal_status"`
	BusinessDigitization string `json:"business_digitization"`
	InterventionNeeds    string `json:"intervention_needs"`
	TrainingStatus       string `json:"training_status"`
	MentoringStatus      string `json:"mentoring_status"`
	CapitalAccess        string `json:"capital_access"`
	Partnership          string `json:"partnership"`
	InitialTurnover      string `json:"initial_turnover"`
	CurrentTurnover      string `json:"current_turnover"`
	District             string `json:"district"`
}

// UpdateEnterpriseRequest is the JSON body for partially updating an
// enterprise. Omitted fields keep their current value. Owners may only set
// name, business_sector, initial_turnover, and current_turnover.
type UpdateEnterpriseRequest struct {
	Name                 *string `json:"name"`
	BusinessSector       *string `json:"business_sector"`
	LegalStatus          *string `json:"legal_status"`
	BusinessDigitization *string `json:"business_digitization"`
	InterventionNeeds    *string `json:"intervention_needs"`
	TrainingStatus       *string `json:"training_status"`
	MentoringStatus      *string `json:"mentoring_status"`
	CapitalAccess        *string `json:"capital_access"`
	Partnership          *string `json:"partnership"`
	InitialTurnover      *string `json:"initial_turnover"`
	CurrentTurnover      *string `json:"current_turnover"`
	District             *string `json:"district"`
	Status               *string `json:"status"`
}

// EnterpriseResponse is the JSON representation of an enterprise. Name and the
// optional assessment fields are null when unset, as is District.
type EnterpriseResponse struct {
	PublicID             string  `json:"public_id"`
	Name                 *string `json:"name"`
	BusinessSector       string  `json:"business_sector"`
	LegalStatus          *string `json:"legal_status"`
	BusinessDigitization *string `json:"business_digitization"`
	InterventionNeeds    *string `json:"intervention_needs"`
	TrainingStatus       *string `json:"training_status"`
	MentoringStatus      *string `json:"mentoring_status"`
	CapitalAccess        *string `json:"capital_access"`
	Partnership          *string `json:"partnership"`
	InitialTurnover      string  `json:"initial_turnover"`
	CurrentTurnover      string  `json:"current_turnover"`
	District             *string `json:"district"`
	Status               string  `json:"status"`
	CreatedAt            int64   `json:"created_at"`
	UpdatedAt            int64   `json:"updated_at"`
}

// EnterpriseListResponse is one page of enterprises with the cursor for the
// next page.
type EnterpriseListResponse struct {
	Enterprises []EnterpriseResponse `json:"enterprises"`
	NextCursor  string               `json:"next_cursor,omitempty"`
}

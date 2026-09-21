package model

// CreateEnterpriseRequest is the JSON body for creating an enterprise. The
// owner and initial status are server-controlled and never read from the body.
type CreateEnterpriseRequest struct {
	EnterpriseName       string `json:"enterprise_name"`
	Description          string `json:"description,omitempty"`
	Address              string `json:"address,omitempty"`
	FocusCommodity       string `json:"focus_commodity,omitempty"`
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
// enterprise. Omitted fields keep their current value. Owners may update
// enterprise_name, business_sector, district, description, address,
// focus_commodity, initial_turnover, and current_turnover.
type UpdateEnterpriseRequest struct {
	EnterpriseName       *string `json:"enterprise_name"`
	Description          *string `json:"description"`
	Address              *string `json:"address"`
	FocusCommodity       *string `json:"focus_commodity"`
	DisporaSupport       *string `json:"dispora_support"`
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

// EnterpriseResponse is the JSON representation of an enterprise.
type EnterpriseResponse struct {
	PublicID             string  `json:"public_id"`
	UserPublicID         string  `json:"user_public_id"`
	FullName             string  `json:"full_name"`
	EnterpriseName       string  `json:"enterprise_name"`
	Description          *string `json:"description"`
	Address              *string `json:"address"`
	FocusCommodity       *string `json:"focus_commodity"`
	DisporaSupport       *string `json:"dispora_support"`
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

// PublicEnterpriseResponse is the public showcase representation of an enterprise.
type PublicEnterpriseResponse struct {
	PublicID          string  `json:"public_id"`
	EnterpriseName    string  `json:"enterprise_name"`
	FullName          string  `json:"full_name"`
	BusinessSector    string  `json:"business_sector"`
	District          *string `json:"district"`
	Description       *string `json:"description"`
	FocusCommodity    *string `json:"focus_commodity"`
	DisporaSupport    *string `json:"dispora_support"`
	InterventionNeeds *string `json:"intervention_needs"`
	CreatedAt         int64   `json:"created_at"`
}

// PublicEnterpriseListResponse is one page of public enterprises with the cursor
// for the next page.
type PublicEnterpriseListResponse struct {
	Enterprises []PublicEnterpriseResponse `json:"enterprises"`
	NextCursor  string                     `json:"next_cursor,omitempty"`
}

package model

import "github.com/tnnz20/youthpreneur-be/internal/entity"

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

// EnterpriseAuditEventResponse is the JSON representation of an enterprise audit event.
type EnterpriseAuditEventResponse struct {
	ID            int            `json:"id"`
	ActorPublicID string         `json:"actor_public_id"`
	ActorEmail    string         `json:"actor_email"`
	ActorName     string         `json:"actor_name"`
	Action        string         `json:"action"`
	ChangedFields map[string]any `json:"changed_fields"`
	CreatedAt     int64          `json:"created_at"`
}

// EnterpriseAuditListResponse is one page of enterprise audit events with the cursor
// for the next page.
type EnterpriseAuditListResponse struct {
	Events     []EnterpriseAuditEventResponse `json:"events"`
	NextCursor string                         `json:"next_cursor,omitempty"`
}

// ToEnterpriseResponse converts an entity.Enterprise to an EnterpriseResponse.
func ToEnterpriseResponse(enterprise entity.Enterprise) EnterpriseResponse {
	return EnterpriseResponse{
		PublicID:             enterprise.PublicID,
		UserPublicID:         enterprise.UserPublicID,
		FullName:             enterprise.OwnerFullName,
		EnterpriseName:       enterprise.EnterpriseName,
		Description:          optionalString(enterprise.Description),
		Address:              optionalString(enterprise.Address),
		FocusCommodity:       optionalString(enterprise.FocusCommodity),
		DisporaSupport:       optionalString(enterprise.DisporaSupport),
		BusinessSector:       string(enterprise.BusinessSector),
		LegalStatus:          optionalString(string(enterprise.LegalStatus)),
		BusinessDigitization: optionalString(string(enterprise.BusinessDigitization)),
		InterventionNeeds:    optionalString(string(enterprise.InterventionNeeds)),
		TrainingStatus:       optionalString(string(enterprise.TrainingStatus)),
		MentoringStatus:      optionalString(string(enterprise.MentoringStatus)),
		CapitalAccess:        optionalString(string(enterprise.CapitalAccess)),
		Partnership:          optionalString(string(enterprise.Partnership)),
		InitialTurnover:      enterprise.InitialTurnover,
		CurrentTurnover:      enterprise.CurrentTurnover,
		District:             optionalString(enterprise.District),
		Status:               string(enterprise.Status),
		CreatedAt:            enterprise.CreatedAt,
		UpdatedAt:            enterprise.UpdatedAt,
	}
}

// ToEnterpriseResponses converts a slice of entity.Enterprise to a slice of EnterpriseResponse.
func ToEnterpriseResponses(enterprises []entity.Enterprise) []EnterpriseResponse {
	responses := make([]EnterpriseResponse, 0, len(enterprises))
	for _, enterprise := range enterprises {
		responses = append(responses, ToEnterpriseResponse(enterprise))
	}

	return responses
}

// ToPublicEnterpriseResponse converts an entity.PublicEnterprise to a PublicEnterpriseResponse.
func ToPublicEnterpriseResponse(item entity.PublicEnterprise) PublicEnterpriseResponse {
	return PublicEnterpriseResponse{
		PublicID:          item.PublicID,
		EnterpriseName:    item.EnterpriseName,
		FullName:          item.OwnerFullName,
		BusinessSector:    string(item.BusinessSector),
		District:          optionalString(item.District),
		Description:       optionalString(item.Description),
		FocusCommodity:    optionalString(item.FocusCommodity),
		DisporaSupport:    optionalString(item.DisporaSupport),
		InterventionNeeds: optionalString(string(item.InterventionNeeds)),
		CreatedAt:         item.CreatedAt,
	}
}

// ToPublicEnterpriseResponses converts a slice of entity.PublicEnterprise to a slice of PublicEnterpriseResponse.
func ToPublicEnterpriseResponses(items []entity.PublicEnterprise) []PublicEnterpriseResponse {
	responses := make([]PublicEnterpriseResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, ToPublicEnterpriseResponse(item))
	}

	return responses
}

// ToEnterpriseAuditEventResponse converts an entity.EnterpriseAuditEventView to an EnterpriseAuditEventResponse.
func ToEnterpriseAuditEventResponse(event entity.EnterpriseAuditEventView) EnterpriseAuditEventResponse {
	changedFields := event.ChangedFields
	if changedFields == nil {
		changedFields = map[string]any{}
	}

	return EnterpriseAuditEventResponse{
		ID:            event.ID,
		ActorPublicID: event.ActorPublicID,
		ActorEmail:    event.ActorEmail,
		ActorName:     event.ActorName,
		Action:        string(event.Action),
		ChangedFields: changedFields,
		CreatedAt:     event.CreatedAt,
	}
}

// ToEnterpriseAuditEventResponses converts a slice of entity.EnterpriseAuditEventView to a slice of EnterpriseAuditEventResponse.
func ToEnterpriseAuditEventResponses(events []entity.EnterpriseAuditEventView) []EnterpriseAuditEventResponse {
	responses := make([]EnterpriseAuditEventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, ToEnterpriseAuditEventResponse(event))
	}

	return responses
}

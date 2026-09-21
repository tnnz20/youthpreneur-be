package model

import "github.com/tnnz20/youthpreneur-be/internal/entity"

// CreateTrainingCatalogRequest is the JSON body for creating a catalog entry.
// Public ID, timestamps, and deletion state are server-controlled.
type CreateTrainingCatalogRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	PicPhone       string `json:"pic_phone"`
	Category       string `json:"category"`
	TrainingSlots  *int   `json:"training_slots"`
	TrainingStatus string `json:"training_status"`
	Link           string `json:"link"`
	TrainingDate   string `json:"training_date"`
	TrainingPeriod string `json:"training_period"`
	Speaker        string `json:"speaker"`
}

// UpdateTrainingCatalogRequest is the JSON body for partially updating a
// catalog entry. Omitted fields keep their current value.
type UpdateTrainingCatalogRequest struct {
	Name           *string `json:"name"`
	Description    *string `json:"description"`
	PicPhone       *string `json:"pic_phone"`
	Category       *string `json:"category"`
	TrainingSlots  *int    `json:"training_slots"`
	TrainingStatus *string `json:"training_status"`
	Link           *string `json:"link"`
	TrainingDate   *string `json:"training_date"`
	TrainingPeriod *string `json:"training_period"`
	Speaker        *string `json:"speaker"`
}

// UpdateTrainingCatalogStatusRequest is the JSON body for changing only the
// catalog training status.
type UpdateTrainingCatalogStatusRequest struct {
	TrainingStatus string `json:"training_status"`
}

// TrainingCatalogResponse is the JSON representation of a catalog entry. Every
// optional field serializes as JSON null when unset.
type TrainingCatalogResponse struct {
	PublicID       string  `json:"public_id"`
	Name           *string `json:"name"`
	Description    *string `json:"description"`
	PicPhone       *string `json:"pic_phone"`
	Category       *string `json:"category"`
	TrainingSlots  *int    `json:"training_slots"`
	TrainingStatus *string `json:"training_status"`
	Link           *string `json:"link"`
	TrainingDate   *string `json:"training_date"`
	TrainingPeriod *string `json:"training_period"`
	Speaker        *string `json:"speaker"`
	CreatedAt      int64   `json:"created_at"`
	UpdatedAt      int64   `json:"updated_at"`
}

// TrainingCatalogListResponse is one page of catalog entries with the cursor
// for the next page.
type TrainingCatalogListResponse struct {
	Catalogs   []TrainingCatalogResponse `json:"training_catalogs"`
	NextCursor string                    `json:"next_cursor,omitempty"`
}

// ToTrainingCatalogResponse converts an entity.TrainingCatalog to a TrainingCatalogResponse.
func ToTrainingCatalogResponse(catalog entity.TrainingCatalog) TrainingCatalogResponse {
	return TrainingCatalogResponse{
		PublicID:       catalog.PublicID,
		Name:           optionalString(catalog.Name),
		Description:    optionalString(catalog.Description),
		PicPhone:       optionalString(catalog.PicPhone),
		Category:       optionalString(catalog.Category),
		TrainingSlots:  catalog.TrainingSlots,
		TrainingStatus: optionalString(string(catalog.TrainingStatus)),
		Link:           optionalString(catalog.Link),
		TrainingDate:   formatOptionalDate(catalog.TrainingDate),
		TrainingPeriod: optionalString(catalog.TrainingPeriod),
		Speaker:        optionalString(catalog.Speaker),
		CreatedAt:      catalog.CreatedAt,
		UpdatedAt:      catalog.UpdatedAt,
	}
}

// ToTrainingCatalogResponses converts a slice of entity.TrainingCatalog to a slice of TrainingCatalogResponse.
func ToTrainingCatalogResponses(catalogs []entity.TrainingCatalog) []TrainingCatalogResponse {
	responses := make([]TrainingCatalogResponse, 0, len(catalogs))
	for _, catalog := range catalogs {
		responses = append(responses, ToTrainingCatalogResponse(catalog))
	}

	return responses
}

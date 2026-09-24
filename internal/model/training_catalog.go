package model

import "github.com/tnnz20/youthpreneur-be/internal/entity"

// CreateTrainingCatalogRequest is the JSON body for creating a catalog entry.
// Public ID, timestamps, and deletion state are server-controlled.
type CreateTrainingCatalogRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	PicPhone       string `json:"pic_phone"`
	Category       string `json:"category"`
	MaxSlots       *int   `json:"max_slots"`
	TrainingStatus string `json:"training_status"`
	Link           string `json:"link"`
	Address        string `json:"address"`
	Thumbnail      string `json:"thumbnail"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	Mentor         string `json:"mentor"`
}

// UpdateTrainingCatalogRequest is the JSON body for partially updating a
// catalog entry. Omitted fields keep their current value.
type UpdateTrainingCatalogRequest struct {
	Title          *string `json:"title"`
	Description    *string `json:"description"`
	PicPhone       *string `json:"pic_phone"`
	Category       *string `json:"category"`
	MaxSlots       *int    `json:"max_slots"`
	TrainingStatus *string `json:"training_status"`
	Link           *string `json:"link"`
	Address        *string `json:"address"`
	Thumbnail      *string `json:"thumbnail"`
	StartDate      *string `json:"start_date"`
	EndDate        *string `json:"end_date"`
	Mentor         *string `json:"mentor"`
}

// UpdateTrainingCatalogStatusRequest is the JSON body for changing only the
// catalog training status.
type UpdateTrainingCatalogStatusRequest struct {
	TrainingStatus string `json:"training_status"`
}

// UploadThumbnailResponse is returned after successfully uploading a training
// catalog thumbnail.
type UploadThumbnailResponse struct {
	ThumbnailURL string `json:"thumbnail_url"`
}

// TrainingCatalogResponse is the JSON representation of a catalog entry. Every
// optional field serializes as JSON null when unset.
type TrainingCatalogResponse struct {
	PublicID        string  `json:"public_id"`
	Title           *string `json:"title"`
	Description     *string `json:"description"`
	PicPhone        *string `json:"pic_phone"`
	Category        *string `json:"category"`
	MaxSlots        *int    `json:"max_slots"`
	RegisteredCount int     `json:"registered_count"`
	TrainingStatus  *string `json:"training_status"`
	Link            *string `json:"link"`
	Address         *string `json:"address"`
	Thumbnail       *string `json:"thumbnail"`
	StartDate       *string `json:"start_date"`
	EndDate         *string `json:"end_date"`
	Mentor          *string `json:"mentor"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
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
		PublicID:        catalog.PublicID,
		Title:           optionalString(catalog.Title),
		Description:     optionalString(catalog.Description),
		PicPhone:        optionalString(catalog.PicPhone),
		Category:        optionalString(string(catalog.Category)),
		MaxSlots:        catalog.MaxSlots,
		RegisteredCount: catalog.RegisteredCount,
		TrainingStatus:  optionalString(string(catalog.TrainingStatus)),
		Link:            optionalString(catalog.Link),
		Address:         optionalString(catalog.Address),
		Thumbnail:       optionalString(catalog.Thumbnail),
		StartDate:       formatOptionalDate(catalog.StartDate),
		EndDate:         formatOptionalDate(catalog.EndDate),
		Mentor:          optionalString(catalog.Mentor),
		CreatedAt:       catalog.CreatedAt,
		UpdatedAt:       catalog.UpdatedAt,
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

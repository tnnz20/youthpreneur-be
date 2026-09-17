package model

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

package model

// CreateTrainingEnrollmentRequest is the JSON body for enrolling the
// authenticated user in a catalog offering. The user is always the
// authenticated identity and is never read from the body.
type CreateTrainingEnrollmentRequest struct {
	CatalogPublicID string `json:"catalog_public_id"`
}

// TrainingEnrollmentResponse is the JSON representation of one enrollment.
// RegisterDate is a `YYYY-MM-DD` string and Catalog is the joined catalog
// snapshot so history stays meaningful after a catalog changes.
type TrainingEnrollmentResponse struct {
	PublicID     string                   `json:"public_id"`
	UserPublicID string                   `json:"user_public_id"`
	RegisterDate *string                  `json:"register_date"`
	CreatedAt    int64                    `json:"created_at"`
	UpdatedAt    int64                    `json:"updated_at"`
	Catalog      *TrainingCatalogResponse `json:"catalog"`
}

// TrainingEnrollmentListResponse is one page of enrollments with the cursor
// for the next page.
type TrainingEnrollmentListResponse struct {
	Enrollments []TrainingEnrollmentResponse `json:"training_enrollments"`
	NextCursor  string                       `json:"next_cursor,omitempty"`
}

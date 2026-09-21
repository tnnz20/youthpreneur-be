package model

import "github.com/tnnz20/youthpreneur-be/internal/entity"

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

// ToTrainingEnrollmentResponse converts an entity.TrainingEnrollment to a TrainingEnrollmentResponse.
func ToTrainingEnrollmentResponse(enrollment entity.TrainingEnrollment) TrainingEnrollmentResponse {
	response := TrainingEnrollmentResponse{
		PublicID:     enrollment.PublicID,
		UserPublicID: enrollment.UserPublicID,
		RegisterDate: formatOptionalDate(enrollment.RegisterDate),
		CreatedAt:    enrollment.CreatedAt,
		UpdatedAt:    enrollment.UpdatedAt,
	}

	if enrollment.Catalog != nil {
		catalog := ToTrainingCatalogResponse(*enrollment.Catalog)
		response.Catalog = &catalog
	}

	return response
}

// ToTrainingEnrollmentResponses converts a slice of entity.TrainingEnrollment to a slice of TrainingEnrollmentResponse.
func ToTrainingEnrollmentResponses(enrollments []entity.TrainingEnrollment) []TrainingEnrollmentResponse {
	responses := make([]TrainingEnrollmentResponse, 0, len(enrollments))
	for _, enrollment := range enrollments {
		responses = append(responses, ToTrainingEnrollmentResponse(enrollment))
	}

	return responses
}

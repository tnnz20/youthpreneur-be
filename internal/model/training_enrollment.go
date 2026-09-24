package model

import "github.com/tnnz20/youthpreneur-be/internal/entity"

// CreateTrainingEnrollmentRequest is the JSON body for enrolling the
// authenticated user in a catalog offering. The user is always the
// authenticated identity and is never read from the body.
type CreateTrainingEnrollmentRequest struct {
	CatalogPublicID string `json:"catalog_public_id"`
}

// UpdateTrainingEnrollmentStatusRequest is the JSON body for updating an
// enrollment's approval status.
type UpdateTrainingEnrollmentStatusRequest struct {
	Status string `json:"status"`
}

// TrainingEnrollmentCatalogResponse is the nested catalog summary included in an enrollment response.
type TrainingEnrollmentCatalogResponse struct {
	PublicID       string `json:"public_id"`
	Title          string `json:"title"`
	TrainingStatus string `json:"training_status"`
}

// TrainingEnrollmentResponse is the JSON representation of one enrollment.
// RegisterDate is a `YYYY-MM-DD` string, Status is the enrollment approval state,
// and Catalog is the joined catalog snapshot containing public_id, title, and training_status.
type TrainingEnrollmentResponse struct {
	PublicID     string                             `json:"public_id"`
	UserPublicID string                             `json:"user_public_id"`
	FullName     string                             `json:"full_name"`
	RegisterDate *string                            `json:"register_date"`
	Status       string                             `json:"status"`
	CreatedAt    int64                              `json:"created_at"`
	UpdatedAt    int64                              `json:"updated_at"`
	Catalog      *TrainingEnrollmentCatalogResponse `json:"catalog"`
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
		FullName:     enrollment.FullName,
		RegisterDate: formatOptionalDate(enrollment.RegisterDate),
		Status:       string(enrollment.Status),
		CreatedAt:    enrollment.CreatedAt,
		UpdatedAt:    enrollment.UpdatedAt,
	}

	if enrollment.Catalog != nil {
		response.Catalog = &TrainingEnrollmentCatalogResponse{
			PublicID:       enrollment.Catalog.PublicID,
			Title:          enrollment.Catalog.Title,
			TrainingStatus: string(enrollment.Catalog.TrainingStatus),
		}
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

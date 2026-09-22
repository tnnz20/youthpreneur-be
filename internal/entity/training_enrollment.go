package entity

import "time"

// TrainingEnrollmentStatus represents the approval status of an enrollment.
type TrainingEnrollmentStatus string

const (
	TrainingEnrollmentStatusPending  TrainingEnrollmentStatus = "pending"
	TrainingEnrollmentStatusAccepted TrainingEnrollmentStatus = "accepted"
	TrainingEnrollmentStatusRejected TrainingEnrollmentStatus = "rejected"
)

// ValidTrainingEnrollmentStatus reports whether status is a known status.
func ValidTrainingEnrollmentStatus(status TrainingEnrollmentStatus) bool {
	switch status {
	case TrainingEnrollmentStatusPending,
		TrainingEnrollmentStatusAccepted,
		TrainingEnrollmentStatusRejected:
		return true
	default:
		return false
	}
}

// TrainingEnrollment links a user to a training catalog offering. Cancellation
// sets DeletedAt and keeps the row so enrollment history survives; a member may
// enroll again afterward. UserPublicID and Catalog are populated by joined
// reads for API responses.
type TrainingEnrollment struct {
	ID                int
	PublicID          string
	UserID            int
	UserPublicID      string
	FullName          string
	TrainingCatalogID int
	RegisterDate      *time.Time
	Status            TrainingEnrollmentStatus
	CreatedAt         int64
	UpdatedAt         int64
	DeletedAt         *int64
	Catalog           *TrainingCatalog
}

// TrainingEnrollmentFilter bounds a cursor-paginated enrollment query.
//
// Cursor is the last seen training_enrollments.id; zero starts from the first
// row. UserID and CatalogID restrict results when non-zero; zero selects every
// user or catalog and must only be used for admin scope.
type TrainingEnrollmentFilter struct {
	UserID    int
	CatalogID int
	Search    string
	Status    TrainingEnrollmentStatus
	Cursor    int
	Limit     int
}

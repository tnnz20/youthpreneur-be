package entity

import "time"

// TrainingCatalog is a training offering. Every descriptive field is optional
// and stored as NULL when empty. TrainingSlots is nil for unlimited capacity.
type TrainingCatalog struct {
	ID             int
	PublicID       string
	Name           string
	Description    string
	PicPhone       string
	Category       string
	TrainingSlots  *int
	TrainingStatus ProcessStatus
	Link           string
	TrainingDate   *time.Time
	TrainingPeriod string
	Speaker        string
	CreatedAt      int64
	UpdatedAt      int64
	DeletedAt      *int64
}

// TrainingCatalogUpdate carries normalized optional catalog field changes. A
// nil field is left untouched; an empty string clears a nullable string field.
// TrainingSlots, when non-nil, must be positive. UpdatedAt is the mutation
// timestamp recorded on the persisted row.
type TrainingCatalogUpdate struct {
	Name           *string
	Description    *string
	PicPhone       *string
	Category       *string
	TrainingSlots  *int
	TrainingStatus *ProcessStatus
	Link           *string
	TrainingDate   *time.Time
	TrainingPeriod *string
	Speaker        *string
	UpdatedAt      int64
}

// TrainingCatalogFilter bounds and filters a cursor-paginated catalog query.
//
// Cursor is the last seen training_catalog.id; zero starts from the first row.
// Every filter is ignored when empty or nil. TrainingStatus only accepts the
// process_status_enum values.
type TrainingCatalogFilter struct {
	Category       string
	TrainingStatus ProcessStatus
	TrainingDate   *time.Time
	TrainingPeriod string
	Cursor         int
	Limit          int
}

// IsOpenForEnrollment reports whether a catalog accepts new enrollments. A
// catalog is open only while its status is planned or ongoing.
func (c TrainingCatalog) IsOpenForEnrollment() bool {
	return c.TrainingStatus == ProcessStatusPlanned || c.TrainingStatus == ProcessStatusOngoing
}

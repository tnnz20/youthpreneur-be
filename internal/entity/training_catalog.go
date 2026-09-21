package entity

import "time"

// TrainingCategory represents training category enum values.
type TrainingCategory string

const (
	TrainingCategoryWirausahaAgribisnis TrainingCategory = "Wirausaha & Agribisnis"
	TrainingCategoryKriyaKreativitas    TrainingCategory = "Kriya & Kreativitas"
	TrainingCategoryDigitalIPTEK        TrainingCategory = "Digital & IPTEK"
	TrainingCategoryOlahragaPrestasi    TrainingCategory = "Olahraga & Prestasi"
	TrainingCategoryKomunitasPemuda     TrainingCategory = "Komunitas & Pemuda"
)

// ValidTrainingCategory reports whether category is a known training category.
func ValidTrainingCategory(category TrainingCategory) bool {
	switch category {
	case TrainingCategoryWirausahaAgribisnis,
		TrainingCategoryKriyaKreativitas,
		TrainingCategoryDigitalIPTEK,
		TrainingCategoryOlahragaPrestasi,
		TrainingCategoryKomunitasPemuda:
		return true
	default:
		return false
	}
}

// TrainingCatalog is a training offering. Every descriptive field is optional
// and stored as NULL when empty. MaxSlots is nil for unlimited capacity.
type TrainingCatalog struct {
	ID              int
	PublicID        string
	Title           string
	Description     string
	PicPhone        string
	Category        TrainingCategory
	MaxSlots        *int
	TrainingStatus  ProcessStatus
	Link            string
	Address         string
	Thumbnail       string
	StartDate       *time.Time
	EndDate         *time.Time
	Mentor          string
	RegisteredCount int
	CreatedAt       int64
	UpdatedAt       int64
	DeletedAt       *int64
}

// TrainingCatalogUpdate carries normalized optional catalog field changes. A
// nil field is left untouched; an empty string clears a nullable string field.
// MaxSlots, when non-nil, must be positive. UpdatedAt is the mutation
// timestamp recorded on the persisted row.
type TrainingCatalogUpdate struct {
	Title          *string
	Description    *string
	PicPhone       *string
	Category       *TrainingCategory
	MaxSlots       *int
	TrainingStatus *ProcessStatus
	Link           *string
	Address        *string
	Thumbnail      *string
	StartDate      *time.Time
	EndDate        *time.Time
	Mentor         *string
	UpdatedAt      int64
}

// TrainingCatalogFilter bounds and filters a cursor-paginated catalog query.
//
// Cursor is the last seen training_catalog.id; zero starts from the first row.
// Every filter is ignored when empty or nil.
type TrainingCatalogFilter struct {
	Category       TrainingCategory
	TrainingStatus ProcessStatus
	StartDate      *time.Time
	Cursor         int
	Limit          int
}

// IsOpenForEnrollment reports whether a catalog accepts new enrollments. A
// catalog is open only while its status is planned or ongoing.
func (c TrainingCatalog) IsOpenForEnrollment() bool {
	return c.TrainingStatus == ProcessStatusPlanned || c.TrainingStatus == ProcessStatusOngoing
}

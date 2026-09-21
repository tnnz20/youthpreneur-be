package entity

// BusinessSector identifies the sector an enterprise operates in.
type BusinessSector string

const (
	// BusinessSectorKuliner identifies culinary enterprises.
	BusinessSectorKuliner BusinessSector = "Kuliner"
	// BusinessSectorPerdaganganRitel identifies retail trade enterprises.
	BusinessSectorPerdaganganRitel BusinessSector = "Perdagangan Ritel"
	// BusinessSectorAgribisnis identifies agribusiness and food security.
	BusinessSectorAgribisnis BusinessSector = "Agribisnis & Ketahanan Pangan"
	// BusinessSectorJasaLayananPublik identifies public services and services.
	BusinessSectorJasaLayananPublik BusinessSector = "Jasa & Layanan Publik"
	// BusinessSectorFashionKonveksi identifies fashion and apparel.
	BusinessSectorFashionKonveksi BusinessSector = "Fashion & Konveksi"
	// BusinessSectorECommerceKreatif identifies e-commerce and the creative
	// economy.
	BusinessSectorECommerceKreatif BusinessSector = "E-Commerce & Ekonomi Kreatif"
)

// EnterpriseStatus identifies the lifecycle state of an enterprise.
type EnterpriseStatus string

const (
	// EnterpriseStatusActive identifies an operating enterprise.
	EnterpriseStatusActive EnterpriseStatus = "active"
	// EnterpriseStatusInactive identifies a suspended enterprise.
	EnterpriseStatusInactive EnterpriseStatus = "inactive"
)

// LegalStatus identifies the legality state of an enterprise.
type LegalStatus string

const (
	// LegalStatusComplete identifies a fully licensed enterprise.
	LegalStatusComplete LegalStatus = "complete"
	// LegalStatusInProgress identifies an enterprise with licensing underway.
	LegalStatusInProgress LegalStatus = "in_progress"
	// LegalStatusNone identifies an enterprise without legal documents.
	LegalStatusNone LegalStatus = "none"
)

// BusinessDigitization identifies how digital an enterprise is.
type BusinessDigitization string

const (
	// BusinessDigitizationHigh identifies a highly digitized enterprise.
	BusinessDigitizationHigh BusinessDigitization = "high"
	// BusinessDigitizationMedium identifies a partly digitized enterprise.
	BusinessDigitizationMedium BusinessDigitization = "medium"
	// BusinessDigitizationLow identifies a minimally digitized enterprise.
	BusinessDigitizationLow BusinessDigitization = "low"
)

// InterventionNeeds identifies the support an enterprise needs.
type InterventionNeeds string

const (
	// InterventionNeedsPelatihan identifies training needs.
	InterventionNeedsPelatihan InterventionNeeds = "Pelatihan"
	// InterventionNeedsMentoring identifies mentoring needs.
	InterventionNeedsMentoring InterventionNeeds = "Mentoring"
	// InterventionNeedsDigitalisasi identifies digitalization needs.
	InterventionNeedsDigitalisasi InterventionNeeds = "Digitalisasi"
	// InterventionNeedsLegalitas identifies legality needs.
	InterventionNeedsLegalitas InterventionNeeds = "Legalitas"
	// InterventionNeedsPermodalan identifies capital needs.
	InterventionNeedsPermodalan InterventionNeeds = "Permodalan"
	// InterventionNeedsKemitraan identifies partnership needs.
	InterventionNeedsKemitraan InterventionNeeds = "Kemitraan"
	// InterventionNeedsPemasaran identifies marketing needs.
	InterventionNeedsPemasaran InterventionNeeds = "Pemasaran"
)

// ProcessStatus identifies the progress of a process such as training or
// mentoring.
type ProcessStatus string

const (
	// ProcessStatusCompleted identifies a finished process.
	ProcessStatusCompleted ProcessStatus = "completed"
	// ProcessStatusOngoing identifies an in-progress process.
	ProcessStatusOngoing ProcessStatus = "ongoing"
	// ProcessStatusPlanned identifies a planned process.
	ProcessStatusPlanned ProcessStatus = "planned"
)

// GeneralStatus identifies a yes/no/in-progress condition such as capital
// access or partnership.
type GeneralStatus string

const (
	// GeneralStatusYes identifies a satisfied condition.
	GeneralStatusYes GeneralStatus = "yes"
	// GeneralStatusNo identifies an unsatisfied condition.
	GeneralStatusNo GeneralStatus = "no"
	// GeneralStatusInProgress identifies a condition in progress.
	GeneralStatusInProgress GeneralStatus = "in_progress"
)

// AuditAction identifies the mutation recorded by an enterprise audit event.
type AuditAction string

const (
	// AuditActionCreate records an enterprise creation.
	AuditActionCreate AuditAction = "create"
	// AuditActionUpdate records an enterprise field update.
	AuditActionUpdate AuditAction = "update"
	// AuditActionDelete records an enterprise soft deletion.
	AuditActionDelete AuditAction = "delete"
)

// Enterprise is a business owned by a user account.
//
// UserID is the internal owner id and is never accepted from a request.
// EnterpriseName is required and non-null. The assessment enums and new text
// fields are optional and stored as NULL when empty. The turnover fields keep
// their exact decimal representation as strings so API clients do not lose
// precision through a float round trip.
type Enterprise struct {
	ID                   int
	PublicID             string
	UserID               int
	UserPublicID         string
	OwnerFullName        string
	EnterpriseName       string
	Description          string
	Address              string
	FocusCommodity       string
	DisporaSupport       string
	BusinessSector       BusinessSector
	LegalStatus          LegalStatus
	BusinessDigitization BusinessDigitization
	InterventionNeeds    InterventionNeeds
	TrainingStatus       ProcessStatus
	MentoringStatus      ProcessStatus
	CapitalAccess        GeneralStatus
	Partnership          GeneralStatus
	InitialTurnover      string
	CurrentTurnover      string
	District             string
	Status               EnterpriseStatus
	CreatedAt            int64
	UpdatedAt            int64
	DeletedAt            *int64
}

// EnterpriseUpdate carries normalized optional enterprise field changes. A nil
// field is left untouched. UpdatedAt is the mutation timestamp recorded on the
// persisted row.
type EnterpriseUpdate struct {
	EnterpriseName       *string
	Description          *string
	Address              *string
	FocusCommodity       *string
	DisporaSupport       *string
	BusinessSector       *BusinessSector
	LegalStatus          *LegalStatus
	BusinessDigitization *BusinessDigitization
	InterventionNeeds    *InterventionNeeds
	TrainingStatus       *ProcessStatus
	MentoringStatus      *ProcessStatus
	CapitalAccess        *GeneralStatus
	Partnership          *GeneralStatus
	InitialTurnover      *string
	CurrentTurnover      *string
	District             *string
	Status               *EnterpriseStatus
	UpdatedAt            int64
}

// NullableAuditValue records an empty cleared string as JSON null in audit
// changed_fields.
func NullableAuditValue(value string) any {
	if value == "" {
		return nil
	}

	return value
}

// EnterpriseAuditEvent records one enterprise mutation and the fields it
// changed. EnterpriseID is filled by the repository for creations.
type EnterpriseAuditEvent struct {
	EnterpriseID  int
	ActorUserID   int
	Action        AuditAction
	ChangedFields map[string]any
	CreatedAt     int64
}

// EnterpriseFilter bounds and filters a cursor-paginated enterprise query.
//
// Cursor is the last seen enterprises.id; zero starts from the first row.
// Every enum and string filter is ignored when empty. OwnerID restricts results
// to one owner when non-zero; zero selects every owner and must only be used
// for admin scope.
type EnterpriseFilter struct {
	Search               string
	District             string
	Status               EnterpriseStatus
	BusinessSector       BusinessSector
	LegalStatus          LegalStatus
	BusinessDigitization BusinessDigitization
	InterventionNeeds    InterventionNeeds
	TrainingStatus       ProcessStatus
	MentoringStatus      ProcessStatus
	CapitalAccess        GeneralStatus
	Partnership          GeneralStatus
	OwnerID              int
	Cursor               int
	Limit                int
}

// PublicEnterprise is a public showcase view of an active enterprise.
type PublicEnterprise struct {
	ID                int
	PublicID          string
	EnterpriseName    string
	OwnerFullName     string
	BusinessSector    BusinessSector
	District          string
	Description       string
	FocusCommodity    string
	DisporaSupport    string
	InterventionNeeds InterventionNeeds
	CreatedAt         int64
}

// PublicEnterpriseFilter defines search, filter, and pagination criteria for
// public enterprise discovery.
type PublicEnterpriseFilter struct {
	Search            string
	District          string
	InterventionNeeds InterventionNeeds
	BusinessSector    BusinessSector
	Cursor            int
	Limit             int
}

// EnterpriseAuditEventView represents an enterprise audit event enriched with actor details.
type EnterpriseAuditEventView struct {
	ID            int
	ActorPublicID string
	ActorEmail    string
	ActorName     string
	Action        AuditAction
	ChangedFields map[string]any
	CreatedAt     int64
}

// EnterpriseAuditFilter bounds a cursor-paginated enterprise audit query.
type EnterpriseAuditFilter struct {
	PublicID string
	OwnerID  int // 0 for admin, >0 for member owner-scoping
	Cursor   int
	Limit    int
}

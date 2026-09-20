package entity

// Refresh revocation reasons recorded on refresh_sessions. Only ReasonRotated
// opens the rotation grace window; every other reason is a hard revocation whose
// old-token reuse is treated as a replay.
const (
	// ReasonRotated marks a session consumed by a normal rotation.
	ReasonRotated = "rotated"
	// ReasonLogout marks a session revoked by logout.
	ReasonLogout = "logout"
	// ReasonPasswordChange marks sessions revoked by a password change.
	ReasonPasswordChange = "password_change"
	// ReasonAdminReset marks sessions revoked by an admin password reset.
	ReasonAdminReset = "admin_reset"
	// ReasonDeactivated marks sessions revoked by account deactivation.
	ReasonDeactivated = "deactivated"
	// ReasonReplay marks sessions revoked because a token family replayed.
	ReasonReplay = "replay"
)

// RefreshSession is a persisted refresh token handle. Only the SHA-256 hash of
// the opaque token value is stored; the raw value never touches the database.
//
// FamilyID groups every session derived from one login. Revoking a family
// contains a replay to the affected login chain instead of every session the
// user owns. ReplacementTokenEnc holds the AES-256-GCM sealed replacement token
// for the grace window and is only populated while a rotation is inside grace.
type RefreshSession struct {
	ID                  int
	UserID              int
	FamilyID            string
	TokenHash           string
	ExpiresAt           int64
	CreatedAt           int64
	RevokedAt           *int64
	RevocationReason    string
	GraceUntil          *int64
	ReplacementTokenEnc []byte
	ReplacedByHash      string
}

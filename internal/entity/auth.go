package entity

// RefreshSession is a persisted refresh token handle. Only the SHA-256 hash of
// the opaque token value is stored; the raw value never touches the database.
type RefreshSession struct {
	ID             int
	UserID         int
	TokenHash      string
	ExpiresAt      int64
	CreatedAt      int64
	RevokedAt      *int64
	ReplacedByHash string
}

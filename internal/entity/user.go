package entity

import "time"

// Role identifies the authorization role assigned to a user account.
type Role string

const (
	// RoleAdmin grants administrative access.
	RoleAdmin Role = "admin"
	// RoleMember grants standard member access.
	RoleMember Role = "member"
)

// Gender identifies the gender recorded on a user profile.
type Gender string

const (
	// GenderMale identifies a male profile.
	GenderMale Gender = "male"
	// GenderFemale identifies a female profile.
	GenderFemale Gender = "female"
)

// User is a user account together with its optional profile.
//
// Password is the bcrypt hash stored in the users.password column. It is part
// of the persistence model and must never be serialized to API clients.
type User struct {
	ID        int
	PublicID  string
	Email     string
	Password  string
	Role      Role
	IsActive  bool
	CreatedAt int64
	UpdatedAt int64
	DeletedAt *int64
	Profile   *Profile
}

// Profile holds the optional descriptive fields of a user.
type Profile struct {
	FullName  string
	NIK       string
	BirthDate *time.Time
	Gender    Gender
	District  string
	Phone     string
	Address   string
}

// UserFilter bounds and filters a cursor-paginated user query.
//
// Cursor is the last seen users.id; zero starts from the first row. District
// and Gender are ignored when empty.
type UserFilter struct {
	District string
	Gender   Gender
	Cursor   int
	Limit    int
}

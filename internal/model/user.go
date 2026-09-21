package model

import "github.com/tnnz20/youthpreneur-be/internal/entity"

// CreateUserRequest is the JSON body for creating a user.
type CreateUserRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FullName  string `json:"full_name"`
	NIK       string `json:"nik"`
	BirthDate string `json:"birth_date"`
	Gender    string `json:"gender"`
	District  string `json:"district"`
	Phone     string `json:"phone"`
	Address   string `json:"address"`
}

// UpdateProfileRequest is the JSON body for replacing profile fields. FullName
// is required; other omitted fields are cleared.
type UpdateProfileRequest struct {
	FullName  string `json:"full_name"`
	NIK       string `json:"nik"`
	BirthDate string `json:"birth_date"`
	Gender    string `json:"gender"`
	District  string `json:"district"`
	Phone     string `json:"phone"`
	Address   string `json:"address"`
}

// UpdateStatusRequest is the JSON body for activating or deactivating a user.
type UpdateStatusRequest struct {
	IsActive *bool `json:"is_active"`
}

// ChangePasswordRequest is the JSON body for changing a known password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ResetPasswordRequest is the JSON body for setting a password without the
// current one.
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// ProfileResponse is the JSON representation of a user profile. BirthDate uses
// the ISO 8601 date format (YYYY-MM-DD).
type ProfileResponse struct {
	FullName  string `json:"full_name,omitempty"`
	NIK       string `json:"nik,omitempty"`
	BirthDate string `json:"birth_date,omitempty"`
	Gender    string `json:"gender,omitempty"`
	District  string `json:"district,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Address   string `json:"address,omitempty"`
}

// UserResponse is the JSON representation of a user. It never includes the
// password or its hash.
type UserResponse struct {
	PublicID  string           `json:"public_id"`
	Email     string           `json:"email"`
	Role      string           `json:"role"`
	IsActive  bool             `json:"is_active"`
	CreatedAt int64            `json:"created_at"`
	UpdatedAt int64            `json:"updated_at"`
	Profile   *ProfileResponse `json:"profile,omitempty"`
}

// UserListResponse is one page of users with the cursor for the next page.
type UserListResponse struct {
	Users      []UserResponse `json:"users"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// ErrorResponse is the JSON body returned for failed requests.
type ErrorResponse struct {
	Error string `json:"error"`
}

const birthDateFormat = "2006-01-02"

// ToUserResponse converts an entity.User to a UserResponse.
func ToUserResponse(user entity.User) UserResponse {
	response := UserResponse{
		PublicID:  user.PublicID,
		Email:     user.Email,
		Role:      string(user.Role),
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	if user.Profile != nil {
		profile := &ProfileResponse{
			FullName: user.Profile.FullName,
			NIK:      user.Profile.NIK,
			Gender:   string(user.Profile.Gender),
			District: user.Profile.District,
			Phone:    user.Profile.Phone,
			Address:  user.Profile.Address,
		}
		if user.Profile.BirthDate != nil {
			profile.BirthDate = user.Profile.BirthDate.Format(birthDateFormat)
		}
		response.Profile = profile
	}

	return response
}

// ToUserResponses converts a slice of entity.User to a slice of UserResponse.
func ToUserResponses(users []entity.User) []UserResponse {
	responses := make([]UserResponse, 0, len(users))
	for _, user := range users {
		responses = append(responses, ToUserResponse(user))
	}

	return responses
}

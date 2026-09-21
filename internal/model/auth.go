package model

import "github.com/tnnz20/youthpreneur-be/internal/entity"

// LoginRequest is the JSON body for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// MeProfileResponse is the minimal profile carried by GET /auth/me.
type MeProfileResponse struct {
	FullName string `json:"full_name"`
}

// MeResponse is the minimal safe representation of the current user returned by
// GET /auth/me. It intentionally omits account state, timestamps, credentials,
// and non-essential profile fields.
type MeResponse struct {
	PublicID string             `json:"public_id"`
	Email    string             `json:"email"`
	Role     string             `json:"role"`
	Profile  *MeProfileResponse `json:"profile"`
}

// ToMeResponse maps the current identity to the minimal GET /auth/me shape.
// A nil profile serializes as JSON null.
func ToMeResponse(user entity.User) MeResponse {
	response := MeResponse{
		PublicID: user.PublicID,
		Email:    user.Email,
		Role:     string(user.Role),
	}

	if user.Profile != nil {
		response.Profile = &MeProfileResponse{FullName: user.Profile.FullName}
	}

	return response
}

// Package token issues and validates access JSON Web Tokens and generates
// opaque refresh token values.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
)

// issuer is the expected JWT issuer for access tokens.
const issuer = "youthpreneur-be"

// refreshTokenBytes is the entropy of a generated opaque refresh token.
const refreshTokenBytes = 32

// ErrInvalidToken indicates an access token is missing, malformed, signed with
// the wrong key, or expired.
var ErrInvalidToken = errors.New("token: invalid access token")

// AccessClaims are the signed claims embedded in an access token.
type AccessClaims struct {
	UserID   int         `json:"user_id"`
	PublicID string      `json:"public_id"`
	Role     entity.Role `json:"role"`
	jwt.RegisteredClaims
}

// Service signs and validates access tokens and generates opaque refresh token
// values.
type Service struct {
	secret    []byte
	accessTTL time.Duration
}

// NewService creates a token service that signs with secret and issues access
// tokens valid for accessTTL.
func NewService(secret string, accessTTL time.Duration) *Service {
	return &Service{secret: []byte(secret), accessTTL: accessTTL}
}

// IssueAccess returns a signed HS256 access token for user, issued at now.
func (s *Service) IssueAccess(user entity.User, now time.Time) (string, error) {
	claims := AccessClaims{
		UserID:   user.ID,
		PublicID: user.PublicID,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   user.PublicID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}

	return signed, nil
}

// ParseAccess validates raw and returns its claims. Any failure returns
// ErrInvalidToken so callers cannot distinguish tampering from expiry.
func (s *Service) ParseAccess(raw string) (AccessClaims, error) {
	var claims AccessClaims

	parsed, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}

		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(issuer))
	if err != nil || !parsed.Valid {
		return AccessClaims{}, ErrInvalidToken
	}

	return claims, nil
}

// GenerateRefresh returns a new cryptographically random opaque refresh token.
func (s *Service) GenerateRefresh() (string, error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashRefresh returns the lowercase hex SHA-256 hash stored for a raw refresh
// token. Only the hash is persisted.
func (s *Service) HashRefresh(raw string) string {
	sum := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(sum[:])
}

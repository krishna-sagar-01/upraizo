package dto

import (
	"time"

	"server/internal/models"

	"github.com/google/uuid"
)

// ─── Request DTOs ────────────────────────────────────────────────────────────

// RegisterRequest carries the payload for POST /auth/register.
type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,secure_password"`
}

// LoginRequest carries the payload for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// Google Auth request 
type GoogleAuthRequest struct {
	IDToken string `json:"id_token" validate:"required"`
}

// AdminLoginRequest carries the triple-check payload for POST /admin/login.
type AdminLoginRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required"`
	SecretKey string `json:"secret_key" validate:"required"`
}

// VerifyEmailRequest carries the payload for POST /auth/verify-email.
type VerifyEmailRequest struct {
	Token string `json:"token" validate:"required,min=64,max=64"`
}

// ForgotPasswordRequest carries the payload for POST /auth/forgot-password.
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordRequest carries the payload for POST /auth/reset-password.
type ResetPasswordRequest struct {
	Token    string `json:"token" validate:"required,min=64,max=64"`
	Password string `json:"password" validate:"required,secure_password"`
}

// ChangePasswordRequest carries the payload for PUT /auth/change-password.
// Requires the user to be authenticated (AuthRequired middleware).
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,secure_password"`
}

// ResendVerificationRequest carries the payload for POST /auth/resend-verification.
type ResendVerificationRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// AdminForgotPasswordRequest carries the triple-check payload.
type AdminForgotPasswordRequest struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Phone string `json:"phone" validate:"required"`
}

// AdminResetPasswordRequest carries the token and new password.
type AdminResetPasswordRequest struct {
	Token    string `json:"token" validate:"required"`
	Password string `json:"password" validate:"required,secure_password"`
}

// AdminForgotSecretRequest carries the quad-check payload.
type AdminForgotSecretRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Phone    string `json:"phone" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// AdminResetSecretRequest carries the token and new secret key.
type AdminResetSecretRequest struct {
	Token        string `json:"token" validate:"required"`
	NewSecretKey string `json:"new_secret_key" validate:"required,min=8"`
}

// UpdateAdminProfileRequest carries optional updates for an admin profile.
type UpdateAdminProfileRequest struct {
	Name  *string `json:"name" validate:"omitempty,min=2,max=50"`
	Phone *string `json:"phone" validate:"omitempty,min=10,max=15"`
}

// ─── Response DTOs ───────────────────────────────────────────────────────────

// SafeUser is the public-facing user representation.
// It deliberately omits PasswordHash and AuthProviderID
// so that sensitive data never crosses the API boundary.
type SafeUser struct {
	ID           uuid.UUID              `json:"id"`
	Name         string                 `json:"name"`
	Email        string                 `json:"email"`
	AvatarURL    *string                `json:"avatar_url,omitempty"`
	Status       models.UserStatus      `json:"status"`
	AuthProvider models.AuthProvider    `json:"auth_provider"`
	IsVerified   bool                   `json:"is_verified"`
	VerifiedAt   *time.Time             `json:"verified_at,omitempty"`
	Preferences  models.UserPreferences `json:"preferences"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// SafeAdmin is the public-facing admin representation.
type SafeAdmin struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AuthResponse is returned by Register, Login, and RefreshTokens.
// AccessToken goes in the body; the refresh token is set as an HttpOnly cookie
// by the handler layer.
type AuthResponse struct {
	User        SafeUser `json:"user"`
	AccessToken string   `json:"access_token"`
	ExpiresAt   int64    `json:"expires_at"`
}

// AdminAuthResponse is specifically for administrative login.
type AdminAuthResponse struct {
	Admin       SafeAdmin `json:"admin"`
	AccessToken string    `json:"access_token"`
	ExpiresAt   int64     `json:"expires_at"`
}

// MessageResponse is a generic success-message envelope used by endpoints
// that do not return entity data (verify email, forgot password, etc.).
type MessageResponse struct {
	Message string `json:"message"`
}

// ─── Converters ──────────────────────────────────────────────────────────────

// ToSafeUser strips sensitive fields from a models.User and returns
// a SafeUser suitable for API responses.
func ToSafeUser(u *models.User) SafeUser {
	return SafeUser{
		ID:           u.ID,
		Name:         u.Name,
		Email:        u.Email,
		AvatarURL:    u.AvatarURL,
		Status:       u.Status,
		AuthProvider: u.AuthProvider,
		IsVerified:   u.IsVerified,
		VerifiedAt:   u.VerifiedAt,
		Preferences:  u.Preferences,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

// ToSafeAdmin converts models.Admin to its public DTO.
func ToSafeAdmin(a *models.Admin) SafeAdmin {
	return SafeAdmin{
		ID:        a.ID,
		Name:      a.Name,
		Email:     a.Email,
		Phone:     a.Phone,
		IsActive:  a.IsActive,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

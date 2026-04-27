package models

import (
	"time"

	"github.com/google/uuid"
)

// ─── Enum Types ───────────────────────────────────────────────────────────────

// UserStatus mirrors the user_status PostgreSQL enum.
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusInactive  UserStatus = "inactive" // default — pending email verification
	UserStatusBanned    UserStatus = "banned"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusDeleted   UserStatus = "deleted"
)

// IsValid reports whether the UserStatus value is a known enum member.
func (s UserStatus) IsValid() bool {
	switch s {
	case UserStatusActive, UserStatusInactive, UserStatusBanned, UserStatusSuspended, UserStatusDeleted:
		return true
	}
	return false
}

// AuthProvider mirrors the auth_provider PostgreSQL enum.
type AuthProvider string

const (
	AuthProviderEmail  AuthProvider = "email"
	AuthProviderGoogle AuthProvider = "google"
	AuthProviderGitHub AuthProvider = "github"
)

// IsValid reports whether the AuthProvider value is a known enum member.
func (p AuthProvider) IsValid() bool {
	switch p {
	case AuthProviderEmail, AuthProviderGoogle, AuthProviderGitHub:
		return true
	}
	return false
}

// IsOAuth reports true for any provider that is not email/password.
func (p AuthProvider) IsOAuth() bool {
	return p != AuthProviderEmail
}

// ─── Preferences (JSONB) ─────────────────────────────────────────────────────

// NotificationPrefs holds per-channel notification toggles.
type NotificationPrefs struct {
	Email   bool `json:"email"`
	Website bool `json:"website"`
}

// UserPreferences maps to the preferences JSONB column.
type UserPreferences struct {
	Notifications NotificationPrefs `json:"notifications"`
	Theme         string            `json:"theme"` // "light" | "dark"
}

// DefaultPreferences returns the same defaults defined in the migration.
func DefaultPreferences() UserPreferences {
	return UserPreferences{
		Notifications: NotificationPrefs{Email: true, Website: true},
		Theme:         "light",
	}
}

// ─── Model ───────────────────────────────────────────────────────────────────

// User represents a single row in the users table.
//
// Nullable columns are modelled as pointer types so that an absent value is
// unambiguously nil — never confused with the zero-value of the base type.
//
// PasswordHash is excluded from JSON serialisation; it must never leave the
// service boundary.
type User struct {
	ID             uuid.UUID       `json:"id"`
	Name           string          `json:"name"`
	AvatarURL      *string         `json:"avatar_url,omitempty"`
	Email          string          `json:"email"`
	PasswordHash   *string         `json:"-"` // excluded from all JSON output
	AuthProvider   AuthProvider    `json:"auth_provider"`
	AuthProviderID *string         `json:"auth_provider_id,omitempty"`
	Status         UserStatus      `json:"status"`
	StatusReason   *string         `json:"status_reason,omitempty"`
	IsVerified     bool            `json:"is_verified"`
	VerifiedAt     *time.Time      `json:"verified_at,omitempty"`
	Preferences    UserPreferences `json:"preferences"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ─── Derived State Helpers ────────────────────────────────────────────────────

// IsActive reports whether the user account is in the active state.
func (u *User) IsActive() bool { return u.Status == UserStatusActive }

// IsBanned reports whether the user has been permanently banned.
func (u *User) IsBanned() bool { return u.Status == UserStatusBanned }

// IsSuspended reports whether the user is temporarily suspended.
func (u *User) IsSuspended() bool { return u.Status == UserStatusSuspended }

// IsOAuth reports true when the user authenticated via an external provider.
func (u *User) IsOAuth() bool { return u.AuthProvider.IsOAuth() }

// CanLogin reports whether the user is allowed to authenticate.
// Banned and suspended users must not receive tokens.
func (u *User) CanLogin() bool {
	return u.IsVerified && u.Status == UserStatusActive
}
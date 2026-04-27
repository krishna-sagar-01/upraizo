package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"server/internal/utils"

	"golang.org/x/crypto/bcrypt"
	"github.com/google/uuid"
)

// ─── Password Hashing (Bcrypt) ───────────────────────────────────────────────

// HashPassword creates a secure bcrypt hash from a plain text password.
// Cost 12 is a good balance between security and speed for production.
func HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		utils.Error("Failed to hash password", err, nil)
		return "", utils.Internal(err)
	}
	return string(hashedBytes), nil
}

// VerifyPassword compares a bcrypt hashed password with its plain-text version.
// It returns true if they match, false otherwise.
func VerifyPassword(hashedPassword, plainPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
	return err == nil
}

// ─── Secure Token Generation ──────────────────────────────────────────────────

// GenerateRandomToken generates a cryptographically secure random hex string.
// length is the number of random bytes (e.g., 32 bytes -> 64 characters hex).
func GenerateRandomToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		utils.Error("Failed to generate random bytes", err, nil)
		return "", utils.Internal(err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateVerificationToken is a semantic wrapper for email verification tokens.
func GenerateVerificationToken() (string, error) {
	return GenerateRandomToken(32) // 64 chars long secure token
}

// GeneratePasswordResetToken is a semantic wrapper for reset tokens.
func GeneratePasswordResetToken() (string, error) {
	return GenerateRandomToken(32)
}

// GenerateSessionID generates a unique identifier for sessions/refresh tokens.
func GenerateSessionID() (string, error) {
	return GenerateRandomToken(24) // 48 chars long secure ID
}

// ─── Verification Logic Helpers ───────────────────────────────────────────────

// FormatTokenKey: Helper to create consistent Redis keys if needed outside repo.
func FormatTokenKey(prefix, token string) string {
	return fmt.Sprintf("%s%s", prefix, token)
}

// Generate new id
func NEWID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.New()
	}
	return id
}
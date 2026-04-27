package dto

import "time"

// SessionMetadata stores rich info about the user's connection
type SessionMetadata struct {
	ID           string    `json:"id"`            // Session ID (JWT JTI)
	UserID       string    `json:"user_id"`
	RefreshToken string    `json:"refresh_token"` // The actual refresh token string
	
	// Device & Browser Info
	IPAddress    string    `json:"ip_address"`
	Browser      string    `json:"browser"`       // e.g., "Chrome", "Firefox"
	OS           string    `json:"os"`            // e.g., "Windows 11", "iOS"
	DeviceName   string    `json:"device_name"`   // e.g., "iPhone 15", "Desktop"
	
	// Location Info (IP se resolve ki hui)
	City         string    `json:"city"`          // e.g., "Delhi"
	Country      string    `json:"country"`       // e.g., "India"
	
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// SafeSession is the DTO sent to the frontend for security.
// Refresh token is omitted, and 'is_current' is added to highlight the current device.
type SafeSession struct {
	ID         string    `json:"id"`
	IPAddress  string    `json:"ip_address"`
	Browser    string    `json:"browser"`
	OS         string    `json:"os"`
	DeviceName string    `json:"device_name"`
	City       string    `json:"city"`
	Country    string    `json:"country"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	IsCurrent  bool      `json:"is_current"`
}
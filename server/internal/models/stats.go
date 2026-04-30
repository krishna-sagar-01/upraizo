package models

import "time"

// PlatformStats maps to the singleton row in the platform_stats table.
type PlatformStats struct {
	ID             int       `json:"id"`
	TotalUsers     int64     `json:"total_users"`
	ActiveUsers    int64     `json:"active_users"`
	InactiveUsers  int64     `json:"inactive_users"`
	SuspendedUsers int64     `json:"suspended_users"`
	BannedUsers    int64     `json:"banned_users"`
	UpdatedAt      time.Time `json:"updated_at"`
}

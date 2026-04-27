package dto

import (
	"server/internal/models"
)

// UpdateProfileRequest carries the payload for PUT /user/profile.
// All fields are pointers to support true partial updates (differentiates between 
// missing field and zero-value).
type UpdateProfileRequest struct {
	Name          *string                  `json:"name" validate:"omitempty,min=2,max=50"`
	Theme         *string                  `json:"theme" validate:"omitempty,oneof=light dark system"`
	Notifications *models.NotificationPrefs `json:"notifications" validate:"omitempty"`
}

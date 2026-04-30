package dto

import "server/internal/models"

// AdminUserUpdateStatusRequest carries the status change payload.
type AdminUserUpdateStatusRequest struct {
	Status models.UserStatus `json:"status" validate:"required"`
	Reason string            `json:"reason" validate:"required_if=Status banned,required_if=Status suspended"`
}

// AdminUserListResponse is the paginated envelope for user management.
type AdminUserListResponse struct {
	Items      []SafeUser `json:"items"`
	Total      int64      `json:"total"`
	Page       int        `json:"page"`
	Limit      int        `json:"limit"`
	TotalPages int        `json:"total_pages"`
}

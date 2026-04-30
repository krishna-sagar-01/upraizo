package dto

import (
	"server/internal/models"
	"time"

	"github.com/google/uuid"
)

type CreateModuleRequest struct {
	CourseID   uuid.UUID `json:"course_id" validate:"required"`
	Title      string    `json:"title" validate:"required,min=3,max=100"`
	OrderIndex int       `json:"order_index"`
}

type UpdateModuleRequest struct {
	Title      *string `json:"title" validate:"omitempty,min=3,max=100"`
	OrderIndex *int    `json:"order_index"`
}

type ModuleResponse struct {
	ID         uuid.UUID `json:"id"`
	CourseID   uuid.UUID `json:"course_id"`
	Title      string    `json:"title"`
	OrderIndex int       `json:"order_index"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func ToModuleResponse(m *models.Module) ModuleResponse {
	return ModuleResponse{
		ID:         m.ID,
		CourseID:   m.CourseID,
		Title:      m.Title,
		OrderIndex: m.OrderIndex,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}

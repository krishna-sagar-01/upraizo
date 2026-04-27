package dto

import (
	"server/internal/models"
	"time"

	"github.com/google/uuid"
)

type CreateTicketRequest struct {
	Subject     string                  `json:"subject" validate:"required,min=5,max=200"`
	Category    string                  `json:"category" validate:"required"`
	Priority    models.TicketPriority `json:"priority" validate:"required"`
	Message     string                  `json:"message" validate:"required,min=1"`
	Metadata    models.TicketMetadata   `json:"metadata"`
	Attachments models.TicketAttachments `json:"attachments"`
}

type AddMessageRequest struct {
	Message     string                  `json:"message" validate:"required,min=1"`
	Attachments models.TicketAttachments `json:"attachments"`
}

type UpdateTicketStatusRequest struct {
	Status models.TicketStatus `json:"status" validate:"required"`
}

type TicketResponse struct {
	ID        uuid.UUID            `json:"id"`
	UserID    uuid.UUID            `json:"user_id"`
	Subject   string               `json:"subject"`
	Category  string               `json:"category"`
	Priority  models.TicketPriority `json:"priority"`
	Status    models.TicketStatus   `json:"status"`
	Metadata  models.TicketMetadata  `json:"metadata"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

type TicketMessageResponse struct {
	ID          uuid.UUID               `json:"id"`
	TicketID    uuid.UUID               `json:"ticket_id"`
	UserID      *uuid.UUID              `json:"user_id,omitempty"`
	AdminID     *uuid.UUID              `json:"admin_id,omitempty"`
	Message     string                  `json:"message"`
	Attachments models.TicketAttachments `json:"attachments"`
	CreatedAt   time.Time               `json:"created_at"`
}

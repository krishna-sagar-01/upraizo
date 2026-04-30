package models

import (
	"time"

	"github.com/google/uuid"
)

// ─── Enum Types ───────────────────────────────────────────────────────────────

// TicketStatus mirrors the ticket_status PostgreSQL enum.
type TicketStatus string

const (
	TicketStatusOpen       TicketStatus = "open"
	TicketStatusInProgress TicketStatus = "in_progress"
	TicketStatusResolved   TicketStatus = "resolved"
	TicketStatusClosed     TicketStatus = "closed"
)

// IsValid reports whether the TicketStatus value is a known enum member.
func (s TicketStatus) IsValid() bool {
	switch s {
	case TicketStatusOpen, TicketStatusInProgress, TicketStatusResolved, TicketStatusClosed:
		return true
	}
	return false
}

// TicketPriority mirrors the ticket_priority PostgreSQL enum.
type TicketPriority string

const (
	TicketPriorityLow    TicketPriority = "low"
	TicketPriorityMedium TicketPriority = "medium"
	TicketPriorityHigh   TicketPriority = "high"
	TicketPriorityUrgent TicketPriority = "urgent"
)

// IsValid reports whether the TicketPriority value is a known enum member.
func (p TicketPriority) IsValid() bool {
	switch p {
	case TicketPriorityLow, TicketPriorityMedium, TicketPriorityHigh, TicketPriorityUrgent:
		return true
	}
	return false
}

// ─── JSONB Types ──────────────────────────────────────────────────────────────

// TicketMetadata maps to the metadata JSONB column.
// Flexible storage for client-side details like Browser, OS version, etc.
type TicketMetadata map[string]any

// DefaultTicketMetadata returns an empty JSON object representation.
func DefaultTicketMetadata() TicketMetadata {
	return make(TicketMetadata)
}

// ─── Models ───────────────────────────────────────────────────────────────────

// Ticket represents a single row in the tickets table.
type Ticket struct {
	ID        uuid.UUID      `json:"id"`
	UserID    uuid.UUID      `json:"user_id"`
	Subject   string         `json:"subject"`
	Category  string         `json:"category"`
	Priority  TicketPriority `json:"priority"`
	Status    TicketStatus   `json:"status"`
	Metadata  TicketMetadata `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// TicketMessage represents a single row in the ticket_messages table.
//
// UserID and AdminID use pointer types (*uuid.UUID) because exactly one
// must be null and one must be populated, enforcing the database XOR constraint.
type TicketMessage struct {
	ID          uuid.UUID         `json:"id"`
	TicketID    uuid.UUID         `json:"ticket_id"`
	UserID      *uuid.UUID        `json:"user_id,omitempty"`
	AdminID     *uuid.UUID        `json:"admin_id,omitempty"`
	Message     string            `json:"message"`
	CreatedAt   time.Time         `json:"created_at"`
}

// ─── Derived State Helpers ────────────────────────────────────────────────────

// --- Ticket Helpers ---

// IsActive reports whether the ticket is actively being handled or awaiting action.
func (t *Ticket) IsActive() bool {
	return t.Status == TicketStatusOpen || t.Status == TicketStatusInProgress
}

// IsUrgent is a quick helper to flag critical issues like payment failures.
func (t *Ticket) IsUrgent() bool {
	return t.Priority == TicketPriorityUrgent
}

// --- TicketMessage Helpers ---

// HasValidSender ensures the in-memory struct respects the database constraint:
// chk_ticket_messages_sender (Exactly ONE sender: either User or Admin).
// Use this before saving to DB or validating incoming API payloads.
func (tm *TicketMessage) HasValidSender() bool {
	return (tm.UserID != nil && tm.AdminID == nil) || (tm.AdminID != nil && tm.UserID == nil)
}

// HasContent ensures the message has text.
func (tm *TicketMessage) HasContent() bool {
	return len(tm.Message) > 0
}

// IsFromAdmin is a quick helper to check if support staff sent the message,
// useful for UI rendering (e.g., aligning admin messages to the right).
func (tm *TicketMessage) IsFromAdmin() bool {
	return tm.AdminID != nil
}

// IsFromUser is a quick helper to check if the customer sent the message.
func (tm *TicketMessage) IsFromUser() bool {
	return tm.UserID != nil
}
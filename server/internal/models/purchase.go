package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ─── Enum Types ───────────────────────────────────────────────────────────────

// PurchaseStatus mirrors the purchase_status PostgreSQL enum.
type PurchaseStatus string

const (
	PurchaseStatusPending   PurchaseStatus = "pending"
	PurchaseStatusCompleted PurchaseStatus = "completed"
	PurchaseStatusFailed    PurchaseStatus = "failed"
	PurchaseStatusRefunded  PurchaseStatus = "refunded"
)

// IsValid reports whether the PurchaseStatus value is a known enum member.
func (s PurchaseStatus) IsValid() bool {
	switch s {
	case PurchaseStatusPending, PurchaseStatusCompleted, PurchaseStatusFailed, PurchaseStatusRefunded:
		return true
	}
	return false
}

// ─── Metadata (JSONB) ─────────────────────────────────────────────────────────

// PurchaseMetadata maps to the metadata JSONB column.
// Using map[string]any allows flexible storage for webhooks, errors, and coupons.
// Alternatively, if you have a strict structure later, you can define a dedicated struct here.
type PurchaseMetadata map[string]any

// DefaultPurchaseMetadata returns an empty JSON object representation.
func DefaultPurchaseMetadata() PurchaseMetadata {
	return make(PurchaseMetadata)
}

// ─── Model ───────────────────────────────────────────────────────────────────

// Purchase represents a single row in the purchases table.
//
// Nullable columns (like RazorpayPaymentID and RazorpaySignature) are modelled 
// as pointer types so that an absent value is unambiguously nil.
type Purchase struct {
	ID                uuid.UUID        `json:"id"`
	UserID            uuid.UUID        `json:"user_id"`
	CourseID          *uuid.UUID       `json:"course_id,omitempty"`
	EbookID           *uuid.UUID       `json:"ebook_id,omitempty"`
	
	// Payment Gateway (Razorpay)
	RazorpayOrderID   string           `json:"razorpay_order_id"`
	RazorpayPaymentID *string          `json:"razorpay_payment_id,omitempty"`
	RazorpaySignature *string          `json:"razorpay_signature,omitempty"`
	
	// Pricing snapshot & Financials
	// Note: decimal.Decimal is used for precision.
	AmountPaid        decimal.Decimal  `json:"amount_paid"` 
	Currency          string           `json:"currency"`
	
	// Extensibility
	Metadata          PurchaseMetadata `json:"metadata"`
	
	// Status & Access Window
	Status            PurchaseStatus   `json:"status"`
	ValidFrom         time.Time        `json:"valid_from"`
	ValidUntil        *time.Time       `json:"valid_until,omitempty"`
	
	// Timestamps
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

// ─── Derived State Helpers ────────────────────────────────────────────────────

// IsCompleted reports whether the purchase payment was successful.
func (p *Purchase) IsCompleted() bool { return p.Status == PurchaseStatusCompleted }

// IsPending reports whether the purchase is awaiting payment completion.
func (p *Purchase) IsPending() bool { return p.Status == PurchaseStatusPending }

// IsRefunded reports whether the purchase has been refunded.
func (p *Purchase) IsRefunded() bool { return p.Status == PurchaseStatusRefunded }

// HasPaymentData reports whether the crucial Razorpay payment payload is present.
// This aligns with the PostgreSQL constraint: chk_purchases_completed_data.
func (p *Purchase) HasPaymentData() bool {
	return p.RazorpayPaymentID != nil && p.RazorpaySignature != nil
}

// HasActiveAccess evaluates if the user currently has access to the course based 
// on the current time and purchase status.
func (p *Purchase) HasActiveAccess(now time.Time) bool {
	if !p.IsCompleted() {
		return false
	}
	
	// Check if access has started
	if now.Before(p.ValidFrom) {
		return false
	}

	// If ValidUntil is nil, it's lifetime access
	if p.ValidUntil == nil {
		return true
	}

	// For courses, check expiry: now < valid_until
	return now.Before(*p.ValidUntil)
}
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Course represents a single row in the courses table.
//
// Pricing fields use decimal.Decimal (shopspring/decimal) to avoid
// floating-point rounding errors on monetary values — NUMERIC(10,2) in
// Postgres maps cleanly to this type.
//
// Discount fields are a coherent group: original_price, discount_label, and
// discount_expires_at are either all absent (no discount) or present together.
// The DB enforces this via chk_courses_discount_fields; HasDiscount() reflects
// the same invariant in Go.
type Course struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	Description *string   `json:"description,omitempty"`
	ThumbnailURL *string  `json:"thumbnail_url,omitempty"`
	Category    string    `json:"category"`

	// ── Pricing ──────────────────────────────────────────────────────────────

	// Price is the actual selling price (what the user pays). Always >= 0.
	Price decimal.Decimal `json:"price"`

	// OriginalPrice is the crossed-out MRP. NULL means no active discount.
	// When set, it is always > Price (enforced by chk_courses_discount).
	OriginalPrice  *decimal.Decimal `json:"original_price,omitempty"`
	DiscountLabel  *string          `json:"discount_label,omitempty"`
	DiscountExpiresAt *time.Time    `json:"discount_expires_at,omitempty"`

	// ── Access ───────────────────────────────────────────────────────────────

	// ValidityDays is the number of days of access granted after purchase. Always > 0.
	ValidityDays int `json:"validity_days"`

	// ── State ────────────────────────────────────────────────────────────────

	IsPublished    bool      `json:"is_published"`
	RazorpayItemID *string   `json:"razorpay_item_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ─── Pricing Helpers ─────────────────────────────────────────────────────────

// HasDiscount reports whether this course currently has an active discount
// configured. It does not check expiry — call IsDiscountActive for that.
func (c *Course) HasDiscount() bool {
	return c.OriginalPrice != nil
}

// IsDiscountActive reports whether the discount is configured and has not yet
// expired. A discount with a nil DiscountExpiresAt never expires.
func (c *Course) IsDiscountActive(now time.Time) bool {
	if !c.HasDiscount() {
		return false
	}
	if c.DiscountExpiresAt == nil {
		return true // no expiry set — discount runs indefinitely
	}
	return now.Before(*c.DiscountExpiresAt)
}

// EffectivePrice returns the price the user would actually pay at the given
// time — Price when no discount is active, same as Price regardless either
// way (Price is always the selling price). Included for clarity at call sites.
func (c *Course) EffectivePrice() decimal.Decimal {
	return c.Price
}

// SavedAmount returns how much cheaper the course is versus the original price.
// Returns zero if no discount is configured.
func (c *Course) SavedAmount() decimal.Decimal {
	if !c.HasDiscount() {
		return decimal.Zero
	}
	return c.OriginalPrice.Sub(c.Price)
}

// DiscountPercent returns the percentage saving relative to the original price,
// rounded to the nearest whole number. Returns 0 if no discount is configured
// or if OriginalPrice is zero (guards against division by zero).
func (c *Course) DiscountPercent() int64 {
	if !c.HasDiscount() || c.OriginalPrice.IsZero() {
		return 0
	}
	// ( saved / original ) * 100
	pct := c.SavedAmount().
		Div(*c.OriginalPrice).
		Mul(decimal.NewFromInt(100)).
		Round(0)
	return pct.IntPart()
}

// ─── State Helpers ────────────────────────────────────────────────────────────

// IsDraft reports whether the course is unpublished (draft mode).
func (c *Course) IsDraft() bool { return !c.IsPublished }
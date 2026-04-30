package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Ebook struct {
	ID                uuid.UUID        `json:"id"`
	Title             string           `json:"title"`
	Slug              string           `json:"slug"`
	Description       *string          `json:"description,omitempty"`
	ThumbnailURL       *string          `json:"thumbnail_url,omitempty"`
	FileURL           string           `json:"file_url"`
	Price             decimal.Decimal  `json:"price"`
	OriginalPrice     *decimal.Decimal `json:"original_price,omitempty"`
	DiscountLabel     *string          `json:"discount_label,omitempty"`
	DiscountExpiresAt *time.Time       `json:"discount_expires_at,omitempty"`
	IsPublished       bool             `json:"is_published"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

func (e *Ebook) HasDiscount() bool {
	return e.OriginalPrice != nil
}

func (e *Ebook) IsDiscountActive(now time.Time) bool {
	if !e.HasDiscount() {
		return false
	}
	if e.DiscountExpiresAt == nil {
		return true
	}
	return now.Before(*e.DiscountExpiresAt)
}

package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type CreateEbookRequest struct {
	Title         string          `json:"title" validate:"required,min=3,max=255"`
	Description   *string         `json:"description"`
	Price         decimal.Decimal `json:"price" validate:"required,gte=0"`
	OriginalPrice *decimal.Decimal `json:"original_price"`
	DiscountLabel *string         `json:"discount_label"`
	DiscountExpiresAt *time.Time  `json:"discount_expires_at"`
	IsPublished   bool            `json:"is_published"`
}

type UpdateEbookRequest struct {
	Title         *string          `json:"title"`
	Description   *string          `json:"description"`
	Price         *decimal.Decimal `json:"price"`
	OriginalPrice *decimal.Decimal `json:"original_price"`
	DiscountLabel *string          `json:"discount_label"`
	DiscountExpiresAt *time.Time   `json:"discount_expires_at"`
	IsPublished   *bool            `json:"is_published"`
}

type EbookResponse struct {
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

type PublicEbookResponse struct {
	ID                uuid.UUID        `json:"id"`
	Title             string           `json:"title"`
	Slug              string           `json:"slug"`
	Description       *string          `json:"description,omitempty"`
	ThumbnailURL       *string          `json:"thumbnail_url,omitempty"`
	Price             decimal.Decimal  `json:"price"`
	OriginalPrice     *decimal.Decimal `json:"original_price,omitempty"`
	DiscountLabel     *string          `json:"discount_label,omitempty"`
	DiscountExpiresAt *time.Time       `json:"discount_expires_at,omitempty"`
	IsPublished       bool             `json:"is_published"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

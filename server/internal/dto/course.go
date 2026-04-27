package dto

import (
	"server/internal/models"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CreateCourseRequest represents the payload for creating a new course.
type CreateCourseRequest struct {
	Title             string           `json:"title" form:"title" validate:"required,min=3,max=100"`
	Description       *string          `json:"description" form:"description"`
	ThumbnailURL      *string          `json:"thumbnail_url" form:"thumbnail_url" validate:"omitempty,url"`
	Category          string           `json:"category" form:"category" validate:"required"`
	Price             decimal.Decimal  `json:"price" form:"price" validate:"required,d_gte=0"`
	OriginalPrice     *decimal.Decimal `json:"original_price" form:"original_price" validate:"omitempty,d_gt_field=Price"`
	DiscountLabel     *string          `json:"discount_label" form:"discount_label"`
	DiscountExpiresAt *time.Time       `json:"discount_expires_at" form:"discount_expires_at"`
	ValidityDays      int              `json:"validity_days" form:"validity_days" validate:"required,gt=0"`
	IsPublished       bool             `json:"is_published" form:"is_published"`
	RazorpayItemID    *string          `json:"razorpay_item_id" form:"razorpay_item_id"`
}

// UpdateCourseRequest represents the payload for updating an existing course.
type UpdateCourseRequest struct {
	Title             *string          `json:"title" form:"title" validate:"omitempty,min=3,max=100"`
	Slug              *string          `json:"slug" form:"slug"`
	Description       *string          `json:"description" form:"description"`
	ThumbnailURL      *string          `json:"thumbnail_url" form:"thumbnail_url" validate:"omitempty,url"`
	Category          *string          `json:"category" form:"category"`
	Price             *decimal.Decimal `json:"price" form:"price" validate:"omitempty,d_gte=0"`
	OriginalPrice     *decimal.Decimal `json:"original_price" form:"original_price"`
	DiscountLabel     *string          `json:"discount_label" form:"discount_label"`
	DiscountExpiresAt *time.Time       `json:"discount_expires_at" form:"discount_expires_at"`
	ValidityDays      *int             `json:"validity_days" form:"validity_days" validate:"omitempty,gt=0"`
	IsPublished       *bool            `json:"is_published" form:"is_published"`
	RazorpayItemID    *string          `json:"razorpay_item_id" form:"razorpay_item_id"`
}

// CourseResponse represents the sanitized course data returned to the client.
type CourseResponse struct {
	ID                uuid.UUID        `json:"id"`
	Title             string           `json:"title"`
	Slug              string           `json:"slug"`
	Description       *string          `json:"description,omitempty"`
	ThumbnailURL      *string          `json:"thumbnail_url,omitempty"`
	Category          string           `json:"category"`
	Price             decimal.Decimal  `json:"price"`
	OriginalPrice     *decimal.Decimal `json:"original_price,omitempty"`
	DiscountLabel     *string          `json:"discount_label,omitempty"`
	DiscountExpiresAt *time.Time       `json:"discount_expires_at,omitempty"`
	ValidityDays      int              `json:"validity_days"`
	IsPublished       bool             `json:"is_published"`
	RazorpayItemID    *string          `json:"razorpay_item_id,omitempty"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

// ToCourseResponse maps a Course model to a CourseResponse DTO.
func ToCourseResponse(c *models.Course) CourseResponse {
	return CourseResponse{
		ID:                c.ID,
		Title:             c.Title,
		Slug:              c.Slug,
		Description:       c.Description,
		ThumbnailURL:      c.ThumbnailURL,
		Category:          c.Category,
		Price:             c.Price,
		OriginalPrice:     c.OriginalPrice,
		DiscountLabel:     c.DiscountLabel,
		DiscountExpiresAt: c.DiscountExpiresAt,
		ValidityDays:      c.ValidityDays,
		IsPublished:       c.IsPublished,
		RazorpayItemID:    c.RazorpayItemID,
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}
}

// PublicCurriculumLesson represents the public view of a lesson.
type PublicCurriculumLesson struct {
	ID              uuid.UUID `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	VideoURL        string    `json:"video_url"`
	OrderIndex      int       `json:"order_index"`
	DurationSeconds *int      `json:"duration_seconds,omitempty"`
	IsPreview       bool            `json:"is_preview"`
	Attachments     []AttachmentDTO `json:"attachments,omitempty"`
}

// PublicCurriculumModule represents the public view of a module with its lessons.
type PublicCurriculumModule struct {
	ID         uuid.UUID                `json:"id"`
	Title      string                   `json:"title"`
	OrderIndex int                      `json:"order_index"`
	Lessons    []PublicCurriculumLesson `json:"lessons"`
}

type PublicCurriculumResponse struct {
	CourseID uuid.UUID                `json:"course_id"`
	Title    string                   `json:"title"`
	Modules  []PublicCurriculumModule `json:"modules"`
}

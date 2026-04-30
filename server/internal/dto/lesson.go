package dto

import (
	"server/internal/models"
	"time"

	"github.com/google/uuid"
)

type CreateLessonRequest struct {
	ModuleID   uuid.UUID `json:"module_id" validate:"required"`
	Title      string    `json:"title" validate:"required,min=3,max=150"`
	OrderIndex int       `json:"order_index"`
	IsPreview  bool      `json:"is_preview"`
}

type UpdateLessonRequest struct {
	Title      *string `json:"title" validate:"omitempty,min=3,max=150"`
	OrderIndex *int    `json:"order_index"`
	IsPreview  *bool   `json:"is_preview"`
}

type LessonResponse struct {
	ID              uuid.UUID        `json:"id"`
	ModuleID        uuid.UUID        `json:"module_id"`
	Title           string           `json:"title"`
	OrderIndex      int              `json:"order_index"`
	VideoURL        *string          `json:"video_url,omitempty"`
	VideoStatus     string           `json:"video_status"`
	DurationSeconds *int             `json:"duration_seconds,omitempty"`
	IsPreview       bool             `json:"is_preview"`
	Attachments     []AttachmentDTO  `json:"attachments,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

type AttachmentDTO struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	FileURL   string    `json:"file_url"`
	FileSize  int64     `json:"file_size"`
	MimeType  string    `json:"mime_type"`
	CreatedAt time.Time `json:"created_at"`
}

func ToLessonResponse(l *models.Lesson, videoURL *string, attachments []*models.LessonAttachment) LessonResponse {
	res := LessonResponse{
		ID:              l.ID,
		ModuleID:        l.ModuleID,
		Title:           l.Title,
		OrderIndex:      l.OrderIndex,
		VideoURL:        videoURL,
		VideoStatus:     string(l.VideoStatus),
		DurationSeconds: l.DurationSeconds,
		IsPreview:       l.IsPreview,
		CreatedAt:       l.CreatedAt,
		UpdatedAt:       l.UpdatedAt,
	}

	if len(attachments) > 0 {
		res.Attachments = make([]AttachmentDTO, 0, len(attachments))
		for _, a := range attachments {
			res.Attachments = append(res.Attachments, AttachmentDTO{
				ID:        a.ID,
				Title:     a.Title,
				FileSize:  a.FileSize,
				MimeType:  a.MimeType,
				CreatedAt: a.CreatedAt,
				// FileURL will be populated by service/handler using pre-signed URL logic or CDN URL
			})
		}
	}

	return res
}

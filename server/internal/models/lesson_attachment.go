package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// LessonAttachment represents a single row in the lesson_attachments table.
//
// FileKey is a Cloudflare R2 object key — unique across all attachments
// (uq_lesson_attachments_key). Never expose it directly in API responses;
// generate a pre-signed URL from it server-side.
//
// FileSize is stored in bytes (BIGINT). Use the helper methods for
// human-readable display. Always > 0 — enforced by the DB constraint.
//
// MimeType is validated in the DB via chk_mime_type regex (^[a-z]+/[a-z0-9.+-]+$).
// No updated_at column — attachments are immutable after creation.
type LessonAttachment struct {
	ID        uuid.UUID `json:"id"`
	LessonID  uuid.UUID `json:"lesson_id"`
	Title     string    `json:"title"`
	FileKey   string    `json:"-"`         // R2 key — never serialised; generate signed URL instead
	FileSize  int64     `json:"file_size"` // bytes; always > 0
	MimeType  string    `json:"mime_type"`
	CreatedAt time.Time `json:"created_at"`
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// FileSizeKB returns file size in kilobytes, rounded down.
func (a *LessonAttachment) FileSizeKB() int64 { return a.FileSize / 1024 }

// FileSizeMB returns file size in megabytes, rounded down.
func (a *LessonAttachment) FileSizeMB() int64 { return a.FileSize / (1024 * 1024) }

// HumanSize returns a short, human-readable size string (e.g. "2.4 MB", "340 KB").
func (a *LessonAttachment) HumanSize() string {
	const kb = 1024
	const mb = 1024 * kb

	switch {
	case a.FileSize >= mb:
		return fmt.Sprintf("%.1f MB", float64(a.FileSize)/float64(mb))
	case a.FileSize >= kb:
		return fmt.Sprintf("%.1f KB", float64(a.FileSize)/float64(kb))
	default:
		return fmt.Sprintf("%d B", a.FileSize)
	}
}

// IsPDF reports whether the attachment is a PDF document.
func (a *LessonAttachment) IsPDF() bool { return a.MimeType == "application/pdf" }
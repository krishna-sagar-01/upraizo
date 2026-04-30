package models

import (
	"time"

	"github.com/google/uuid"
)

// ─── Enum Type ────────────────────────────────────────────────────────────────

type VideoStatus string

const (
	VideoStatusPending    VideoStatus = "pending"    // no video_key yet
	VideoStatusProcessing VideoStatus = "processing" // transcoding in progress
	VideoStatusReady      VideoStatus = "ready"      // available for viewing
	VideoStatusFailed     VideoStatus = "failed"     // transcoding failed
)

// IsValid reports whether the value is a known enum member.
func (s VideoStatus) IsValid() bool {
	switch s {
	case VideoStatusPending, VideoStatusProcessing, VideoStatusReady, VideoStatusFailed:
		return true
	}
	return false
}

// IsTerminal reports whether the status will not change via normal processing.
// Both ready and failed are terminal; pending and processing are transient.
func (s VideoStatus) IsTerminal() bool {
	return s == VideoStatusReady || s == VideoStatusFailed
}

// ─── Model ───────────────────────────────────────────────────────────────────

// Lesson represents a single row in the lessons table.
//
// VideoKey and VideoStatus are a coherent pair enforced by chk_lessons_video:
//   - VideoKey == nil  →  VideoStatus must be pending
//   - VideoKey != nil  →  VideoStatus must be processing | ready | failed
//
// DurationSeconds is NULL until FFmpeg transcoding sets it after a successful
// upload — do not treat zero as "no duration"; use the pointer.
type Lesson struct {
	ID              uuid.UUID   `json:"id"`
	ModuleID        uuid.UUID   `json:"module_id"`
	Title           string      `json:"title"`
	OrderIndex      int         `json:"order_index"` // zero-based; unique within module
	VideoKey        *string     `json:"video_key,omitempty"`
	VideoStatus     VideoStatus `json:"video_status"`
	DurationSeconds *int        `json:"duration_seconds,omitempty"` // set post-transcode
	IsPreview       bool        `json:"is_preview"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

// ─── State Helpers ────────────────────────────────────────────────────────────

// IsWatchable reports whether the lesson video is ready to stream.
func (l *Lesson) IsWatchable() bool { return l.VideoStatus == VideoStatusReady }

// IsProcessing reports whether the video is currently being transcoded.
func (l *Lesson) IsProcessing() bool { return l.VideoStatus == VideoStatusProcessing }

// HasFailed reports whether video transcoding failed for this lesson.
func (l *Lesson) HasFailed() bool { return l.VideoStatus == VideoStatusFailed }

// IsUploaded reports whether a video file has been attached to this lesson,
// regardless of transcoding outcome.
func (l *Lesson) IsUploaded() bool { return l.VideoKey != nil }

// IsAccessibleWithout Purchase reports whether the lesson can be watched
// without buying the course (free preview flag).
func (l *Lesson) IsFreePreview() bool { return l.IsPreview }

// DurationMinutes returns duration as whole minutes, or 0 if not yet known.
// Useful for display strings; use DurationSeconds for precise calculations.
func (l *Lesson) DurationMinutes() int {
	if l.DurationSeconds == nil {
		return 0
	}
	return *l.DurationSeconds / 60
}
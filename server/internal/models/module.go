package models

import (
	"time"

	"github.com/google/uuid"
)

// Module represents a single row in the modules table.
//
// Modules are ordered sections within a course. OrderIndex is zero-based and
// unique per course — enforced by uq_modules_course_order in the DB.
// The (course_id, order_index) pair is the natural sort key for display.
type Module struct {
	ID         uuid.UUID `json:"id"`
	CourseID   uuid.UUID `json:"course_id"`
	Title      string    `json:"title"`
	OrderIndex int       `json:"order_index"` // zero-based; unique within course
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// IsFirst reports whether this module is the first in its course.
func (m *Module) IsFirst() bool { return m.OrderIndex == 0 }
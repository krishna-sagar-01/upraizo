package dto

import "github.com/shopspring/decimal"

type StudentDashboardSummary struct {
	TotalInvestment decimal.Decimal `json:"total_investment"`
	EnrolledCourses int             `json:"enrolled_courses"`
	OverallProgress float64         `json:"overall_progress"`
}

type MyCourseResponse struct {
	ID                 string  `json:"id"`
	Slug               string  `json:"slug"`
	Title              string  `json:"title"`
	Category           string  `json:"category"`
	ThumbnailUrl       string  `json:"thumbnail_url"`
	InstructorName     string  `json:"instructor_name"`
	ProgressPercentage float64 `json:"progress_percentage"`
	TotalLessons       int     `json:"total_lessons"`
	CompletedLessons   int     `json:"completed_lessons"`
	ValidUntil         *string `json:"valid_until,omitempty"`
}

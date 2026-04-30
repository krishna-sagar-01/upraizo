package service

import (
	"context"
	"server/internal/dto"
	"server/internal/repository"
	"server/internal/utils"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"time"
)

type StudentDashboardService struct {
	purchaseRepo *repository.PurchaseRepository
	courseRepo   *repository.CourseRepository
	ebookRepo    *repository.EbookRepository
}

func NewStudentDashboardService(
	purchaseRepo *repository.PurchaseRepository,
	courseRepo *repository.CourseRepository,
	ebookRepo *repository.EbookRepository,
) *StudentDashboardService {
	return &StudentDashboardService{
		purchaseRepo: purchaseRepo,
		courseRepo:   courseRepo,
		ebookRepo:    ebookRepo,
	}
}

func (s *StudentDashboardService) GetSummary(ctx context.Context, userID uuid.UUID) (*dto.StudentDashboardSummary, error) {
	// 1. Get Investment
	totalInvestment, err := s.purchaseRepo.GetTotalInvestment(ctx, userID)
	if err != nil {
		utils.Error("Failed to fetch student investment", err, map[string]any{"user_id": userID})
		totalInvestment = decimal.Zero
	}

	// 2. Get Enrolled IDs (Total count of items)
	enrollments, err := s.purchaseRepo.GetActiveEnrollments(ctx, userID)
	if err != nil {
		return nil, utils.Internal(err)
	}

	return &dto.StudentDashboardSummary{
		TotalInvestment: totalInvestment,
		EnrolledCourses: len(enrollments),
		OverallProgress: 0,
	}, nil
}

func (s *StudentDashboardService) GetMyCourses(ctx context.Context, userID uuid.UUID) ([]dto.MyCourseResponse, error) {
	enrollments, err := s.purchaseRepo.GetActiveEnrollments(ctx, userID)
	if err != nil {
		return nil, utils.Internal(err)
	}

	if len(enrollments) == 0 {
		return []dto.MyCourseResponse{}, nil
	}

	var results []dto.MyCourseResponse
	for _, rec := range enrollments {
		// Only process Courses here. Ebooks are handled by EbookService.GetMyEbooks
		if rec.CourseID == nil {
			continue
		}

		course, err := s.courseRepo.GetByID(ctx, *rec.CourseID)
		if err != nil || course == nil {
			continue
		}

		thumbnail := ""
		if course.ThumbnailURL != nil {
			thumbnail = *course.ThumbnailURL
		}

		res := dto.MyCourseResponse{
			ID:             course.ID.String(),
			Slug:           course.Slug,
			Title:          course.Title,
			Category:       course.Category,
			ThumbnailUrl:   thumbnail,
			InstructorName: "Upraizo Instructor",
		}

		if rec.ValidUntil != nil {
			s := rec.ValidUntil.Format(time.RFC3339)
			res.ValidUntil = &s
		}

		results = append(results, res)
	}

	return results, nil
}

package service

import (
	"context"
	"server/internal/dto"
	"server/internal/repository"
	"server/internal/utils"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type StudentDashboardService struct {
	purchaseRepo *repository.PurchaseRepository
	courseRepo   *repository.CourseRepository
}

func NewStudentDashboardService(
	purchaseRepo *repository.PurchaseRepository,
	courseRepo *repository.CourseRepository,
) *StudentDashboardService {
	return &StudentDashboardService{
		purchaseRepo: purchaseRepo,
		courseRepo:   courseRepo,
	}
}

func (s *StudentDashboardService) GetSummary(ctx context.Context, userID uuid.UUID) (*dto.StudentDashboardSummary, error) {
	// 1. Get Investment
	totalInvestment, err := s.purchaseRepo.GetTotalInvestment(ctx, userID)
	if err != nil {
		utils.Error("Failed to fetch student investment", err, map[string]any{"user_id": userID})
		totalInvestment = decimal.Zero
	}

	// 2. Get Enrolled IDs
	enrolledIDs, err := s.purchaseRepo.GetEnrolledCourseIDs(ctx, userID)
	if err != nil {
		return nil, utils.Internal(err)
	}


	return &dto.StudentDashboardSummary{
		TotalInvestment: totalInvestment,
		EnrolledCourses: len(enrolledIDs),
		OverallProgress: 0,
	}, nil
}

func (s *StudentDashboardService) GetMyCourses(ctx context.Context, userID uuid.UUID) ([]dto.MyCourseResponse, error) {
	enrolledIDs, err := s.purchaseRepo.GetEnrolledCourseIDs(ctx, userID)
	if err != nil {
		return nil, utils.Internal(err)
	}

	if len(enrolledIDs) == 0 {
		return []dto.MyCourseResponse{}, nil
	}

	var results []dto.MyCourseResponse
	for _, courseID := range enrolledIDs {
		course, err := s.courseRepo.GetByID(ctx, courseID)
		if err != nil {
			continue
		}


		// Handle nullable thumbnail pointer
		thumbnail := ""
		if course.ThumbnailURL != nil {
			thumbnail = *course.ThumbnailURL
		}

		results = append(results, dto.MyCourseResponse{
			ID:                 course.ID.String(),
			Slug:               course.Slug,
			Title:              course.Title,
			ThumbnailUrl:       thumbnail,
			InstructorName:     "Upraizo Instructor", 
			ProgressPercentage: 0,
			TotalLessons:       0,
			CompletedLessons:   0,
		})
	}

	return results, nil
}

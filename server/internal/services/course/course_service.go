package service

import (
	"context"
	"fmt"
	"regexp"
	"server/internal/config"
	"server/internal/dto"
	"server/internal/models"
	"server/internal/queue"
	"server/internal/repository"
	"server/internal/storage"
	"server/internal/utils"
	"strings"
	"time"

	"github.com/google/uuid"
)

type CourseService struct {
	repo         *repository.CourseRepository
	purchaseRepo *repository.PurchaseRepository
	queueMgr     *queue.Manager
	r2           *storage.R2Client
	cfg          *config.Config
}

func NewCourseService(
	repo *repository.CourseRepository, 
	purchaseRepo *repository.PurchaseRepository,
	queueMgr *queue.Manager, 
	r2 *storage.R2Client,
	cfg *config.Config,
) *CourseService {
	return &CourseService{
		repo:         repo,
		purchaseRepo: purchaseRepo,
		queueMgr:     queueMgr,
		r2:           r2,
		cfg:          cfg,
	}
}

func (s *CourseService) Config() *config.Config {
	return s.cfg
}

// ───────────────── READ ─────────────────

func (s *CourseService) GetByID(ctx context.Context, id uuid.UUID) (dto.CourseResponse, error) {
	course, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return dto.CourseResponse{}, utils.Internal(err)
	}
	if course == nil {
		return dto.CourseResponse{}, utils.NotFound("Course not found")
	}
	return dto.ToCourseResponse(course), nil
}

func (s *CourseService) GetBySlug(ctx context.Context, slug string) (dto.CourseResponse, error) {
	course, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return dto.CourseResponse{}, utils.Internal(err)
	}
	if course == nil {
		return dto.CourseResponse{}, utils.NotFound("Course not found")
	}
	return dto.ToCourseResponse(course), nil
}

func (s *CourseService) GetCurriculum(ctx context.Context, slug string, userID *uuid.UUID) (*dto.PublicCurriculumResponse, error) {
	course, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, utils.Internal(err)
	}
	if course == nil {
		return nil, utils.NotFound("Course not found")
	}

	// 1. Check if user has purchased the course
	isPurchased := false
	if userID != nil {
		has, _ := s.purchaseRepo.HasPurchasedCourse(ctx, *userID, course.ID)
		isPurchased = has
	}

	modules, err := s.repo.GetStudentCurriculum(ctx, course.ID)
	if err != nil {
		return nil, utils.Internal(err)
	}

	// 2. Post-Process URLs with Security (Presigned URLs for purchased/preview, Masked for others)
	for mIdx := range modules {
		for lIdx := range modules[mIdx].Lessons {
			lesson := &modules[mIdx].Lessons[lIdx]
			canAccess := isPurchased || lesson.IsPreview

			if lesson.VideoURL != "" {
				if canAccess {
					cdn := strings.TrimSuffix(s.r2.CDN, "/")
					lesson.VideoURL = cdn + "/" + strings.TrimPrefix(lesson.VideoURL, "/")
				} else {
					lesson.VideoURL = ""
				}
			}

			// Handle attachments (Direct CDN links, no expiry)
			for aIdx := range lesson.Attachments {
				attachment := &lesson.Attachments[aIdx]
				if canAccess && attachment.FileURL != "" {
					cdn := strings.TrimSuffix(s.r2.CDN, "/")
					attachment.FileURL = cdn + "/" + strings.TrimPrefix(attachment.FileURL, "/")
				} else {
					attachment.FileURL = ""
				}
			}
		}
	}

	return &dto.PublicCurriculumResponse{
		CourseID: course.ID,
		Title:    course.Title,
		Modules:  modules,
	}, nil
}

func (s *CourseService) List(ctx context.Context, onlyPublished bool) ([]dto.CourseResponse, error) {
	courses, err := s.repo.List(ctx, onlyPublished)
	if err != nil {
		return nil, utils.Internal(err)
	}

	res := make([]dto.CourseResponse, 0, len(courses))
	for _, c := range courses {
		res = append(res, dto.ToCourseResponse(c))
	}
	return res, nil
}

func (s *CourseService) QueueThumbnailUpdate(ctx context.Context, courseID uuid.UUID, localPath string) error {
	course, err := s.repo.GetByID(ctx, courseID)
	if err != nil {
		return err
	}

	oldURL := ""
	if course.ThumbnailURL != nil {
		oldURL = *course.ThumbnailURL
	}

	task := queue.CourseThumbnailTask{
		CourseID:        courseID.String(),
		LocalPath:       localPath,
		OldThumbnailURL: oldURL,
	}

	return s.queueMgr.Publish(s.cfg.RabbitMQ.CourseThumbnailQueue, task)
}

// ───────────────── WRITE ─────────────────

func (s *CourseService) Create(ctx context.Context, req dto.CreateCourseRequest) (dto.CourseResponse, error) {
	slug := s.GenerateSlug(req.Title)
	existing, _ := s.repo.GetBySlug(ctx, slug)
	if existing != nil {
		slug = fmt.Sprintf("%s-%s", slug, strings.ToLower(uuid.New().String()[:5]))
	}

	var discountExpiresAt *time.Time
	if req.DiscountExpiresAt != nil && *req.DiscountExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.DiscountExpiresAt)
		if err != nil {
			return dto.CourseResponse{}, utils.BadRequest("Invalid discount expiry date format")
		}
		discountExpiresAt = &t
	}

	course := &models.Course{
		Title:             req.Title,
		Slug:              slug,
		Description:       req.Description,
		ThumbnailURL:      req.ThumbnailURL,
		Category:          req.Category,
		Price:             req.Price,
		OriginalPrice:     req.OriginalPrice,
		DiscountLabel:     req.DiscountLabel,
		DiscountExpiresAt: discountExpiresAt,
		ValidityDays:      req.ValidityDays,
		IsPublished:       req.IsPublished,
		RazorpayItemID:    req.RazorpayItemID,
	}

	if err := s.repo.Create(ctx, course); err != nil {
		return dto.CourseResponse{}, utils.Internal(err)
	}

	return dto.ToCourseResponse(course), nil
}

func (s *CourseService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateCourseRequest) (dto.CourseResponse, error) {
	course, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return dto.CourseResponse{}, utils.Internal(err)
	}
	if course == nil {
		return dto.CourseResponse{}, utils.NotFound("Course not found")
	}

	updated := false

	if req.Title != nil && *req.Title != course.Title {
		course.Title = *req.Title
		updated = true
	}

	if req.Slug != nil && *req.Slug != course.Slug {
		course.Slug = s.GenerateSlug(*req.Slug)
		updated = true
	}

	if req.Description != nil { course.Description = req.Description; updated = true }
	if req.ThumbnailURL != nil { course.ThumbnailURL = req.ThumbnailURL; updated = true }
	if req.Category != nil { course.Category = *req.Category; updated = true }
	if req.RazorpayItemID != nil { course.RazorpayItemID = req.RazorpayItemID; updated = true }
	if req.ValidityDays != nil { course.ValidityDays = *req.ValidityDays; updated = true }

	if req.Price != nil { course.Price = *req.Price; updated = true }
	if req.OriginalPrice != nil { course.OriginalPrice = req.OriginalPrice; updated = true }
	if req.DiscountLabel != nil { course.DiscountLabel = req.DiscountLabel; updated = true }
	if req.DiscountExpiresAt != nil {
		if *req.DiscountExpiresAt == "" {
			course.DiscountExpiresAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, *req.DiscountExpiresAt)
			if err != nil {
				return dto.CourseResponse{}, utils.BadRequest("Invalid discount expiry date format")
			}
			course.DiscountExpiresAt = &t
		}
		updated = true
	}

	if req.IsPublished != nil && *req.IsPublished != course.IsPublished {
		course.IsPublished = *req.IsPublished
		updated = true
	}

	if !updated {
		return dto.ToCourseResponse(course), nil
	}

	if err := s.repo.Update(ctx, course); err != nil {
		return dto.CourseResponse{}, utils.Internal(err)
	}

	return dto.ToCourseResponse(course), nil
}

func (s *CourseService) Delete(ctx context.Context, id uuid.UUID, permanent bool) error {
	course, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return utils.Internal(err)
	}
	if course == nil {
		return utils.NotFound("Course not found")
	}

	if permanent {
		// 1. Check for active purchases before permanent delete
		count, err := s.repo.CountDependencies(ctx, id)
		if err != nil {
			return utils.Internal(err)
		}
		if count > 0 {
			return utils.Conflict("Cannot permanently delete course with active students.")
		}

		// 2. Recursive R2 Cleanup
		// 2.1 Thumbnail
		if course.ThumbnailURL != nil {
			_ = s.r2.Delete(*course.ThumbnailURL)
		}

		// 2.2 Lesson Videos & Attachments
		modules, _ := s.repo.GetStudentCurriculum(ctx, id)
		for _, m := range modules {
			for _, l := range m.Lessons {
				if l.VideoURL != "" {
					_ = s.r2.Delete(l.VideoURL)
				}
				for _, a := range l.Attachments {
					_ = s.r2.Delete(a.FileURL)
				}
			}
		}

		return s.repo.PermanentDelete(ctx, id)
	}

	return s.repo.SoftDelete(ctx, id)
}

func (s *CourseService) GenerateSlug(title string) string {
	res := strings.ToLower(title)
	reg := regexp.MustCompile("[^a-z0-9]+")
	res = reg.ReplaceAllString(res, "-")
	return strings.Trim(res, "-")
}

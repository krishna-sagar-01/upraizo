package service

import (
	"context"
	"fmt"
	"regexp"
	"server/internal/dto"
	"server/internal/models"
	"server/internal/queue"
	"server/internal/repository"
	"server/internal/utils"
	"server/internal/config"
	"strings"

	"github.com/google/uuid"
)

type CourseService struct {
	repo     *repository.CourseRepository
	queueMgr *queue.Manager
	cfg      *config.Config
}

func NewCourseService(repo *repository.CourseRepository, queueMgr *queue.Manager, cfg *config.Config) *CourseService {
	return &CourseService{
		repo:     repo,
		queueMgr: queueMgr,
		cfg:      cfg,
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

func (s *CourseService) GetCurriculum(ctx context.Context, slug string) (*dto.PublicCurriculumResponse, error) {
	course, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, utils.Internal(err)
	}
	if course == nil {
		return nil, utils.NotFound("Course not found")
	}

	modules, err := s.repo.GetStudentCurriculum(ctx, course.ID)
	if err != nil {
		return nil, utils.Internal(err)
	}

	// ─── Post-Process Video URLs with R2 Public URL ───
	r2Base := s.cfg.R2.PublicURL
	if r2Base != "" {
		// Ensure r2Base ends with / for cleaner concatenation
		if !strings.HasSuffix(r2Base, "/") {
			r2Base += "/"
		}

		for mIdx := range modules {
			for lIdx := range modules[mIdx].Lessons {
				vURL := modules[mIdx].Lessons[lIdx].VideoURL
				if vURL != "" && !strings.HasPrefix(vURL, "http") {
					// Trim leading slash from vURL if present to avoid //
					vURL = strings.TrimPrefix(vURL, "/")
					modules[mIdx].Lessons[lIdx].VideoURL = r2Base + vURL
				}

				// Prepend R2 URL to attachments
				for aIdx := range modules[mIdx].Lessons[lIdx].Attachments {
					aURL := modules[mIdx].Lessons[lIdx].Attachments[aIdx].FileURL
					if aURL != "" && !strings.HasPrefix(aURL, "http") {
						aURL = strings.TrimPrefix(aURL, "/")
						modules[mIdx].Lessons[lIdx].Attachments[aIdx].FileURL = r2Base + aURL
					}
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

	course := &models.Course{
		Title:             req.Title,
		Slug:              slug,
		Description:       req.Description,
		ThumbnailURL:      req.ThumbnailURL,
		Category:          req.Category,
		Price:             req.Price,
		OriginalPrice:     req.OriginalPrice,
		DiscountLabel:     req.DiscountLabel,
		DiscountExpiresAt: req.DiscountExpiresAt,
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
	detailsChanged := false
	pricingChanged := false

	if req.Title != nil && *req.Title != course.Title {
		course.Title = *req.Title
		updated = true
		detailsChanged = true
	}

	if req.Slug != nil && *req.Slug != course.Slug {
		course.Slug = s.GenerateSlug(*req.Slug)
		updated = true
		detailsChanged = true
	}

	if req.Description != nil { course.Description = req.Description; updated = true; detailsChanged = true }
	if req.ThumbnailURL != nil { course.ThumbnailURL = req.ThumbnailURL; updated = true; detailsChanged = true }
	if req.Category != nil { course.Category = *req.Category; updated = true; detailsChanged = true }
	if req.RazorpayItemID != nil { course.RazorpayItemID = req.RazorpayItemID; updated = true; detailsChanged = true }

	if req.Price != nil { course.Price = *req.Price; updated = true; pricingChanged = true }
	if req.OriginalPrice != nil { course.OriginalPrice = req.OriginalPrice; updated = true; pricingChanged = true }
	if req.DiscountLabel != nil { course.DiscountLabel = req.DiscountLabel; updated = true; pricingChanged = true }
	if req.DiscountExpiresAt != nil { course.DiscountExpiresAt = req.DiscountExpiresAt; updated = true; pricingChanged = true }

	if req.IsPublished != nil && *req.IsPublished != course.IsPublished {
		course.IsPublished = *req.IsPublished
		updated = true
	}

	if !updated {
		return dto.ToCourseResponse(course), nil
	}

	if detailsChanged {
		if err := s.repo.UpdateDetails(ctx, course); err != nil {
			return dto.CourseResponse{}, utils.Internal(err)
		}
	}

	if pricingChanged {
		if err := s.repo.UpdatePricing(ctx, course); err != nil {
			return dto.CourseResponse{}, utils.Internal(err)
		}
	}

	if req.IsPublished != nil && !detailsChanged && !pricingChanged {
		if err := s.repo.UpdateStatus(ctx, id, *req.IsPublished); err != nil {
			return dto.CourseResponse{}, utils.Internal(err)
		}
	}

	return dto.ToCourseResponse(course), nil
}

func (r *CourseService) Delete(ctx context.Context, id uuid.UUID, permanent bool) error {
	if permanent {
		count, err := r.repo.CountDependencies(ctx, id)
		if err != nil {
			return utils.Internal(err)
		}
		
		if count > 0 {
			return utils.Conflict("Course is not empty.")
		}

		return r.repo.PermanentDelete(ctx, id)
	}
	return r.repo.SoftDelete(ctx, id)
}

func (s *CourseService) GenerateSlug(title string) string {
	res := strings.ToLower(title)
	reg := regexp.MustCompile("[^a-z0-9]+")
	res = reg.ReplaceAllString(res, "-")
	return strings.Trim(res, "-")
}

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"server/internal/dto"
	"server/internal/models"
	"server/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CourseRepository struct {
	db *pgxpool.Pool
}

func NewCourseRepository(db *pgxpool.Pool) *CourseRepository {
	return &CourseRepository{db: db}
}

// ───────────────── Helpers ─────────────────

func (r *CourseRepository) withWriteContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func (r *CourseRepository) scanCourse(row pgx.Row) (*models.Course, error) {
	c := &models.Course{}
	err := row.Scan(
		&c.ID, &c.Title, &c.Slug, &c.Description, &c.ThumbnailURL, &c.Category,
		&c.Price, &c.OriginalPrice, &c.DiscountLabel, &c.DiscountExpiresAt,
		&c.ValidityDays, &c.IsPublished, &c.RazorpayItemID, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *CourseRepository) performUpdate(query, opName string, courseID uuid.UUID, args ...any) error {
	ctx, cancel := r.withWriteContext()
	defer cancel()

	result, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		utils.Error("Course DB error during "+opName, err, map[string]any{"course_id": courseID})
		return fmt.Errorf("%s failed: %w", opName, err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("course not found or no changes made")
	}

	return nil
}

// ───────────────── CREATE ─────────────────

func (r *CourseRepository) Create(ctx context.Context, c *models.Course) error {
	writeCtx, cancel := r.withWriteContext()
	defer cancel()

	query := `
		INSERT INTO courses (
			id, title, slug, description, thumbnail_url, category, 
			price, original_price, discount_label, discount_expires_at, 
			validity_days, is_published, razorpay_item_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now

	_, err := r.db.Exec(writeCtx, query,
		c.ID, c.Title, c.Slug, c.Description, c.ThumbnailURL, c.Category,
		c.Price, c.OriginalPrice, c.DiscountLabel, c.DiscountExpiresAt,
		c.ValidityDays, c.IsPublished, c.RazorpayItemID, c.CreatedAt, c.UpdatedAt,
	)

	if err != nil {
		utils.Error("Failed to create course", err, map[string]any{"slug": c.Slug})
		return err
	}
	return nil
}

// ───────────────── READ ─────────────────

func (r *CourseRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Course, error) {
	query := `SELECT id, title, slug, description, thumbnail_url, category, 
	                 price, original_price, discount_label, discount_expires_at, 
	                 validity_days, is_published, razorpay_item_id, created_at, updated_at 
	          FROM courses WHERE id = $1`

	c, err := r.scanCourse(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

func (r *CourseRepository) GetBySlug(ctx context.Context, slug string) (*models.Course, error) {
	query := `SELECT id, title, slug, description, thumbnail_url, category, 
	                 price, original_price, discount_label, discount_expires_at, 
	                 validity_days, is_published, razorpay_item_id, created_at, updated_at 
	          FROM courses WHERE slug = $1`

	c, err := r.scanCourse(r.db.QueryRow(ctx, query, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

func (r *CourseRepository) List(ctx context.Context, onlyPublished bool) ([]*models.Course, error) {
	query := `SELECT id, title, slug, description, thumbnail_url, category, 
	                 price, original_price, discount_label, discount_expires_at, 
	                 validity_days, is_published, razorpay_item_id, created_at, updated_at 
	          FROM courses`

	if onlyPublished {
		query += " WHERE is_published = TRUE"
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var courses []*models.Course
	for rows.Next() {
		c, err := r.scanCourse(rows)
		if err != nil {
			return nil, err
		}
		courses = append(courses, c)
	}

	return courses, nil
}

func (r *CourseRepository) GetPublicCurriculum(ctx context.Context, courseID uuid.UUID) ([]dto.PublicCurriculumModule, error) {
	return r.GetStudentCurriculum(ctx, courseID)
}

func (r *CourseRepository) GetStudentCurriculum(ctx context.Context, courseID uuid.UUID) ([]dto.PublicCurriculumModule, error) {
	query := `
		SELECT 
			m.id, m.title, m.order_index,
			l.id, l.title, l.video_key, l.order_index, l.duration_seconds, l.is_preview,
			(
				SELECT COALESCE(json_agg(json_build_object(
					'id', la.id,
					'title', la.title,
					'file_url', la.file_key,
					'file_size', la.file_size,
					'mime_type', la.mime_type,
					'created_at', la.created_at
				)), '[]'::json)
				FROM lesson_attachments la
				WHERE la.lesson_id = l.id
			) as attachments
		FROM modules m
		LEFT JOIN lessons l ON m.id = l.module_id
		WHERE m.course_id = $1
		ORDER BY m.order_index ASC, l.order_index ASC
	`

	rows, err := r.db.Query(ctx, query, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modules []dto.PublicCurriculumModule
	var currentModule *dto.PublicCurriculumModule

	for rows.Next() {
		var mID uuid.UUID
		var mTitle string
		var mOrderIndex int

		var lID *uuid.UUID
		var lTitle *string
		var lVideoKey *string
		var lOrderIndex *int
		var lDurationSeconds *int
		var lIsPreview *bool
		var lAttachments []byte

		if err := rows.Scan(
			&mID, &mTitle, &mOrderIndex,
			&lID, &lTitle, &lVideoKey, &lOrderIndex, &lDurationSeconds, &lIsPreview, &lAttachments,
		); err != nil {
			return nil, err
		}

		if currentModule == nil || currentModule.ID != mID {
			if currentModule != nil {
				modules = append(modules, *currentModule)
			}
			currentModule = &dto.PublicCurriculumModule{
				ID:         mID,
				Title:      mTitle,
				OrderIndex: mOrderIndex,
				Lessons:    make([]dto.PublicCurriculumLesson, 0),
			}
		}

		if lID != nil {
			// Convert VideoKey to VideoURL (In this project, they are the same or VideoKey is the path)
			vURL := ""
			if lVideoKey != nil {
				vURL = *lVideoKey
			}

			var attachments []dto.AttachmentDTO
			if len(lAttachments) > 0 {
				json.Unmarshal(lAttachments, &attachments)
			}

			lesson := dto.PublicCurriculumLesson{
				ID:              *lID,
				Title:           *lTitle,
				Description:     "", // Table doesn't have description yet
				VideoURL:        vURL,
				OrderIndex:      *lOrderIndex,
				DurationSeconds: lDurationSeconds,
				IsPreview:       *lIsPreview,
				Attachments:     attachments,
			}
			currentModule.Lessons = append(currentModule.Lessons, lesson)
		}
	}

	if currentModule != nil {
		modules = append(modules, *currentModule)
	}

	return modules, nil
}

// ───────────────── UPDATE (Specialized) ─────────────────

// UpdateDetails: Title, Description, Thumbnail, Category, and Slug
func (r *CourseRepository) UpdateDetails(ctx context.Context, c *models.Course) error {
	query := `
		UPDATE courses SET 
			title = $2, slug = $3, description = $4, thumbnail_url = $5, 
			category = $6, razorpay_item_id = $7, updated_at = $8 
		WHERE id = $1`

	return r.performUpdate(query, "details update", c.ID,
		c.ID, c.Title, c.Slug, c.Description, c.ThumbnailURL, c.Category, c.RazorpayItemID, time.Now())
}

// UpdatePricing: Handled separately as it's sensitive and might involve discount logic
func (r *CourseRepository) UpdatePricing(ctx context.Context, c *models.Course) error {
	query := `
		UPDATE courses SET 
			price = $2, original_price = $3, discount_label = $4, 
			discount_expires_at = $5, updated_at = $6 
		WHERE id = $1`

	return r.performUpdate(query, "pricing update", c.ID,
		c.ID, c.Price, c.OriginalPrice, c.DiscountLabel, c.DiscountExpiresAt, time.Now())
}

// UpdateThumbnailURL: Specific update for background processor
func (r *CourseRepository) UpdateThumbnailURL(ctx context.Context, id uuid.UUID, url string) error {
	query := `UPDATE courses SET thumbnail_url = $2, updated_at = $3 WHERE id = $1`
	return r.performUpdate(query, "thumbnail update", id, id, url, time.Now())
}

// UpdateStatus: Publish/Unpublish toggle
func (r *CourseRepository) UpdateStatus(ctx context.Context, id uuid.UUID, isPublished bool) error {
	query := `UPDATE courses SET is_published = $2, updated_at = $3 WHERE id = $1`
	return r.performUpdate(query, "status update", id, id, isPublished, time.Now())
}

// ───────────────── DELETE ─────────────────

// SoftDelete for Courses (usually by setting is_published = false or a deleted_at if you add it)
func (r *CourseRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	// Let's assume for courses, unpublishing is the "soft" delete
	return r.UpdateStatus(ctx, id, false)
}

func (r *CourseRepository) PermanentDelete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM courses WHERE id = $1`
	return r.performUpdate(query, "permanent delete", id, id)
}

func (r *CourseRepository) CountDependencies(ctx context.Context, id uuid.UUID) (int, error) {
	var total int
	query := `SELECT COUNT(*) FROM purchases WHERE course_id = $1`
	err := r.db.QueryRow(ctx, query, id).Scan(&total)
	return total, err
}

func (r *CourseRepository) CountLessons(ctx context.Context, id uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM lessons WHERE course_id = $1`
	var count int
	err := r.db.QueryRow(ctx, query, id).Scan(&count)
	return count, err
}

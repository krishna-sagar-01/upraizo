package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"server/internal/models"
	"server/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LessonRepository struct {
	db *pgxpool.Pool
}

func NewLessonRepository(db *pgxpool.Pool) *LessonRepository {
	return &LessonRepository{db: db}
}

// ───────────────── Helpers ─────────────────

func (r *LessonRepository) withWriteContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func (r *LessonRepository) scanLesson(row pgx.Row) (*models.Lesson, error) {
	l := &models.Lesson{}
	err := row.Scan(
		&l.ID, &l.ModuleID, &l.Title, &l.OrderIndex,
		&l.VideoKey, &l.VideoStatus, &l.DurationSeconds,
		&l.IsPreview, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (r *LessonRepository) performUpdate(query, opName string, lessonID uuid.UUID, args ...any) error {
	ctx, cancel := r.withWriteContext()
	defer cancel()

	result, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		utils.Error("Lesson DB error during "+opName, err, map[string]any{"lesson_id": lessonID})
		return fmt.Errorf("%s failed: %w", opName, err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("lesson not found or no changes made")
	}

	return nil
}

// ───────────────── CREATE ─────────────────

func (r *LessonRepository) Create(ctx context.Context, l *models.Lesson) error {
	writeCtx, cancel := r.withWriteContext()
	defer cancel()

	query := `
		INSERT INTO lessons (
			id, module_id, title, order_index, video_status, is_preview, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	if l.ID == uuid.Nil { l.ID = uuid.New() }
	if l.VideoStatus == "" { l.VideoStatus = models.VideoStatusPending }
	
	now := time.Now()
	l.CreatedAt = now
	l.UpdatedAt = now

	_, err := r.db.Exec(writeCtx, query,
		l.ID, l.ModuleID, l.Title, l.OrderIndex, l.VideoStatus, l.IsPreview, l.CreatedAt, l.UpdatedAt,
	)

	if err != nil {
		utils.Error("Failed to create lesson", err, map[string]any{"module_id": l.ModuleID})
		return err
	}
	return nil
}

// ───────────────── READ ─────────────────

func (r *LessonRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Lesson, error) {
	query := `SELECT id, module_id, title, order_index, video_key, video_status, duration_seconds, is_preview, created_at, updated_at 
	          FROM lessons WHERE id = $1`
	l, err := r.scanLesson(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return l, nil
}

// GetByModule: Returns all lessons in a module sorted by OrderIndex
func (r *LessonRepository) GetByModuleID(ctx context.Context, moduleID uuid.UUID) ([]*models.Lesson, error) {
	query := `SELECT id, module_id, title, order_index, video_key, video_status, duration_seconds, is_preview, created_at, updated_at 
	          FROM lessons WHERE module_id = $1 ORDER BY order_index ASC`

	rows, err := r.db.Query(ctx, query, moduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []*models.Lesson
	for rows.Next() {
		l, err := r.scanLesson(rows)
		if err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}
	return lessons, nil
}

// ───────────────── UPDATE (Specialized) ─────────────────

// UpdateMetadata: Title, OrderIndex, and Preview status
func (r *LessonRepository) UpdateMetadata(ctx context.Context, l *models.Lesson) error {
	query := `UPDATE lessons SET title = $2, order_index = $3, is_preview = $4, updated_at = $5 WHERE id = $1`
	return r.performUpdate(query, "metadata update", l.ID, l.ID, l.Title, l.OrderIndex, l.IsPreview, time.Now())
}

// UpdateVideoStatus: Used by Transcoder worker to update processing state
// It updates VideoKey, Status, and Duration atomically.
func (r *LessonRepository) UpdateVideoStatus(ctx context.Context, lessonID uuid.UUID, key *string, status models.VideoStatus, duration *int) error {
	query := `
		UPDATE lessons SET 
			video_key = $2, 
			video_status = $3, 
			duration_seconds = $4, 
			updated_at = $5 
		WHERE id = $1`
	
	return r.performUpdate(query, "video status update", lessonID, lessonID, key, status, duration, time.Now())
}

// ───────────────── DELETE ─────────────────

// SoftDelete: We'll perform a permanent delete since the schema doesn't support deleted_at
func (r *LessonRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM lessons WHERE id = $1`
	return r.performUpdate(query, "permanent delete (via soft delete)", id, id)
}

func (r *LessonRepository) PermanentDelete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM lessons WHERE id = $1`
	return r.performUpdate(query, "permanent delete", id, id)
}
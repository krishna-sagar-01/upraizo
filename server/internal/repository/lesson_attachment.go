package repository

import (
	"context"
	"errors"
	"time"

	"server/internal/models"
	"server/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AttachmentRepository struct {
	db *pgxpool.Pool
}

func NewAttachmentRepository(db *pgxpool.Pool) *AttachmentRepository {
	return &AttachmentRepository{db: db}
}

// ───────────────── Helpers ─────────────────

func (r *AttachmentRepository) withWriteContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func (r *AttachmentRepository) scanAttachment(row pgx.Row) (*models.LessonAttachment, error) {
	a := &models.LessonAttachment{}
	err := row.Scan(
		&a.ID, &a.LessonID, &a.Title, &a.FileKey, 
		&a.FileSize, &a.MimeType, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// ───────────────── CREATE ─────────────────

func (r *AttachmentRepository) Create(ctx context.Context, a *models.LessonAttachment) error {
	writeCtx, cancel := r.withWriteContext()
	defer cancel()

	query := `
		INSERT INTO lesson_attachments (
			id, lesson_id, title, file_key, file_size, mime_type, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	if a.ID == uuid.Nil { a.ID = uuid.New() }
	a.CreatedAt = time.Now()

	_, err := r.db.Exec(writeCtx, query,
		a.ID, a.LessonID, a.Title, a.FileKey, a.FileSize, a.MimeType, a.CreatedAt,
	)

	if err != nil {
		utils.Error("Failed to create lesson attachment", err, map[string]any{
			"lesson_id": a.LessonID,
			"title":     a.Title,
		})
		return err
	}
	return nil
}

// ───────────────── READ ─────────────────

func (r *AttachmentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.LessonAttachment, error) {
	query := `SELECT id, lesson_id, title, file_key, file_size, mime_type, created_at 
	          FROM lesson_attachments WHERE id = $1`
	a, err := r.scanAttachment(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

// GetByLessonID: Returns all files (PDFs, Notes etc.) for a specific lesson
func (r *AttachmentRepository) GetByLessonID(ctx context.Context, lessonID uuid.UUID) ([]*models.LessonAttachment, error) {
	query := `SELECT id, lesson_id, title, file_key, file_size, mime_type, created_at 
	          FROM lesson_attachments WHERE lesson_id = $1 ORDER BY created_at ASC`

	rows, err := r.db.Query(ctx, query, lessonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []*models.LessonAttachment
	for rows.Next() {
		a, err := r.scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, a)
	}
	return attachments, nil
}

// ───────────────── DELETE ─────────────────

// PermanentDelete: Attachments are usually deleted permanently to save R2 storage
func (r *AttachmentRepository) PermanentDelete(ctx context.Context, id uuid.UUID) error {
	writeCtx, cancel := r.withWriteContext()
	defer cancel()

	query := `DELETE FROM lesson_attachments WHERE id = $1`
	
	result, err := r.db.Exec(writeCtx, query, id)
	if err != nil {
		utils.Error("Failed to delete attachment from DB", err, map[string]any{"attachment_id": id})
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("attachment not found")
	}

	return nil
}
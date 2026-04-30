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

type EbookRepository struct {
	db *pgxpool.Pool
}

func NewEbookRepository(db *pgxpool.Pool) *EbookRepository {
	return &EbookRepository{db: db}
}

func (r *EbookRepository) scanEbook(row pgx.Row) (*models.Ebook, error) {
	e := &models.Ebook{}
	err := row.Scan(
		&e.ID, &e.Title, &e.Slug, &e.Description, &e.ThumbnailURL, &e.FileURL,
		&e.Price, &e.OriginalPrice, &e.DiscountLabel, &e.DiscountExpiresAt,
		&e.IsPublished, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *EbookRepository) Create(ctx context.Context, e *models.Ebook) error {
	query := `
		INSERT INTO ebooks (
			id, title, slug, description, thumbnail_url, file_url,
			price, original_price, discount_label, discount_expires_at,
			is_published, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	now := time.Now()
	e.CreatedAt = now
	e.UpdatedAt = now

	_, err := r.db.Exec(ctx, query,
		e.ID, e.Title, e.Slug, e.Description, e.ThumbnailURL, e.FileURL,
		e.Price, e.OriginalPrice, e.DiscountLabel, e.DiscountExpiresAt,
		e.IsPublished, e.CreatedAt, e.UpdatedAt,
	)

	if err != nil {
		utils.Error("Failed to create ebook", err, map[string]any{"slug": e.Slug})
		return err
	}
	return nil
}

func (r *EbookRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Ebook, error) {
	query := `SELECT id, title, slug, description, thumbnail_url, file_url,
	                 price, original_price, discount_label, discount_expires_at,
	                 is_published, created_at, updated_at 
	          FROM ebooks WHERE id = $1`

	e, err := r.scanEbook(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return e, nil
}

func (r *EbookRepository) GetBySlug(ctx context.Context, slug string) (*models.Ebook, error) {
	query := `SELECT id, title, slug, description, thumbnail_url, file_url,
	                 price, original_price, discount_label, discount_expires_at,
	                 is_published, created_at, updated_at 
	          FROM ebooks WHERE slug = $1`

	e, err := r.scanEbook(r.db.QueryRow(ctx, query, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return e, nil
}

func (r *EbookRepository) List(ctx context.Context, onlyPublished bool) ([]*models.Ebook, error) {
	query := `SELECT id, title, slug, description, thumbnail_url, file_url,
	                 price, original_price, discount_label, discount_expires_at,
	                 is_published, created_at, updated_at 
	          FROM ebooks`

	if onlyPublished {
		query += " WHERE is_published = TRUE"
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ebooks []*models.Ebook
	for rows.Next() {
		e, err := r.scanEbook(rows)
		if err != nil {
			return nil, err
		}
		ebooks = append(ebooks, e)
	}

	return ebooks, nil
}

func (r *EbookRepository) Update(ctx context.Context, e *models.Ebook) error {
	query := `
		UPDATE ebooks SET 
			title = $2, slug = $3, description = $4, thumbnail_url = $5, 
			file_url = $6, price = $7, original_price = $8, discount_label = $9, 
			discount_expires_at = $10, is_published = $11, updated_at = $12 
		WHERE id = $1`

	result, err := r.db.Exec(ctx, query,
		e.ID, e.Title, e.Slug, e.Description, e.ThumbnailURL, e.FileURL,
		e.Price, e.OriginalPrice, e.DiscountLabel, e.DiscountExpiresAt,
		e.IsPublished, time.Now())

	if err != nil {
		return fmt.Errorf("ebook update failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("ebook not found")
	}

	return nil
}

func (r *EbookRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM ebooks WHERE id = $1`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("ebook not found")
	}
	return nil
}

func (r *EbookRepository) GetPurchasedEbooks(ctx context.Context, userID uuid.UUID) ([]*models.Ebook, error) {
	query := `
		SELECT e.id, e.title, e.slug, e.description, e.thumbnail_url, e.file_url,
		       e.price, e.original_price, e.discount_label, e.discount_expires_at,
		       e.is_published, e.created_at, e.updated_at
		FROM ebooks e
		JOIN purchases p ON e.id = p.ebook_id
		WHERE p.user_id = $1 AND p.status = 'completed'
		ORDER BY p.created_at DESC`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ebooks []*models.Ebook
	for rows.Next() {
		e, err := r.scanEbook(rows)
		if err != nil {
			return nil, err
		}
		ebooks = append(ebooks, e)
	}

	return ebooks, nil
}

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

type AdminRepository struct {
	db *pgxpool.Pool
}

func NewAdminRepository(db *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{db: db}
}

// ───────────────── Helpers ─────────────────

func (r *AdminRepository) scanAdmin(row pgx.Row) (*models.Admin, error) {
	a := &models.Admin{}
	err := row.Scan(
		&a.ID, &a.Name, &a.Email, &a.Phone,
		&a.PasswordHash, &a.SecretKeyHash,
		&a.IsActive, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// ───────────────── READ ─────────────────

func (r *AdminRepository) GetByEmail(ctx context.Context, email string) (*models.Admin, error) {
	query := `
		SELECT id, name, email, phone, password_hash, secret_key_hash, is_active, created_at, updated_at
		FROM admins WHERE email = $1`
	
	a, err := r.scanAdmin(r.db.QueryRow(ctx, query, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

func (r *AdminRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Admin, error) {
	query := `
		SELECT id, name, email, phone, password_hash, secret_key_hash, is_active, created_at, updated_at
		FROM admins WHERE id = $1`
	
	a, err := r.scanAdmin(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

// ───────────────── WRITE ─────────────────

func (r *AdminRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	query := `UPDATE admins SET password_hash = $2, updated_at = $3 WHERE id = $1`
	
	result, err := r.db.Exec(ctx, query, id, hash, time.Now())
	if err != nil {
		utils.Error("Admin DB error during password update", err, map[string]any{"admin_id": id})
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("admin not found")
	}

	return nil
}


func (r *AdminRepository) UpdateSecretKey(ctx context.Context, id uuid.UUID, hash string) error {
	query := `UPDATE admins SET secret_key_hash = $2, updated_at = $3 WHERE id = $1`
	
	result, err := r.db.Exec(ctx, query, id, hash, time.Now())
	if err != nil {
		utils.Error("Admin DB error during secret key update", err, map[string]any{"admin_id": id})
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("admin not found")
	}

	return nil
}

func (r *AdminRepository) UpdateProfile(ctx context.Context, id uuid.UUID, name, phone *string) error {
	if name == nil && phone == nil {
		return nil
	}

	query := "UPDATE admins SET "
	args := []any{id}
	argID := 2

	if name != nil {
		query += fmt.Sprintf("name = $%d, ", argID)
		args = append(args, *name)
		argID++
	}
	if phone != nil {
		query += fmt.Sprintf("phone = $%d, ", argID)
		args = append(args, *phone)
		argID++
	}

	query += fmt.Sprintf("updated_at = $%d WHERE id = $1", argID)
	args = append(args, time.Now())

	result, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		utils.Error("Admin DB error during profile update", err, map[string]any{"admin_id": id})
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("admin not found")
	}

	return nil
}


func (r *AdminRepository) SetActiveStatus(ctx context.Context, id uuid.UUID, active bool) error {
	query := `UPDATE admins SET is_active = $2, updated_at = $3 WHERE id = $1`
	
	_, err := r.db.Exec(ctx, query, id, active, time.Now())
	return err
}

func (r *AdminRepository) Create(ctx context.Context, a *models.Admin) error {
	query := `
		INSERT INTO admins (id, name, email, phone, password_hash, secret_key_hash, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	if a.ID == uuid.Nil { a.ID = uuid.New() }
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now

	_, err := r.db.Exec(ctx, query, a.ID, a.Name, a.Email, a.Phone, a.PasswordHash, a.SecretKeyHash, a.IsActive, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		utils.Error("Failed to create admin", err, map[string]any{"email": a.Email})
		return fmt.Errorf("failed to create admin: %w", err)
	}
	return nil
}

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

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// ───────────────── Helpers (Optimization & Integrity) ─────────────────

// withWriteContext creates a detached context for safe Write operations.
func (r *UserRepository) withWriteContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 7*time.Second)
}

// scanUser: Centralized scanning to keep code DRY and maintainable.
func (r *UserRepository) scanUser(row pgx.Row) (*models.User, error) {
	u := &models.User{}
	err := row.Scan(
		&u.ID, &u.Name, &u.AvatarURL, &u.Email, &u.PasswordHash,
		&u.AuthProvider, &u.AuthProviderID, &u.Status, &u.StatusReason, &u.IsVerified,
		&u.VerifiedAt, &u.Preferences, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// performUpdate: Generic helper for single-row updates to reduce boilerplate.
func (r *UserRepository) performUpdate(query, opName string, userID uuid.UUID, args ...any) error {
	ctx, cancel := r.withWriteContext()
	defer cancel()

	result, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		utils.Error("Database error during "+opName, err, map[string]any{"user_id": userID})
		return fmt.Errorf("%s failed: %w", opName, err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("user not found or no changes made")
	}

	return nil
}

// ───────────────── CREATE ─────────────────

func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	writeCtx, cancel := r.withWriteContext()
	defer cancel()

	query := `
		INSERT INTO users (
			id, name, avatar_url, email, password_hash, auth_provider, 
			auth_provider_id, status, status_reason, is_verified, verified_at, preferences, 
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	_, err := r.db.Exec(writeCtx, query,
		u.ID, u.Name, u.AvatarURL, u.Email, u.PasswordHash, u.AuthProvider,
		u.AuthProviderID, u.Status, u.StatusReason, u.IsVerified, u.VerifiedAt, u.Preferences,
		u.CreatedAt, u.UpdatedAt,
	)

	if err != nil {
		utils.Error("Failed to create user", err, map[string]any{"email": u.Email})
		return err
	}
	return nil
}

// ───────────────── READ ─────────────────

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `SELECT id, name, avatar_url, email, password_hash, auth_provider, 
	                 auth_provider_id, status, status_reason, is_verified, verified_at, 
	                 preferences, created_at, updated_at 
	          FROM users WHERE id = $1 AND status != 'deleted'`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, name, avatar_url, email, password_hash, auth_provider, 
	                 auth_provider_id, status, status_reason, is_verified, verified_at, 
	                 preferences, created_at, updated_at 
	          FROM users WHERE email = $1 AND status != 'deleted'`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

// ───────────────── UPDATE (Split Operations) ─────────────────

// Update: Profile information update (Name, Avatar etc.)
func (r *UserRepository) Update(ctx context.Context, u *models.User) error {
	query := `UPDATE users SET name = $2, avatar_url = $3, updated_at = $4 WHERE id = $1`
	return r.performUpdate(query, "profile update", u.ID, u.ID, u.Name, u.AvatarURL, time.Now())
}

// UpdateStatus: Specifically for changing user state (Active, Banned etc.) with reason.
func (r *UserRepository) UpdateStatus(ctx context.Context, userID uuid.UUID, status models.UserStatus, reason string) error {
	query := `UPDATE users SET status = $2, status_reason = $3, updated_at = $4 WHERE id = $1`
	return r.performUpdate(query, "status update", userID, userID, status, reason, time.Now())
}

// UpdatePreferences: JSONB update optimization.
func (r *UserRepository) UpdatePreferences(ctx context.Context, userID uuid.UUID, prefs models.UserPreferences) error {
	query := `UPDATE users SET preferences = $2, updated_at = $3 WHERE id = $1`
	return r.performUpdate(query, "preferences update", userID, userID, prefs, time.Now())
}

// UpdatePassword changes only the password hash (used after password reset).
func (r *UserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, hashedPassword string) error {
	query := `UPDATE users SET password_hash = $2, updated_at = $3 WHERE id = $1`
	return r.performUpdate(query, "password update", userID, userID, hashedPassword, time.Now())
}

// VerifyUser atomically sets is_verified, verified_at, and promotes status to active.
func (r *UserRepository) VerifyUser(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	query := `UPDATE users SET is_verified = true, verified_at = $2, status = 'active', updated_at = $3 WHERE id = $1`
	return r.performUpdate(query, "verify user", userID, userID, now, now)
}

// ───────────────── DELETE (Soft & Permanent) ─────────────────

// SoftDelete: Status change and email masking (so email can be reused if needed)
func (r *UserRepository) SoftDelete(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	query := `
		UPDATE users SET 
			status = 'deleted', 
			email = email || '.deleted.' || $2,
			updated_at = $3 
		WHERE id = $1`
	
	return r.performUpdate(query, "soft delete", userID, userID, now.Unix(), now)
}

// PermanentDelete: Direct row removal (Careful!)
func (r *UserRepository) PermanentDelete(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`
	return r.performUpdate(query, "permanent delete", userID, userID)
}

// ───────────────── ADMIN UTILS ─────────────────

// ListPaginated: Optimized user list for admin dashboard with optional status filtering.
func (r *UserRepository) ListPaginated(ctx context.Context, offset, limit int, status string) ([]models.User, error) {
	query := `SELECT id, name, avatar_url, email, password_hash, auth_provider, 
	                 auth_provider_id, status, status_reason, is_verified, verified_at, 
	                 preferences, created_at, updated_at 
	          FROM users 
	          WHERE status::text != 'deleted' 
	          AND ($3 = '' OR status::text = $3)
	          ORDER BY created_at DESC 
	          LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, query, limit, offset, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		u, err := r.scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}

	return users, nil
}

// CountAll: Total number of users matching a status filter.
func (r *UserRepository) CountAll(ctx context.Context, status string) (int64, error) {
	query := `SELECT COUNT(*) FROM users WHERE status::text != 'deleted' AND ($1 = '' OR status::text = $1)`
	var count int64
	err := r.db.QueryRow(ctx, query, status).Scan(&count)
	return count, err
}
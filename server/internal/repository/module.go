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

type ModuleRepository struct {
	db *pgxpool.Pool
}

func NewModuleRepository(db *pgxpool.Pool) *ModuleRepository {
	return &ModuleRepository{db: db}
}

// ───────────────── Helpers ─────────────────

func (r *ModuleRepository) withWriteContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func (r *ModuleRepository) scanModule(row pgx.Row) (*models.Module, error) {
	m := &models.Module{}
	err := row.Scan(&m.ID, &m.CourseID, &m.Title, &m.OrderIndex, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *ModuleRepository) performUpdate(query, opName string, moduleID uuid.UUID, args ...any) error {
	ctx, cancel := r.withWriteContext()
	defer cancel()

	result, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		utils.Error("Module DB error during "+opName, err, map[string]any{"module_id": moduleID})
		return fmt.Errorf("%s failed: %w", opName, err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("module not found or no changes made")
	}

	return nil
}

// ───────────────── CREATE ─────────────────

func (r *ModuleRepository) Create(ctx context.Context, m *models.Module) error {
	writeCtx, cancel := r.withWriteContext()
	defer cancel()

	query := `
		INSERT INTO modules (id, course_id, title, order_index, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	if m.ID == uuid.Nil { m.ID = uuid.New() }
	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now

	_, err := r.db.Exec(writeCtx, query, m.ID, m.CourseID, m.Title, m.OrderIndex, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		utils.Error("Failed to create module", err, map[string]any{"course_id": m.CourseID})
		return err
	}
	return nil
}

// ───────────────── READ (Optimized for Course View) ─────────────────

func (r *ModuleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Module, error) {
	query := `SELECT id, course_id, title, order_index, created_at, updated_at FROM modules WHERE id = $1`
	m, err := r.scanModule(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return m, nil
}

// GetByCourse: Sabse important query, ye order_index se sorted data degi
func (r *ModuleRepository) GetByCourseID(ctx context.Context, courseID uuid.UUID) ([]*models.Module, error) {
	query := `SELECT id, course_id, title, order_index, created_at, updated_at 
	          FROM modules WHERE course_id = $1 ORDER BY order_index ASC`

	rows, err := r.db.Query(ctx, query, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modules []*models.Module
	for rows.Next() {
		m, err := r.scanModule(rows)
		if err != nil {
			return nil, err
		}
		modules = append(modules, m)
	}
	return modules, nil
}

// ───────────────── UPDATE (Split Operations) ─────────────────

// UpdateTitle: Sirf title badalne ke liye
func (r *ModuleRepository) UpdateTitle(ctx context.Context, id uuid.UUID, title string) error {
	query := `UPDATE modules SET title = $2, updated_at = $3 WHERE id = $1`
	return r.performUpdate(query, "title update", id, id, title, time.Now())
}

// UpdateOrder: Jab dashboard se module up/down move karein
func (r *ModuleRepository) UpdateOrder(ctx context.Context, id uuid.UUID, newOrder int) error {
	query := `UPDATE modules SET order_index = $2, updated_at = $3 WHERE id = $1`
	return r.performUpdate(query, "order update", id, id, newOrder, time.Now())
}

// ───────────────── DELETE (Soft & Permanent) ─────────────────

// SoftDelete: We'll perform a permanent delete since the schema doesn't support deleted_at
func (r *ModuleRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM modules WHERE id = $1`
	return r.performUpdate(query, "permanent delete (via soft delete)", id, id)
}

func (r *ModuleRepository) PermanentDelete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM modules WHERE id = $1`
	return r.performUpdate(query, "permanent delete", id, id)
}
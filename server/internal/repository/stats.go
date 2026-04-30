package repository

import (
	"context"
	"server/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StatsRepository struct {
	db *pgxpool.Pool
}

func NewStatsRepository(db *pgxpool.Pool) *StatsRepository {
	return &StatsRepository{db: db}
}

// GetGlobalStats fetches real-time platform metrics (optimized counts)
// GetGlobalStats fetches platform metrics using optimized counts.
func (r *StatsRepository) GetGlobalStats(ctx context.Context) (*models.PlatformStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_users,
			COUNT(*) FILTER (WHERE is_verified = true) as active_users,
			COUNT(*) FILTER (WHERE is_verified = false) as inactive_users,
			COUNT(*) FILTER (WHERE status = 'suspended') as suspended_users,
			COUNT(*) FILTER (WHERE status = 'banned') as banned_users,
			NOW() as updated_at
		FROM users
	`

	s := &models.PlatformStats{}
	err := r.db.QueryRow(ctx, query).Scan(
		&s.TotalUsers,
		&s.ActiveUsers,
		&s.InactiveUsers,
		&s.SuspendedUsers,
		&s.BannedUsers,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// GetDashboardMetrics fetches cross-domain counts for the dashboard.
func (r *StatsRepository) GetDashboardMetrics(ctx context.Context) (map[string]int64, error) {
	// For cross-table counts, subqueries are still needed but we can keep them clean.
	query := `
		SELECT 
			(SELECT COUNT(*) FROM courses) as total_courses,
			(SELECT COUNT(*) FROM courses WHERE is_published = true) as published_courses,
			(SELECT COUNT(*) FROM tickets WHERE status IN ('open', 'in_progress')) as active_tickets
	`
	
	var totalCourses, publishedCourses, activeTickets int64
	err := r.db.QueryRow(ctx, query).Scan(&totalCourses, &publishedCourses, &activeTickets)
	if err != nil {
		return nil, err
	}

	return map[string]int64{
		"total_courses":     totalCourses,
		"published_courses": publishedCourses,
		"active_tickets":    activeTickets,
	}, nil
}

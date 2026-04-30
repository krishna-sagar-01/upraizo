package service

import (
	"context"

	"server/internal/dto"
	"server/internal/models"
	"server/internal/redis"
	"server/internal/repository"
	"server/internal/utils"

	"github.com/google/uuid"
)

type UserMgmtService struct {
	userRepo    *repository.UserRepository
	statsRepo   *repository.StatsRepository
	sessionRepo *redis.SessionRepository
}

func NewUserMgmtService(
	userRepo *repository.UserRepository, 
	statsRepo *repository.StatsRepository,
	sessionRepo *redis.SessionRepository,
) *UserMgmtService {
	return &UserMgmtService{
		userRepo:    userRepo,
		statsRepo:   statsRepo,
		sessionRepo: sessionRepo,
	}
}

// GetStats returns global platform statistics (Trigger-based optimized)
func (s *UserMgmtService) GetStats(ctx context.Context) (*models.PlatformStats, error) {
	stats, err := s.statsRepo.GetGlobalStats(ctx)
	if err != nil {
		return nil, utils.Internal(err)
	}
	return stats, nil
}

// ListUsers handles paginated user retrieval with optional status filtering.
func (s *UserMgmtService) ListUsers(ctx context.Context, page, limit int, status string) (*dto.AdminUserListResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	offset := (page - 1) * limit

	// 1. Fetch filtered count
	total, err := s.userRepo.CountAll(ctx, status)
	if err != nil {
		return nil, utils.Internal(err)
	}

	// 2. Fetch paginated items
	users, err := s.userRepo.ListPaginated(ctx, offset, limit, status)
	if err != nil {
		return nil, utils.Internal(err)
	}

	// 3. Convert to SafeUser DTOs
	items := make([]dto.SafeUser, len(users))
	for i, u := range users {
		items[i] = dto.ToSafeUser(&u)
	}

	// 4. Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	return &dto.AdminUserListResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// UpdateUserStatus handles changing a user's status with a mandatory reason for negative actions.
func (s *UserMgmtService) UpdateUserStatus(ctx context.Context, userID uuid.UUID, status models.UserStatus, reason string) error {
	// 1. Validate status
	if !status.IsValid() {
		return utils.BadRequest("Invalid user status")
	}

	// 2. Enforce reason for suspensions and bans
	if (status == models.UserStatusSuspended || status == models.UserStatusBanned) && reason == "" {
		return utils.BadRequest("An audit reason is mandatory for suspensions or bans")
	}

	// 3. Update in database
	if err := s.userRepo.UpdateStatus(ctx, userID, status, reason); err != nil {
		return err
	}

	// 4. Revoke active sessions if user is banned or suspended
	if status == models.UserStatusSuspended || status == models.UserStatusBanned {
		if err := s.sessionRepo.RevokeAllSessions(ctx, userID.String()); err != nil {
			utils.Error("Failed to revoke user sessions after ban", err, map[string]any{"user_id": userID})
			// Non-fatal for the primary action, but logged for audit
		}
	}

	utils.Info("User status updated by admin", map[string]any{
		"user_id": userID,
		"status":  status,
		"reason":  reason,
	})

	return nil
}

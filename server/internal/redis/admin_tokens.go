package redis

import (
	"context"
	"errors"
	"time"

	"server/internal/utils"

	goredis "github.com/redis/go-redis/v9"
)

const (
	PrefixAdminReset       = "admin_reset:"        // admin_reset:<token> -> adminID
	PrefixAdminSecretReset = "admin_secret_reset:" // admin_secret_reset:<token> -> adminID
)

type AdminTokenRepository struct{}

func NewAdminTokenRepository() *AdminTokenRepository {
	return &AdminTokenRepository{}
}

// ─── Reset Tokens ──────────────────────────────────────────

func (r *AdminTokenRepository) StoreResetToken(ctx context.Context, token, adminID string, ttl time.Duration) error {
	key := PrefixAdminReset + token
	return Client.Set(ctx, key, adminID, ttl).Err()
}

func (r *AdminTokenRepository) GetAdminIDByToken(ctx context.Context, token string) (string, error) {
	key := PrefixAdminReset + token
	adminID, err := Client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", utils.NotFound("Reset token not found or expired")
		}
		return "", utils.Internal(err)
	}

	// Delete token immediately after use (ensure it's single-use)
	_ = Client.Del(ctx, key)
	
	return adminID, nil
}

// ─── Secret Reset Tokens ──────────────────────────────────

func (r *AdminTokenRepository) StoreSecretToken(ctx context.Context, token, adminID string, ttl time.Duration) error {
	key := PrefixAdminSecretReset + token
	return Client.Set(ctx, key, adminID, ttl).Err()
}

func (r *AdminTokenRepository) GetAdminIDBySecretToken(ctx context.Context, token string) (string, error) {
	key := PrefixAdminSecretReset + token
	adminID, err := Client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", utils.NotFound("Secret reset token not found or expired")
		}
		return "", utils.Internal(err)
	}

	_ = Client.Del(ctx, key)
	return adminID, nil
}

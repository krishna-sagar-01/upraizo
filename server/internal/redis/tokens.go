package redis

import (
	"context"
	"errors"
	"time"

	"server/internal/utils"

	"github.com/redis/go-redis/v9"
)

// ─── Key Prefixes ────────────────────────────────────────────────────────────
const (
	PrefixEmailVerify = "token:email_verify:"
	PrefixPasswordReset = "token:pwd_reset:"
)

type TokenRepository struct{}

func NewTokenRepository() *TokenRepository {
	return &TokenRepository{}
}

// ───────────────── Email Verification Tokens ─────────────────

// StoreEmailToken stores a verification token linked to a UserID
func (r *TokenRepository) StoreEmailToken(ctx context.Context, userID string, token string, ttl time.Duration) error {
	key := PrefixEmailVerify + token
	err := Client.Set(ctx, key, userID, ttl).Err()
	if err != nil {
		utils.Error("Failed to store email verification token in Redis", err, map[string]any{"user_id": userID})
		return err
	}
	return nil
}

// GetUserByEmailToken retrieves the UserID associated with a token.
// Deletes the token immediately after retrieval to ensure single-use.
func (r *TokenRepository) GetUserByEmailToken(ctx context.Context, token string) (string, error) {
	key := PrefixEmailVerify + token
	userID, err := Client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", utils.NotFound("Email verification token not found or expired")
		}
		return "", utils.Internal(err)
	}

	// Single-use enforcement
	_ = Client.Del(ctx, key)
	
	return userID, nil
}

// DeleteEmailToken removes the token after successful verification
func (r *TokenRepository) DeleteEmailToken(ctx context.Context, token string) error {
	return Client.Del(ctx, PrefixEmailVerify+token).Err()
}

// ───────────────── Password Reset Tokens ─────────────────

func (r *TokenRepository) StorePasswordResetToken(ctx context.Context, userID string, token string, ttl time.Duration) error {
	key := PrefixPasswordReset + token
	return Client.Set(ctx, key, userID, ttl).Err()
}

func (r *TokenRepository) GetUserByResetToken(ctx context.Context, token string) (string, error) {
	key := PrefixPasswordReset + token
	userID, err := Client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", utils.NotFound("Password reset token not found or expired")
		}
		return "", utils.Internal(err)
	}

	// Single-use enforcement
	_ = Client.Del(ctx, key)

	return userID, nil
}

func (r *TokenRepository) DeleteResetToken(ctx context.Context, token string) error {
	return Client.Del(ctx, PrefixPasswordReset+token).Err()
}
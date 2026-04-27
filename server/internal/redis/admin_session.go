package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"server/internal/dto"
	"server/internal/utils"

	goredis "github.com/redis/go-redis/v9"
)

const (
	PrefixAdminSession      = "admin_session:"       // admin_session:<session_id>
	PrefixAdminUserSessions = "admin_user_sessions:" // admin_user_sessions:<admin_id>
)

type AdminSessionRepository struct{}

func NewAdminSessionRepository() *AdminSessionRepository {
	return &AdminSessionRepository{}
}

func (r *AdminSessionRepository) StoreSession(
	ctx context.Context,
	meta *dto.SessionMetadata,
	ttl time.Duration,
) error {
	if meta == nil {
		return utils.Internal(nil)
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return utils.Internal(err)
	}

	sessionKey := PrefixAdminSession + meta.ID
	userKey := PrefixAdminUserSessions + meta.UserID

	const userSetTTL = 90 * 24 * time.Hour

	pipe := Client.Pipeline()
	pipe.Set(ctx, sessionKey, data, ttl)
	pipe.SAdd(ctx, userKey, meta.ID)
	pipe.Expire(ctx, userKey, userSetTTL)

	if _, err = pipe.Exec(ctx); err != nil {
		return utils.Internal(err)
	}

	return nil
}

func (r *AdminSessionRepository) GetSession(
	ctx context.Context,
	sessionID string,
) (*dto.SessionMetadata, error) {
	raw, err := Client.Get(ctx, PrefixAdminSession+sessionID).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, utils.NotFound("Admin session not found")
		}
		return nil, utils.Internal(err)
	}

	var meta dto.SessionMetadata
	if err = json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, utils.Internal(err)
	}

	return &meta, nil
}

func (r *AdminSessionRepository) RevokeSession(ctx context.Context, adminID, sessionID string) error {
	pipe := Client.Pipeline()
	pipe.Del(ctx, PrefixAdminSession+sessionID)
	pipe.SRem(ctx, PrefixAdminUserSessions+adminID, sessionID)

	if _, err := pipe.Exec(ctx); err != nil {
		return utils.Internal(err)
	}
	return nil
}


func (r *AdminSessionRepository) RevokeAllSessions(ctx context.Context, adminID string) error {
	userKey := PrefixAdminUserSessions + adminID

	sessionIDs, err := Client.SMembers(ctx, userKey).Result()
	if err != nil {
		return utils.Internal(err)
	}

	if len(sessionIDs) == 0 {
		return nil
	}

	pipe := Client.Pipeline()
	for _, id := range sessionIDs {
		pipe.Del(ctx, PrefixAdminSession+id)
	}
	pipe.Del(ctx, userKey)

	if _, err = pipe.Exec(ctx); err != nil {
		return utils.Internal(err)
	}
	return nil
}

func (r *AdminSessionRepository) GetActiveSessions(
	ctx context.Context,
	adminID string,
) ([]*dto.SessionMetadata, error) {
	userKey := PrefixAdminUserSessions + adminID

	sessionIDs, err := Client.SMembers(ctx, userKey).Result()
	if err != nil {
		return nil, utils.Internal(err)
	}

	if len(sessionIDs) == 0 {
		return []*dto.SessionMetadata{}, nil
	}

	keys := make([]string, len(sessionIDs))
	for i, id := range sessionIDs {
		keys[i] = PrefixAdminSession + id
	}

	results, err := Client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, utils.Internal(err)
	}

	sessions := make([]*dto.SessionMetadata, 0, len(sessionIDs))
	var staleIDs []interface{}

	for i, res := range results {
		if res == nil {
			staleIDs = append(staleIDs, sessionIDs[i])
			continue
		}

		raw, ok := res.(string)
		if !ok {
			staleIDs = append(staleIDs, sessionIDs[i])
			continue
		}

		var meta dto.SessionMetadata
		if err = json.Unmarshal([]byte(raw), &meta); err != nil {
			utils.Error("corrupted admin session metadata; pruning", err,
				map[string]any{"session_id": sessionIDs[i], "admin_id": adminID},
			)
			staleIDs = append(staleIDs, sessionIDs[i])
			continue
		}

		sessions = append(sessions, &meta)
	}

	if len(staleIDs) > 0 {
		if sremErr := Client.SRem(ctx, userKey, staleIDs...).Err(); sremErr != nil {
			utils.Error("failed to prune stale admin session IDs", sremErr,
				map[string]any{"admin_id": adminID},
			)
		}
	}

	return sessions, nil
}


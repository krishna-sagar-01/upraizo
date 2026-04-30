package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"server/internal/utils"
	"server/internal/dto"

	goredis "github.com/redis/go-redis/v9"
)

const (
	PrefixSession      = "session:"       // session:<session_id>
	PrefixUserSessions = "user_sessions:" // user_sessions:<user_id> → Set of session IDs
)

type SessionRepository struct{}

func NewSessionRepository() *SessionRepository {
	return &SessionRepository{}
}

// ───────────────────────────────────────────────────────
//  Session Operations
// ───────────────────────────────────────────────────────

// StoreSession persists session metadata and registers the session ID under
// the user's active-session set. Both writes are pipelined for atomicity.
//
// The user-session set is kept with a fixed long-lived TTL so it outlasts any
// individual session; individual session keys carry their own TTL.
func (r *SessionRepository) StoreSession(
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

	sessionKey := PrefixSession + meta.ID
	userKey := PrefixUserSessions + meta.UserID

	// Use a fixed, generous TTL for the user-sessions set so it is never
	// shorter than the longest possible individual session.
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

// GetSession retrieves session metadata by session ID.
// Returns utils.NotFound if the session has expired or does not exist.
func (r *SessionRepository) GetSession(
	ctx context.Context,
	sessionID string,
) (*dto.SessionMetadata, error) {
	raw, err := Client.Get(ctx, PrefixSession+sessionID).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, utils.NotFound("session not found or expired")
		}
		return nil, utils.Internal(err)
	}

	var meta dto.SessionMetadata
	if err = json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, utils.Internal(err)
	}

	return &meta, nil
}

// RevokeSession removes a single session (single-device logout).
func (r *SessionRepository) RevokeSession(
	ctx context.Context,
	userID string,
	sessionID string,
) error {
	pipe := Client.Pipeline()
	pipe.Del(ctx, PrefixSession+sessionID)
	pipe.SRem(ctx, PrefixUserSessions+userID, sessionID)

	if _, err := pipe.Exec(ctx); err != nil {
		return utils.Internal(err)
	}

	return nil
}

// RevokeAllSessions removes every active session for a user (logout-all /
// security-breach revocation). No-ops cleanly when no sessions exist.
func (r *SessionRepository) RevokeAllSessions(
	ctx context.Context,
	userID string,
) error {
	userKey := PrefixUserSessions + userID

	sessionIDs, err := Client.SMembers(ctx, userKey).Result()
	if err != nil {
		return utils.Internal(err)
	}

	if len(sessionIDs) == 0 {
		return nil
	}

	pipe := Client.Pipeline()
	for _, id := range sessionIDs {
		pipe.Del(ctx, PrefixSession+id)
	}
	pipe.Del(ctx, userKey)

	if _, err = pipe.Exec(ctx); err != nil {
		return utils.Internal(err)
	}

	return nil
}

// GetActiveSessions returns metadata for every live session owned by the
// user. Expired entries found in the set are pruned in a single SRem call
// so the set stays consistent over time.
//
// Uses MGet to retrieve all session blobs in a single round-trip instead of
// issuing one GET per session (avoids N+1 latency).
func (r *SessionRepository) GetActiveSessions(
	ctx context.Context,
	userID string,
) ([]*dto.SessionMetadata, error) {
	userKey := PrefixUserSessions + userID

	sessionIDs, err := Client.SMembers(ctx, userKey).Result()
	if err != nil {
		return nil, utils.Internal(err)
	}

	if len(sessionIDs) == 0 {
		return []*dto.SessionMetadata{}, nil
	}

	// Build the key slice for a single MGet round-trip.
	keys := make([]string, len(sessionIDs))
	for i, id := range sessionIDs {
		keys[i] = PrefixSession + id
	}

	results, err := Client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, utils.Internal(err)
	}

	sessions := make([]*dto.SessionMetadata, 0, len(sessionIDs))
	var staleIDs []interface{}

	for i, res := range results {
		if res == nil {
			// Session TTL expired but set entry was not yet cleaned up.
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
			// Corrupted entry — log and prune.
			utils.Error("corrupted session metadata; pruning", err,
				map[string]any{"session_id": sessionIDs[i], "user_id": userID},
			)
			staleIDs = append(staleIDs, sessionIDs[i])
			continue
		}

		sessions = append(sessions, &meta)
	}

	// Prune all stale/corrupted IDs in one shot.
	if len(staleIDs) > 0 {
		if sremErr := Client.SRem(ctx, userKey, staleIDs...).Err(); sremErr != nil {
			// Non-fatal — log but do not fail the request.
			utils.Error("failed to prune stale session IDs", sremErr,
				map[string]any{"user_id": userID},
			)
		}
	}

	return sessions, nil
}
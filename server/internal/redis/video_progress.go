package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type VideoProgress struct {
	Percentage int    `json:"percentage"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	UpdatedAt  int64  `json:"updated_at"`
}

type VideoProgressRepository struct{}

func NewVideoProgressRepository() *VideoProgressRepository {
	return &VideoProgressRepository{}
}

func (r *VideoProgressRepository) key(lessonID uuid.UUID) string {
	return fmt.Sprintf("video_progress:%s", lessonID.String())
}

// SetProgress updates the progress in Redis with a 10-minute TTL (sufficient for most transcodes)
func (r *VideoProgressRepository) SetProgress(ctx context.Context, lessonID uuid.UUID, percentage int, status string, message string) error {
	progress := VideoProgress{
		Percentage: percentage,
		Status:     status,
		Message:    message,
		UpdatedAt:  time.Now().Unix(),
	}

	data, err := json.Marshal(progress)
	if err != nil {
		return err
	}

	return Client.Set(ctx, r.key(lessonID), data, 10*time.Minute).Err()
}

func (r *VideoProgressRepository) GetProgress(ctx context.Context, lessonID uuid.UUID) (*VideoProgress, error) {
	data, err := Client.Get(ctx, r.key(lessonID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var progress VideoProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil, err
	}

	return &progress, nil
}

func (r *VideoProgressRepository) ClearProgress(ctx context.Context, lessonID uuid.UUID) error {
	return Client.Del(ctx, r.key(lessonID)).Err()
}

package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/jpeg"
	_ "image/png" // Register PNG decoder
	"os"
	"server/internal/broker"
	"server/internal/config"
	"server/internal/repository"
	"server/internal/storage"
	"server/internal/utils"
	"time"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	_ "golang.org/x/image/webp" // Register WebP decoder
)

type AvatarTask struct {
	UserID       string `json:"user_id"`
	LocalPath    string `json:"local_path"`
	OldAvatarURL string `json:"old_avatar_url"`
	RetryCount   int    `json:"retry_count"`
}

type AvatarWorker struct {
	cfg      *config.Config
	userRepo *repository.UserRepository
	r2       *storage.R2Client
}

func NewAvatarWorker(cfg *config.Config, userRepo *repository.UserRepository, r2 *storage.R2Client) *AvatarWorker {
	return &AvatarWorker{
		cfg:      cfg,
		userRepo: userRepo,
		r2:       r2,
	}
}

func (w *AvatarWorker) Start(ctx context.Context) error {
	msgs, err := broker.Channel.Consume(
		w.cfg.RabbitMQ.AvatarQueue, // queue
		"avatar_worker",            // consumer
		false,                      // auto-ack (MANUAL ACKING is safer)
		false,                      // exclusive
		false,                      // no-local
		false,                      // no-wait
		nil,                        // args
	)
	if err != nil {
		return fmt.Errorf("failed to start avatar consumer: %w", err)
	}

	utils.Info("Avatar Worker started", map[string]any{"queue": w.cfg.RabbitMQ.AvatarQueue})

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-msgs:
				if !ok {
					return
				}
				w.process(d)
			}
		}
	}()

	return nil
}

func (w *AvatarWorker) process(d amqp.Delivery) {
	var task AvatarTask
	if err := json.Unmarshal(d.Body, &task); err != nil {
		utils.Error("Failed to unmarshal avatar task", err, nil)
		d.Nack(false, false) // Don't requeue malformed JSON
		return
	}

	defer func() {
		// Cleanup local temp file ALWAYS
		if err := os.Remove(task.LocalPath); err != nil && !os.IsNotExist(err) {
			utils.Warn("Failed to delete temp avatar file", map[string]any{"path": task.LocalPath, "error": err.Error()})
		}
	}()

	// 1. Load and process image
	img, err := imaging.Open(task.LocalPath)
	if err != nil {
		w.handleError(d, task, "Failed to open temp avatar image", err)
		return
	}

	// 2. Resize to 400x400 (Crop center to maintain aspect ratio)
	processed := imaging.Fill(img, 400, 400, imaging.Center, imaging.Lanczos)

	// 3. Compress to JPEG (High quality 85% is a good balance)
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, processed, &jpeg.Options{Quality: 85}); err != nil {
		w.handleError(d, task, "Failed to encode processed avatar", err)
		return
	}

	// 4. Upload to R2
	fileName := fmt.Sprintf("avatars/%s/%d.jpg", task.UserID, time.Now().Unix())
	publicURL, err := w.r2.Upload(fileName, buf.Bytes(), "image/jpeg")
	if err != nil {
		w.handleError(d, task, "Failed to upload avatar to R2", err)
		return
	}

	// 5. Update DB
	uid, _ := uuid.Parse(task.UserID)
	user, err := w.userRepo.GetByID(context.Background(), uid)
	if err == nil && user != nil {
		user.AvatarURL = &publicURL
		if err := w.userRepo.Update(context.Background(), user); err != nil {
			utils.Error("Failed to update user avatar in DB", err, map[string]any{"user_id": task.UserID})
			// Even if DB update fails, we don't requeue because file is already in R2
		}
	}

	// 6. Delete OLD avatar from R2
	if task.OldAvatarURL != "" {
		if err := w.r2.Delete(task.OldAvatarURL); err != nil {
			utils.Warn("Failed to delete old avatar from R2", map[string]any{"url": task.OldAvatarURL, "error": err.Error()})
		}
	}

	utils.Info("Avatar processed successfully", map[string]any{"user_id": task.UserID, "url": publicURL})
	d.Ack(false)
}

func (w *AvatarWorker) handleError(d amqp.Delivery, task AvatarTask, msg string, err error) {
	utils.Error(msg, err, map[string]any{"user_id": task.UserID, "retry": task.RetryCount})

	if task.RetryCount < w.cfg.RabbitMQ.MaxRetries {
		task.RetryCount++
		// Wait before retrying (Simple backoff)
		time.Sleep(5 * time.Second)
		
		body, _ := json.Marshal(task)
		_ = broker.Channel.Publish("", w.cfg.RabbitMQ.AvatarQueue, false, false, amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		})
		d.Ack(false)
	} else {
		utils.Error("Avatar task failed after max retries", err, map[string]any{"user_id": task.UserID})
		d.Nack(false, false)
	}
}

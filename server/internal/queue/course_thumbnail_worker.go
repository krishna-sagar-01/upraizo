package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/jpeg"
	_ "image/png"
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
	_ "golang.org/x/image/webp"
)

type CourseThumbnailTask struct {
	CourseID        string `json:"course_id"`
	LocalPath       string `json:"local_path"`
	OldThumbnailURL string `json:"old_thumbnail_url"`
	RetryCount      int    `json:"retry_count"`
}

type CourseThumbnailWorker struct {
	cfg        *config.Config
	courseRepo *repository.CourseRepository
	r2         *storage.R2Client
}

func NewCourseThumbnailWorker(cfg *config.Config, courseRepo *repository.CourseRepository, r2 *storage.R2Client) *CourseThumbnailWorker {
	return &CourseThumbnailWorker{
		cfg:        cfg,
		courseRepo: courseRepo,
		r2:         r2,
	}
}

func (w *CourseThumbnailWorker) Start(ctx context.Context) error {
	msgs, err := broker.Channel.Consume(
		w.cfg.RabbitMQ.CourseThumbnailQueue,
		"course_thumbnail_worker",
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start course thumbnail consumer: %w", err)
	}

	utils.Info("Course Thumbnail Worker started", map[string]any{"queue": w.cfg.RabbitMQ.CourseThumbnailQueue})

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

func (w *CourseThumbnailWorker) process(d amqp.Delivery) {
	var task CourseThumbnailTask
	if err := json.Unmarshal(d.Body, &task); err != nil {
		utils.Error("Failed to unmarshal course thumbnail task", err, nil)
		d.Nack(false, false)
		return
	}

	defer func() {
		// Cleanup local temp file
		if err := os.Remove(task.LocalPath); err != nil && !os.IsNotExist(err) {
			utils.Warn("Failed to delete temp course thumbnail file", map[string]any{"path": task.LocalPath, "error": err.Error()})
		}
	}()

	// 1. Load and process image
	img, err := imaging.Open(task.LocalPath)
	if err != nil {
		w.handleError(d, task, "Failed to open temp course thumbnail image", err)
		return
	}

	// 2. Resize to 1280x720 (User specified)
	processed := imaging.Fill(img, 1280, 720, imaging.Center, imaging.Lanczos)

	// 3. Compress to JPEG
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, processed, &jpeg.Options{Quality: 85}); err != nil {
		w.handleError(d, task, "Failed to encode processed course thumbnail", err)
		return
	}

	// 4. Upload to R2
	fileName := fmt.Sprintf("courses/%s/thumbnail_%d.jpg", task.CourseID, time.Now().Unix())
	publicURL, err := w.r2.Upload(fileName, buf.Bytes(), "image/jpeg")
	if err != nil {
		w.handleError(d, task, "Failed to upload course thumbnail to R2", err)
		return
	}

	// 5. Update DB
	cid, _ := uuid.Parse(task.CourseID)
	if err := w.courseRepo.UpdateThumbnailURL(context.Background(), cid, publicURL); err != nil {
		utils.Error("Failed to update course thumbnail in DB", err, map[string]any{"course_id": task.CourseID})
	}

	// 6. Delete OLD thumbnail from R2
	if task.OldThumbnailURL != "" {
		if err := w.r2.Delete(task.OldThumbnailURL); err != nil {
			utils.Warn("Failed to delete old course thumbnail from R2", map[string]any{"url": task.OldThumbnailURL, "error": err.Error()})
		}
	}

	utils.Info("Course thumbnail processed successfully", map[string]any{"course_id": task.CourseID, "url": publicURL})
	d.Ack(false)
}

func (w *CourseThumbnailWorker) handleError(d amqp.Delivery, task CourseThumbnailTask, msg string, err error) {
	utils.Error(msg, err, map[string]any{"course_id": task.CourseID, "retry": task.RetryCount})

	if task.RetryCount < w.cfg.RabbitMQ.MaxRetries {
		task.RetryCount++
		// Wait before retrying (Simple backoff)
		time.Sleep(5 * time.Second)
		
		body, _ := json.Marshal(task)
		_ = broker.Channel.Publish("", w.cfg.RabbitMQ.CourseThumbnailQueue, false, false, amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		})
		d.Ack(false)
	} else {
		utils.Error("Course thumbnail task failed after max retries", err, map[string]any{"course_id": task.CourseID})
		d.Nack(false, false) // Finished retrying, drop or move to DLQ
	}
}

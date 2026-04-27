package queue

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"server/internal/broker"
	"server/internal/config"
	"server/internal/models"
	"server/internal/redis"
	"server/internal/repository"
	"server/internal/storage"
	"server/internal/utils"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type VideoTask struct {
	LessonID     string  `json:"lesson_id"`
	TempFilePath string  `json:"temp_file_path"`
	OldVideoKey  *string `json:"old_video_key"`
	RetryCount   int     `json:"retry_count"`
}

type VideoWorker struct {
	cfg        *config.Config
	lessonRepo *repository.LessonRepository
	r2         *storage.R2Client
	progressRepo *redis.VideoProgressRepository
}

func NewVideoWorker(cfg *config.Config, lessonRepo *repository.LessonRepository, r2 *storage.R2Client, progressRepo *redis.VideoProgressRepository) *VideoWorker {
	return &VideoWorker{
		cfg:          cfg,
		lessonRepo:   lessonRepo,
		r2:           r2,
		progressRepo: progressRepo,
	}
}

func (w *VideoWorker) Start(ctx context.Context) error {
	msgs, err := broker.Channel.Consume(
		w.cfg.RabbitMQ.VideoQueue, // queue
		"video_worker",            // consumer
		false,                     // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start video consumer: %w", err)
	}

	utils.Info("Video Worker started", map[string]any{"queue": w.cfg.RabbitMQ.VideoQueue})

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

func (w *VideoWorker) process(d amqp.Delivery) {
	var task VideoTask
	if err := json.Unmarshal(d.Body, &task); err != nil {
		utils.Error("Failed to unmarshal video task", err, nil)
		d.Nack(false, false)
		return
	}

	lessonID, _ := uuid.Parse(task.LessonID)
	
	defer func() {
		// Cleanup temp file
		_ = os.Remove(task.TempFilePath)
	}()

	// 1. Get Video Duration using ffprobe
	duration, err := w.getVideoDuration(task.TempFilePath)
	if err != nil {
		w.handleError(d, task, lessonID, "Failed to get video duration", err)
		return
	}

	// 2. Initial Status in Redis
	_ = w.progressRepo.SetProgress(context.Background(), lessonID, 5, "processing", "Getting video details")

	// 3. Compress Video using FFmpeg
	compressedPath := filepath.Join(w.cfg.App.VideoTempPath(), "comp_"+filepath.Base(task.TempFilePath))
	defer os.Remove(compressedPath)

	err = w.compressVideo(lessonID, task.TempFilePath, compressedPath, duration)
	if err != nil {
		_ = w.progressRepo.SetProgress(context.Background(), lessonID, 0, "failed", err.Error())
		w.handleError(d, task, lessonID, "Failed to compress video", err)
		return
	}

	_ = w.progressRepo.SetProgress(context.Background(), lessonID, 90, "processing", "Uploading to cloud storage")

	// 4. Upload to R2
	videoKey := fmt.Sprintf("lessons/videos/%s.mp4", task.LessonID)
	
	// Read compressed file
	videoData, err := os.ReadFile(compressedPath)
	if err != nil {
		w.handleError(d, task, lessonID, "Failed to read compressed video", err)
		return
	}

	_, err = w.r2.Upload(videoKey, videoData, "video/mp4")
	if err != nil {
		w.handleError(d, task, lessonID, "Failed to upload video to R2", err)
		return
	}

	// 4. Update DB
	err = w.lessonRepo.UpdateVideoStatus(context.Background(), lessonID, &videoKey, models.VideoStatusReady, &duration)
	if err != nil {
		_ = w.progressRepo.SetProgress(context.Background(), lessonID, 0, "failed", "DB update failed")
		w.handleError(d, task, lessonID, "Failed to update DB final status", err)
		return
	}

	_ = w.progressRepo.SetProgress(context.Background(), lessonID, 100, "ready", "Processing complete")

	// 5. Cleanup OLD video from R2
	if task.OldVideoKey != nil && *task.OldVideoKey != videoKey {
		_ = w.r2.Delete(*task.OldVideoKey)
	}

	utils.Info("Video processed successfully", map[string]any{"lesson_id": task.LessonID, "key": videoKey})
	d.Ack(false)
}

func (w *VideoWorker) getVideoDuration(path string) (int, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	
	durationStr := strings.TrimSpace(string(out))
	durationFloat, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, err
	}
	
	return int(durationFloat), nil
}

func (w *VideoWorker) compressVideo(lessonID uuid.UUID, input, output string, totalDuration int) error {
	// Fastest compression: ultrafast preset
	// Use -progress pipe:1 to get real-time stats
	cmd := exec.Command("ffmpeg", "-i", input, "-vcodec", "libx264", "-preset", "ultrafast", "-crf", "23", "-acodec", "aac", "-progress", "pipe:1", "-y", output)
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "out_time_ms=") {
			timeMsStr := strings.TrimPrefix(line, "out_time_ms=")
			timeMs, err := strconv.ParseInt(timeMsStr, 10, 64)
			if err == nil && totalDuration > 0 {
				currentSec := int(timeMs / 1000000)
				percentage := (currentSec * 80) / totalDuration // 80% because upload is remaining
				if percentage > 80 { percentage = 80 }
				_ = w.progressRepo.SetProgress(context.Background(), lessonID, 10+percentage, "processing", "Transcoding video...")
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg error: %w", err)
	}
	return nil
}

func (w *VideoWorker) handleError(d amqp.Delivery, task VideoTask, lessonID uuid.UUID, msg string, err error) {
	utils.Error(msg, err, map[string]any{"lesson_id": task.LessonID, "retry": task.RetryCount})
	
	if task.RetryCount < w.cfg.RabbitMQ.MaxRetries {
		task.RetryCount++
		// Wait before retrying (Simple backoff)
		time.Sleep(5 * time.Second)
		
		// Re-publish with incremented retry count
		body, _ := json.Marshal(task)
		_ = broker.Channel.Publish("", w.cfg.RabbitMQ.VideoQueue, false, false, amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		})
		d.Ack(false) // Ack from current queue because we re-published
	} else {
		// Mark as failed in DB after 3 retries
		_ = w.lessonRepo.UpdateVideoStatus(context.Background(), lessonID, nil, models.VideoStatusFailed, nil)
		d.Nack(false, false) // Don't requeue
	}
}

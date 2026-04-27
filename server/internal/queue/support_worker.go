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

type SupportAttachmentTask struct {
	MessageID  string   `json:"message_id"`
	LocalPaths []string `json:"local_paths"`
	RetryCount int      `json:"retry_count"`
}

type SupportWorker struct {
	cfg        *config.Config
	ticketRepo *repository.TicketRepository
	r2         *storage.R2Client
}

func NewSupportWorker(cfg *config.Config, ticketRepo *repository.TicketRepository, r2 *storage.R2Client) *SupportWorker {
	return &SupportWorker{
		cfg:        cfg,
		ticketRepo: ticketRepo,
		r2:         r2,
	}
}

func (w *SupportWorker) Start(ctx context.Context) error {
	msgs, err := broker.Channel.Consume(
		w.cfg.RabbitMQ.SupportQueue,
		"support_worker",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start support consumer: %w", err)
	}

	utils.Info("Support Attachment Worker started", map[string]any{"queue": w.cfg.RabbitMQ.SupportQueue})

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

func (w *SupportWorker) process(d amqp.Delivery) {
	var task SupportAttachmentTask
	if err := json.Unmarshal(d.Body, &task); err != nil {
		utils.Error("Failed to unmarshal support attachment task", err, nil)
		d.Nack(false, false)
		return
	}

	defer func() {
		// Cleanup ALL local temp files
		for _, path := range task.LocalPaths {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				utils.Warn("Failed to delete temp support file", map[string]any{"path": path, "error": err.Error()})
			}
		}
	}()

	msgID, _ := uuid.Parse(task.MessageID)
	var finalURLs []string

	for i, localPath := range task.LocalPaths {
		img, err := imaging.Open(localPath)
		if err != nil {
			// If not an image, upload raw or handle error
			utils.Warn("Support Worker: Not an image or failed to open", map[string]any{"path": localPath})
			continue 
		}

		// Resize if too large (e.g., max width 1200px)
		processed := imaging.Fit(img, 1200, 1200, imaging.Lanczos)

		// Compress to JPEG
		buf := new(bytes.Buffer)
		if err := jpeg.Encode(buf, processed, &jpeg.Options{Quality: 80}); err != nil {
			w.handleError(d, task, "Failed to encode support attachment", err)
			return
		}

		// Upload to R2
		fileName := fmt.Sprintf("support/%s/%d-%d.jpg", task.MessageID, time.Now().Unix(), i)
		publicURL, err := w.r2.Upload(fileName, buf.Bytes(), "image/jpeg")
		if err != nil {
			w.handleError(d, task, "Failed to upload support attachment to R2", err)
			return
		}
		finalURLs = append(finalURLs, publicURL)
	}

	// Update DB with final URLs
	if len(finalURLs) > 0 {
		if err := w.ticketRepo.UpdateMessageAttachments(context.Background(), msgID, finalURLs); err != nil {
			utils.Error("Failed to update message attachments in DB", err, map[string]any{"message_id": task.MessageID})
			// Nack and retry if DB is down
			w.handleError(d, task, "DB Update failed for support attachments", err)
			return
		}
	}

	utils.Info("Support attachments processed successfully", map[string]any{"message_id": task.MessageID, "count": len(finalURLs)})
	d.Ack(false)
}

func (w *SupportWorker) handleError(d amqp.Delivery, task SupportAttachmentTask, msg string, err error) {
	utils.Error(msg, err, map[string]any{"message_id": task.MessageID, "retry": task.RetryCount})

	if task.RetryCount < w.cfg.RabbitMQ.MaxRetries {
		task.RetryCount++
		// Wait before retrying (Simple backoff)
		time.Sleep(5 * time.Second)
		
		body, _ := json.Marshal(task)
		_ = broker.Channel.Publish("", w.cfg.RabbitMQ.SupportQueue, false, false, amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		})
		d.Ack(false)
	} else {
		utils.Error("Support attachment task failed after max retries", err, map[string]any{"message_id": task.MessageID})
		d.Nack(false, false)
	}
}

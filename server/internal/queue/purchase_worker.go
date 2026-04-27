package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"server/internal/broker"
	"server/internal/config"
	"server/internal/repository"
	"server/internal/utils"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type PurchaseTask struct {
	PurchaseID string `json:"purchase_id"`
	UserID     string `json:"user_id"`
	CourseID   string `json:"course_id"`
	TraceID    string `json:"trace_id"`
	RetryCount int    `json:"retry_count"`
}

type PurchaseWorker struct {
	cfg          *config.Config
	purchaseRepo *repository.PurchaseRepository
	courseRepo   *repository.CourseRepository
}

func NewPurchaseWorker(cfg *config.Config, purchaseRepo *repository.PurchaseRepository, courseRepo *repository.CourseRepository) *PurchaseWorker {
	return &PurchaseWorker{
		cfg:          cfg,
		purchaseRepo: purchaseRepo,
		courseRepo:   courseRepo,
	}
}

func (w *PurchaseWorker) Start(ctx context.Context) error {
	msgs, err := broker.Channel.Consume(
		w.cfg.RabbitMQ.PurchaseQueue, // queue
		"purchase_worker",            // consumer
		false,                        // auto-ack (MANUAL ACKING is safer)
		false,                        // exclusive
		false,                        // no-local
		false,                        // no-wait
		nil,                          // args
	)
	if err != nil {
		return fmt.Errorf("failed to start purchase consumer: %w", err)
	}

	utils.Info("Purchase Worker started", map[string]any{"queue": w.cfg.RabbitMQ.PurchaseQueue})

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

func (w *PurchaseWorker) process(d amqp.Delivery) {
	var task PurchaseTask
	if err := json.Unmarshal(d.Body, &task); err != nil {
		utils.Error("Failed to unmarshal purchase task", err, nil)
		d.Nack(false, false)
		return
	}

	traceID := task.TraceID

	uid, err := uuid.Parse(task.PurchaseID)
	if err != nil {
		utils.Error("Invalid purchase ID in task", err, map[string]any{"purchase_id": task.PurchaseID, "trace_id": traceID})
		d.Nack(false, false)
		return
	}

	// 1. Verify Purchase State
	purchase, err := w.purchaseRepo.GetByID(context.Background(), uid)
	if err != nil {
		w.handleError(d, task, "Failed to fetch purchase during worker processing", err)
		return
	}

	if !purchase.IsCompleted() {
		utils.Warn("Worker processed incomplete purchase, requeuing", map[string]any{
			"purchase_id": task.PurchaseID, 
			"status":      purchase.Status,
			"trace_id":    traceID,
		})
		d.Nack(false, true)
		return
	}

	// 2. Grant Access (Activate Validity)
	course, err := w.courseRepo.GetByID(context.Background(), purchase.CourseID)
	if err != nil {
		w.handleError(d, task, "Worker: Failed to fetch course for activation", err)
		return
	}

	err = w.purchaseRepo.ActivateAccess(context.Background(), purchase.ID, course.ValidityDays)
	if err != nil {
		w.handleError(d, task, "Worker: Failed to activate access window", err)
		return
	}

	utils.Info("Purchase activated successfully", map[string]any{
		"purchase_id":   task.PurchaseID,
		"user_id":       task.UserID,
		"course_id":     task.CourseID,
		"validity_days": course.ValidityDays,
		"trace_id":      traceID,
	})

	// 4. Acknowledge the message
	d.Ack(false)
}

func (w *PurchaseWorker) handleError(d amqp.Delivery, task PurchaseTask, msg string, err error) {
	utils.Error(msg, err, map[string]any{"purchase_id": task.PurchaseID, "retry": task.RetryCount})

	if task.RetryCount < w.cfg.RabbitMQ.MaxRetries {
		task.RetryCount++
		// Wait before retrying (Simple backoff)
		time.Sleep(5 * time.Second)

		body, _ := json.Marshal(task)
		_ = broker.Channel.Publish("", w.cfg.RabbitMQ.PurchaseQueue, false, false, amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		})
		d.Ack(false)
	} else {
		utils.Error("Purchase activation failed after max retries", err, map[string]any{"purchase_id": task.PurchaseID})
		d.Nack(false, false)
	}
}

package queue

import (
	"encoding/json"
	"fmt"
	"server/internal/broker"
	"server/internal/config"
	"server/internal/utils"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Manager struct {
	cfg *config.Config
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

// SetupQueues declares all necessary queues for the application
func (m *Manager) SetupQueues() error {
	queues := []string{
		m.cfg.RabbitMQ.AvatarQueue,
		m.cfg.RabbitMQ.PurchaseQueue,
		m.cfg.RabbitMQ.CourseThumbnailQueue,
	}

	for _, qName := range queues {
		_, err := broker.Channel.QueueDeclare(
			qName, // name
			true,  // durable
			false, // delete when unused
			false, // exclusive
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", qName, err)
		}
	}

	utils.Info("Queues initialized successfully", map[string]any{
		"avatar_queue":   m.cfg.RabbitMQ.AvatarQueue,
		"purchase_queue": m.cfg.RabbitMQ.PurchaseQueue,
		"thumbnail_queue": m.cfg.RabbitMQ.CourseThumbnailQueue,
	})
	return nil
}

// Publish serializes data and sends it to the specified queue
func (m *Manager) Publish(queueName string, body interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal queue body: %w", err)
	}

	err = broker.Channel.Publish(
		"",        // exchange
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         jsonBody,
			DeliveryMode: amqp.Persistent, // Ensure message survives broker restart
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish to %s: %w", queueName, err)
	}

	return nil
}

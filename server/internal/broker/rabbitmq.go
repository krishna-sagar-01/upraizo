package broker

import (
	"fmt"
	"sync"
	"time"

	"server/internal/config"
	"server/internal/utils"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	Conn    *amqp.Connection
	Channel *amqp.Channel
	mu      sync.Mutex
)

// Connect initializes the RabbitMQ connection and opens a default channel
func Connect(cfg *config.Config) error {
	return connectWithRetry(cfg)
}

func connectWithRetry(cfg *config.Config) error {
	mu.Lock()
	defer mu.Unlock()

	url := cfg.RabbitMQ.GetURL()
	var err error

	for i := 1; i <= cfg.RabbitMQ.MaxRetries; i++ {
		Conn, err = amqp.Dial(url)
		if err == nil {
			break
		}

		utils.Warn(fmt.Sprintf("RabbitMQ connection failed, attempt %d/%d", i, cfg.RabbitMQ.MaxRetries), map[string]any{
			"error": err.Error(),
		})

		if i < cfg.RabbitMQ.MaxRetries {
			time.Sleep(cfg.RabbitMQ.RetryDelay)
		}
	}

	if err != nil {
		return fmt.Errorf("rabbitmq connection failed: %w", err)
	}

	Channel, err = Conn.Channel()
	if err != nil {
		return fmt.Errorf("rabbitmq channel creation failed: %w", err)
	}

	utils.Info("RabbitMQ connected successfully", map[string]any{
		"host":  cfg.RabbitMQ.Host,
		"vhost": cfg.RabbitMQ.VHost,
	})

	go watchConnectionDrops(cfg)

	return nil
}

// watchConnectionDrops listens for unexpected connection closures and attempts to reconnect
func watchConnectionDrops(cfg *config.Config) {
	err := <-Conn.NotifyClose(make(chan *amqp.Error))
	if err != nil {
		utils.Error("RabbitMQ connection dropped unexpectedly, attempting to reconnect...", err, nil)
		
		// Attempt to reconnect indefinitely in the background
		for {
			time.Sleep(5 * time.Second)
			if err := connectWithRetry(cfg); err == nil {
				utils.Info("RabbitMQ reconnected successfully after drop", nil)
				return
			}
			utils.Warn("RabbitMQ reconnection attempt failed, retrying in 5s...", nil)
		}
	}
}

// Close gracefully shuts down the Channel and Connection
func Close() {
	mu.Lock()
	defer mu.Unlock()

	if Channel != nil {
		_ = Channel.Close()
	}
	if Conn != nil {
		_ = Conn.Close()
		utils.Info("RabbitMQ connection closed gracefully", nil)
	}
}
package redis

import (
	"context"
	"fmt"
	"time"

	"server/internal/config"
	"server/internal/utils"

	"github.com/redis/go-redis/v9"
)

// Client is the globally accessible Redis client
var Client *redis.Client

// Connect initializes the Redis connection pool
func Connect(cfg *config.Config) error {
	// Configuration mapping for go-redis/v9
	opts := &redis.Options{
		Addr:     cfg.Redis.GetRedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,

		// Connection Pool Settings (Crucial for high traffic)
		PoolSize:        cfg.Redis.PoolSize,
		PoolTimeout:     cfg.Redis.PoolTimeout,
		ConnMaxLifetime: cfg.Redis.MaxConnAge,
		ConnMaxIdleTime: cfg.Redis.IdleTimeout,

		// Timeouts
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,

		// Retry Logic
		MaxRetries:      cfg.Redis.MaxRetries,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
	}

	Client = redis.NewClient(opts)

	// Context with timeout for the initial Ping
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Implement manual connection retry on startup (useful for Docker/K8s environments)
	var err error
	for i := 1; i <= cfg.Redis.RetryAttempts; i++ {
		err = Client.Ping(ctx).Err()
		if err == nil {
			utils.Info("Redis connected successfully", map[string]any{
				"host":      cfg.Redis.Host,
				"port":      cfg.Redis.Port,
				"db":        cfg.Redis.DB,
				"pool_size": cfg.Redis.PoolSize,
			})
			return nil
		}

		utils.Warn(fmt.Sprintf("Redis connection failed, attempt %d/%d", i, cfg.Redis.RetryAttempts), map[string]any{
			"error": err.Error(),
		})

		if i < cfg.Redis.RetryAttempts {
			time.Sleep(cfg.Redis.RetryDelay)
		}
	}

	// Agar saare retries fail ho jayein
	utils.Error("Failed to connect to Redis after maximum retries", err, nil)
	return fmt.Errorf("redis connection failed: %w", err)
}

// Close gracefully shuts down the Redis client
func Close() {
	if Client != nil {
		if err := Client.Close(); err != nil {
			utils.Error("Error closing Redis connection", err, nil)
		} else {
			utils.Info("Redis connection closed gracefully", nil)
		}
	}
}
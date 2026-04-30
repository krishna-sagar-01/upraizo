package db

import (
	"context"
	"fmt"
	"time"

	"server/internal/config"
	"server/internal/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the globally accessible pgx connection pool
var Pool *pgxpool.Pool

// Connect initializes the pure PostgreSQL connection pool with retries
func Connect(cfg *config.Config) error {
	dsn := cfg.Database.GetDSN()

	// 1. Parse connection string into pgx pool config
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		utils.Error("Failed to parse database DSN config", err, nil)
		return err
	}

	// 2. Tune Connection Pool for Maximum Performance & Stability
	poolConfig.MaxConns = int32(cfg.Database.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.Database.MaxIdleConns) // Minimum idle connections
	poolConfig.MaxConnLifetime = cfg.Database.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.Database.ConnMaxIdleTime

	// Optional Health Check: Ensure connections are alive before using them
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	var p *pgxpool.Pool

	// 3. Startup Retry Logic (Safe for Docker / Cloud deployments)
	for i := 1; i <= cfg.Database.RetryAttempts; i++ {
		// Context with timeout to prevent hanging forever during connection
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Database.ConnectTimeout)
		
		// Create the pool and connect
		p, err = pgxpool.NewWithConfig(ctx, poolConfig)
		
		if err == nil {
			// Ping database to ensure it's actually accepting queries
			err = p.Ping(ctx)
			cancel()
			if err == nil {
				break // Success! Exit the retry loop
			}
		} else {
			cancel()
		}

		utils.Warn(fmt.Sprintf("Database connection failed, attempt %d/%d", i, cfg.Database.RetryAttempts), map[string]any{
			"error": err.Error(),
		})

		// Wait before retrying
		if i < cfg.Database.RetryAttempts {
			time.Sleep(cfg.Database.RetryDelay)
		}
	}

	// If all retries failed
	if err != nil {
		utils.Error("Failed to connect to Database after maximum retries", err, nil)
		return fmt.Errorf("database connection failed: %w", err)
	}

	// Assign to global variable
	Pool = p
	utils.Info("PostgreSQL database connected successfully (Pure pgxpool)", map[string]any{
		"host":       cfg.Database.Host,
		"port":       cfg.Database.Port,
		"db_name":    cfg.Database.DBName,
		"max_conns":  cfg.Database.MaxOpenConns,
		"driver":     "pgx/v5",
	})

	return nil
}

// Close gracefully shuts down the database connection pool
func Close() {
	if Pool != nil {
		Pool.Close()
		utils.Info("Database connection pool closed gracefully", nil)
	}
}
package middleware

import (
	"time"

	"server/internal/config"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Logger struct {
	logger *zerolog.Logger
	cfg    *config.Config
}

func NewLogger(u *utils.Logger) *Logger {
	return &Logger{
		logger: u.GetRawLogger(),
		cfg:    u.GetConfig(),
	}
}

/* ───────────────── HTTP Request Middleware ───────────────── */

func (l *Logger) HTTPMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		
		// 1. Setup Unique Request ID
		requestID, ok := c.Locals("requestid").(string)
		if !ok || requestID == "" {
			requestID = uuid.New().String()
			c.Locals("requestid", requestID)
		}

		// 2. Execute actual route handler
		err := c.Next()

		// 3. Calculate metrics
		duration := time.Since(start)
		status := c.Response().StatusCode()

		// If status is still 200 but Next() returned a fiber error, use the error code.
		// This is because Fiber's global error handler runs AFTER this middleware in the chain.
		if err != nil {
			if e, ok := err.(*fiber.Error); ok && status == fiber.StatusOK {
				status = e.Code
			}
		}

		// 4. Determine Log Level
		level := l.logger.Info
		if status >= 500 {
			level = l.logger.Error
		} else if status >= 400 {
			level = l.logger.Warn
		}

		// 5. Build Event (Timestamp & Env are already added by Init)
		event := level().
			Str("type", "http").
			Str("request_id", requestID).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", status).
			Dur("duration_ms", duration).
			Str("ip", c.IP()).
			Str("user_agent", c.Get("User-Agent"))

		// If JWT middleware sets a user_id, grab it!
		if userID, ok := c.Locals("user_id").(string); ok && userID != "" {
			event.Str("user_id", userID)
		}

		if err != nil {
			event.Err(err)
		}

		event.Msg("HTTP request completed")
		return err
	}
}

/* ───────────────── Panic Recovery Middleware ───────────────── */

func (l *Logger) RecoveryMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				requestID, _ := c.Locals("requestid").(string)
				
				event := l.logger.Error().
					Str("type", "panic").
					Str("request_id", requestID).
					Str("method", c.Method()).
					Str("path", c.Path()).
					Interface("panic", r)

				if l.cfg.Logger.EnableStackTrace {
					event.Stack() 
				}

				event.Msg("Critical application panic recovered")

				// Send clean error to the frontend user (do not expose stack trace)
				c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"error":   "Oops! Something went wrong on our end.",
					"ref_id":  requestID, // User can give this ID in support ticket
				})
			}
		}()
		return c.Next()
	}
}
package utils

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"server/internal/config"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

/* ───────────────── Context Keys ───────────────── */
type ContextKey string

const (
	RequestIDKey ContextKey = "request_id"
	UserIDKey    ContextKey = "user_id"
)

/* ───────────────── Logger Struct ───────────────── */
type Logger struct {
	logger zerolog.Logger
	cfg    *config.Config
}

var (
	globalLogger *Logger
	initOnce     sync.Once
)

/* ───────────────── Initialization ───────────────── */

func Init(cfg *config.Config) {
	initOnce.Do(func() {
		// 1. Set Log Level
		level, err := zerolog.ParseLevel(cfg.Logger.Level)
		if err != nil {
			level = zerolog.InfoLevel
		}

		// 2. Set Global Time Format (RFC3339 is best for production & parsing)
		zerolog.TimeFieldFormat = time.RFC3339

		// 3. Setup Outputs (Console + File)
		var writers []io.Writer

		if cfg.Logger.EnableConsole {
			if cfg.App.IsDevelopment() {
				// Pretty print for local development
				writers = append(writers, zerolog.ConsoleWriter{
					Out:        os.Stdout,
					TimeFormat: time.RFC3339,
				})
			} else {
				// Fast JSON for production stdout (Docker/K8s)
				writers = append(writers, os.Stdout)
			}
		}

		if cfg.Logger.EnableFile && cfg.Logger.FilePath != "" {
			if err := os.MkdirAll(filepath.Dir(cfg.Logger.FilePath), 0755); err == nil {
				writers = append(writers, &lumberjack.Logger{
					Filename:   cfg.Logger.FilePath,
					MaxSize:    cfg.Logger.MaxSize,
					MaxBackups: cfg.Logger.MaxBackups,
					MaxAge:     cfg.Logger.MaxAge,
					Compress:   cfg.Logger.Compress,
				})
			} else {
				fmt.Fprintf(os.Stderr, "Logger Warning: failed to create log directory: %v\n", err)
			}
		}

		output := zerolog.MultiLevelWriter(writers...)

		// 4. Build Core Logger (Timestamps and Env are injected here globally)
		zCtx := zerolog.New(output).
			Level(level).
			With().
			Timestamp().               // Automatically adds exact time to EVERY log
			Str("env", cfg.App.Env)    // Automatically adds 'development' or 'production'

		if cfg.Logger.EnableCaller {
			zCtx = zCtx.Caller()
		}

		zLogger := zCtx.Logger()

		// 5. Enable Log Sampling for High Traffic (Saves disk space)
		if cfg.Logger.EnableSampling {
			zLogger = zLogger.Sample(&zerolog.BasicSampler{
				N: uint32(cfg.Logger.SamplingThereafter),
			})
		}

		globalLogger = &Logger{
			logger: zLogger,
			cfg:    cfg,
		}
		zerolog.DefaultContextLogger = &globalLogger.logger
	})
}

// Get returns the global logger safely
func Get() *Logger {
	if globalLogger == nil {
		fallback := zerolog.New(os.Stdout).With().Timestamp().Logger()
		return &Logger{logger: fallback}
	}
	return globalLogger
}

/* ───────────────── Context Extractors ───────────────── */

func (l *Logger) WithFiberContext(c *fiber.Ctx) *zerolog.Logger {
	logger := l.logger

	// Extract Request ID
	if reqID, ok := c.Locals("requestid").(string); ok && reqID != "" {
		logger = logger.With().Str("request_id", reqID).Logger()
	}
	
	// Extract User ID if logged in (Useful for tracking which student is doing what)
	if userID, ok := c.Locals("user_id").(string); ok && userID != "" {
		logger = logger.With().Str("user_id", userID).Logger()
	}

	logger = logger.With().
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("ip", c.IP()).
		Logger()

	return &logger
}

// WithContext extracts metadata from a standard context (e.g., from a background worker)
func (l *Logger) WithContext(ctx context.Context) *zerolog.Logger {
	logger := l.logger
	// Extract Request ID/Correlation ID if present
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		logger = logger.With().Str("request_id", reqID).Logger()
	}
	if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
		logger = logger.With().Str("user_id", userID).Logger()
	}
	return &logger
}

/* ───────────────── Standard Methods (Safe map[string]any) ───────────────── */

func (l *Logger) Info(msg string, fields map[string]any) { l.logger.Info().Fields(fields).Msg(msg) }
func (l *Logger) Debug(msg string, fields map[string]any) { l.logger.Debug().Fields(fields).Msg(msg) }
func (l *Logger) Warn(msg string, fields map[string]any)  { l.logger.Warn().Fields(fields).Msg(msg) }
func (l *Logger) Error(msg string, err error, fields map[string]any) {
	l.logger.Error().Err(err).Fields(fields).Msg(msg)
}
func (l *Logger) Fatal(msg string, err error, fields map[string]any) {
	l.logger.Fatal().Err(err).Fields(fields).Msg(msg)
}

/* ───────────────── LMS Specific Business Logging ───────────────── */

// LogPurchase records when a user buys a course (Razorpay webhook tracking)
func (l *Logger) LogPurchase(userID, courseID string, amount float64, status string, err error) {
	event := l.logger.Info()
	msg := "Course purchase successful"
	
	if err != nil || status == "failed" {
		event = l.logger.Error().Err(err)
		msg = "Course purchase failed"
	}

	event.
		Str("type", "purchase").
		Str("user_id", userID).
		Str("course_id", courseID).
		Float64("amount", amount).
		Str("status", status).
		Msg(msg)
}

// LogVideoProgress tracks if background updates are working
func (l *Logger) LogVideoProgress(userID, lessonID string, positionSeconds int) {
	l.logger.Debug().
		Str("type", "video_progress").
		Str("user_id", userID).
		Str("lesson_id", lessonID).
		Int("position_seconds", positionSeconds).
		Msg("Video progress updated")
}

// LogTicket tracks support system events
func (l *Logger) LogTicket(userID, ticketID, action string) {
	l.logger.Info().
		Str("type", "support_ticket").
		Str("user_id", userID).
		Str("ticket_id", ticketID).
		Str("action", action). // e.g., "created", "replied", "resolved"
		Msg("Ticket event")
}

/* ───────────────── Global Wrappers ───────────────── */
func Info(msg string, fields map[string]any)             { Get().Info(msg, fields) }
func Debug(msg string, fields map[string]any)            { Get().Debug(msg, fields) }
func Warn(msg string, fields map[string]any)             { Get().Warn(msg, fields) }
func Error(msg string, err error, fields map[string]any) { Get().Error(msg, err, fields) }
func Fatal(msg string, err error)                        { Get().Fatal(msg, err, nil) }

/* ───────────────── Accessors ───────────────── */
// GetRawLogger returns a pointer to the underlying zerolog instance
func (l *Logger) GetRawLogger() *zerolog.Logger {
	return &l.logger
}

// GetConfig returns the logger's app config
func (l *Logger) GetConfig() *config.Config {
	return l.cfg
}
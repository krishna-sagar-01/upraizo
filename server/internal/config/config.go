package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// ============================================================================
// Config
// ============================================================================
type Config struct {
	Database DatabaseConfig
	Redis    RedisConfig
	RabbitMQ RabbitMQConfig
	JWT      JWTConfig
	AdminJWT JWTConfig
	Security SecurityConfig
	Frontend FrontendConfig
	AWS      AWSConfig
	Google   GoogleConfig
	App      AppConfig
	R2       R2Config
	Logger   LoggerConfig
	Razorpay RazorpayConfig
}

// ============================================================================
// App Config
// ============================================================================
type AppConfig struct {
	Env               string
	Port              int
	RequestBodyLimit  int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	TrustedProxies    []string
	AllowedOrigins    []string
	ReadHeaderTimeout time.Duration
	EnableMetrics     bool
	EnableHealth      bool
	TempStoragePath   string
}

// ============================================================================
// Database Config
// ============================================================================
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         bool
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
	RetryAttempts   int
	RetryDelay      time.Duration
}

// ============================================================================
// Redis Config
// ============================================================================
type RedisConfig struct {
	Host               string
	Port               int
	Password           string
	DB                 int
	MaxRetries         int
	PoolSize           int
	PoolTimeout        time.Duration
	MaxConnAge         time.Duration
	IdleTimeout        time.Duration
	IdleCheckFrequency time.Duration
	DialTimeout        time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	RetryAttempts      int
	RetryDelay         time.Duration
}

// ============================================================================
// RabbitMQ Config
// ============================================================================
type RabbitMQConfig struct {
	Host          string
	Port          int
	User          string
	Password      string
	VHost         string
	MaxRetries    int
	RetryDelay    time.Duration
	AvatarQueue   string
	PurchaseQueue string
	CourseThumbnailQueue string
}

// ============================================================================
// JWT Config
// ============================================================================
type JWTConfig struct {
	Secret               string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
	Issuer               string
}

// ============================================================================
// Security Config
// ============================================================================
type SecurityConfig struct {
	MaxLoginAttempts int
	LockoutDuration  time.Duration
	RateLimitMax     int
}

// ============================================================================
// Frontend Config
// ============================================================================
type FrontendConfig struct {
	FrontendUrl           string
	VerificationUrl       string
	ResetUrl              string
	AdminResetPasswordUrl string
	SecretResetUrl        string
}

// ============================================================================
// AWS Config
// ============================================================================
type AWSConfig struct {
	AWSAccessKey string
	AWSSecretKey string
	AWSRegion    string
	AWSFromEmail string
}

// ============================================================================
// Google OAuth Config
// ============================================================================
type GoogleConfig struct {
	ClientID string
}

// ============================================================================
// Razorpay Config
// ============================================================================
type RazorpayConfig struct {
	KeyID         string
	KeySecret     string
	WebhookSecret string
}

// ============================================================================
// Cloudflare R2 Config
// ============================================================================
type R2Config struct {
	AccountID  string
	AccessKey  string
	SecretKey  string
	BucketName string
	PublicURL  string // e.g. http://upraizo.com
}

// ============================================================================
// Logger Config
// ============================================================================
type LoggerConfig struct {
	// Log Level
	Level string

	// Output
	EnableConsole bool
	EnableFile    bool
	FilePath      string

	// File Rotation
	MaxSize    int  // Maximum size in MB before rotation
	MaxBackups int  // Maximum number of old log files to retain
	MaxAge     int  // Maximum number of days to retain old log files
	Compress   bool // Compress old log files

	// Performance
	EnableSampling     bool
	SamplingInitial    int // Log first N of each level
	SamplingThereafter int // Then 1 every N

	// Features
	EnableCaller     bool // Add file:line to logs
	EnableStackTrace bool // Add stack trace on error/fatal
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvAsInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", ""),
			DBName:          getEnv("DB_NAME", "postgres"),
			SSLMode:         getEnvAsBool("DB_SSLMODE", false),
			MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 30),
			MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 50),
			ConnMaxLifetime: getEnvAsDuration("DB_CONN_MAX_LIFETIME", 2*time.Minute),
			ConnMaxIdleTime: getEnvAsDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
			ConnectTimeout:  getEnvAsDuration("DB_CONNECT_TIMEOUT", 10*time.Second),
			QueryTimeout:    getEnvAsDuration("DB_QUERY_TIMEOUT", 30*time.Second),
			RetryAttempts:   getEnvAsInt("DB_RETRY_ATTEMPTS", 3),
			RetryDelay:      getEnvAsDuration("DB_RETRY_DELAY", 5*time.Second),
		},
		Redis: RedisConfig{
			Host:               getEnv("REDIS_HOST", "localhost"),
			Port:               getEnvAsInt("REDIS_PORT", 6379),
			Password:           getEnv("REDIS_PASSWORD", ""),
			DB:                 getEnvAsInt("REDIS_DB", 0),
			MaxRetries:         getEnvAsInt("REDIS_MAX_RETRIES", 3),
			PoolSize:           getEnvAsInt("REDIS_POOL_SIZE", 10),
			PoolTimeout:        getEnvAsDuration("REDIS_POOL_TIMEOUT", 4*time.Second),
			MaxConnAge:         getEnvAsDuration("REDIS_MAX_CONN_AGE", 0),
			IdleTimeout:        getEnvAsDuration("REDIS_IDLE_TIMEOUT", 5*time.Minute),
			IdleCheckFrequency: getEnvAsDuration("REDIS_IDLE_CHECK_FREQ", 1*time.Minute),
			DialTimeout:        getEnvAsDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:        getEnvAsDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout:       getEnvAsDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
			RetryAttempts:      getEnvAsInt("REDIS_RETRY_ATTEMPTS", 3),
			RetryDelay:         getEnvAsDuration("REDIS_RETRY_DELAY", 5*time.Second),
		},
		RabbitMQ: RabbitMQConfig{
			Host:          getEnv("RABBITMQ_HOST", "localhost"),
			Port:          getEnvAsInt("RABBITMQ_PORT", 5672),
			User:          getEnv("RABBITMQ_USER", "guest"),
			Password:      getEnv("RABBITMQ_PASSWORD", "guest"),
			VHost:         getEnv("RABBITMQ_VHOST", ""),
			MaxRetries:    getEnvAsInt("RABBITMQ_MAX_RETRIES", 5),
			RetryDelay:    getEnvAsDuration("RABBITMQ_RETRY_DELAY", 5*time.Second),
			AvatarQueue:   getEnv("RABBITMQ_AVATAR_QUEUE", "avatar_process"),
			PurchaseQueue: getEnv("RABBITMQ_PURCHASE_QUEUE", "purchase_process"),
			CourseThumbnailQueue: getEnv("RABBITMQ_COURSE_THUMBNAIL_QUEUE", "course_thumbnail_process"),
		},
		JWT: JWTConfig{
			Secret:               getEnv("JWT_SECRET", ""),
			AccessTokenDuration:  getEnvAsDuration("ACCESS_TOKEN_DURATION", 15*time.Minute),
			RefreshTokenDuration: getEnvAsDuration("REFRESH_TOKEN_DURATION", 7*24*time.Hour),
			Issuer:               getEnv("JWT_ISSUER", "upraizo.com"),
		},
		AdminJWT: JWTConfig{
			Secret:               getEnv("ADMIN_JWT_SECRET", ""),
			AccessTokenDuration:  getEnvAsDuration("ADMIN_ACCESS_TOKEN_DURATION", 15*time.Minute),
			RefreshTokenDuration: getEnvAsDuration("ADMIN_REFRESH_TOKEN_DURATION", 7*24*time.Hour),
			Issuer:               getEnv("ADMIN_JWT_ISSUER", "admin.upraizo.com"),
		},
		Security: SecurityConfig{
			MaxLoginAttempts: getEnvAsInt("MAX_LOGIN_ATTEMPTS", 5),
			LockoutDuration:  getEnvAsDuration("LOCKOUT_DURATION", 30*time.Minute),
			RateLimitMax:     getEnvAsInt("RATE_LIMIT_MAX", 100),
		},
		Frontend: FrontendConfig{
			FrontendUrl:     getEnv("FRONTEND_URL", "http://localhost:4321"),
			VerificationUrl: getEnv("VERIFICATION_URL", "http://localhost:4321/auth/verify"),
			ResetUrl:        getEnv("RESET_URL", "http://localhost:4321/auth/reset"),
			AdminResetPasswordUrl: getEnv("ADMIN_RESET_PASSWORD_URL", "http://localhost:4321/admin/reset-password"),
			SecretResetUrl:  getEnv("SECRET_RESET_URL", "http://localhost:4321/auth/secret-reset"),
		},
		AWS: AWSConfig{
			AWSAccessKey: getEnv("AWS_ACCESS_KEY_ID", ""),
			AWSSecretKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
			AWSRegion:    getEnv("AWS_REGION", "eu-central-1"),
			AWSFromEmail: getEnv("AWS_FROM_EMAIL", ""),
		},
		Google: GoogleConfig{
			ClientID: getEnv("GOOGLE_CLIENT_ID", ""),
		},
		Razorpay: RazorpayConfig{
			KeyID:         getEnv("RAZORPAY_KEY_ID", ""),
			KeySecret:     getEnv("RAZORPAY_KEY_SECRET", ""),
			WebhookSecret: getEnv("RAZORPAY_WEBHOOK_SECRET", ""),
		},
		App: AppConfig{
			Env:               getEnv("ENV", "development"),
			Port:              getEnvAsInt("PORT", 8080),
			RequestBodyLimit:  getEnvAsInt("REQUEST_BODY_LIMIT", 10*1024*1024),
			ReadTimeout:       getEnvAsDuration("READ_TIMEOUT", 15*time.Second),
			WriteTimeout:      getEnvAsDuration("WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:       getEnvAsDuration("IDLE_TIMEOUT", 1*time.Minute),
			TrustedProxies:    getEnvAsSlice("TRUSTED_PROXIES", []string{"localhost", "[IP_ADDRESS]"}),
			AllowedOrigins:    getEnvAsSlice("ALLOWED_ORIGINS", []string{"*"}),
			ReadHeaderTimeout: getEnvAsDuration("READ_HEADER_TIMEOUT", 10*time.Second),
			EnableMetrics:     getEnvAsBool("ENABLE_METRICS", true),
			EnableHealth:      getEnvAsBool("ENABLE_HEALTH", true),
			TempStoragePath:   getEnv("TEMP_STORAGE_PATH", "./storage/tmp"),
		},
		Logger: LoggerConfig{
			Level:              getEnv("LOG_LEVEL", "info"),
			EnableConsole:      getEnvAsBool("LOG_ENABLE_CONSOLE", true),
			EnableFile:         getEnvAsBool("LOG_ENABLE_FILE", false),
			FilePath:           getEnv("LOG_FILE_PATH", "/var/log/app/app.log"),
			MaxSize:            getEnvAsInt("LOG_MAX_SIZE", 100),
			MaxBackups:         getEnvAsInt("LOG_MAX_BACKUPS", 10),
			MaxAge:             getEnvAsInt("LOG_MAX_AGE", 30),
			Compress:           getEnvAsBool("LOG_COMPRESS", true),
			EnableSampling:     getEnvAsBool("LOG_ENABLE_SAMPLING", false),
			SamplingInitial:    getEnvAsInt("LOG_SAMPLING_INITIAL", 10),
			SamplingThereafter: getEnvAsInt("LOG_SAMPLING_THEREAFTER", 100),
			EnableCaller:       getEnvAsBool("LOG_ENABLE_CALLER", false),
			EnableStackTrace:   getEnvAsBool("LOG_ENABLE_STACKTRACE", true),
		},
		R2: R2Config{
			AccountID:  getEnv("R2_ACCOUNT_ID", ""),
			AccessKey:  getEnv("R2_ACCESS_KEY", ""),
			SecretKey:  getEnv("R2_SECRET_KEY", ""),
			BucketName: getEnv("R2_BUCKET_NAME", ""),
			PublicURL:  getEnv("R2_PUBLIC_URL", "http://upraizo.com"),
		},
	}

	// Validate required fields
	if cfg.Database.Password == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required")
	}

	if cfg.JWT.Secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	if len(cfg.JWT.Secret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	if cfg.AdminJWT.Secret == "" {
		return nil, fmt.Errorf("ADMIN_JWT_SECRET is required")
	}

	if len(cfg.AdminJWT.Secret) < 32 {
		return nil, fmt.Errorf("ADMIN_JWT_SECRET must be at least 32 characters")
	}

	if cfg.App.Env == "production" && cfg.Redis.Password == "" {
		return nil, fmt.Errorf("REDIS_PASSWORD is required in production")
	}

	if cfg.AWS.AWSAccessKey == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID is required")
	}

	if cfg.AWS.AWSSecretKey == "" {
		return nil, fmt.Errorf("AWS_SECRET_ACCESS_KEY is required")
	}

	if cfg.Google.ClientID == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID is required")
	}

	if cfg.Razorpay.KeyID == "" {
		return nil, fmt.Errorf("RAZORPAY_KEY_ID is required")
	}

	if cfg.Razorpay.KeySecret == "" {
		return nil, fmt.Errorf("RAZORPAY_KEY_SECRET is required")
	}

	if cfg.App.IsProduction() && cfg.Razorpay.WebhookSecret == "" {
		return nil, fmt.Errorf("RAZORPAY_WEBHOOK_SECRET is required in production")
	}

	// Validate log level
	validLevels := []string{"trace", "debug", "info", "warn", "error", "fatal", "panic"}
	if !contains(validLevels, cfg.Logger.Level) {
		return nil, fmt.Errorf("LOG_LEVEL must be one of: %v", validLevels)
	}

	return cfg, nil
}

/* ───────────────── Helper Functions ───────────────── */

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return fallback
}

func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return fallback
}

func getEnvAsSlice(key string, fallback []string) []string {
	if value := os.Getenv(key); value != "" {
		// Split by comma and trim spaces
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return fallback
}

func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

/* ───────────────── DSN Builders ───────────────── */

// GetDSN builds PostgreSQL connection string
func (c *DatabaseConfig) GetDSN() string {
	sslModeStr := "disable"
	if c.SSLMode {
		sslModeStr = "require"
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		c.Host,
		c.Port,
		c.User,
		c.Password,
		c.DBName,
		sslModeStr,
		int(c.ConnectTimeout.Seconds()),
	)

	return dsn
}

// GetRedisAddr returns Redis address
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetURL returns RabbitMQ connection URL
func (c *RabbitMQConfig) GetURL() string {
	// amqp://user:password@host:port/vhost
	return fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		c.User, c.Password, c.Host, c.Port, c.VHost)
}

/* ───────────────── Environment-Specific Helpers ───────────────── */

// IsDevelopment checks if running in development mode
func (c *AppConfig) IsDevelopment() bool {
	return c.Env == "development"
}

// IsProduction checks if running in production mode
func (c *AppConfig) IsProduction() bool {
	return c.Env == "production"
}

// IsStaging checks if running in staging mode
func (c *AppConfig) IsStaging() bool {
	return c.Env == "staging"
}

func (c *AppConfig) AvatarTempPath() string {
	return filepath.Join(c.TempStoragePath, "avatars")
}

func (c *AppConfig) VideoTempPath() string {
	return filepath.Join(c.TempStoragePath, "videos")
}

func (c *AppConfig) CourseTempPath() string {
	return filepath.Join(c.TempStoragePath, "courses")
}

func (c *AppConfig) SupportTempPath() string {
	return filepath.Join(c.TempStoragePath, "support")
}

// EnsureTempDirs creates all necessary temporary directories
func (c *AppConfig) EnsureTempDirs() error {
	dirs := []string{
		c.TempStoragePath,
		c.AvatarTempPath(),
		c.VideoTempPath(),
		c.CourseTempPath(),
		c.SupportTempPath(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

package middleware

import (
	"context"
	"fmt"
	"strconv"
	"time"

	redisstore "server/internal/redis"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
)

// ───────────────────────────────────────────────────────
//  Config
// ───────────────────────────────────────────────────────

// LimitConfig defines the behaviour of a single rate-limit rule.
type LimitConfig struct {
	// Max is the number of requests allowed within Window.
	Max int

	// Window is the duration of the fixed time bucket (e.g. time.Minute).
	Window time.Duration

	// KeyFunc derives the rate-limit key from the request.
	// Defaults to the client IP address when nil.
	KeyFunc func(*fiber.Ctx) string

	// Message is the user-facing 429 error message.
	// Defaults to a sensible generic message when empty.
	Message string

	// SkipFunc, when non-nil, bypasses the limiter for matched requests.
	// Useful for health-check routes or internal traffic.
	SkipFunc func(*fiber.Ctx) bool
}

// ───────────────────────────────────────────────────────
//  Lua script — atomic INCR + conditional EXPIRE
// ───────────────────────────────────────────────────────
//
// Using a Lua script ensures the increment and the TTL assignment happen
// in one atomic Redis round-trip, which eliminates the race condition that
// exists when INCR and EXPIRE are issued as two separate commands.
const luaIncrExpire = `
local count = redis.call('INCR', KEYS[1])
if count == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return count
`

// ───────────────────────────────────────────────────────
//  Core middleware
// ───────────────────────────────────────────────────────

// RateLimiter returns a Fiber middleware that enforces a fixed-window rate
// limit backed by Redis.
//
// The counter key is scoped to the resolved identifier AND the current
// time-window slot, so the bucket automatically resets after cfg.Window
// without any background cleanup jobs.
//
// On Redis failure the middleware fails open (lets the request through) and
// logs the error so that a Redis outage does not take down the API.
func RateLimiter(cfg LimitConfig) fiber.Handler {
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = ipKeyFunc
	}
	if cfg.Message == "" {
		cfg.Message = "Too many requests, please slow down."
	}

	windowSecs := int64(cfg.Window.Seconds())
	script := redisstore.Client.ScriptLoad(context.Background(), luaIncrExpire)

	return func(c *fiber.Ctx) error {
		if cfg.SkipFunc != nil && cfg.SkipFunc(c) {
			return c.Next()
		}

		// Derive a stable bucket key for the current time window.
		slot := time.Now().Unix() / windowSecs
		key := fmt.Sprintf("rl:%s:%d", cfg.KeyFunc(c), slot)

		ctx := c.Context()

		// Run the atomic Lua script.
		sha, err := script.Result()
		if err != nil {
			utils.Error("rate limiter: failed to load Lua script", err, nil)
			return c.Next() // fail open
		}

		res, err := redisstore.Client.EvalSha(ctx, sha, []string{key},
			strconv.FormatInt(windowSecs, 10),
		).Int64()
		if err != nil {
			utils.Error("rate limiter: Redis eval error", err,
				map[string]any{"key": key},
			)
			return c.Next() // fail open
		}

		count := int(res)
		remaining := max(cfg.Max - count, 0)

		// TTL for Retry-After / X-RateLimit-Reset.
		ttl, _ := redisstore.Client.TTL(ctx, key).Result()
		resetAt := time.Now().Add(ttl).Unix()

		// Standard rate-limit response headers.
		c.Set("X-RateLimit-Limit", strconv.Itoa(cfg.Max))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))

		if count > cfg.Max {
			c.Set("Retry-After", strconv.Itoa(int(ttl.Seconds())))
			appErr := utils.TooManyRequests(cfg.Message)
			return c.Status(appErr.Code).JSON(appErr)
		}

		return c.Next()
	}
}

// ───────────────────────────────────────────────────────
//  Key-func helpers  (compose these in your own limits)
// ───────────────────────────────────────────────────────

// ipKeyFunc keys the limit on the client's remote IP.
func ipKeyFunc(c *fiber.Ctx) string {
	return "ip:" + c.IP()
}

// UserIDKeyFunc keys the limit on the authenticated user ID stored in
// Fiber locals (falls back to IP for unauthenticated requests).
func UserIDKeyFunc(c *fiber.Ctx) string {
	if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
		return "uid:" + uid
	}
	return "ip:" + c.IP()
}

// RouteKeyFunc keys the limit on IP + route path, useful when you want
// per-endpoint limits without writing a separate middleware per route.
func RouteKeyFunc(c *fiber.Ctx) string {
	return fmt.Sprintf("ip:%s:route:%s", c.IP(), c.Path())
}

// ───────────────────────────────────────────────────────
//  Preset limiters  (drop-in usage in route definitions)
// ───────────────────────────────────────────────────────

// General is the default limit for public, non-sensitive endpoints.
// 120 requests per minute per IP — comfortable for normal browsing/API use.
func General() fiber.Handler {
	return RateLimiter(LimitConfig{
		Max:     120,
		Window:  time.Minute,
		Message: "Too many requests. Please wait a moment and try again.",
	})
}

// Strict is for sensitive read endpoints (e.g. user profile, course detail).
// 30 requests per minute per IP — tighter but not disruptive for real users.
func Strict() fiber.Handler {
	return RateLimiter(LimitConfig{
		Max:     30,
		Window:  time.Minute,
		Message: "Too many requests. Please slow down.",
	})
}

// Auth is for login and registration endpoints.
// 10 attempts per 5 minutes per IP — enough for legitimate retries, tight
// enough to make credential-stuffing impractical.
func Auth() fiber.Handler {
	return RateLimiter(LimitConfig{
		Max:     10,
		Window:  5 * time.Minute,
		Message: "Too many login attempts. Please wait 5 minutes and try again.",
	})
}

// OTP is for one-time-password send/verify endpoints.
// 3 attempts per 10 minutes per IP — prevents OTP flooding and SMS abuse.
func OTP() fiber.Handler {
	return RateLimiter(LimitConfig{
		Max:     3,
		Window:  10 * time.Minute,
		Message: "OTP request limit reached. Please wait 10 minutes before requesting again.",
	})
}

// API is for authenticated API consumers (mobile app, third-party clients).
// 600 requests per hour keyed on user ID so shared IPs (offices, NAT) are
// not penalised as a group.
func API() fiber.Handler {
	return RateLimiter(LimitConfig{
		Max:     600,
		Window:  time.Hour,
		KeyFunc: UserIDKeyFunc,
		Message: "API rate limit exceeded. Your limit resets in one hour.",
	})
}
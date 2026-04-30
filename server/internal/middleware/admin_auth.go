package middleware

import (
	"server/internal/redis"
	"server/internal/security"

	"github.com/gofiber/fiber/v2"
)

// AdminAuthRequired is the middleware for protecting administrative routes.
// It verifies that the incoming token is a valid Admin JWT and that the
// session still exists in Redis (Stateful security).
func AdminAuthRequired(m *security.AdminJWTManager, r *redis.AdminSessionRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		tokenStr, err := security.ExtractToken(authHeader)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, err.Error())
		}

		claims, err := m.ValidateToken(tokenStr, security.Access)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, err.Error())
		}

		// ── Stateful session check ──
		sessionID := claims.RegisteredClaims.ID
		_, err = r.GetSession(c.Context(), sessionID)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "Session expired or invalid")
		}

		// Store admin info in context for downstream handlers
		c.Locals("admin_id", claims.AdminID)
		c.Locals("admin_email", claims.Email)
		c.Locals("session_id", sessionID)
		c.Locals("role", "admin")

		return c.Next()
	}
}

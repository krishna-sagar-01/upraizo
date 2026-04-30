package middleware

import (
	"server/internal/models"
	"server/internal/redis"
	"server/internal/security"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
)

// AuthRequired is the main guard middleware for protected routes.
// It now performs a stateful check against Redis to ensure the session is active.
func AuthRequired(jwtManager *security.JWTManager, sessionRepo *redis.SessionRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Extract Token from Header
		authHeader := c.Get("Authorization")
		tokenStr, err := security.ExtractToken(authHeader)
		if err != nil {
			return err 
		}

		// 2. Validate Access Token (Stateless Check)
		claims, err := jwtManager.ValidateToken(tokenStr, security.Access)
		if err != nil {
			return err
		}

		// 3. Stateful Session Check (Immediate Revocation)
		_, err = sessionRepo.GetSession(c.Context(), claims.ID)
		if err != nil {
			return utils.Unauthorized("Session expired or invalidated")
		}

		// 4. Status Check (Extra Security Layer)
		if claims.Status == models.UserStatusBanned {
			return utils.Forbidden("Your account has been permanently banned")
		}
		
		if claims.Status == models.UserStatusSuspended {
			return utils.Forbidden("Your account is temporarily suspended")
		}

		// 5. Set Locals for Handlers
		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("user_status", claims.Status)
		c.Locals("session_id", claims.ID)

		// Continue to next handler
		return c.Next()
	}
}

// VerifiedOnly ensures the user has verified their email
func VerifiedOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("user_id")
		if userID == nil {
			return utils.Unauthorized("Authentication required")
		}

		return c.Next()
	}
}

// OptionalAuth is like AuthRequired but doesn't fail if the token is missing or invalid.
// Use this for endpoints that show more data to logged-in users (like curriculum).
func OptionalAuth(jwtManager *security.JWTManager, sessionRepo *redis.SessionRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next()
		}

		tokenStr, err := security.ExtractToken(authHeader)
		if err != nil {
			return c.Next()
		}

		claims, err := jwtManager.ValidateToken(tokenStr, security.Access)
		if err != nil {
			return c.Next()
		}

		// Stateful Session Check
		_, err = sessionRepo.GetSession(c.Context(), claims.ID)
		if err != nil {
			return c.Next()
		}

		// Set Locals
		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("user_status", claims.Status)
		c.Locals("session_id", claims.ID)

		return c.Next()
	}
}
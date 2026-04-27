package middleware

import (
	"server/internal/models"
	"server/internal/security"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
)

// AuthRequired is the main guard middleware for protected routes
func AuthRequired(jwtManager *security.JWTManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Extract Token from Header
		authHeader := c.Get("Authorization")
		tokenStr, err := security.ExtractToken(authHeader)
		if err != nil {
			// Returns 401 Unauthorized via our AppError system
			return err 
		}

		// 2. Validate Access Token
		claims, err := jwtManager.ValidateToken(tokenStr, security.Access)
		if err != nil {
			return err
		}

		// 3. Status Check (Extra Security Layer)
		if claims.Status == models.UserStatusBanned {
			return utils.Forbidden("Your account has been permanently banned")
		}
		
		if claims.Status == models.UserStatusSuspended {
			return utils.Forbidden("Your account is temporarily suspended")
		}

		// 4. Set Locals for Handlers
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
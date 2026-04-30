package handler

import (
	"net/http"

	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ─── POST /auth/refresh ─────────────────────────────────────────────────────

// RefreshTokens reads the refresh token from the HttpOnly cookie, rotates
// the session, and returns a fresh token pair.  No AuthRequired middleware
// needed — the refresh token itself proves identity.
func (h *AuthHandler) RefreshTokens(c *fiber.Ctx) error {
	refreshToken := c.Cookies("refresh_token")
	if refreshToken == "" {
		return utils.Unauthorized("No refresh token provided")
	}

	resp, newRefreshToken, err := h.sessionService.RefreshTokens(
		c.Context(), refreshToken, c.IP(), c.Get("User-Agent"),
	)
	if err != nil {
		h.clearRefreshCookie(c) // nuke stale cookie on any failure
		return err
	}

	h.setRefreshCookie(c, newRefreshToken)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// ─── POST /auth/logout ──────────────────────────────────────────────────────

// Logout revokes the single session linked to the refresh-token cookie.
// Does NOT require AuthRequired — the user might call this after access-token
// expiry.  Always clears the cookie regardless of backend outcome.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	refreshToken := c.Cookies("refresh_token")

	resp, err := h.sessionService.Logout(c.Context(), refreshToken)
	if err != nil {
		return err
	}

	h.clearRefreshCookie(c)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// ─── POST /auth/logout-all ──────────────────────────────────────────────────

// LogoutAll revokes every active session for the authenticated user.
// Requires AuthRequired middleware (access token must be valid).
func (h *AuthHandler) LogoutAll(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return utils.Unauthorized("Authentication required")
	}

	resp, err := h.sessionService.LogoutAll(c.Context(), userID.String())
	if err != nil {
		return err
	}

	h.clearRefreshCookie(c)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// ─── GET /auth/sessions ─────────────────────────────────────────────────────

// GetSessions returns all active sessions for the user.
func (h *AuthHandler) GetSessions(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	sessionID, _ := c.Locals("session_id").(string)

	if !ok {
		return utils.Unauthorized("Authentication required")
	}

	sessions, err := h.sessionService.GetActiveSessions(c.Context(), userID.String(), sessionID)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    sessions,
	})
}

// ─── DELETE /auth/sessions/:id ──────────────────────────────────────────────

// RevokeSession revokes a specific session by its ID.
func (h *AuthHandler) RevokeSession(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return utils.Unauthorized("Authentication required")
	}

	targetID := c.Params("id")
	if targetID == "" {
		return utils.BadRequest("Session ID is required")
	}

	resp, err := h.sessionService.RevokeSession(c.Context(), userID.String(), targetID)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}
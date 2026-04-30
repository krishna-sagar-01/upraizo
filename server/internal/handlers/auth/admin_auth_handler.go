package handler

import (
	"net/http"
	"time"

	"server/internal/dto"
	service "server/internal/services/auth"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)


type AdminAuthHandler struct {
	svc *service.AdminAuthService
}

func NewAdminAuthHandler(svc *service.AdminAuthService) *AdminAuthHandler {
	return &AdminAuthHandler{svc: svc}
}

// ─── Login ──────────────────────────────────────────

func (h *AdminAuthHandler) Login(c *fiber.Ctx) error {
	var req dto.AdminLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return err
	}

	ip := c.IP()
	userAgent := c.Get("User-Agent")

	resp, refreshToken, err := h.svc.Login(c.Context(), req.Email, req.Password, req.SecretKey, ip, userAgent)
	if err != nil {
		return err
	}

	h.setRefreshTokenCookie(c, refreshToken)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// ─── Logout ──────────────────────────────────────────


func (h *AdminAuthHandler) Logout(c *fiber.Ctx) error {
	refreshToken := c.Cookies("admin_refresh_token")

	if err := h.svc.Logout(c.Context(), refreshToken); err != nil {
		// Log but don't fail logout
		utils.Error("Admin logout error", err, nil)
	}

	h.clearRefreshTokenCookie(c)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Admin logged out successfully",
	})
}

// ─── Logout All ──────────────────────────────────────

func (h *AdminAuthHandler) LogoutAll(c *fiber.Ctx) error {
	adminID, ok := c.Locals("admin_id").(uuid.UUID)
	if !ok {
		return utils.Unauthorized("Authentication required")
	}

	if err := h.svc.RevokeAllSessions(c.Context(), adminID.String()); err != nil {
		return err
	}

	h.clearRefreshTokenCookie(c)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "All admin sessions revoked successfully",
	})
}

// ─── GET Sessions ────────────────────────────────────

func (h *AdminAuthHandler) GetSessions(c *fiber.Ctx) error {
	adminID, ok := c.Locals("admin_id").(uuid.UUID)
	sessionID, _ := c.Locals("session_id").(string)

	if !ok {
		return utils.Unauthorized("Authentication required")
	}

	sessions, err := h.svc.GetActiveSessions(c.Context(), adminID.String(), sessionID)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    sessions,
	})
}

// ─── Revoke Session ──────────────────────────────────


func (h *AdminAuthHandler) RevokeSession(c *fiber.Ctx) error {
	adminID, ok := c.Locals("admin_id").(uuid.UUID)
	if !ok {
		return utils.Unauthorized("Authentication required")
	}

	targetID := c.Params("id")
	if targetID == "" {
		return utils.BadRequest("Session ID is required")
	}

	if err := h.svc.RevokeSession(c.Context(), adminID.String(), targetID); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Admin session revoked successfully",
	})
}

// ─── Profile Management ──────────────────────────────

func (h *AdminAuthHandler) GetMe(c *fiber.Ctx) error {
	adminID, ok := c.Locals("admin_id").(uuid.UUID)
	if !ok {
		return utils.Unauthorized("Authentication required")
	}

	admin, err := h.svc.GetByID(c.Context(), adminID)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    admin,
	})
}

func (h *AdminAuthHandler) UpdateProfile(c *fiber.Ctx) error {
	adminID, ok := c.Locals("admin_id").(uuid.UUID)
	if !ok {
		return utils.Unauthorized("Authentication required")
	}

	var req dto.UpdateAdminProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return err
	}

	updatedAdmin, err := h.svc.UpdateProfile(c.Context(), adminID, req.Name, req.Phone)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Profile updated successfully",
		"data":    updatedAdmin,
	})
}



// ─── Refresh ──────────────────────────────────────────

func (h *AdminAuthHandler) Refresh(c *fiber.Ctx) error {
	refreshToken := c.Cookies("admin_refresh_token")
	if refreshToken == "" {
		return utils.Unauthorized("Missing admin refresh token")
	}

	ip := c.IP()
	userAgent := c.Get("User-Agent")

	resp, newRefreshToken, err := h.svc.RefreshTokens(c.Context(), refreshToken, ip, userAgent)
	if err != nil {
		h.clearRefreshTokenCookie(c)
		return err
	}

	h.setRefreshTokenCookie(c, newRefreshToken)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// ─── Password Recovery ──────────────────────────────────────

func (h *AdminAuthHandler) ForgotPassword(c *fiber.Ctx) error {
	var req dto.AdminForgotPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return err
	}

	if err := h.svc.ForgotPassword(c.Context(), req.Name, req.Email, req.Phone); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "If matching credentials exist, a reset link has been sent to your email.",
	})
}

func (h *AdminAuthHandler) ResetPassword(c *fiber.Ctx) error {
	var req dto.AdminResetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return err
	}

	if err := h.svc.ResetPassword(c.Context(), req.Token, req.Password); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Password has been reset successfully. All active sessions have been revoked.",
	})
}

// ─── Secret Key Recovery ────────────────────────────────────

func (h *AdminAuthHandler) ForgotSecret(c *fiber.Ctx) error {
	var req dto.AdminForgotSecretRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return err
	}

	if err := h.svc.ForgotSecretRequest(c.Context(), req.Name, req.Email, req.Phone, req.Password); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "If matching credentials exist, a secret reset link has been sent to your email.",
	})
}

func (h *AdminAuthHandler) ResetSecret(c *fiber.Ctx) error {
	var req dto.AdminResetSecretRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return err
	}

	if err := h.svc.ResetSecretKey(c.Context(), req.Token, req.NewSecretKey); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Secret key has been reset successfully. All active sessions have been revoked.",
	})
}

// ─── Cookie Helpers ──────────────────────────────────────────

func (h *AdminAuthHandler) setRefreshTokenCookie(c *fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name:     "admin_refresh_token",
		Value:    token,
		Path:     "/",
		HTTPOnly: true,
		Secure:   true, // Requirement for production
		SameSite: "Lax",
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	})
}

func (h *AdminAuthHandler) clearRefreshTokenCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "admin_refresh_token",
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
		Expires:  time.Now().Add(-1 * time.Hour),
	})
}

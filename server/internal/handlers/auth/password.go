package handler

import (
	"net/http"

	"server/internal/dto"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ─── POST /auth/forgot-password ──────────────────────────────────────────────

func (h *AuthHandler) ForgotPassword(c *fiber.Ctx) error {
	var req dto.ForgotPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if appErr := utils.ValidateStruct(&req); appErr != nil {
		return appErr
	}

	resp, err := h.passwordService.ForgotPassword(c.Context(), req.Email)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// ─── POST /auth/reset-password ───────────────────────────────────────────────

func (h *AuthHandler) ResetPassword(c *fiber.Ctx) error {
	var req dto.ResetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if appErr := utils.ValidateStruct(&req); appErr != nil {
		return appErr
	}

	resp, err := h.passwordService.ResetPassword(c.Context(), req.Token, req.Password)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// ─── PUT /auth/change-password (AuthRequired) ────────────────────────────────

func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return utils.Unauthorized("Authentication required")
	}

	var req dto.ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if appErr := utils.ValidateStruct(&req); appErr != nil {
		return appErr
	}

	resp, err := h.passwordService.ChangePassword(c.Context(), userID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}
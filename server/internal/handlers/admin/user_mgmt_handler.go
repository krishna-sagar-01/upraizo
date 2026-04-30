package handler

import (
	"net/http"
	"strconv"

	"server/internal/dto"
	"server/internal/services/admin"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type UserMgmtHandler struct {
	svc *service.UserMgmtService
}

func NewUserMgmtHandler(svc *service.UserMgmtService) *UserMgmtHandler {
	return &UserMgmtHandler{svc: svc}
}

// ListUsers handles GET /admin/users?page=1&limit=10&status=active
func (h *UserMgmtHandler) ListUsers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	status := c.Query("status", "")

	resp, err := h.svc.ListUsers(c.Context(), page, limit, status)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// GetStats handles GET /admin/stats
func (h *UserMgmtHandler) GetStats(c *fiber.Ctx) error {
	stats, err := h.svc.GetStats(c.Context())
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    stats,
	})
}

// UpdateUserStatus handles PATCH /admin/users/:id/status
func (h *UserMgmtHandler) UpdateUserStatus(c *fiber.Ctx) error {
	idParam := c.Params("id")
	userID, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequest("Invalid user ID format")
	}

	var req dto.AdminUserUpdateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return err
	}

	if err := h.svc.UpdateUserStatus(c.Context(), userID, req.Status, req.Reason); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "User status updated successfully",
	})
}

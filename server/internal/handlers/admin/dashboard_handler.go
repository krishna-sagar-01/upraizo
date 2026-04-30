package handler

import (
	"server/internal/services/admin"
	"github.com/gofiber/fiber/v2"
)

type DashboardHandler struct {
	svc *service.DashboardService
}

func NewDashboardHandler(svc *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

// GetSummary godoc
// @Summary Get unified dashboard summary (Admin)
// @Tags Admin Dashboard
// @Security AdminAuth
// @Produce json
// @Success 200 {object} dto.AdminDashboardSummary
// @Router /api/v1/admin/dashboard/summary [get]
func (h *DashboardHandler) GetSummary(c *fiber.Ctx) error {
	summary, err := h.svc.GetSummary(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to generate dashboard summary",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    summary,
	})
}

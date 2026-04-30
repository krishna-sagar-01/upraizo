package handler

import (
	"server/internal/services/admin"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type AdminPurchaseHandler struct {
	adminPurchaseSvc *service.AdminPurchaseService
}

func NewAdminPurchaseHandler(adminPurchaseSvc *service.AdminPurchaseService) *AdminPurchaseHandler {
	return &AdminPurchaseHandler{
		adminPurchaseSvc: adminPurchaseSvc,
	}
}

// ListPayments godoc
// @Summary List all course payments (Admin)
// @Tags Admin Payments
// @Security AdminAuth
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} []dto.AdminPurchaseResponse
// @Router /api/v1/admin/payments [get]
func (h *AdminPurchaseHandler) ListPayments(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	if page < 1 { page = 1 }
	if limit < 1 || limit > 100 { limit = 20 }

	payments, err := h.adminPurchaseSvc.ListAllPayments(c.Context(), page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch payments",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    payments,
	})
}

// GetSalesStats godoc
// @Summary Get sales performance metrics (Admin)
// @Tags Admin Payments
// @Security AdminAuth
// @Produce json
// @Success 200 {object} dto.AdminSalesStats
// @Router /api/v1/admin/payments/stats [get]
func (h *AdminPurchaseHandler) GetSalesStats(c *fiber.Ctx) error {
	stats, err := h.adminPurchaseSvc.GetSalesStats(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch sales stats",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    stats,
	})
}

func (h *AdminPurchaseHandler) GetPaymentDetails(c *fiber.Ctx) error {
	// For now, details are included in ListPayments, but we can implement specific lookup if needed.
	// But let's keep it simple for now as requested.
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"success": false,
		"message": "Specific payment details endpoint not yet implemented. Use the main list.",
	})
}

package handler

import (
	"net/http"
	service "server/internal/services/user"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type StudentDashboardHandler struct {
	svc *service.StudentDashboardService
}

func NewStudentDashboardHandler(svc *service.StudentDashboardService) *StudentDashboardHandler {
	return &StudentDashboardHandler{svc: svc}
}

// GetSummary handles GET /api/v1/user/dashboard/summary
func (h *StudentDashboardHandler) GetSummary(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return utils.Unauthorized("User not found in session")
	}

	summary, err := h.svc.GetSummary(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    summary,
	})
}

// GetMyCourses handles GET /api/v1/user/courses/my
func (h *StudentDashboardHandler) GetMyCourses(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return utils.Unauthorized("User not found in session")
	}

	courses, err := h.svc.GetMyCourses(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    courses,
	})
}

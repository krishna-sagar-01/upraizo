package handler

import (
	"net/http"
	"server/internal/dto"
	service "server/internal/services/course"
	"server/internal/utils"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ModuleHandler struct {
	svc *service.ModuleService
}

func NewModuleHandler(svc *service.ModuleService) *ModuleHandler {
	return &ModuleHandler{svc: svc}
}

func (h *ModuleHandler) Create(c *fiber.Ctx) error {
	idStr := c.Params("id")
	courseID, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest("Invalid course ID in URL")
	}

	var req dto.CreateModuleRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	req.CourseID = courseID

	if err := utils.ValidateStruct(&req); err != nil {
		return err
	}

	res, err := h.svc.Create(c.Context(), req)
	if err != nil {
		return err
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    res,
	})
}

func (h *ModuleHandler) GetByCourseID(c *fiber.Ctx) error {
	courseIDStr := c.Params("id")
	courseID, err := uuid.Parse(courseIDStr)
	if err != nil {
		return utils.BadRequest("Invalid course ID")
	}

	res, err := h.svc.GetByCourseID(c.Context(), courseID)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    res,
	})
}

func (h *ModuleHandler) Update(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest("Invalid module ID")
	}

	var req dto.UpdateModuleRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return err
	}

	res, err := h.svc.Update(c.Context(), id, req)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    res,
	})
}

func (h *ModuleHandler) Delete(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest("Invalid module ID")
	}

	permanent, _ := strconv.ParseBool(c.Query("permanent", "false"))

	if err := h.svc.Delete(c.Context(), id, permanent); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Module deleted successfully",
	})
}

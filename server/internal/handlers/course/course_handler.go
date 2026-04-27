package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"server/internal/dto"
	service "server/internal/services/course"
	"server/internal/utils"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type CourseHandler struct {
	svc *service.CourseService
}

func NewCourseHandler(svc *service.CourseService) *CourseHandler {
	return &CourseHandler{svc: svc}
}

// Create: POST /courses
func (h *CourseHandler) Create(c *fiber.Ctx) error {
	var req dto.CreateCourseRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return err
	}

	if req.DiscountExpiresAt == nil {
		dateStr := c.FormValue("discount_expires_at")
		if dateStr != "" {
			t, err := time.Parse(time.RFC3339, dateStr)
			if err == nil {
				req.DiscountExpiresAt = &t
			}
		}
	}

	res, err := h.svc.Create(c.Context(), req)
	if err != nil {
		return err
	}

	file, err := c.FormFile("thumbnail")
	if err == nil && file != nil {
		if file.Size > 5*1024*1024 {
			return utils.BadRequest("Thumbnail too large (max 5MB)")
		}

		tempDir := h.svc.Config().App.CourseTempPath()
		tempFileName := fmt.Sprintf("%s_%d%s", res.ID.String(), time.Now().Unix(), filepath.Ext(file.Filename))
		localPath := filepath.Join(tempDir, tempFileName)

		if err := c.SaveFile(file, localPath); err != nil {
			utils.Error("Failed to save temp thumbnail", err, map[string]any{"course_id": res.ID})
		} else {
			if err := h.svc.QueueThumbnailUpdate(c.Context(), res.ID, localPath); err != nil {
				utils.Error("Failed to queue thumbnail task", err, map[string]any{"course_id": res.ID})
			}
		}
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    res,
	})
}

// List: GET /courses (Public)
func (h *CourseHandler) List(c *fiber.Ctx) error {
	onlyPublished := true
	if c.Query("all") == "true" {
		onlyPublished = false
	}

	res, err := h.svc.List(c.Context(), onlyPublished)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"count":   len(res),
		"data":    res,
	})
}

// GetByID: GET /courses/:id (Admin)
func (h *CourseHandler) GetByID(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest("Invalid course ID")
	}

	res, err := h.svc.GetByID(c.Context(), id)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    res,
	})
}

// GetBySlug: GET /courses/:slug (Public)
func (h *CourseHandler) GetBySlug(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return utils.BadRequest("Slug is required")
	}

	res, err := h.svc.GetBySlug(c.Context(), slug)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    res,
	})
}

// GetCurriculum: GET /courses/:slug/curriculum (Public/Student)
func (h *CourseHandler) GetCurriculum(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return utils.BadRequest("Slug is required")
	}

	res, err := h.svc.GetCurriculum(c.Context(), slug)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    res,
	})
}

// Update: PUT /courses/:id
func (h *CourseHandler) Update(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest("Invalid course ID")
	}

	var req dto.UpdateCourseRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return err
	}

	if req.DiscountExpiresAt == nil {
		dateStr := c.FormValue("discount_expires_at")
		if dateStr != "" {
			t, err := time.Parse(time.RFC3339, dateStr)
			if err == nil {
				req.DiscountExpiresAt = &t
			}
		}
	}

	res, err := h.svc.Update(c.Context(), id, req)
	if err != nil {
		return err
	}

	file, err := c.FormFile("thumbnail")
	if err == nil && file != nil {
		tempDir := h.svc.Config().App.CourseTempPath()
		tempFileName := fmt.Sprintf("%s_%d%s", id.String(), time.Now().Unix(), filepath.Ext(file.Filename))
		localPath := filepath.Join(tempDir, tempFileName)

		if err := c.SaveFile(file, localPath); err != nil {
			utils.Error("Failed to save temp thumbnail for update", err, map[string]any{"course_id": id})
		} else {
			if err := h.svc.QueueThumbnailUpdate(c.Context(), id, localPath); err != nil {
				utils.Error("Failed to queue thumbnail task for update", err, map[string]any{"course_id": id})
			}
		}
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    res,
	})
}

// Delete: DELETE /courses/:id
func (h *CourseHandler) Delete(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest("Invalid course ID")
	}

	permanent, _ := strconv.ParseBool(c.Query("permanent", "false"))

	if err := h.svc.Delete(c.Context(), id, permanent); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Course deleted successfully",
	})
}

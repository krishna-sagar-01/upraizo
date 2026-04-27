package handler

import (
	"net/http"
	"server/internal/dto"
	service "server/internal/services/course"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type LessonHandler struct {
	svc *service.LessonService
}

func NewLessonHandler(svc *service.LessonService) *LessonHandler {
	return &LessonHandler{svc: svc}
}

// ───────────────── LESSON CRUD ─────────────────

func (h *LessonHandler) Create(c *fiber.Ctx) error {
	var req dto.CreateLessonRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

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

func (h *LessonHandler) GetByID(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest("Invalid lesson ID")
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

func (h *LessonHandler) ListByModule(c *fiber.Ctx) error {
	moduleIDStr := c.Params("moduleId")
	moduleID, err := uuid.Parse(moduleIDStr)
	if err != nil {
		return utils.BadRequest("Invalid module ID")
	}

	res, err := h.svc.ListByModule(c.Context(), moduleID)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    res,
	})
}

func (h *LessonHandler) Update(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest("Invalid lesson ID")
	}

	var req dto.UpdateLessonRequest
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

func (h *LessonHandler) Delete(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest("Invalid lesson ID")
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Lesson deleted successfully",
	})
}

// ───────────────── VIDEO & ATTACHMENTS ─────────────────

func (h *LessonHandler) UploadVideo(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest("Invalid lesson ID")
	}

	file, err := c.FormFile("video")
	if err != nil {
		return utils.BadRequest("Video file is required")
	}

	// Validate file type (basic)
	if file.Header.Get("Content-Type") != "video/mp4" && file.Header.Get("Content-Type") != "video/quicktime" {
		// Just a warning for now, let FFmpeg handle actual validation or strict type checking
	}

	fh, err := file.Open()
	if err != nil {
		return utils.Internal(err)
	}
	defer fh.Close()

	if err := h.svc.UploadVideo(c.Context(), id, fh, file.Filename); err != nil {
		return err
	}

	return c.Status(http.StatusAccepted).JSON(fiber.Map{
		"success": true,
		"message": "Video upload started and moved to processing queue",
	})
}

func (h *LessonHandler) GetVideoProgress(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest("Invalid lesson ID")
	}

	res, err := h.svc.GetVideoProgress(c.Context(), id)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    res,
	})
}

func (h *LessonHandler) AddAttachment(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest("Invalid lesson ID")
	}

	title := c.FormValue("title")
	if title == "" {
		return utils.BadRequest("Attachment title is required")
	}

	file, err := c.FormFile("file")
	if err != nil {
		return utils.BadRequest("File is required")
	}

	fh, err := file.Open()
	if err != nil {
		return utils.Internal(err)
	}
	defer fh.Close()

	res, err := h.svc.AddAttachment(c.Context(), id, title, fh, file.Filename, file.Header.Get("Content-Type"), file.Size)
	if err != nil {
		return err
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    res,
	})
}

func (h *LessonHandler) DeleteAttachment(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequest("Invalid attachment ID")
	}

	if err := h.svc.DeleteAttachment(c.Context(), id); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Attachment deleted successfully",
	})
}

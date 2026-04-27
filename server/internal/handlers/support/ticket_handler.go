package handler

import (
	"net/http"
	"server/internal/dto"
	service "server/internal/services/support"
	"server/internal/utils"

	"fmt"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type TicketHandler struct {
	svc *service.TicketService
}

func NewTicketHandler(svc *service.TicketService) *TicketHandler {
	return &TicketHandler{svc: svc}
}

// ───────────────── USER ENDPOINTS ─────────────────

func (h *TicketHandler) OpenTicket(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok { return utils.Unauthorized("Auth required") }

	var req dto.CreateTicketRequest
	// In multipart, we might need to parse fields manually or use BodyParser
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request fields")
	}

	// 1. Handle Multiple Attachments
	localPaths, err := h.saveAttachments(c)
	if err != nil { return err }

	ticket, err := h.svc.OpenTicket(c.Context(), userID, req, localPaths)
	if err != nil { return err }

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    ticket,
	})
}

func (h *TicketHandler) GetMyTickets(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok { return utils.Unauthorized("Auth required") }

	tickets, err := h.svc.GetUserTickets(c.Context(), userID)
	if err != nil { return err }

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    tickets,
	})
}

func (h *TicketHandler) GetTicket(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok { return utils.Unauthorized("Auth required") }

	ticketID, err := uuid.Parse(c.Params("id"))
	if err != nil { return utils.BadRequest("Invalid ticket ID") }

	ticket, err := h.svc.GetTicket(c.Context(), ticketID, userID, false)
	if err != nil { return err }

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    ticket,
	})
}

func (h *TicketHandler) GetConversation(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok { return utils.Unauthorized("Auth required") }

	ticketID, err := uuid.Parse(c.Params("id"))
	if err != nil { return utils.BadRequest("Invalid ticket ID") }

	messages, err := h.svc.GetConversation(c.Context(), ticketID, userID, false)
	if err != nil { return err }

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    messages,
	})
}

func (h *TicketHandler) UserReply(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok { return utils.Unauthorized("Auth required") }

	ticketID, err := uuid.Parse(c.Params("id"))
	if err != nil { return utils.BadRequest("Invalid ticket ID") }

	var req dto.AddMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid body")
	}

	// Handle Attachments
	localPaths, err := h.saveAttachments(c)
	if err != nil { return err }

	if err := h.svc.AddUserReply(c.Context(), userID, ticketID, req, localPaths); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Reply sent",
	})
}

// ───────────────── ADMIN ENDPOINTS ─────────────────

func (h *TicketHandler) ListAllTickets(c *fiber.Ctx) error {
	tickets, err := h.svc.GetAllTickets(c.Context())
	if err != nil { return err }

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    tickets,
	})
}

func (h *TicketHandler) AdminGetConversation(c *fiber.Ctx) error {
	adminID, ok := c.Locals("admin_id").(uuid.UUID)
	if !ok { return utils.Unauthorized("Admin auth required") }

	ticketID, err := uuid.Parse(c.Params("id"))
	if err != nil { return utils.BadRequest("Invalid ticket ID") }

	messages, err := h.svc.GetConversation(c.Context(), ticketID, adminID, true)
	if err != nil { return err }

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    messages,
	})
}

func (h *TicketHandler) AdminReply(c *fiber.Ctx) error {
	adminID, ok := c.Locals("admin_id").(uuid.UUID)
	if !ok { return utils.Unauthorized("Admin auth required") }

	ticketID, err := uuid.Parse(c.Params("id"))
	if err != nil { return utils.BadRequest("Invalid ticket ID") }

	var req dto.AddMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid body")
	}

	// Handle Attachments
	localPaths, err := h.saveAttachments(c)
	if err != nil { return err }

	if err := h.svc.AddAdminReply(c.Context(), adminID, ticketID, req, localPaths); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Admin reply sent",
	})
}

func (h *TicketHandler) UserUpdateStatus(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok { return utils.Unauthorized("Auth required") }

	ticketID, err := uuid.Parse(c.Params("id"))
	if err != nil { return utils.BadRequest("Invalid ticket ID") }

	var req dto.UpdateTicketStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid body")
	}

	// 1. Fetch ticket to check ownership
	messages, err := h.svc.GetConversation(c.Context(), ticketID, userID, false)
	if err != nil {
		return err // This will return Forbidden if user doesn't own it
	}
	// If messages returned, it means user has access (owns the ticket)
	if len(messages) == 0 {
		// Even if no messages, service check in GetConversation handles ownership
	}

	if err := h.svc.UpdateStatus(c.Context(), ticketID, req.Status); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Ticket status updated",
	})
}

func (h *TicketHandler) AdminUpdateStatus(c *fiber.Ctx) error {
	ticketID, err := uuid.Parse(c.Params("id"))
	if err != nil { return utils.BadRequest("Invalid ticket ID") }

	var req dto.UpdateTicketStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid body")
	}

	if err := h.svc.UpdateStatus(c.Context(), ticketID, req.Status); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Ticket status updated to " + string(req.Status),
	})
}

// ───────────────── PRIVATE HELPERS ─────────────────

func (h *TicketHandler) saveAttachments(c *fiber.Ctx) ([]string, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, nil // No attachments provided
	}

	files := form.File["attachments"]
	var paths []string

	for _, file := range files {
		// Validate size (e.g. 5MB)
		if file.Size > 5*1024*1024 {
			return nil, utils.BadRequest(fmt.Sprintf("File %s is too large (Max 5MB)", file.Filename))
		}

		ext := strings.ToLower(filepath.Ext(file.Filename))
		validExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
		if !validExts[ext] {
			return nil, utils.BadRequest(fmt.Sprintf("File %s has an invalid type. Only images (JPG, PNG, WEBP) are allowed.", file.Filename))
		}

		tempName := fmt.Sprintf("%s%s", uuid.New().String(), ext)
		localPath := filepath.Join(h.svc.Config().App.SupportTempPath(), tempName)

		if err := c.SaveFile(file, localPath); err != nil {
			utils.Error("Failed to save temp support attachment", err, nil)
			return nil, utils.Internal(err)
		}
		paths = append(paths, localPath)
	}

	return paths, nil
}

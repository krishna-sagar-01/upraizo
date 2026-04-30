package handler

import (
	"net/http"
	"server/internal/dto"
	service "server/internal/services/support"
	"server/internal/utils"

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
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request fields")
	}

	ticket, err := h.svc.OpenTicket(c.Context(), userID, req)
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

	if err := h.svc.AddUserReply(c.Context(), userID, ticketID, req); err != nil {
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

	if err := h.svc.AddAdminReply(c.Context(), adminID, ticketID, req); err != nil {
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

	// Ensure user owns the ticket before updating status
	if _, err := h.svc.GetTicket(c.Context(), ticketID, userID, false); err != nil {
		return err // Will return 403 Forbidden if not owned
	}

	var req dto.UpdateTicketStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid body")
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
		"message": "Ticket status updated",
	})
}

func (h *TicketHandler) AdminDeleteTicket(c *fiber.Ctx) error {
	ticketID, err := uuid.Parse(c.Params("id"))
	if err != nil { return utils.BadRequest("Invalid ticket ID") }

	if err := h.svc.DeleteTicket(c.Context(), ticketID); err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Ticket deleted successfully",
	})
}

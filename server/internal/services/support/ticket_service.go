package service

import (
	"context"
	"server/internal/dto"
	"server/internal/models"
	"server/internal/repository"
	"server/internal/utils"
	"server/internal/queue"
	"server/internal/config"

	"github.com/google/uuid"
)

type TicketService struct {
	repo      *repository.TicketRepository
	queueMgr  *queue.Manager
	cfg       *config.Config
}

func NewTicketService(
	repo *repository.TicketRepository,
	queueMgr *queue.Manager,
	cfg *config.Config,
) *TicketService {
	return &TicketService{
		repo:      repo,
		queueMgr:  queueMgr,
		cfg:       cfg,
	}
}

func (s *TicketService) Config() *config.Config {
	return s.cfg
}

// ───────────────── USER ACTIONS ─────────────────

func (s *TicketService) OpenTicket(ctx context.Context, userID uuid.UUID, req dto.CreateTicketRequest) (*models.Ticket, error) {
	ticket := &models.Ticket{
		ID:       uuid.New(),
		UserID:   userID,
		Subject:  req.Subject,
		Category: req.Category,
		Priority: req.Priority,
		Status:   models.TicketStatusOpen,
		Metadata: req.Metadata,
	}

	if ticket.Metadata == nil {
		ticket.Metadata = models.DefaultTicketMetadata()
	}

	if err := s.repo.Create(ctx, ticket); err != nil {
		return nil, utils.Internal(err)
	}

	// Add initial message
	msg := &models.TicketMessage{
		TicketID:    ticket.ID,
		UserID:      &userID,
		Message:     req.Message,
	}

	if err := s.repo.AddMessage(ctx, msg); err != nil {
		return nil, utils.Internal(err)
	}

	utils.Info("Support: New ticket opened", map[string]any{"ticket_id": ticket.ID, "user_id": userID})
	
	return ticket, nil
}

func (s *TicketService) AddUserReply(ctx context.Context, userID uuid.UUID, ticketID uuid.UUID, req dto.AddMessageRequest) error {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return utils.Internal(err)
	}
	if ticket == nil {
		return utils.NotFound("Ticket not found")
	}

	if ticket.UserID != userID {
		return utils.Forbidden("You do not have access to this ticket")
	}

	msg := &models.TicketMessage{
		TicketID: ticketID,
		UserID:   &userID,
		Message:  req.Message,
	}

	if err := s.repo.AddMessage(ctx, msg); err != nil {
		return utils.Internal(err)
	}

	// Always update ticket status to 'open' when a user replies (handles reopening)
	if err := s.repo.UpdateStatus(ctx, ticketID, models.TicketStatusOpen); err != nil {
		utils.Error("Support: Failed to update status on user reply", err, map[string]any{"ticket_id": ticketID})
	}

	utils.Info("Support: User replied to ticket", map[string]any{"ticket_id": ticketID})
	return nil
}

// ───────────────── ADMIN ACTIONS ─────────────────

func (s *TicketService) AddAdminReply(ctx context.Context, adminID uuid.UUID, ticketID uuid.UUID, req dto.AddMessageRequest) error {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return utils.Internal(err)
	}
	if ticket == nil {
		return utils.NotFound("Ticket not found")
	}

	msg := &models.TicketMessage{
		TicketID: ticketID,
		AdminID:  &adminID,
		Message:  req.Message,
	}

	if err := s.repo.AddMessage(ctx, msg); err != nil {
		return utils.Internal(err)
	}

	// Update ticket status to 'in_progress' when an admin replies
	if ticket.Status == models.TicketStatusOpen {
		if err := s.repo.UpdateStatus(ctx, ticketID, models.TicketStatusInProgress); err != nil {
			utils.Error("Support: Failed to update status on admin reply", err, map[string]any{"ticket_id": ticketID})
		}
	}

	return nil
}

func (s *TicketService) UpdateStatus(ctx context.Context, ticketID uuid.UUID, status models.TicketStatus) error {
	if !status.IsValid() {
		return utils.BadRequest("Invalid ticket status")
	}
	return s.repo.UpdateStatus(ctx, ticketID, status)
}

func (s *TicketService) DeleteTicket(ctx context.Context, ticketID uuid.UUID) error {
	return s.repo.Delete(ctx, ticketID)
}

// ───────────────── READ ACTIONS ─────────────────

func (s *TicketService) GetUserTickets(ctx context.Context, userID uuid.UUID) ([]*models.Ticket, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *TicketService) GetTicket(ctx context.Context, ticketID uuid.UUID, accessorID uuid.UUID, isAdmin bool) (*models.Ticket, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, utils.Internal(err)
	}
	if ticket == nil {
		return nil, utils.NotFound("Ticket not found")
	}

	if !isAdmin && ticket.UserID != accessorID {
		return nil, utils.Forbidden("Access denied")
	}

	return ticket, nil
}

func (s *TicketService) GetAllTickets(ctx context.Context) ([]*models.Ticket, error) {
	return s.repo.GetAll(ctx)
}

func (s *TicketService) GetConversation(ctx context.Context, ticketID uuid.UUID, accessorID uuid.UUID, isAdmin bool) ([]*models.TicketMessage, error) {
	ticket, err := s.repo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, utils.Internal(err)
	}
	if ticket == nil {
		return nil, utils.NotFound("Ticket not found")
	}

	// Safety check: only admin or the ticket owner can see messages
	if !isAdmin && ticket.UserID != accessorID {
		return nil, utils.Forbidden("Access denied")
	}

	return s.repo.GetConversation(ctx, ticketID)
}

package repository

import (
	"context"
	"errors"
	"time"

	"server/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TicketRepository struct {
	db *pgxpool.Pool
}

func NewTicketRepository(db *pgxpool.Pool) *TicketRepository {
	return &TicketRepository{db: db}
}

// ───────────────── Helpers ─────────────────

func (r *TicketRepository) withWriteContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func (r *TicketRepository) scanTicket(row pgx.Row) (*models.Ticket, error) {
	t := &models.Ticket{}
	err := row.Scan(
		&t.ID, &t.UserID, &t.Subject, &t.Category,
		&t.Priority, &t.Status, &t.Metadata,
		&t.CreatedAt, &t.UpdatedAt,
	)
	return t, err
}

func (r *TicketRepository) scanMessage(row pgx.Row) (*models.TicketMessage, error) {
	tm := &models.TicketMessage{}
	err := row.Scan(
		&tm.ID, &tm.TicketID, &tm.UserID, &tm.AdminID,
		&tm.Message, &tm.CreatedAt,
	)
	return tm, err
}

// ───────────────── TICKET OPERATIONS ─────────────────

// Create: Naya support ticket open karne ke liye
func (r *TicketRepository) Create(ctx context.Context, t *models.Ticket) error {
	writeCtx, cancel := r.withWriteContext()
	defer cancel()

	query := `
		INSERT INTO tickets (id, user_id, subject, category, priority, status, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	now := time.Now()
	t.CreatedAt, t.UpdatedAt = now, now
	if t.Status == "" {
		t.Status = models.TicketStatusOpen
	}

	_, err := r.db.Exec(writeCtx, query,
		t.ID, t.UserID, t.Subject, t.Category, t.Priority, t.Status, t.Metadata, t.CreatedAt, t.UpdatedAt,
	)
	return err
}

// GetByUserID: User ke saare tickets list karne ke liye
func (r *TicketRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Ticket, error) {
	query := `SELECT id, user_id, subject, category, priority, status, metadata, created_at, updated_at 
	          FROM tickets WHERE user_id = $1 ORDER BY updated_at DESC`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []*models.Ticket
	for rows.Next() {
		t, err := r.scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, nil
}

// UpdateStatus: Status badalne ke liye (Open -> In Progress -> Resolved)
func (r *TicketRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.TicketStatus) error {
	query := `UPDATE tickets SET status = $2, updated_at = $3 WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id, status, time.Now())
	return err
}

// GetByID: Specific ticket details
func (r *TicketRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Ticket, error) {
	query := `SELECT id, user_id, subject, category, priority, status, metadata, created_at, updated_at 
	          FROM tickets WHERE id = $1`
	t, err := r.scanTicket(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

// Delete: Ticket aur uski conversation permanently delete karne ke liye
func (r *TicketRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM tickets WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

// GetAll: Admin list view (All tickets)
func (r *TicketRepository) GetAll(ctx context.Context) ([]*models.Ticket, error) {
	query := `SELECT id, user_id, subject, category, priority, status, metadata, created_at, updated_at 
	          FROM tickets ORDER BY updated_at DESC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []*models.Ticket
	for rows.Next() {
		t, err := r.scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, nil
}

// ───────────────── MESSAGE OPERATIONS ─────────────────

// AddMessage: Ticket mein reply add karne ke liye (User ya Admin dono ke liye)
func (r *TicketRepository) AddMessage(ctx context.Context, tm *models.TicketMessage) error {
	writeCtx, cancel := r.withWriteContext()
	defer cancel()

	// XOR validation helper check (Double safety before DB)
	if !tm.HasValidSender() {
		return errors.New("message must have exactly one sender: user_id or admin_id")
	}

	query := `
		INSERT INTO ticket_messages (id, ticket_id, user_id, admin_id, message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	if tm.ID == uuid.Nil {
		tm.ID = uuid.New()
	}
	tm.CreatedAt = time.Now()

	_, err := r.db.Exec(writeCtx, query,
		tm.ID, tm.TicketID, tm.UserID, tm.AdminID, tm.Message, tm.CreatedAt,
	)
	return err
}

// GetConversation: Pura chat thread nikalne ke liye
func (r *TicketRepository) GetConversation(ctx context.Context, ticketID uuid.UUID) ([]*models.TicketMessage, error) {
	query := `SELECT id, ticket_id, user_id, admin_id, message, created_at 
	          FROM ticket_messages WHERE ticket_id = $1 ORDER BY created_at ASC`

	rows, err := r.db.Query(ctx, query, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.TicketMessage
	for rows.Next() {
		msg, err := r.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

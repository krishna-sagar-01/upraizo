package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"server/internal/models"
	"server/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type PurchaseRepository struct {
	db *pgxpool.Pool
}

func NewPurchaseRepository(db *pgxpool.Pool) *PurchaseRepository {
	return &PurchaseRepository{db: db}
}

// ───────────────── Helpers ─────────────────

func (r *PurchaseRepository) withWriteContext() (context.Context, context.CancelFunc) {
	// Financial transactions ke liye thoda extra time safe hai (7s)
	return context.WithTimeout(context.Background(), 7*time.Second)
}

func (r *PurchaseRepository) scanPurchase(row pgx.Row) (*models.Purchase, error) {
	p := &models.Purchase{}
	err := row.Scan(
		&p.ID, &p.UserID, &p.CourseID, &p.EbookID,
		&p.RazorpayOrderID, &p.RazorpayPaymentID, &p.RazorpaySignature,
		&p.AmountPaid, &p.Currency, &p.Metadata,
		&p.Status, &p.ValidFrom, &p.ValidUntil,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *PurchaseRepository) performUpdate(query, opName string, purchaseID uuid.UUID, args ...any) error {
	ctx, cancel := r.withWriteContext()
	defer cancel()

	result, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		utils.Error("Purchase DB error during "+opName, err, map[string]any{"purchase_id": purchaseID})
		return fmt.Errorf("%s failed: %w", opName, err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("purchase record not found")
	}

	return nil
}

// ───────────────── CREATE (Initiate Order) ─────────────────

func (r *PurchaseRepository) Create(ctx context.Context, p *models.Purchase) error {
	writeCtx, cancel := r.withWriteContext()
	defer cancel()

	query := `
		INSERT INTO purchases (
			id, user_id, course_id, ebook_id, razorpay_order_id, amount_paid, 
			currency, metadata, status, valid_from, valid_until, 
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	if p.ID == uuid.Nil { p.ID = uuid.New() }
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.Status == "" { p.Status = models.PurchaseStatusPending }

	_, err := r.db.Exec(writeCtx, query,
		p.ID, p.UserID, p.CourseID, p.EbookID, p.RazorpayOrderID, p.AmountPaid,
		p.Currency, p.Metadata, p.Status, p.ValidFrom, p.ValidUntil,
		p.CreatedAt, p.UpdatedAt,
	)

	if err != nil {
		utils.Error("Failed to initiate purchase", err, map[string]any{"order_id": p.RazorpayOrderID})
		return err
	}
	return nil
}

// ───────────────── READ ─────────────────

func (r *PurchaseRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Purchase, error) {
	query := `SELECT id, user_id, course_id, ebook_id, razorpay_order_id, razorpay_payment_id, razorpay_signature, 
	                 amount_paid, currency, metadata, status, valid_from, valid_until, created_at, updated_at 
	          FROM purchases WHERE id = $1`
	p, err := r.scanPurchase(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// GetByOrderID: Razorpay webhooks ke liye sabse zyada use hone wali query
func (r *PurchaseRepository) GetByOrderID(ctx context.Context, orderID string) (*models.Purchase, error) {
	query := `SELECT id, user_id, course_id, ebook_id, razorpay_order_id, razorpay_payment_id, razorpay_signature, 
	                 amount_paid, currency, metadata, status, valid_from, valid_until, created_at, updated_at 
	          FROM purchases WHERE razorpay_order_id = $1`
	
	p, err := r.scanPurchase(r.db.QueryRow(ctx, query, orderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// GetActivePurchase: Check karne ke liye ki user ke paas already access hai ya nahi
func (r *PurchaseRepository) GetActivePurchase(ctx context.Context, userID, courseID uuid.UUID) (*models.Purchase, error) {
	query := `SELECT id, user_id, course_id, ebook_id, razorpay_order_id, razorpay_payment_id, razorpay_signature, 
	                 amount_paid, currency, metadata, status, valid_from, valid_until, created_at, updated_at 
	          FROM purchases 
	          WHERE user_id = $1 AND course_id = $2 AND status = 'completed' 
	          AND valid_until > $3 ORDER BY created_at DESC LIMIT 1`

	p, err := r.scanPurchase(r.db.QueryRow(ctx, query, userID, courseID, time.Now()))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func (r *PurchaseRepository) GetActiveEbookPurchase(ctx context.Context, userID, ebookID uuid.UUID) (*models.Purchase, error) {
	query := `SELECT id, user_id, course_id, ebook_id, razorpay_order_id, razorpay_payment_id, razorpay_signature, 
	                 amount_paid, currency, metadata, status, valid_from, valid_until, created_at, updated_at 
	          FROM purchases 
	          WHERE user_id = $1 AND ebook_id = $2 AND status = 'completed' 
	          ORDER BY created_at DESC LIMIT 1`

	p, err := r.scanPurchase(r.db.QueryRow(ctx, query, userID, ebookID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// ListByUserID: Fetch all purchases for a specific user with pagination
func (r *PurchaseRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Purchase, error) {
	query := `SELECT id, user_id, course_id, ebook_id, razorpay_order_id, razorpay_payment_id, razorpay_signature, 
	                 amount_paid, currency, metadata, status, valid_from, valid_until, created_at, updated_at 
	          FROM purchases 
	          WHERE user_id = $1 
	          ORDER BY created_at DESC 
	          LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var purchases []*models.Purchase
	for rows.Next() {
		p, err := r.scanPurchase(rows)
		if err != nil {
			return nil, err
		}
		purchases = append(purchases, p)
	}

	return purchases, nil
}

// ───────────────── STATUS UPDATES ─────────────────

func (r *PurchaseRepository) ActivateAccess(ctx context.Context, id uuid.UUID, validityDays int) error {
	from := time.Now()
	// Using explicit casting to ensure pgx deduces the correct types
	query := `
		UPDATE purchases SET 
			valid_from = $2::timestamp, 
			valid_until = CASE 
				WHEN course_id IS NOT NULL THEN ($2::timestamp + ($3::integer || ' days')::INTERVAL)
				ELSE NULL 
			END, 
			updated_at = $2::timestamp 
		WHERE id = $1`

	_, err := r.db.Exec(ctx, query, id, from, validityDays)
	return err
}

// CompletePurchase: Jab payment success ho jaye (Webhook/Callback)
func (r *PurchaseRepository) CompletePurchase(ctx context.Context, id uuid.UUID, paymentID, signature string) error {
	query := `
		UPDATE purchases SET 
			status = 'completed', 
			razorpay_payment_id = $2, 
			razorpay_signature = $3, 
			updated_at = $4 
		WHERE id = $1 AND status != 'completed'`
	
	return r.performUpdate(query, "complete purchase", id, id, paymentID, signature, time.Now())
}


// UpdateStatus: Failed ya Refunded mark karne ke liye
func (r *PurchaseRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.PurchaseStatus, metadata models.PurchaseMetadata) error {
	query := `UPDATE purchases SET status = $2, metadata = $3, updated_at = $4 WHERE id = $1`
	return r.performUpdate(query, "status update", id, id, status, metadata, time.Now())
}

// ───────────────── ADMIN QUERIES ─────────────────

// ListAllWithDetails fetches purchases joined with user and course info
func (r *PurchaseRepository) ListAllWithDetails(ctx context.Context, limit, offset int) ([]map[string]any, error) {
	query := `
		SELECT 
			p.id, p.user_id, u.name as user_name, u.email as user_email, 
			p.course_id, c.title as course_title,
			p.ebook_id, e.title as ebook_title,
			p.razorpay_order_id, p.razorpay_payment_id, p.amount_paid, 
			p.currency, p.status, p.valid_until, p.created_at
		FROM purchases p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN courses c ON p.course_id = c.id
		LEFT JOIN ebooks e ON p.ebook_id = e.id
		ORDER BY p.created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id, userID uuid.UUID
		var courseID, ebookID *uuid.UUID
		var userName, userEmail, razorpayOrderID, currency, status string
		var courseTitle, ebookTitle, razorpayPaymentID *string
		var amountPaid decimal.Decimal 
		var validUntil *time.Time
		var createdAt time.Time

		err := rows.Scan(
			&id, &userID, &userName, &userEmail,
			&courseID, &courseTitle,
			&ebookID, &ebookTitle,
			&razorpayOrderID, &razorpayPaymentID, &amountPaid,
			&currency, &status, &validUntil, &createdAt,
		)
		if err != nil {
			return nil, err
		}

		res := map[string]any{
			"id":                  id,
			"user_id":             userID,
			"user_name":           userName,
			"user_email":          userEmail,
			"razorpay_order_id":   razorpayOrderID,
			"razorpay_payment_id": razorpayPaymentID,
			"amount_paid":         amountPaid,
			"currency":            currency,
			"status":              status,
			"valid_until":         validUntil,
			"created_at":          createdAt,
		}

		if courseID != nil {
			res["course_id"] = *courseID
			if courseTitle != nil {
				res["course_title"] = *courseTitle
			}
		}

		if ebookID != nil {
			res["ebook_id"] = *ebookID
			if ebookTitle != nil {
				res["ebook_title"] = *ebookTitle
			}
		}

		results = append(results, res)
	}

	return results, nil
}

// GetSalesStats calculates overall performance metrics
func (r *PurchaseRepository) GetSalesStats(ctx context.Context) (map[string]any, error) {
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN status = 'completed' THEN amount_paid ELSE 0.0 END), 0.0) as total_revenue,
			COUNT(*) as total_sales,
			COUNT(*) FILTER (WHERE status = 'completed') as successful_sales,
			COUNT(*) FILTER (WHERE status = 'pending') as pending_sales,
			COUNT(*) FILTER (WHERE status = 'failed') as failed_sales
		FROM purchases`

	var totalRevenue decimal.Decimal
	var totalSales, successfulSales, pendingSales, failedSales int64

	err := r.db.QueryRow(ctx, query).Scan(
		&totalRevenue, &totalSales, &successfulSales, &pendingSales, &failedSales,
	)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"total_revenue":    totalRevenue,
		"total_sales":      totalSales,
		"successful_sales": successfulSales,
		"pending_sales":    pendingSales,
		"failed_sales":     failedSales,
	}, nil
}

// GetTotalInvestment calculates total successful spend by a user
func (r *PurchaseRepository) GetTotalInvestment(ctx context.Context, userID uuid.UUID) (decimal.Decimal, error) {
	query := `SELECT COALESCE(SUM(amount_paid), 0.0) FROM purchases WHERE user_id = $1 AND status = 'completed'`
	var total decimal.Decimal
	err := r.db.QueryRow(ctx, query, userID).Scan(&total)
	if err != nil {
		return decimal.Zero, err
	}
	return total, nil
}

type EnrollmentRecord struct {
	CourseID   *uuid.UUID
	EbookID    *uuid.UUID
	ValidUntil *time.Time
}

// GetActiveEnrollments returns both courses and ebooks the user has bought
func (r *PurchaseRepository) GetActiveEnrollments(ctx context.Context, userID uuid.UUID) ([]EnrollmentRecord, error) {
	query := `
		SELECT course_id, ebook_id, valid_until 
		FROM purchases 
		WHERE user_id = $1 AND status = 'completed'
		ORDER BY created_at DESC`
	
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []EnrollmentRecord
	for rows.Next() {
		var rec EnrollmentRecord
		if err := rows.Scan(&rec.CourseID, &rec.EbookID, &rec.ValidUntil); err == nil {
			records = append(records, rec)
		}
	}
	return records, nil
}
func (r *PurchaseRepository) HasPurchasedCourse(ctx context.Context, userID, courseID uuid.UUID) (bool, error) {
	// A course is accessible if:
	// 1. Status is 'completed'
	// 2. AND (valid_until is NULL OR valid_until > current_time)
	query := `
		SELECT EXISTS(
			SELECT 1 FROM purchases 
			WHERE user_id = $1 
			AND course_id = $2 
			AND status = 'completed' 
			AND (valid_until IS NULL OR valid_until > $3)
		)`
	
	var exists bool
	// Using time.Now() to check against valid_until
	err := r.db.QueryRow(ctx, query, userID, courseID, time.Now()).Scan(&exists)
	if err != nil {
		utils.Error("Error checking course purchase access", err, map[string]any{"user_id": userID, "course_id": courseID})
		return false, err
	}
	return exists, nil
}

// HasPurchasedEbook: Simplified check for e-book access
func (r *PurchaseRepository) HasPurchasedEbook(ctx context.Context, userID, ebookID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id = $1 AND ebook_id = $2 AND status = 'completed')`
	var exists bool
	err := r.db.QueryRow(ctx, query, userID, ebookID).Scan(&exists)
	return exists, err
}

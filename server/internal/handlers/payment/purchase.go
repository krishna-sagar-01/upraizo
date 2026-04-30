package payment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"server/internal/config"
	"server/internal/dto"
	"server/internal/models"
	"server/internal/queue"
	"server/internal/repository"
	paymentservice "server/internal/services/payment"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"time"
)

type PurchaseHandler struct {
	razorpayService *paymentservice.RazorpayService
	purchaseRepo    *repository.PurchaseRepository
	courseRepo      *repository.CourseRepository
	ebookRepo       *repository.EbookRepository
	queueManager    *queue.Manager
	cfg             *config.Config
}

func NewPurchaseHandler(
	razorpayService *paymentservice.RazorpayService,
	purchaseRepo *repository.PurchaseRepository,
	courseRepo *repository.CourseRepository,
	ebookRepo *repository.EbookRepository,
	queueManager *queue.Manager,
	cfg *config.Config,
) *PurchaseHandler {
	return &PurchaseHandler{
		razorpayService: razorpayService,
		purchaseRepo:    purchaseRepo,
		courseRepo:      courseRepo,
		ebookRepo:       ebookRepo,
		queueManager:    queueManager,
		cfg:             cfg,
	}
}

// ─── POST /payment/orders ─────────────────────────────────────────────────────

func (h *PurchaseHandler) CreateOrder(c *fiber.Ctx) error {
	var req dto.CreateOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if appErr := utils.ValidateStruct(&req); appErr != nil {
		return appErr
	}

	userID := c.Locals("user_id").(uuid.UUID)
	var finalPrice decimal.Decimal
	var currency = "INR"
	var itemName string
	var itemID string
	var validityDays int
	var courseID *uuid.UUID
	var ebookID *uuid.UUID
	var razorpayItemID *string

	if req.CourseID != "" {
		cid, err := uuid.Parse(req.CourseID)
		if err != nil {
			return utils.BadRequest("Invalid course ID")
		}
		course, err := h.courseRepo.GetByID(c.Context(), cid)
		if err != nil || course == nil {
			return utils.NotFound("Course not found")
		}
		
		// Check for active purchase
		active, _ := h.purchaseRepo.GetActivePurchase(c.Context(), userID, cid)
		if active != nil {
			return utils.BadRequest("This course is already purchased")
		}

		finalPrice = course.Price
		itemName = course.Title
		itemID = cid.String()
		validityDays = course.ValidityDays
		courseID = &cid
		razorpayItemID = course.RazorpayItemID
	} else if req.EbookID != "" {
		eid, err := uuid.Parse(req.EbookID)
		if err != nil {
			return utils.BadRequest("Invalid ebook ID")
		}
		ebook, err := h.ebookRepo.GetByID(c.Context(), eid)
		if err != nil || ebook == nil {
			return utils.NotFound("Ebook not found")
		}

		// Check for active purchase
		active, _ := h.purchaseRepo.GetActiveEbookPurchase(c.Context(), userID, eid)
		if active != nil {
			return utils.BadRequest("This e-book is already purchased")
		}

		finalPrice = ebook.Price
		itemName = ebook.Title
		itemID = eid.String()
		validityDays = 36500 // 100 years
		ebookID = &eid
	} else {
		return utils.BadRequest("Either course_id or ebook_id is required")
	}

	if finalPrice.IsZero() {
		return utils.BadRequest("This item is free, use the enrollment logic")
	}

	receiptID := fmt.Sprintf("rcpt_u%s_i%s", userID.String()[:8], itemID[:8])
	
	notes := map[string]any{
		"user_id":   userID.String(),
		"item_name": itemName,
	}
	if courseID != nil {
		notes["course_id"] = courseID.String()
	}
	if ebookID != nil {
		notes["ebook_id"] = ebookID.String()
	}
	if razorpayItemID != nil {
		notes["razorpay_item_id"] = *razorpayItemID
	}

	razorpayOrderID, err := h.razorpayService.CreateOrder(finalPrice, currency, receiptID, notes)
	if err != nil {
		return utils.Internal(err)
	}

	// Calculate initial access window (Required for DB constraints even if pending)
	expiry := time.Now().AddDate(0, 0, validityDays)
	validUntil := &expiry

	purchase := &models.Purchase{
		UserID:          userID,
		CourseID:        courseID,
		EbookID:         ebookID,
		RazorpayOrderID: razorpayOrderID,
		AmountPaid:      finalPrice,
		Currency:        currency,
		Metadata:        models.DefaultPurchaseMetadata(),
		Status:          models.PurchaseStatusPending,
		ValidFrom:       time.Now(),
		ValidUntil:      validUntil,
	}

	if err := h.purchaseRepo.Create(c.Context(), purchase); err != nil {
		return utils.Internal(err)
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": dto.CreateOrderResponse{
			OrderID:  razorpayOrderID,
			Amount:   finalPrice,
			Currency: currency,
			CourseID: req.CourseID,
			EbookID:  req.EbookID,
		},
	})
}

// ─── POST /payment/webhook ────────────────────────────────────────────────────

type RazorpayWebhookPayload struct {
	Event   string `json:"event"`
	Payload struct {
		Payment struct {
			Entity struct {
				ID      string `json:"id"`
				OrderID string `json:"order_id"`
			} `json:"entity"`
		} `json:"payment"`
	} `json:"payload"`
}

func (h *PurchaseHandler) Webhook(c *fiber.Ctx) error {
	utils.Info("Razorpay Webhook received", map[string]any{"method": c.Method(), "path": c.Path()})
	signature := c.Get("X-Razorpay-Signature")
	if signature == "" {
		utils.Warn("Missing Razorpay signature in webhook", nil)
		return c.Status(http.StatusUnauthorized).SendString("Missing signature")
	}

	body := c.Body()

	// 1. Verify Signature Securely
	if !h.razorpayService.VerifyWebhookSignature(body, signature) {
		utils.Warn("Invalid razorpay webhook signature received", map[string]any{"ip": c.IP()})
		return c.Status(http.StatusUnauthorized).SendString("Invalid signature")
	}

	// 2. Parse Event
	var payload RazorpayWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		utils.Error("Failed to parse Razorpay webhook payload", err, nil)
		return c.Status(http.StatusBadRequest).SendString("Invalid json")
	}

	utils.Info("Razorpay Webhook event parsed", map[string]any{"event": payload.Event})

	// We only care about successful payments for completing a purchase for now
	if payload.Event != "payment.captured" && payload.Event != "order.paid" {
		// Log and acknowledge other events cleanly without action
		return c.SendStatus(http.StatusOK)
	}

	orderID := payload.Payload.Payment.Entity.OrderID
	paymentID := payload.Payload.Payment.Entity.ID

	// 1. Trace ID extraction for correlation
	traceID, _ := c.Locals("requestid").(string)

	// 2. Fetch Purchase from DB using razorpay_order_id
	purchase, err := h.purchaseRepo.GetByOrderID(c.Context(), orderID)
	if err != nil {
		utils.Error("Webhook error: fetching purchase failed", err, map[string]any{"order_id": orderID, "trace_id": traceID})
		return c.Status(http.StatusInternalServerError).SendString("Database error")
	}
	if purchase == nil {
		utils.Warn("Webhook received for unknown order", map[string]any{"order_id": orderID, "trace_id": traceID})
		return c.SendStatus(http.StatusOK)
	}

	if purchase.IsCompleted() {
		return c.SendStatus(http.StatusOK)
	}

	// 2.5 Double-check if user already has an active purchase for this item 
	// to avoid unique constraint violations if multiple orders were initiated
	if purchase.CourseID != nil {
		active, _ := h.purchaseRepo.GetActivePurchase(c.Context(), purchase.UserID, *purchase.CourseID)
		if active != nil {
			utils.Info("Webhook: User already has active access, skipping duplicate completion", map[string]any{"user_id": purchase.UserID, "course_id": purchase.CourseID})
			return c.SendStatus(http.StatusOK)
		}
	} else if purchase.EbookID != nil {
		active, _ := h.purchaseRepo.GetActiveEbookPurchase(c.Context(), purchase.UserID, *purchase.EbookID)
		if active != nil {
			utils.Info("Webhook: User already has active ebook access, skipping duplicate completion", map[string]any{"user_id": purchase.UserID, "ebook_id": purchase.EbookID})
			return c.SendStatus(http.StatusOK)
		}
	}


	// 3. Update the DB
	err = h.purchaseRepo.CompletePurchase(c.Context(), purchase.ID, paymentID, signature)
	if err != nil {
		// If it still fails due to a race condition or constraint, log and acknowledge
		utils.Warn("Failed to complete purchase in DB (possible duplicate)", map[string]any{
			"purchase_id": purchase.ID.String(),
			"error":       err.Error(),
		})
		return c.SendStatus(http.StatusOK) // Acknowledge to Razorpay anyway
	}

	// 4. Publish to RabbitMQ directly
	task := queue.PurchaseTask{
		PurchaseID: purchase.ID.String(),
		UserID:     purchase.UserID.String(),
		TraceID:    traceID,
	}
	if purchase.CourseID != nil {
		task.CourseID = purchase.CourseID.String()
	}
	if purchase.EbookID != nil {
		task.EbookID = purchase.EbookID.String()
	}

	if err := h.queueManager.Publish(h.cfg.RabbitMQ.PurchaseQueue, task); err != nil {
		utils.Error("Failed to publish purchase task to queue", err, map[string]any{
			"purchase_id": purchase.ID.String(),
			"trace_id":    traceID,
		})
	}

	return c.SendStatus(http.StatusOK)
}

// ─── GET /payment/my ──────────────────────────────────────────────────────────

func (h *PurchaseHandler) GetMyPayments(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uuid.UUID)
	limit := c.QueryInt("limit", 10)
	offset := c.QueryInt("offset", 0)

	purchases, err := h.purchaseRepo.ListByUserID(c.Context(), userID, limit, offset)
	if err != nil {
		return utils.Internal(err)
	}

	response := []dto.PurchaseResponse{}
	for _, p := range purchases {
		res := dto.PurchaseResponse{
			ID:                p.ID.String(),
			RazorpayOrderID:   p.RazorpayOrderID,
			RazorpayPaymentID: p.RazorpayPaymentID,
			AmountPaid:        p.AmountPaid,
			Currency:          p.Currency,
			Status:            string(p.Status),
			CreatedAt:         p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		if p.CourseID != nil {
			res.CourseID = p.CourseID.String()
			course, err := h.courseRepo.GetByID(c.Context(), *p.CourseID)
			if err == nil && course != nil {
				res.CourseTitle = course.Title
				if course.ThumbnailURL != nil {
					res.CourseThumbnail = *course.ThumbnailURL
				}
			}
		}

		if p.EbookID != nil {
			res.EbookID = p.EbookID.String()
			ebook, err := h.ebookRepo.GetByID(c.Context(), *p.EbookID)
			if err == nil && ebook != nil {
				res.EbookTitle = ebook.Title
				if ebook.ThumbnailURL != nil {
					res.EbookThumbnail = *ebook.ThumbnailURL
				}
			}
		}

		response = append(response, res)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
		"meta": fiber.Map{
			"limit":  limit,
			"offset": offset,
			"count":  len(response),
		},
	})
}

// ─── GET /payment/:id ─────────────────────────────────────────────────────────

func (h *PurchaseHandler) GetPayment(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uuid.UUID)
	purchaseID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest("Invalid purchase ID")
	}

	purchase, err := h.purchaseRepo.GetByID(c.Context(), purchaseID)
	if err != nil {
		return utils.NotFound("Payment record not found")
	}

	// Security check: ensure the payment belongs to the user
	if purchase.UserID != userID {
		return utils.Forbidden("Access denied")
	}

	res := dto.PurchaseResponse{
		ID:                purchase.ID.String(),
		RazorpayOrderID:   purchase.RazorpayOrderID,
		RazorpayPaymentID: purchase.RazorpayPaymentID,
		AmountPaid:        purchase.AmountPaid,
		Currency:          purchase.Currency,
		Status:            string(purchase.Status),
		CreatedAt:         purchase.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if purchase.CourseID != nil {
		res.CourseID = purchase.CourseID.String()
		course, err := h.courseRepo.GetByID(c.Context(), *purchase.CourseID)
		if err == nil && course != nil {
			res.CourseTitle = course.Title
			if course.ThumbnailURL != nil {
				res.CourseThumbnail = *course.ThumbnailURL
			}
		}
	}

	if purchase.EbookID != nil {
		res.EbookID = purchase.EbookID.String()
		ebook, err := h.ebookRepo.GetByID(c.Context(), *purchase.EbookID)
		if err == nil && ebook != nil {
			res.EbookTitle = ebook.Title
			if ebook.ThumbnailURL != nil {
				res.EbookThumbnail = *ebook.ThumbnailURL
			}
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    res,
	})
}

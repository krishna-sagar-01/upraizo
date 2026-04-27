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
	"time"
)

type PurchaseHandler struct {
	razorpayService *paymentservice.RazorpayService
	purchaseRepo    *repository.PurchaseRepository
	courseRepo      *repository.CourseRepository
	queueManager    *queue.Manager
	cfg             *config.Config
}

func NewPurchaseHandler(
	razorpayService *paymentservice.RazorpayService,
	purchaseRepo *repository.PurchaseRepository,
	courseRepo *repository.CourseRepository,
	queueManager *queue.Manager,
	cfg *config.Config,
) *PurchaseHandler {
	return &PurchaseHandler{
		razorpayService: razorpayService,
		purchaseRepo:    purchaseRepo,
		courseRepo:      courseRepo,
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

	courseID, err := uuid.Parse(req.CourseID)
	if err != nil {
		return utils.BadRequest("Invalid course ID")
	}

	userID := c.Locals("user_id").(uuid.UUID)

	// Fetch course from DB to get the price
	course, err := h.courseRepo.GetByID(c.Context(), courseID)
	if err != nil {
		return utils.NotFound("Course not found")
	}

	// Verify price logic (discount etc.)
	finalPrice := course.Price
	if finalPrice.IsZero() {
		return utils.BadRequest("This course is free, use the enroll endpoint")
	}

	// Metadata initialization
	metadata := models.DefaultPurchaseMetadata()

	// Active Purchase check
	activePurchase, err := h.purchaseRepo.GetActivePurchase(c.Context(), userID, courseID)
	if err == nil && activePurchase != nil {
		return utils.BadRequest("You already have an active subscription for this course")
	}

	// Standard order receipt ID with UserID for better auditability
	receiptID := fmt.Sprintf("rcpt_u%s_c%s", userID.String()[:8], courseID.String()[:8])
	currency := "INR" // or USD based on your config

	// 1. Prepare Notes (Metadata for Razorpay Dashboard)
	notes := map[string]any{
		"user_id":   userID.String(),
		"course_id": courseID.String(),
	}
	if course.RazorpayItemID != nil {
		notes["razorpay_item_id"] = *course.RazorpayItemID
	}

	// 2. Create order on Razorpay
	razorpayOrderID, err := h.razorpayService.CreateOrder(finalPrice, currency, receiptID, notes)
	if err != nil {
		return utils.Internal(err)
	}

	// 2. Prepare DB Record
	purchase := &models.Purchase{
		UserID:          userID,
		CourseID:        courseID,
		RazorpayOrderID: razorpayOrderID,
		AmountPaid:      finalPrice,
		Currency:        currency,
		Metadata:        metadata,
		Status:          models.PurchaseStatusPending,
		ValidFrom:       time.Now(),
		ValidUntil:      time.Now().AddDate(0, 0, course.ValidityDays),
	}

	// 3. Save to DB
	if err := h.purchaseRepo.Create(c.Context(), purchase); err != nil {
		return utils.Internal(err)
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": dto.CreateOrderResponse{
			OrderID:  razorpayOrderID,
			Amount:   finalPrice,
			Currency: currency,
			CourseID: course.ID.String(),
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


	// 3. Update the DB
	err = h.purchaseRepo.CompletePurchase(c.Context(), purchase.ID, paymentID, signature)
	if err != nil {
		utils.Error("Failed to complete purchase in DB", err, map[string]any{
			"purchase_id": purchase.ID.String(),
			"trace_id":    traceID,
		})
		return c.Status(http.StatusInternalServerError).SendString("Database update failed")
	}

	// 4. Publish to RabbitMQ directly
	task := queue.PurchaseTask{
		PurchaseID: purchase.ID.String(),
		UserID:     purchase.UserID.String(),
		CourseID:   purchase.CourseID.String(),
		TraceID:    traceID,
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

	var response []dto.PurchaseResponse
	for _, p := range purchases {
		res := dto.PurchaseResponse{
			ID:                p.ID.String(),
			CourseID:          p.CourseID.String(),
			RazorpayOrderID:   p.RazorpayOrderID,
			RazorpayPaymentID: p.RazorpayPaymentID,
			AmountPaid:        p.AmountPaid,
			Currency:          p.Currency,
			Status:            string(p.Status),
			CreatedAt:         p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		// Optionally fetch course title/thumbnail
		course, err := h.courseRepo.GetByID(c.Context(), p.CourseID)
		if err == nil && course != nil {
			res.CourseTitle = course.Title
			if course.ThumbnailURL != nil {
				res.CourseThumbnail = *course.ThumbnailURL
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
		CourseID:          purchase.CourseID.String(),
		RazorpayOrderID:   purchase.RazorpayOrderID,
		RazorpayPaymentID: purchase.RazorpayPaymentID,
		AmountPaid:        purchase.AmountPaid,
		Currency:          purchase.Currency,
		Status:            string(purchase.Status),
		CreatedAt:         purchase.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	course, err := h.courseRepo.GetByID(c.Context(), purchase.CourseID)
	if err == nil && course != nil {
		res.CourseTitle = course.Title
		if course.ThumbnailURL != nil {
			res.CourseThumbnail = *course.ThumbnailURL
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    res,
	})
}

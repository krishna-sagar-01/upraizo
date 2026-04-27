package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"server/internal/config"
	"server/internal/utils"

	"github.com/razorpay/razorpay-go"
	"github.com/shopspring/decimal"
)

type RazorpayService struct {
	client *razorpay.Client
	cfg    *config.Config
}

func NewRazorpayService(cfg *config.Config) *RazorpayService {
	client := razorpay.NewClient(cfg.Razorpay.KeyID, cfg.Razorpay.KeySecret)
	return &RazorpayService{
		client: client,
		cfg:    cfg,
	}
}

// CreateOrder calls Razorpay API to generate a new order ID
func (s *RazorpayService) CreateOrder(amount decimal.Decimal, currency string, receiptID string, notes map[string]any) (string, error) {
	// Razorpay expects amount in the smallest currency unit (e.g., paise for INR)
	amountInSmallestUnit := amount.Mul(decimal.NewFromInt(100)).IntPart()

	data := map[string]interface{}{
		"amount":          amountInSmallestUnit,
		"currency":        currency,
		"receipt":         receiptID,
		"payment_capture": 1, // Auto capture
		"notes":           notes,
	}

	body, err := s.client.Order.Create(data, nil)
	if err != nil {
		utils.Error("Razorpay order creation failed", err, map[string]any{"receipt": receiptID, "notes": notes})
		return "", fmt.Errorf("razorpay error: %w", err)
	}

	orderID, ok := body["id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid razorpay response format: missing or invalid id")
	}

	return orderID, nil
}

// VerifyWebhookSignature securely validates if a webhook genuinely originated from Razorpay
func (s *RazorpayService) VerifyWebhookSignature(body []byte, signature string) bool {
	secret := s.cfg.Razorpay.WebhookSecret

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

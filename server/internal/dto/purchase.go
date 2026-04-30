package dto

import "github.com/shopspring/decimal"

type CreateOrderRequest struct {
	CourseID string `json:"course_id" validate:"omitempty,uuid"`
	EbookID  string `json:"ebook_id" validate:"omitempty,uuid"`
}

type CreateOrderResponse struct {
	OrderID  string          `json:"order_id"`
	Amount   decimal.Decimal `json:"amount"`
	Currency string          `json:"currency"`
	CourseID string          `json:"course_id,omitempty"`
	EbookID  string          `json:"ebook_id,omitempty"`
}

// Any Razorpay webhook related DTOs if complex casting is required
// but webhooks are generally map[string]any when first unmarshaled

type PurchaseResponse struct {
	ID                string          `json:"id"`
	CourseID          string          `json:"course_id,omitempty"`
	CourseTitle       string          `json:"course_title,omitempty"`
	CourseThumbnail   string          `json:"course_thumbnail,omitempty"`
	EbookID           string          `json:"ebook_id,omitempty"`
	EbookTitle        string          `json:"ebook_title,omitempty"`
	EbookThumbnail    string          `json:"ebook_thumbnail,omitempty"`
	RazorpayOrderID   string          `json:"razorpay_order_id"`
	RazorpayPaymentID *string         `json:"razorpay_payment_id,omitempty"`
	AmountPaid        decimal.Decimal `json:"amount_paid"`
	Currency          string          `json:"currency"`
	Status            string          `json:"status"`
	ValidUntil        *string         `json:"valid_until,omitempty"`
	CreatedAt         string          `json:"created_at"`
}

type AdminPurchaseResponse struct {
	ID                string          `json:"id"`
	UserID            string          `json:"user_id"`
	UserName          string          `json:"user_name"`
	UserEmail         string          `json:"user_email"`
	CourseID          string          `json:"course_id,omitempty"`
	CourseTitle       string          `json:"course_title,omitempty"`
	EbookID           string          `json:"ebook_id,omitempty"`
	EbookTitle        string          `json:"ebook_title,omitempty"`
	RazorpayOrderID   string          `json:"razorpay_order_id"`
	RazorpayPaymentID *string         `json:"razorpay_payment_id,omitempty"`
	AmountPaid        decimal.Decimal `json:"amount_paid"`
	Currency          string          `json:"currency"`
	Status            string          `json:"status"`
	ValidUntil        *string         `json:"valid_until,omitempty"`
	CreatedAt         string          `json:"created_at"`
}

type AdminSalesStats struct {
	TotalRevenue    decimal.Decimal `json:"total_revenue"`
	TotalSales      int64           `json:"total_sales"`
	SuccessfulSales int64           `json:"successful_sales"`
	PendingSales    int64           `json:"pending_sales"`
	FailedSales     int64           `json:"failed_sales"`
}

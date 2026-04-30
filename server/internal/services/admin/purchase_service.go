package service

import (
	"context"
	"fmt"
	"server/internal/dto"
	"server/internal/repository"
	"server/internal/utils"
	"time"

	"github.com/shopspring/decimal"
)

type AdminPurchaseService struct {
	purchaseRepo *repository.PurchaseRepository
}

func NewAdminPurchaseService(purchaseRepo *repository.PurchaseRepository) *AdminPurchaseService {
	return &AdminPurchaseService{
		purchaseRepo: purchaseRepo,
	}
}

func (s *AdminPurchaseService) ListAllPayments(ctx context.Context, page, limit int) ([]dto.AdminPurchaseResponse, error) {
	offset := (page - 1) * limit
	results, err := s.purchaseRepo.ListAllWithDetails(ctx, limit, offset)
	if err != nil {
		utils.Error("Failed to list all payments for admin", err, nil)
		return nil, err
	}

	var response []dto.AdminPurchaseResponse
	for _, r := range results {
		var amountPaid decimal.Decimal
		
		// Handle different possible types for amountPaid from pgx
		switch v := r["amount_paid"].(type) {
		case decimal.Decimal:
			amountPaid = v
		case float64:
			amountPaid = decimal.NewFromFloat(v)
		case string:
			d, err := decimal.NewFromString(v)
			if err == nil {
				amountPaid = d
			}
		default:
			// Fallback or log error
		}

		resp := dto.AdminPurchaseResponse{
			ID:                r["id"].(fmt.Stringer).String(),
			UserID:            r["user_id"].(fmt.Stringer).String(),
			UserName:          r["user_name"].(string),
			UserEmail:         r["user_email"].(string),
			RazorpayOrderID:   r["razorpay_order_id"].(string),
			AmountPaid:        amountPaid,
			Currency:          r["currency"].(string),
			Status:            r["status"].(string),
			CreatedAt:         r["created_at"].(time.Time).Format(time.RFC3339),
		}

		if cID, ok := r["course_id"]; ok && cID != nil {
			resp.CourseID = cID.(fmt.Stringer).String()
			if cTitle, ok := r["course_title"]; ok && cTitle != nil {
				resp.CourseTitle = cTitle.(string)
			}
		}

		if eID, ok := r["ebook_id"]; ok && eID != nil {
			resp.EbookID = eID.(fmt.Stringer).String()
			if eTitle, ok := r["ebook_title"]; ok && eTitle != nil {
				resp.EbookTitle = eTitle.(string)
			}
		}
		
		if pID, ok := r["razorpay_payment_id"].(*string); ok {
			resp.RazorpayPaymentID = pID
		}

		response = append(response, resp)
	}

	return response, nil
}

func (s *AdminPurchaseService) GetSalesStats(ctx context.Context) (*dto.AdminSalesStats, error) {
	stats, err := s.purchaseRepo.GetSalesStats(ctx)
	if err != nil {
		utils.Error("Failed to get sales stats for admin", err, nil)
		return nil, err
	}

	var totalRevenue decimal.Decimal
	switch v := stats["total_revenue"].(type) {
	case decimal.Decimal:
		totalRevenue = v
	case float64:
		totalRevenue = decimal.NewFromFloat(v)
	case string:
		d, err := decimal.NewFromString(v)
		if err == nil {
			totalRevenue = d
		}
	}

	return &dto.AdminSalesStats{
		TotalRevenue:    totalRevenue,
		TotalSales:      stats["total_sales"].(int64),
		SuccessfulSales: stats["successful_sales"].(int64),
		PendingSales:    stats["pending_sales"].(int64),
		FailedSales:     stats["failed_sales"].(int64),
	}, nil
}

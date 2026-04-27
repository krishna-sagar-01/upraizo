package service

import (
	"context"
	"server/internal/dto"
	"server/internal/repository"
	"server/internal/utils"
	"github.com/shopspring/decimal"
)

type DashboardService struct {
	statsRepo    *repository.StatsRepository
	purchaseRepo *repository.PurchaseRepository
}

func NewDashboardService(statsRepo *repository.StatsRepository, purchaseRepo *repository.PurchaseRepository) *DashboardService {
	return &DashboardService{
		statsRepo:    statsRepo,
		purchaseRepo: purchaseRepo,
	}
}

func (s *DashboardService) GetSummary(ctx context.Context) (*dto.AdminDashboardSummary, error) {
	// 1. Get User Stats
	userStats, err := s.statsRepo.GetGlobalStats(ctx)
	if err != nil {
		utils.Error("Dashboard: Failed to get user stats", err, nil)
		return nil, err
	}

	// 2. Get Sales Stats
	salesRaw, err := s.purchaseRepo.GetSalesStats(ctx)
	if err != nil {
		utils.Error("Dashboard: Failed to get sales stats", err, nil)
		return nil, err
	}

	var totalRevenue decimal.Decimal
	switch v := salesRaw["total_revenue"].(type) {
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

	// 3. Get Course & Support Stats
	metrics, err := s.statsRepo.GetDashboardMetrics(ctx)
	if err != nil {
		utils.Error("Dashboard: Failed to get metrics", err, nil)
		return nil, err
	}

	return &dto.AdminDashboardSummary{
		Users: *userStats,
		Sales: dto.DashboardSalesStats{
			TotalRevenue:    totalRevenue,
			TotalSales:      salesRaw["total_sales"].(int64),
			SuccessfulSales: salesRaw["successful_sales"].(int64),
		},
		Courses: dto.DashboardCourseStats{
			TotalCourses:     metrics["total_courses"],
			PublishedCourses: metrics["published_courses"],
		},
		Support: dto.DashboardSupportStats{
			ActiveTickets: metrics["active_tickets"],
		},
	}, nil
}

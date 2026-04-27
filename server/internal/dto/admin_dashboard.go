package dto

import (
	"server/internal/models"
	"github.com/shopspring/decimal"
)

type AdminDashboardSummary struct {
	Users    models.PlatformStats `json:"users"`
	Sales    DashboardSalesStats  `json:"sales"`
	Courses  DashboardCourseStats `json:"courses"`
	Support  DashboardSupportStats `json:"support"`
}

type DashboardSalesStats struct {
	TotalRevenue    decimal.Decimal `json:"total_revenue"`
	TotalSales      int64           `json:"total_sales"`
	SuccessfulSales int64           `json:"successful_sales"`
}

type DashboardCourseStats struct {
	TotalCourses     int64 `json:"total_courses"`
	PublishedCourses int64 `json:"published_courses"`
}

type DashboardSupportStats struct {
	ActiveTickets int64 `json:"active_tickets"`
}

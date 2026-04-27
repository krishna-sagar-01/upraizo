package main

import (
	"strings"

	"server/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func SetupRoutes(app *fiber.App, deps *AppDependencies) {
	// ==========================================
	// GLOBAL MIDDLEWARES
	// ==========================================
	allowedOrigins := strings.Join(deps.Config.App.AllowedOrigins, ", ")

	if allowedOrigins == "*" {
		allowedOrigins = "http://localhost:5173"
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: true,
	}))

	// Fallback recover (Fiber's built-in) in case logger one fails
	app.Use(recover.New())

	// Custom Logger Middlewares (HTTP Request Logging + Panic Recovery)
	logMiddleware := middleware.NewLogger(deps.Logger)
	app.Use(logMiddleware.HTTPMiddleware())
	app.Use(logMiddleware.RecoveryMiddleware())

	// ==========================================
	// HEALTH CHECK
	// ==========================================
	if deps.Config.App.EnableHealth {
		app.Get("/health", func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"success": true,
				"status":  "healthy",
				"env":     deps.Config.App.Env,
				"version": "1.0.0",
			})
		})
	}

	// ==========================================
	// API V1 GROUP
	// ==========================================
	api := app.Group("/api/v1")

	// ==========================================
	// AUTH ROUTES (PUBLIC & SEMI-PROTECTED)
	// ==========================================
	auth := api.Group("/auth")
	authLimit := middleware.Auth()
	otpLimit := middleware.OTP()

	auth.Post("/register", authLimit, deps.AuthHdl.Register)
	auth.Post("/login", authLimit, deps.AuthHdl.Login)
	auth.Post("/google", authLimit, deps.AuthHdl.GoogleAuth)
	auth.Post("/verify-email", otpLimit, deps.AuthHdl.VerifyEmail)
	auth.Post("/resend-verification", otpLimit, deps.AuthHdl.ResendVerification)
	auth.Post("/forgot-password", otpLimit, deps.AuthHdl.ForgotPassword)
	auth.Post("/reset-password", otpLimit, deps.AuthHdl.ResetPassword)
	auth.Post("/refresh", middleware.General(), deps.AuthHdl.RefreshTokens)
	auth.Post("/logout", middleware.General(), deps.AuthHdl.Logout)

	// Protected Auth Actions
	authLevel := auth.Group("/", middleware.AuthRequired(deps.JWTManager))
	authLevel.Put("/change-password", deps.AuthHdl.ChangePassword)
	authLevel.Post("/logout-all", deps.AuthHdl.LogoutAll)
	authLevel.Get("/sessions", deps.AuthHdl.GetSessions)
	authLevel.Delete("/sessions/:id", deps.AuthHdl.RevokeSession)

	// ==========================================
	// ADMIN AUTH (PUBLIC)
	// ==========================================
	adminAuthPublic := api.Group("/admin/auth")
	adminAuthPublic.Post("/login", deps.AdminAuthHdl.Login)
	adminAuthPublic.Post("/forgot-password", deps.AdminAuthHdl.ForgotPassword)
	adminAuthPublic.Post("/reset-password", deps.AdminAuthHdl.ResetPassword)
	adminAuthPublic.Post("/forgot-secret", deps.AdminAuthHdl.ForgotSecret)
	adminAuthPublic.Post("/reset-secret", deps.AdminAuthHdl.ResetSecret)

	// ==========================================
	// ADMIN ROUTES (PROTECTED GROUP)
	// ==========================================
	admin := api.Group("/admin", middleware.AdminAuthRequired(deps.AdminJWTManager, deps.AdminSessionRepo))

	// ============================
	// ADMIN ROUTES AUTH & PROFILE
	// =============================
	admin.Post("/auth/logout", deps.AdminAuthHdl.Logout)
	admin.Post("/auth/refresh", deps.AdminAuthHdl.Refresh)
	admin.Get("/auth/me", deps.AdminAuthHdl.GetMe)
	admin.Put("/auth/profile", deps.AdminAuthHdl.UpdateProfile)

	// Admin Sessions
	adminSessions := admin.Group("/auth/sessions")
	adminSessions.Get("/", deps.AdminAuthHdl.GetSessions)
	adminSessions.Delete("/:id", deps.AdminAuthHdl.RevokeSession)
	adminSessions.Post("/logout-all", deps.AdminAuthHdl.LogoutAll)

	// ============================
	// ADMIN ROUTES USER MANAGEMENT
	// =============================
	adminUsers := admin.Group("/users")
	adminUsers.Get("/", deps.UserMgmtHdl.ListUsers)
	adminUsers.Get("/stats", deps.UserMgmtHdl.GetStats)
	adminUsers.Patch("/:id/status", deps.UserMgmtHdl.UpdateUserStatus)

	// ============================
	// ADMIN ROUTES COURSE
	// =============================
	adminCourses := admin.Group("/courses")
	adminCourses.Get("/:id", deps.CourseHdl.GetByID) // Get by Internal ID for editing
	adminCourses.Post("/", deps.CourseHdl.Create)
	adminCourses.Put("/:id", deps.CourseHdl.Update)
	adminCourses.Delete("/:id", deps.CourseHdl.Delete)

	// ============================
	// ADMIN ROUTES MODULE
	// =============================
	adminModules := admin.Group("/courses/:id/modules")
	adminModules.Post("/", deps.ModuleHdl.Create)
	adminModules.Get("/", deps.ModuleHdl.GetByCourseID)
	
	adminModuleOps := admin.Group("/modules/:id")
	adminModuleOps.Put("/", deps.ModuleHdl.Update)
	adminModuleOps.Delete("/", deps.ModuleHdl.Delete)

	// ============================
	// ADMIN ROUTES LESSON
	// =============================
	adminLessons := admin.Group("/modules/:moduleId/lessons")
	adminLessons.Post("/", deps.LessonHdl.Create)
	adminLessons.Get("/", deps.LessonHdl.ListByModule)

	adminLessonOps := admin.Group("/lessons/:id")
	adminLessonOps.Put("/", deps.LessonHdl.Update)
	adminLessonOps.Delete("/", deps.LessonHdl.Delete)
	adminLessonOps.Post("/video", deps.LessonHdl.UploadVideo)
	adminLessonOps.Get("/video-progress", deps.LessonHdl.GetVideoProgress)
	adminLessonOps.Post("/attachments", deps.LessonHdl.AddAttachment)
	
	admin.Delete("/attachments/:id", deps.LessonHdl.DeleteAttachment)

	// ============================
	// ADMIN ROUTES PAYMENTS & SALES
	// =============================
	adminPayments := admin.Group("/payments")
	adminPayments.Get("/", deps.AdminPurchaseHdl.ListPayments)
	adminPayments.Get("/stats", deps.AdminPurchaseHdl.GetSalesStats)

	admin.Get("/dashboard/summary", deps.DashboardHdl.GetSummary)

	// ============================
	// ADMIN ROUTES COUPONS, TICKETS, REVIEWS
	// =============================
	// Tickets
	adminTickets := admin.Group("/tickets")
	adminTickets.Get("/", deps.TicketHdl.ListAllTickets)
	adminTickets.Get("/:id", deps.TicketHdl.AdminGetConversation)
	adminTickets.Post("/:id/reply", deps.TicketHdl.AdminReply)
	adminTickets.Patch("/:id/status", deps.TicketHdl.AdminUpdateStatus)

	// ==========================================
	// USER PROTECTED ROUTES
	// ==========================================
	user := api.Group("/user", middleware.AuthRequired(deps.JWTManager))

	user.Get("/profile", deps.UserHdl.GetProfile)
	user.Get("/dashboard/summary", deps.StudentDashboardHdl.GetSummary)
	user.Get("/courses/my", deps.StudentDashboardHdl.GetMyCourses)
	user.Put("/profile", deps.UserHdl.UpdateProfile)
	user.Post("/avatar", deps.UserHdl.UploadAvatar)
	user.Get("/lessons/:id", deps.LessonHdl.GetByID)

	// Support Tickets (User)
	support := api.Group("/support", middleware.AuthRequired(deps.JWTManager))
	support.Post("/tickets", deps.TicketHdl.OpenTicket)
	support.Get("/tickets", deps.TicketHdl.GetMyTickets)
	support.Get("/tickets/:id", deps.TicketHdl.GetTicket)
	support.Get("/tickets/:id/messages", deps.TicketHdl.GetConversation)
	support.Post("/tickets/:id/reply", deps.TicketHdl.UserReply)
	// User can close their own ticket
	support.Patch("/tickets/:id/status", deps.TicketHdl.UserUpdateStatus)

	// ==========================================
	// PUBLIC DATA ROUTES
	// ==========================================
	courses := api.Group("/courses")
	courses.Get("/", deps.CourseHdl.List)
	courses.Get("/:slug", deps.CourseHdl.GetBySlug)
	courses.Get("/:slug/curriculum", deps.CourseHdl.GetCurriculum)

	// ==========================================
	// PAYMENT ROUTES
	// ==========================================
	payment := api.Group("/payment")
	payment.Post("/orders", middleware.AuthRequired(deps.JWTManager), deps.PurchaseHdl.CreateOrder)
	payment.Get("/my", middleware.AuthRequired(deps.JWTManager), deps.PurchaseHdl.GetMyPayments)
	payment.Get("/:id", middleware.AuthRequired(deps.JWTManager), deps.PurchaseHdl.GetPayment)
	payment.Post("/webhook", deps.PurchaseHdl.Webhook)

	// ==========================================
	// 404 HANDLER
	// ==========================================
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    fiber.StatusNotFound,
				"message": "Endpoint not found",
			},
		})
	})
}

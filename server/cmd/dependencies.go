package main

import (
	"log"

	"server/internal/broker"
	"server/internal/config"
	"server/internal/db"
	"server/internal/redis"
	"server/internal/security"
	"server/internal/utils"

	"server/internal/repository"
	"server/internal/queue"
	"server/internal/storage"
	authSvc "server/internal/services/auth"
	userSvc "server/internal/services/user"
	courseSvc "server/internal/services/course"
	emailSvc "server/internal/services/email"
	adminSvc "server/internal/services/admin"
	supportSvc "server/internal/services/support"
	
	paymentSvc "server/internal/services/payment"
	ebookSvc "server/internal/services/ebook"
	
	authHdl "server/internal/handlers/auth"
	userHdl "server/internal/handlers/user"
	courseHdl "server/internal/handlers/course"
	adminHdl "server/internal/handlers/admin"
	paymentHdl "server/internal/handlers/payment"
	supportHdl "server/internal/handlers/support"
	ebookHdl "server/internal/handlers/ebook"

	"context"
)

type AppDependencies struct {
	Config    *config.Config
	Logger    *utils.Logger
	AuthHdl    *authHdl.AuthHandler
	UserHdl    *userHdl.UserHandler
	CourseHdl     *courseHdl.CourseHandler
	UserMgmtHdl   *adminHdl.UserMgmtHandler
	AdminAuthHdl  *authHdl.AdminAuthHandler
	AdminSessionRepo *redis.AdminSessionRepository
	SessionRepo      *redis.SessionRepository
	JWTManager    *security.JWTManager
	AdminJWTManager *security.AdminJWTManager
	PurchaseHdl   *paymentHdl.PurchaseHandler
	ModuleHdl     *courseHdl.ModuleHandler
	LessonHdl     *courseHdl.LessonHandler
	TicketHdl     *supportHdl.TicketHandler
	AdminPurchaseHdl *adminHdl.AdminPurchaseHandler
	DashboardHdl     *adminHdl.DashboardHandler
	StudentDashboardHdl *userHdl.StudentDashboardHandler
	EbookHdl          *ebookHdl.EbookHandler
}

func SetupDependencies() *AppDependencies {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Setup Logger
	utils.Init(cfg)
	logger := utils.Get()
	utils.Info("Starting UPRAIZO backend...", nil)
	
	// 2.1 Ensure temp directories exist
	if err := cfg.App.EnsureTempDirs(); err != nil {
		utils.Fatal("Failed to create temp directories", err)
	}

	// 3. Connect to PostgreSQL
	if err := db.Connect(cfg); err != nil {
		utils.Fatal("Database setup failed", err)
	}

	// 4. Connect to Redis
	if err := redis.Connect(cfg); err != nil {
		utils.Fatal("Redis setup failed", err)
	}

	// 5. Connect to RabbitMQ
	if err := broker.Connect(cfg); err != nil {
		utils.Fatal("RabbitMQ setup failed", err)
	}

	// 6. Security setup
	jwtManager := security.NewJWTManager(&cfg.JWT)
	adminJWTManager := security.NewAdminJWTManager(&cfg.AdminJWT)

	// 7. Storage & Queue setup
	r2Client, err := storage.NewR2Client(cfg)
	if err != nil {
		utils.Fatal("Failed to initialize R2 client", err)
	}

	queueMgr := queue.NewManager(cfg)
	if err = queueMgr.SetupQueues(); err != nil {
		utils.Fatal("Failed to setup RabbitMQ queues", err)
	}

	// 8. Repositories
	userRepo := repository.NewUserRepository(db.Pool)
	adminRepo := repository.NewAdminRepository(db.Pool)
	courseRepo := repository.NewCourseRepository(db.Pool)
	tokenRepo := redis.NewTokenRepository()
	sessionRepo := redis.NewSessionRepository()
	adminSessionRepo := redis.NewAdminSessionRepository()
	adminTokenRepo := redis.NewAdminTokenRepository()
	statsRepo := repository.NewStatsRepository(db.Pool)
	purchaseRepo := repository.NewPurchaseRepository(db.Pool)
	moduleRepo := repository.NewModuleRepository(db.Pool)
	lessonRepo := repository.NewLessonRepository(db.Pool)
	attachmentRepo := repository.NewAttachmentRepository(db.Pool)
	ticketRepo := repository.NewTicketRepository(db.Pool)
	ebookRepo := repository.NewEbookRepository(db.Pool)

	// 8. Services
	emailService := emailSvc.NewEmailService(cfg)
	if err := emailService.DeclareQueue(); err != nil {
		utils.Fatal("Failed to declare email queue", err)
	}
	
	// Start background queue listener for emails
	if err := emailSvc.StartEmailConsumer(cfg); err != nil {
		utils.Fatal("Failed to start email consumer", err)
	}

	accountService := authSvc.NewAccountService(userRepo, tokenRepo, sessionRepo, jwtManager, emailService, cfg)
	passwordService := authSvc.NewPasswordService(userRepo, tokenRepo, sessionRepo, emailService, cfg)
	sessionService := authSvc.NewSessionService(userRepo, sessionRepo, jwtManager, cfg)
	userService := userSvc.NewUserService(userRepo, queueMgr, r2Client, cfg)
	courseService := courseSvc.NewCourseService(courseRepo, purchaseRepo, queueMgr, r2Client, cfg)
	moduleService := courseSvc.NewModuleService(moduleRepo)
	lessonService := courseSvc.NewLessonService(lessonRepo, attachmentRepo, queueMgr, r2Client, cfg)
	ticketService := supportSvc.NewTicketService(ticketRepo, queueMgr, cfg)
	adminAuthService := authSvc.NewAdminAuthService(adminRepo, adminSessionRepo, adminTokenRepo, adminJWTManager, emailService, cfg)
	userMgmtService := adminSvc.NewUserMgmtService(userRepo, statsRepo, sessionRepo)
	adminPurchaseService := adminSvc.NewAdminPurchaseService(purchaseRepo)
	dashboardService := adminSvc.NewDashboardService(statsRepo, purchaseRepo)
	studentDashboardService := userSvc.NewStudentDashboardService(purchaseRepo, courseRepo, ebookRepo)
	ebookService := ebookSvc.NewEbookService(ebookRepo, purchaseRepo, r2Client)
	razorpayService := paymentSvc.NewRazorpayService(cfg)

	// 9. Workers
	avatarWorker := queue.NewAvatarWorker(cfg, userRepo, r2Client)
	if err := avatarWorker.Start(context.Background()); err != nil {
		utils.Fatal("Failed to start avatar worker", err)
	}

	purchaseWorker := queue.NewPurchaseWorker(cfg, purchaseRepo, courseRepo, ebookRepo)
	if err := purchaseWorker.Start(context.Background()); err != nil {
		utils.Fatal("Failed to start purchase worker", err)
	}


	courseThumbnailWorker := queue.NewCourseThumbnailWorker(cfg, courseRepo, r2Client)
	if err := courseThumbnailWorker.Start(context.Background()); err != nil {
		utils.Fatal("Failed to start course thumbnail worker", err)
	}


	// 10. Handlers
	authHandler := authHdl.NewAuthHandler(accountService, passwordService, sessionService, cfg)
	userHandler := userHdl.NewUserHandler(userService)
	courseHandler := courseHdl.NewCourseHandler(courseService)
	adminAuthHandler := authHdl.NewAdminAuthHandler(adminAuthService)
	userMgmtHandler := adminHdl.NewUserMgmtHandler(userMgmtService)
	purchaseHandler := paymentHdl.NewPurchaseHandler(razorpayService, purchaseRepo, courseRepo, ebookRepo, queueMgr, cfg)
	moduleHandler := courseHdl.NewModuleHandler(moduleService)
	lessonHandler := courseHdl.NewLessonHandler(lessonService)
	ticketHandler := supportHdl.NewTicketHandler(ticketService)
	adminPurchaseHandler := adminHdl.NewAdminPurchaseHandler(adminPurchaseService)
	dashboardHandler := adminHdl.NewDashboardHandler(dashboardService)
	studentDashboardHandler := userHdl.NewStudentDashboardHandler(studentDashboardService)
	ebookHandler := ebookHdl.NewEbookHandler(ebookService)

	return &AppDependencies{
		Config:    cfg,
		Logger:    logger,
		AuthHdl:   authHandler,
		UserHdl:   userHandler,
		CourseHdl: courseHandler,
		UserMgmtHdl: userMgmtHandler,
		AdminAuthHdl: adminAuthHandler,
		AdminSessionRepo: adminSessionRepo,
		SessionRepo:      sessionRepo,
		JWTManager: jwtManager,
		AdminJWTManager: adminJWTManager,
		PurchaseHdl: purchaseHandler,
		ModuleHdl:   moduleHandler,
		LessonHdl:   lessonHandler,
		TicketHdl:   ticketHandler,
		AdminPurchaseHdl: adminPurchaseHandler,
		DashboardHdl:     dashboardHandler,
		StudentDashboardHdl: studentDashboardHandler,
		EbookHdl:          ebookHandler,
	}
}

func CloseDependencies() {
	db.Close()
	redis.Close()
	broker.Close()
	utils.Info("All dependencies closed correctly", nil)
}
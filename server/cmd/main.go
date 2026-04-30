package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"server/internal/middleware"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// 1. Initialize all dependencies (DB, Redis, MQ, Services)
	deps := SetupDependencies()
	defer CloseDependencies()

	// 2. Configure Fiber App with Custom Error Handler
	app := fiber.New(fiber.Config{
		ErrorHandler:          middleware.FiberErrorHandler,
		DisableStartupMessage: true,
		BodyLimit:             deps.Config.App.RequestBodyLimit,
		ReadTimeout:           deps.Config.App.ReadTimeout,
		WriteTimeout:          deps.Config.App.WriteTimeout,
		IdleTimeout:           deps.Config.App.IdleTimeout,
		ProxyHeader:           "X-Forwarded-For",
		TrustedProxies:        deps.Config.App.TrustedProxies,
	})

	// 3. Register Middlewares & Routes
	SetupRoutes(app, deps)

	// 4. Start Server with Graceful Shutdown
	go func() {
		port := fmt.Sprintf(":%d", deps.Config.App.Port)
		utils.Info("Server listening", map[string]any{"port": deps.Config.App.Port, "env": deps.Config.App.Env})
		
		if err := app.Listen(port); err != nil {
			utils.Fatal("Server forced to shutdown", err)
		}
	}()

	// 5. Setup Listeners for OS Interrupts (Ctrl+C, Docker Stop)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	utils.Info("Gracefully shutting down server...", nil)
	if err := app.Shutdown(); err != nil {
		utils.Error("Fiber shutdown error", err, nil)
	}

	utils.Info("Server stopped cleanly", nil)
}

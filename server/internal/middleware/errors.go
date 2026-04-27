package middleware

import (
	"errors"
	"net/http"

	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
)

// FiberErrorHandler handles all errors returned by Fiber routes
func FiberErrorHandler(c *fiber.Ctx, err error) error {
	// Default Fallback
	code := http.StatusInternalServerError
	message := "Internal Server Error"
	var fields map[string]string

	log := utils.Get().WithFiberContext(c)

	// Check type of error
	var e *utils.AppError
	if errors.As(err, &e) {
		code = e.Code
		message = e.Message
		fields = e.Fields
		// If 500+ error (DB fail, Logic fail), then only print Error in server console
		if code >= 500 {
			log.Error().Err(e.Err).Msg("Internal App Error")
		} else {
			// 400s (Client side errors) ko bas Debug mein daal do (console ganda nahi hoga)
			log.Warn().Err(e.Err).Msg("Client Request Error")
		}
	} else if fiberErr, ok := err.(*fiber.Error); ok {
		code = fiberErr.Code
		message = fiberErr.Message
	} else {
		log.Error().Err(err).Msg("Unhandled Server Error")
	}

	// Final standard JSON structure for Frontend
	response := fiber.Map{
		"success": false,
		"error": fiber.Map{
			"code":    code,
			"message": message,
		},
	}

	// Validation fields tabhi add karo jab zaroorat ho
	if len(fields) > 0 {
		response["error"].(fiber.Map)["fields"] = fields
	}

	return c.Status(code).JSON(response)
}
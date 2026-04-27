package handler

import (
	"net/http"

	"server/internal/config"
	"server/internal/dto"
	service "server/internal/services/auth"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
)

// ─── Handler ─────────────────────────────────────────────────────────────────
type AuthHandler struct {
	accountService  *service.AccountService
	passwordService *service.PasswordService
	sessionService  *service.SessionService
	cfg             *config.Config
}

// NewAuthHandler constructs the handler with all three auth sub-services.
func NewAuthHandler(
	accountService *service.AccountService,
	passwordService *service.PasswordService,
	sessionService *service.SessionService,
	cfg *config.Config,
) *AuthHandler {
	return &AuthHandler{
		accountService:  accountService,
		passwordService: passwordService,
		sessionService:  sessionService,
		cfg:             cfg,
	}
}

// ─── Cookie Helpers ──────────────────────────────────────────────────────────
func (h *AuthHandler) setRefreshCookie(c *fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		HTTPOnly: true,
		Secure:   h.cfg.App.IsProduction(),
		SameSite: "Strict",
		MaxAge:   int(h.cfg.JWT.RefreshTokenDuration.Seconds()),
	})
}

// clearRefreshCookie expires the cookie immediately.
func (h *AuthHandler) clearRefreshCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		HTTPOnly: true,
		Secure:   h.cfg.App.IsProduction(),
		SameSite: "Strict",
		MaxAge:   -1,
	})
}

// ─── POST /auth/register ─────────────────────────────────────────────────────

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req dto.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if appErr := utils.ValidateStruct(&req); appErr != nil {
		return appErr
	}

	resp, err := h.accountService.Register(
		c.Context(), &req, c.IP(), c.Get("User-Agent"),
	)
	if err != nil {
		return err
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// ─── POST /auth/login ────────────────────────────────────────────────────────

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if appErr := utils.ValidateStruct(&req); appErr != nil {
		return appErr
	}

	resp, refreshToken, err := h.accountService.Login(
		c.Context(), &req, c.IP(), c.Get("User-Agent"),
	)
	if err != nil {
		return err
	}

	h.setRefreshCookie(c, refreshToken)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// ─── POST /auth/verify-email ─────────────────────────────────────────────────

func (h *AuthHandler) VerifyEmail(c *fiber.Ctx) error {
	var req dto.VerifyEmailRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if appErr := utils.ValidateStruct(&req); appErr != nil {
		return appErr
	}

	resp, refreshToken, err := h.accountService.VerifyEmail(
		c.Context(), req.Token, c.IP(), c.Get("User-Agent"),
	)
	if err != nil {
		return err
	}

	h.setRefreshCookie(c, refreshToken)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// ─── POST /auth/resend-verification ──────────────────────────────────────────

func (h *AuthHandler) ResendVerification(c *fiber.Ctx) error {
	var req dto.ResendVerificationRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if appErr := utils.ValidateStruct(&req); appErr != nil {
		return appErr
	}

	resp, err := h.accountService.ResendVerification(c.Context(), req.Email)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// ─── POST /auth/google ───────────────────────────────────────────────────────

func (h *AuthHandler) GoogleAuth(c *fiber.Ctx) error {
	var req dto.GoogleAuthRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if appErr := utils.ValidateStruct(&req); appErr != nil {
		return appErr
	}

	resp, refreshToken, err := h.accountService.GoogleAuth(
		c.Context(), req.IDToken, c.IP(), c.Get("User-Agent"),
	)
	if err != nil {
		return err
	}

	h.setRefreshCookie(c, refreshToken)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}
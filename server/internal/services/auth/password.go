package service

import (
	"context"
	"strings"
	"time"

	"server/internal/config"
	"server/internal/dto"
	"server/internal/models"
	"server/internal/redis"
	"server/internal/repository"
	"server/internal/security"
	emailsvc "server/internal/services/email"
	"server/internal/utils"

	"github.com/google/uuid"
)

// ─── Service ─────────────────────────────────────────────────────────────────

// PasswordService owns the ForgotPassword and ResetPassword use-cases.
type PasswordService struct {
	userRepo     *repository.UserRepository
	tokenRepo    *redis.TokenRepository
	sessionRepo  *redis.SessionRepository
	emailService *emailsvc.EmailService
	cfg          *config.Config
}

// NewPasswordService wires up dependencies.
func NewPasswordService(
	userRepo *repository.UserRepository,
	tokenRepo *redis.TokenRepository,
	sessionRepo *redis.SessionRepository,
	emailService *emailsvc.EmailService,
	cfg *config.Config,
) *PasswordService {
	return &PasswordService{
		userRepo:     userRepo,
		tokenRepo:    tokenRepo,
		sessionRepo:  sessionRepo,
		emailService: emailService,
		cfg:          cfg,
	}
}

// ─── Forgot Password ─────────────────────────────────────────────────────────

// ForgotPassword generates a reset token (1 h TTL) and queues the reset email.
//
// The response message is always the same regardless of whether the email
// exists or belongs to an OAuth account — this prevents email enumeration.
func (s *PasswordService) ForgotPassword(
	ctx context.Context,
	email string,
) (*dto.MessageResponse, error) {

	email = strings.ToLower(strings.TrimSpace(email))
	successMsg := &dto.MessageResponse{
		Message: "If an account exists with this email, a password reset link has been sent",
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, utils.Internal(err)
	}
	if user == nil {
		return successMsg, nil // don't reveal non-existence
	}

	// OAuth accounts don't have a password to reset
	if user.AuthProvider != models.AuthProviderEmail {
		return successMsg, nil
	}

	// Generate + store reset token (1 hour TTL)
	resetToken, err := security.GeneratePasswordResetToken()
	if err != nil {
		return nil, err
	}
	if err = s.tokenRepo.StorePasswordResetToken(ctx, user.ID.String(), resetToken, 1*time.Hour); err != nil {
		return nil, utils.Internal(err)
	}

	go func() {
		if emailErr := s.emailService.SendPasswordResetEmail(user.Name, email, resetToken); emailErr != nil {
			utils.Error("Failed to queue password reset email", emailErr, map[string]any{
				"user_id": user.ID,
			})
		}
	}()

	return successMsg, nil
}

// ─── Reset Password ──────────────────────────────────────────────────────────

// ResetPassword consumes the one-time reset token, hashes the new password,
// persists it, and then revokes every active session for the user so that
// stolen tokens cannot be reused.
func (s *PasswordService) ResetPassword(
	ctx context.Context,
	token, newPassword string,
) (*dto.MessageResponse, error) {

	// 1. Redis token lookup
	userIDStr, err := s.tokenRepo.GetUserByResetToken(ctx, token)
	if err != nil || userIDStr == "" {
		return nil, utils.BadRequest("Invalid or expired reset token")
	}

	// 2. Parse UUID
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, utils.Internal(err)
	}

	// 3. Fetch user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, utils.Internal(err)
	}
	if user == nil {
		return nil, utils.NotFound("User not found")
	}

	// 4. Hash the new password
	hashedPassword, err := security.HashPassword(newPassword)
	if err != nil {
		return nil, err
	}

	// 5. Persist
	if err = s.userRepo.UpdatePassword(ctx, userID, hashedPassword); err != nil {
		return nil, utils.Internal(err)
	}

	// 6. Burn the reset token
	_ = s.tokenRepo.DeleteResetToken(ctx, token)

	// 7. Security: revoke ALL active sessions — forces re-login on every device
	if err = s.sessionRepo.RevokeAllSessions(ctx, userID.String()); err != nil {
		// Non-fatal: password is already updated
		utils.Error("Failed to revoke sessions after password reset", err, map[string]any{
			"user_id": userID,
		})
	}

	return &dto.MessageResponse{
		Message: "Password has been reset successfully. Please login with your new password",
	}, nil
}

// ─── Change Password ─────────────────────────────────────────────────────────

// ChangePassword lets an authenticated user update their password by providing
// the current one. Only works for email-based accounts.
//
// After a successful change, all OTHER sessions are revoked so that any
// device that might have the old credentials is forced to re-authenticate.
// The current session (caller) is kept alive.
func (s *PasswordService) ChangePassword(
	ctx context.Context,
	userID uuid.UUID,
	currentPassword, newPassword string,
) (*dto.MessageResponse, error) {

	// 1. Fetch user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, utils.Internal(err)
	}
	if user == nil {
		return nil, utils.NotFound("User not found")
	}

	// 2. Only email-based accounts have passwords
	if user.AuthProvider != models.AuthProviderEmail {
		return nil, utils.BadRequest("Password change is not available for " + string(user.AuthProvider) + " accounts")
	}

	// 3. Verify current password
	if user.PasswordHash == nil || !security.VerifyPassword(*user.PasswordHash, currentPassword) {
		return nil, utils.Unauthorized("Current password is incorrect")
	}

	// 4. Prevent reusing the same password
	if security.VerifyPassword(*user.PasswordHash, newPassword) {
		return nil, utils.BadRequest("New password must be different from your current password")
	}

	// 5. Hash + persist
	hashedPassword, err := security.HashPassword(newPassword)
	if err != nil {
		return nil, err
	}
	if err = s.userRepo.UpdatePassword(ctx, userID, hashedPassword); err != nil {
		return nil, utils.Internal(err)
	}

	// 6. Revoke all sessions (other devices) for security
	if err = s.sessionRepo.RevokeAllSessions(ctx, userID.String()); err != nil {
		utils.Error("Failed to revoke sessions after password change", err, map[string]any{
			"user_id": userID,
		})
	}

	return &dto.MessageResponse{
		Message: "Password changed successfully",
	}, nil
}
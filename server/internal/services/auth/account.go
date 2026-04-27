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

// use-cases.  It orchestrates the user repository, Redis token + session 
// stores, JWT manager, and the email producer.
type AccountService struct {
	userRepo     *repository.UserRepository
	tokenRepo    *redis.TokenRepository
	sessionRepo  *redis.SessionRepository
	jwtManager   *security.JWTManager
	emailService *emailsvc.EmailService
	cfg          *config.Config
}

// NewAccountService wires up all dependencies.
func NewAccountService(
	userRepo *repository.UserRepository,
	tokenRepo *redis.TokenRepository,
	sessionRepo *redis.SessionRepository,
	jwtManager *security.JWTManager,
	emailService *emailsvc.EmailService,
	cfg *config.Config,
) *AccountService {
	return &AccountService{
		userRepo:     userRepo,
		tokenRepo:    tokenRepo,
		sessionRepo:  sessionRepo,
		jwtManager:   jwtManager,
		emailService: emailService,
		cfg:          cfg,
	}
}

// ─── Register ────────────────────────────────────────────────────────────────
func (s *AccountService) Register(
	ctx context.Context,
	req *dto.RegisterRequest,
	ip, userAgent string,
) (*dto.MessageResponse, error) {

	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.Name)

	// 1. Email uniqueness check
	existing, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, utils.Internal(err)
	}
	if existing != nil {
		return nil, utils.Conflict("An account with this email already exists")
	}

	// 2. Hash password (bcrypt cost 12)
	hashedPassword, err := security.HashPassword(req.Password)
	if err != nil {
		return nil, err // already wrapped by crypto
	}

	// 3. Build the user model
	now := time.Now()
	user := &models.User{
		ID:           security.NEWID(),
		Name:         name,
		Email:        email,
		PasswordHash: &hashedPassword,
		AuthProvider: models.AuthProviderEmail,
		Status:       models.UserStatusInactive, // awaiting verification
		IsVerified:   false,
		Preferences:  models.DefaultPreferences(),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// 4. Persist user
	if err = s.userRepo.Create(ctx, user); err != nil {
		return nil, utils.Internal(err)
	}

	// 5. Store a verification token in Redis (24 h TTL)
	verifyToken, err := security.GenerateVerificationToken()
	if err != nil {
		return nil, err
	}
	if err = s.tokenRepo.StoreEmailToken(ctx, user.ID.String(), verifyToken, 24*time.Hour); err != nil {
		utils.Error("Failed to store verification token", err, map[string]any{
			"user_id": user.ID,
		})
	}

	// 7. Queue the verification email (fire-and-forget)
	go func() {
		if emailErr := s.emailService.SendVerificationEmail(name, email, verifyToken); emailErr != nil {
			utils.Error("Failed to queue verification email", emailErr, map[string]any{
				"user_id": user.ID,
			})
		}
	}()

	// 8. Return success message
	return &dto.MessageResponse{
		Message: "Registration successful. Please verify your email to continue.",
	}, nil
}

// ─── Login ───────────────────────────────────────────────────────────────────
func (s *AccountService) Login(
	ctx context.Context,
	req *dto.LoginRequest,
	ip, userAgent string,
) (*dto.AuthResponse, string, error) {

	email := strings.ToLower(strings.TrimSpace(req.Email))

	// 1. Fetch user
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", utils.Internal(err)
	}
	if user == nil {
		return nil, "", utils.Unauthorized("Invalid email or password")
	}

	// 2. Only email-based accounts can use this endpoint
	if user.AuthProvider != models.AuthProviderEmail {
		return nil, "", utils.BadRequest(
			"This account uses " + string(user.AuthProvider) + " sign-in. Please use that method instead",
		)
	}

	// 3. Account status gates
	if user.IsBanned() {
		return nil, "", utils.Forbidden("Your account has been permanently banned")
	}
	if user.IsSuspended() {
		return nil, "", utils.Forbidden("Your account is temporarily suspended")
	}

	// 4. Verify password (constant-time via bcrypt)
	if user.PasswordHash == nil || !security.VerifyPassword(*user.PasswordHash, req.Password) {
		return nil, "", utils.Unauthorized("Invalid email or password")
	}

	// 5. Email verification check
	if !user.IsVerified {
		return nil, "", utils.Forbidden("Please verify your email address before logging in")
	}

	// 6. Final safety: account must be active
	if !user.IsActive() {
		return nil, "", utils.Forbidden("Your account is not currently active")
	}

	// 7. Issue tokens + persist session
	return s.createAuthSession(ctx, user, ip, userAgent)
}

// ─── Verify Email ────────────────────────────────────────────────────────────
func (s *AccountService) VerifyEmail(
	ctx context.Context,
	token, ip, userAgent string,
) (*dto.AuthResponse, string, error) {

	// 1. Redis lookup
	userIDStr, err := s.tokenRepo.GetUserByEmailToken(ctx, token)
	if err != nil || userIDStr == "" {
		return nil, "", utils.BadRequest("Invalid or expired verification token")
	}

	// 2. Parse UUID
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, "", utils.Internal(err)
	}

	// 3. Fetch user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, "", utils.Internal(err)
	}
	if user == nil {
		return nil, "", utils.NotFound("User not found")
	}

	// 4. Idempotency guard
	if user.IsVerified {
		return nil, "", utils.BadRequest("Email is already verified")
	}

	// 5. Promote to verified + active
	if err = s.userRepo.VerifyUser(ctx, userID); err != nil {
		return nil, "", utils.Internal(err)
	}

	user.IsVerified = true // Update local model for session creation

	// 7. Create session and return auth response
	return s.createAuthSession(ctx, user, ip, userAgent)
}

// ─── Resend Verification ─────────────────────────────────────────────────────
func (s *AccountService) ResendVerification(
	ctx context.Context,
	email string,
) (*dto.MessageResponse, error) {

	email = strings.ToLower(strings.TrimSpace(email))
	successMsg := &dto.MessageResponse{
		Message: "If an account exists with this email, a verification link has been sent",
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, utils.Internal(err)
	}
	if user == nil {
		return successMsg, nil // anti-enumeration
	}
	if user.IsVerified {
		return successMsg, nil // already verified — no-op
	}

	// Generate + store new token (overwrites any existing one)
	verifyToken, err := security.GenerateVerificationToken()
	if err != nil {
		return nil, err
	}
	if err = s.tokenRepo.StoreEmailToken(ctx, user.ID.String(), verifyToken, 24*time.Hour); err != nil {
		return nil, utils.Internal(err)
	}

	go func() {
		if emailErr := s.emailService.SendVerificationEmail(user.Name, email, verifyToken); emailErr != nil {
			utils.Error("Failed to queue resend-verification email", emailErr, map[string]any{
				"user_id": user.ID,
			})
		}
	}()

	return successMsg, nil
}

// ─── Internal Helper ─────────────────────────────────────────────────────────

// createAuthSession generates a JWT pair, stores a session in Redis keyed by
// the refresh-token's JTI, and returns the AuthResponse + raw refresh-token.
func (s *AccountService) createAuthSession(
	ctx context.Context,
	user *models.User,
	ip, userAgent string,
) (*dto.AuthResponse, string, error) {

	// 1. Generate access + refresh JWTs
	tokenPair, err := s.jwtManager.GenerateTokenPair(user)
	if err != nil {
		return nil, "", utils.Internal(err)
	}

	// 2. Extract the refresh-token's JTI — serves as the session ID
	refreshClaims, err := s.jwtManager.ValidateToken(tokenPair.RefreshToken, security.Refresh)
	if err != nil {
		return nil, "", utils.Internal(err)
	}

	// 3. Build session metadata
	info := utils.ParseClientInfo(ip, userAgent)
	now := time.Now()
	session := &dto.SessionMetadata{
		ID:           refreshClaims.RegisteredClaims.ID, // JTI = session key
		UserID:       user.ID.String(),
		RefreshToken: tokenPair.RefreshToken,
		IPAddress:    ip,
		Browser:      info.Browser,
		OS:           info.OS,
		DeviceName:   info.DeviceName,
		City:         info.City,
		Country:      info.Country,
		CreatedAt:    now,
		ExpiresAt:    now.Add(s.cfg.JWT.RefreshTokenDuration),
	}

	// 4. Persist in Redis with refresh-token TTL
	if err = s.sessionRepo.StoreSession(ctx, session, s.cfg.JWT.RefreshTokenDuration); err != nil {
		return nil, "", utils.Internal(err)
	}

	// 5. Build the JSON-safe response
	resp := &dto.AuthResponse{
		User:        dto.ToSafeUser(user),
		AccessToken: tokenPair.AccessToken,
		ExpiresAt:   tokenPair.ExpiresAt,
	}
	return resp, tokenPair.RefreshToken, nil
}
package service

import (
	"context"
	"time"

	"server/internal/config"
	"server/internal/dto"
	"server/internal/redis"
	"server/internal/repository"
	"server/internal/security"
	emailsvc "server/internal/services/email"
	"server/internal/utils"

	"github.com/google/uuid"
)

type AdminAuthService struct {
	adminRepo       *repository.AdminRepository
	sessionRepo     *redis.AdminSessionRepository
	tokenRepo       *redis.AdminTokenRepository
	adminJWTManager *security.AdminJWTManager
	emailService    *emailsvc.EmailService
	cfg             *config.Config
}

func NewAdminAuthService(
	adminRepo *repository.AdminRepository,
	sessionRepo *redis.AdminSessionRepository,
	tokenRepo *redis.AdminTokenRepository,
	adminJWTManager *security.AdminJWTManager,
	emailService *emailsvc.EmailService,
	cfg *config.Config,
) *AdminAuthService {
	return &AdminAuthService{
		adminRepo:       adminRepo,
		sessionRepo:     sessionRepo,
		tokenRepo:       tokenRepo,
		adminJWTManager: adminJWTManager,
		emailService:    emailService,
		cfg:             cfg,
	}
}

// ─── Admin Login ──────────────────────────────────────────

func (s *AdminAuthService) Login(
	ctx context.Context,
	email, password, secretKey, ip, userAgent string,
) (*dto.AdminAuthResponse, string, error) {

	// 1. Fetch Admin
	admin, err := s.adminRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", utils.Unauthorized("Invalid administrative credentials")
	}
	if admin == nil || !admin.IsActive {
		return nil, "", utils.Unauthorized("Invalid administrative credentials")
	}

	// 2. Verify Password
	if !security.VerifyPassword(admin.PasswordHash, password) {
		return nil, "", utils.Unauthorized("Invalid administrative credentials")
	}

	// 3. Verify Secret Key
	if !security.VerifyPassword(admin.SecretKeyHash, secretKey) {
		return nil, "", utils.Unauthorized("Invalid administrative credentials")
	}

	// 4. Generate Admin Token Pair
	tokenPair, err := s.adminJWTManager.GenerateTokenPair(admin)
	if err != nil {
		return nil, "", utils.Internal(err)
	}

	// 5. Store Session in Redis (Admin prefix)
	claims, _ := s.adminJWTManager.ValidateToken(tokenPair.RefreshToken, security.Refresh)

	info := utils.ParseClientInfo(ip, userAgent)
	now := time.Now()
	session := &dto.SessionMetadata{
		ID:           claims.RegisteredClaims.ID,
		UserID:       admin.ID.String(),
		RefreshToken: tokenPair.RefreshToken,
		IPAddress:    ip,
		Browser:      info.Browser,
		OS:           info.OS,
		DeviceName:   info.DeviceName,
		CreatedAt:    now,
		ExpiresAt:    now.Add(s.cfg.AdminJWT.RefreshTokenDuration),
	}

	if err = s.sessionRepo.StoreSession(ctx, session, s.cfg.AdminJWT.RefreshTokenDuration); err != nil {
		return nil, "", utils.Internal(err)
	}

	// 6. Response
	resp := &dto.AdminAuthResponse{
		Admin:       dto.ToSafeAdmin(admin),
		AccessToken: tokenPair.AccessToken,
		ExpiresAt:   tokenPair.ExpiresAt,
	}

	return resp, tokenPair.RefreshToken, nil
}

// ─── Admin Logout ──────────────────────────────────────────


func (s *AdminAuthService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}

	claims, err := s.adminJWTManager.ValidateToken(refreshToken, security.Refresh)
	if err != nil {
		return nil // Token invalid or expired, nothing to revoke
	}

	return s.sessionRepo.RevokeSession(ctx, claims.AdminID.String(), claims.RegisteredClaims.ID)
}

func (s *AdminAuthService) GetActiveSessions(ctx context.Context, adminID, currentSessionID string) ([]dto.SafeSession, error) {
	metas, err := s.sessionRepo.GetActiveSessions(ctx, adminID)
	if err != nil {
		return nil, err
	}

	sessions := make([]dto.SafeSession, 0, len(metas))
	for _, m := range metas {
		sessions = append(sessions, dto.SafeSession{
			ID:         m.ID,
			IPAddress:  m.IPAddress,
			Browser:    m.Browser,
			OS:         m.OS,
			DeviceName: m.DeviceName,
			City:       m.City,
			Country:    m.Country,
			CreatedAt:  m.CreatedAt,
			ExpiresAt:  m.ExpiresAt,
			IsCurrent:  m.ID == currentSessionID,
		})
	}

	return sessions, nil
}

func (s *AdminAuthService) RevokeSession(ctx context.Context, adminID, sessionID string) error {
	return s.sessionRepo.RevokeSession(ctx, adminID, sessionID)
}


func (s *AdminAuthService) RevokeAllSessions(ctx context.Context, adminID string) error {
	return s.sessionRepo.RevokeAllSessions(ctx, adminID)
}

func (s *AdminAuthService) GetByID(ctx context.Context, id uuid.UUID) (dto.SafeAdmin, error) {
	admin, err := s.adminRepo.GetByID(ctx, id)
	if err != nil {
		return dto.SafeAdmin{}, err
	}
	if admin == nil {
		return dto.SafeAdmin{}, utils.NotFound("Admin not found")
	}
	return dto.ToSafeAdmin(admin), nil
}

func (s *AdminAuthService) UpdateProfile(ctx context.Context, id uuid.UUID, name, phone *string) (dto.SafeAdmin, error) {
	if err := s.adminRepo.UpdateProfile(ctx, id, name, phone); err != nil {
		return dto.SafeAdmin{}, err
	}

	// Fetch updated
	return s.GetByID(ctx, id)
}



func (s *AdminAuthService) RefreshTokens(ctx context.Context, refreshToken, ip, userAgent string) (*dto.AdminAuthResponse, string, error) {
	claims, err := s.adminJWTManager.ValidateToken(refreshToken, security.Refresh)
	if err != nil {
		return nil, "", err
	}

	// Verify session in Redis
	session, err := s.sessionRepo.GetSession(ctx, claims.RegisteredClaims.ID)
	if err != nil {
		return nil, "", utils.Unauthorized("Admin session has expired")
	}

	if session.RefreshToken != refreshToken {
		_ = s.sessionRepo.RevokeAllSessions(ctx, session.UserID)
		return nil, "", utils.Unauthorized("Replay attack detected, all admin sessions revoked")
	}

	admin, err := s.adminRepo.GetByID(ctx, claims.AdminID)
	if err != nil || admin == nil || !admin.IsActive {
		return nil, "", utils.Unauthorized("Admin account not found or disabled")
	}

	// Revoke old and issue new
	_ = s.sessionRepo.RevokeSession(ctx, admin.ID.String(), session.ID)

	tokenPair, err := s.adminJWTManager.GenerateTokenPair(admin)
	if err != nil {
		return nil, "", utils.Internal(err)
	}

	newClaims, _ := s.adminJWTManager.ValidateToken(tokenPair.RefreshToken, security.Refresh)
	info := utils.ParseClientInfo(ip, userAgent)
	now := time.Now()
	newSession := &dto.SessionMetadata{
		ID:           newClaims.RegisteredClaims.ID,
		UserID:       admin.ID.String(),
		RefreshToken: tokenPair.RefreshToken,
		IPAddress:    ip,
		Browser:      info.Browser,
		OS:           info.OS,
		DeviceName:   info.DeviceName,
		CreatedAt:    now,
		ExpiresAt:    now.Add(s.cfg.AdminJWT.RefreshTokenDuration),
	}

	if err = s.sessionRepo.StoreSession(ctx, newSession, s.cfg.AdminJWT.RefreshTokenDuration); err != nil {
		return nil, "", utils.Internal(err)
	}

	resp := &dto.AdminAuthResponse{
		Admin:       dto.ToSafeAdmin(admin),
		AccessToken: tokenPair.AccessToken,
		ExpiresAt:   tokenPair.ExpiresAt,
	}

	return resp, tokenPair.RefreshToken, nil
}

// ─── Password Recovery ──────────────────────────────────────

func (s *AdminAuthService) ForgotPassword(ctx context.Context, name, email, phone string) error {
	// 1. Triple-Check: Fetch Admin by Email and verify Name/Phone
	admin, err := s.adminRepo.GetByEmail(ctx, email)
	if err != nil {
		// Security: Don't leak exists/not-exists
		return nil
	}
	if admin == nil || admin.Name != name || admin.Phone != phone {
		return nil
	}

	// 2. Generate 15-min Reset Token
	token, err := security.GenerateRandomToken(32)
	if err != nil {
		return utils.Internal(err)
	}

	if err := s.tokenRepo.StoreResetToken(ctx, token, admin.ID.String(), 15*time.Minute); err != nil {
		return utils.Internal(err)
	}

	// 3. Queue Email in Background
	go func() {
		if err := s.emailService.SendAdminResetPasswordEmail(admin.Name, admin.Email, token); err != nil {
			utils.Error("Failed to queue admin reset email asynchronously", err, map[string]any{
				"admin_email": admin.Email,
			})
		}
	}()

	return nil
}

func (s *AdminAuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// 1. Validate Token
	adminIDStr, err := s.tokenRepo.GetAdminIDByToken(ctx, token)
	if err != nil {
		return err
	}

	adminID, _ := uuid.Parse(adminIDStr)

	// 2. Update Password
	hash, err := security.HashPassword(newPassword)
	if err != nil {
		return utils.Internal(err)
	}

	if err := s.adminRepo.UpdatePassword(ctx, adminID, hash); err != nil {
		return err
	}

	// 3. Revoke all active sessions (Security reset)
	_ = s.sessionRepo.RevokeAllSessions(ctx, adminIDStr)

	return nil
}

// ─── Secret Key Recovery ────────────────────────────────────

func (s *AdminAuthService) ForgotSecretRequest(ctx context.Context, name, email, phone, password string) error {
	// 1. Quadruple-Check: Email must exist, Name and Phone must match
	admin, err := s.adminRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil // Security: Don't leak
	}
	if admin == nil || admin.Name != name || admin.Phone != phone {
		return nil
	}

	// 2. Authenticate with Password
	if !security.VerifyPassword(admin.PasswordHash, password) {
		return nil
	}

	// 3. Generate 15-min Token
	token, err := security.GenerateRandomToken(32)
	if err != nil {
		return utils.Internal(err)
	}

	if err := s.tokenRepo.StoreSecretToken(ctx, token, admin.ID.String(), 15*time.Minute); err != nil {
		return utils.Internal(err)
	}

	// 4. Queue Email (Async)
	go func() {
		if err := s.emailService.SendSecretResetEmail(admin.Name, admin.Email, token); err != nil {
			utils.Error("Failed to queue admin secret reset email", err, map[string]any{
				"admin_email": admin.Email,
			})
		}
	}()

	return nil
}

func (s *AdminAuthService) ResetSecretKey(ctx context.Context, token, newSecret string) error {
	// 1. Validate Token
	adminIDStr, err := s.tokenRepo.GetAdminIDBySecretToken(ctx, token)
	if err != nil {
		return err
	}

	adminID, _ := uuid.Parse(adminIDStr)

	// 2. Hash and Update new Secret Key
	hash, err := security.HashPassword(newSecret)
	if err != nil {
		return utils.Internal(err)
	}

	if err := s.adminRepo.UpdateSecretKey(ctx, adminID, hash); err != nil {
		return err
	}

	// 3. Revoke all sessions (Force re-login with new secret)
	_ = s.sessionRepo.RevokeAllSessions(ctx, adminIDStr)

	return nil
}

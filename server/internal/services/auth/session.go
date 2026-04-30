package service

import (
	"context"
	"time"

	"server/internal/config"
	"server/internal/dto"
	"server/internal/redis"
	"server/internal/repository"
	"server/internal/security"
	"server/internal/utils"
)

// ─── Service ─────────────────────────────────────────────────────────────────

// SessionService owns RefreshTokens, Logout (single device), and LogoutAll.
type SessionService struct {
	userRepo    *repository.UserRepository
	sessionRepo *redis.SessionRepository
	jwtManager  *security.JWTManager
	cfg         *config.Config
}

// NewSessionService wires up dependencies.
func NewSessionService(
	userRepo *repository.UserRepository,
	sessionRepo *redis.SessionRepository,
	jwtManager *security.JWTManager,
	cfg *config.Config,
) *SessionService {
	return &SessionService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		jwtManager:  jwtManager,
		cfg:         cfg,
	}
}

// ─── Refresh Tokens ──────────────────────────────────────────────────────────

// RefreshTokens validates the old refresh token, rotates the session (revoke
// old → create new), and returns a fresh token pair.
//
// If the stored refresh token does not match the presented one, ALL sessions
// for the user are revoked (potential token-reuse / replay attack).
func (s *SessionService) RefreshTokens(
	ctx context.Context,
	refreshToken, ip, userAgent string,
) (*dto.AuthResponse, string, error) {

	// 1. Validate the refresh JWT
	claims, err := s.jwtManager.ValidateToken(refreshToken, security.Refresh)
	if err != nil {
		return nil, "", err // already an AppError
	}

	sessionID := claims.RegisteredClaims.ID

	// 2. Look up the session in Redis
	session, err := s.sessionRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, "", utils.Unauthorized("Session expired or invalid")
	}

	// 3. Verify stored refresh token matches (replay-attack detection)
	if session.RefreshToken != refreshToken {
		_ = s.sessionRepo.RevokeAllSessions(ctx, session.UserID)
		utils.Warn("Refresh token mismatch — possible replay attack, all sessions revoked", map[string]any{
			"user_id":    session.UserID,
			"session_id": sessionID,
			"ip":         ip,
		})
		return nil, "", utils.Unauthorized("Session has been invalidated for security reasons")
	}

	// 4. Fetch user from DB (ensure account is still active)
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, "", utils.Internal(err)
	}
	if user == nil {
		return nil, "", utils.Unauthorized("User account not found")
	}
	if !user.CanLogin() {
		return nil, "", utils.Forbidden("Your account is no longer active")
	}

	// 5. Revoke old session
	_ = s.sessionRepo.RevokeSession(ctx, session.UserID, sessionID)

	// 6. Generate fresh token pair
	tokenPair, err := s.jwtManager.GenerateTokenPair(user)
	if err != nil {
		return nil, "", utils.Internal(err)
	}

	// 7. Extract new JTI for the new session key
	newClaims, err := s.jwtManager.ValidateToken(tokenPair.RefreshToken, security.Refresh)
	if err != nil {
		return nil, "", utils.Internal(err)
	}

	// 8. Persist new session
	info := utils.ParseClientInfo(ip, userAgent)
	now := time.Now()
	newSession := &dto.SessionMetadata{
		ID:           newClaims.RegisteredClaims.ID,
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
	if err = s.sessionRepo.StoreSession(ctx, newSession, s.cfg.JWT.RefreshTokenDuration); err != nil {
		return nil, "", utils.Internal(err)
	}

	resp := &dto.AuthResponse{
		User:        dto.ToSafeUser(user),
		AccessToken: tokenPair.AccessToken,
		ExpiresAt:   tokenPair.ExpiresAt,
	}

	return resp, tokenPair.RefreshToken, nil
}

// ─── Logout (Single Device) ─────────────────────────────────────────────────

// Logout revokes the session associated with the presented refresh token
// (single-device logout). Safe to call even when the token is absent or
// already expired.
func (s *SessionService) Logout(
	ctx context.Context,
	refreshToken string,
) (*dto.MessageResponse, error) {

	if refreshToken != "" {
		claims, err := s.jwtManager.ValidateToken(refreshToken, security.Refresh)
		if err == nil {
			_ = s.sessionRepo.RevokeSession(
				ctx,
				claims.UserID.String(),
				claims.RegisteredClaims.ID,
			)
		}
	}

	return &dto.MessageResponse{Message: "Logged out successfully"}, nil
}

// ─── Logout All ──────────────────────────────────────────────────────────────

// LogoutAll revokes every active session for the user (all-device logout).
// Requires a valid access token (AuthRequired middleware).
func (s *SessionService) LogoutAll(
	ctx context.Context,
	userID string,
) (*dto.MessageResponse, error) {

	if err := s.sessionRepo.RevokeAllSessions(ctx, userID); err != nil {
		utils.Error("Failed to revoke all sessions", err, map[string]any{
			"user_id": userID,
		})
	}

	return &dto.MessageResponse{Message: "Logged out from all devices successfully"}, nil
}

// ─── List & Revoke Specific ──────────────────────────────────────────────────

// GetActiveSessions returns a sanitized list of all sessions for the user.
// currentSessionID is used to mark the 'is_current' flag in the response.
func (s *SessionService) GetActiveSessions(
	ctx context.Context,
	userID string,
	currentSessionID string,
) ([]dto.SafeSession, error) {

	metas, err := s.sessionRepo.GetActiveSessions(ctx, userID)
	if err != nil {
		return nil, err
	}

	safeSessions := make([]dto.SafeSession, 0, len(metas))
	for _, m := range metas {
		safeSessions = append(safeSessions, dto.SafeSession{
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

	return safeSessions, nil
}

// RevokeSession revokes a specific session after verifying ownership.
func (s *SessionService) RevokeSession(
	ctx context.Context,
	userID string,
	sessionID string,
) (*dto.MessageResponse, error) {

	// 1. Fetch the session to verify ownership
	session, err := s.sessionRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if session.UserID != userID {
		return nil, utils.Forbidden("You do not have permission to revoke this session")
	}

	// 2. Revoke
	if err = s.sessionRepo.RevokeSession(ctx, userID, sessionID); err != nil {
		return nil, err
	}

	return &dto.MessageResponse{Message: "Session revoked successfully"}, nil
}
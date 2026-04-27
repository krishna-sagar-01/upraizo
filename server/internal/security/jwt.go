package security

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"server/internal/config"
	"server/internal/models"
	"server/internal/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType defines the purpose of the token
type TokenType string

const (
	Access  TokenType = "access"
	Refresh TokenType = "refresh"
)

// CustomClaims represents the payload of our JWT
type CustomClaims struct {
	UserID uuid.UUID         `json:"uid"`
	Email  string            `json:"eml"`
	Status models.UserStatus `json:"st"`
	Type   TokenType         `json:"typ"`
	jwt.RegisteredClaims
}

// TokenPair holds both access and refresh tokens
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

type JWTManager struct {
	cfg         *config.JWTConfig
	secretBytes []byte
}

func NewJWTManager(cfg *config.JWTConfig) *JWTManager {
	return &JWTManager{
		cfg:         cfg,
		secretBytes: []byte(cfg.Secret),
	}
}

// ─── Token Generation ────────────────────────────────────────────────────────

// GenerateTokenPair creates both Access and Refresh tokens for a user
func (m *JWTManager) GenerateTokenPair(user *models.User) (*TokenPair, error) {
	sessionID := uuid.New().String()

	// Create Access Token
	accessToken, err := m.generateToken(user, Access, m.cfg.AccessTokenDuration, sessionID)
	if err != nil {
		return nil, err
	}

	// Create Refresh Token
	refreshToken, err := m.generateToken(user, Refresh, m.cfg.RefreshTokenDuration, sessionID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(m.cfg.AccessTokenDuration).Unix(),
	}, nil
}

func (m *JWTManager) generateToken(user *models.User, tokenType TokenType, duration time.Duration, sessionID string) (string, error) {
	now := time.Now()
	
	claims := CustomClaims{
		UserID: user.ID,
		Email:  user.Email,
		Status: user.Status,
		Type:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.cfg.Issuer,
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			NotBefore: jwt.NewNumericDate(now),
			ID:        sessionID, // Shared JTI across both tokens
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(m.secretBytes)
	if err != nil {
		return "", utils.Internal(err)
	}

	return signedToken, nil
}

// ─── Token Validation ────────────────────────────────────────────────────────

// ValidateToken parses and validates a token string against its expected type
func (m *JWTManager) ValidateToken(tokenStr string, expectedType TokenType) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
		// Only allow HS256 algorithm
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secretBytes, nil
	})

	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, utils.Unauthorized("Token has expired")
		case errors.Is(err, jwt.ErrTokenSignatureInvalid):
			return nil, utils.Unauthorized("Invalid token signature")
		default:
			return nil, utils.Unauthorized("Invalid or malformed token")
		}
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, utils.Unauthorized("Invalid token claims")
	}

	// Safety Checks
	if claims.Type != expectedType {
		return nil, utils.Unauthorized(fmt.Sprintf("Invalid token type: expected %s", expectedType))
	}

	if claims.Issuer != m.cfg.Issuer {
		return nil, utils.Unauthorized("Invalid token issuer")
	}

	// Status Check (Don't allow banned users even with a valid token)
	if claims.Status == models.UserStatusBanned {
		return nil, utils.Forbidden("Your account has been banned")
	}

	return claims, nil
}

// ───────────────── Helpers ─────────────────

// ExtractToken removes "Bearer " prefix from the Authorization header
func ExtractToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", utils.Unauthorized("Missing authorization header")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", utils.Unauthorized("Invalid authorization format (use Bearer <token>)")
	}

	return parts[1], nil
}
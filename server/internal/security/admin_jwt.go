package security

import (
	"errors"
	"fmt"
	"time"

	"server/internal/config"
	"server/internal/models"
	"server/internal/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AdminCustomClaims represents the payload of an Admin JWT
type AdminCustomClaims struct {
	AdminID uuid.UUID `json:"aid"`
	Email   string    `json:"eml"`
	Role    string    `json:"rol"` // Always "admin"
	Type    TokenType `json:"typ"`
	jwt.RegisteredClaims
}

type AdminJWTManager struct {
	cfg         *config.JWTConfig
	secretBytes []byte
}

func NewAdminJWTManager(cfg *config.JWTConfig) *AdminJWTManager {
	return &AdminJWTManager{
		cfg:         cfg,
		secretBytes: []byte(cfg.Secret),
	}
}

// ─── Token Generation ────────────────────────────────────────────────────────

func (m *AdminJWTManager) GenerateTokenPair(admin *models.Admin) (*TokenPair, error) {
	sessionID := uuid.New().String()

	// Create Access Token
	accessToken, err := m.generateToken(admin, Access, m.cfg.AccessTokenDuration, sessionID)
	if err != nil {
		return nil, err
	}

	// Create Refresh Token
	refreshToken, err := m.generateToken(admin, Refresh, m.cfg.RefreshTokenDuration, sessionID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(m.cfg.AccessTokenDuration).Unix(),
	}, nil
}

func (m *AdminJWTManager) generateToken(admin *models.Admin, tokenType TokenType, duration time.Duration, sessionID string) (string, error) {
	now := time.Now()
	
	claims := AdminCustomClaims{
		AdminID: admin.ID,
		Email:   admin.Email,
		Role:    "admin",
		Type:    tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.cfg.Issuer,
			Subject:   admin.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			NotBefore: jwt.NewNumericDate(now),
			ID:        sessionID,
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

func (m *AdminJWTManager) ValidateToken(tokenStr string, expectedType TokenType) (*AdminCustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &AdminCustomClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secretBytes, nil
	})

	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, utils.Unauthorized("Admin session has expired")
		case errors.Is(err, jwt.ErrTokenSignatureInvalid):
			return nil, utils.Unauthorized("Invalid admin token signature")
		default:
			return nil, utils.Unauthorized("Invalid or malformed admin token")
		}
	}

	claims, ok := token.Claims.(*AdminCustomClaims)
	if !ok || !token.Valid {
		return nil, utils.Unauthorized("Invalid admin token claims")
	}

	if claims.Type != expectedType {
		return nil, utils.Unauthorized(fmt.Sprintf("Invalid token type: expected %s", expectedType))
	}

	if claims.Issuer != m.cfg.Issuer {
		return nil, utils.Unauthorized("Invalid admin token issuer")
	}

	return claims, nil
}

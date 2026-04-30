package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"context"

	"server/internal/dto"
	"server/internal/models"
	"server/internal/security"
	"server/internal/utils"
)

// ─── Google Token Verification ───────────────────────────────────────────────

const googleTokenInfoURL = "https://oauth2.googleapis.com/tokeninfo?id_token="

var googleHTTPClient = &http.Client{Timeout: 10 * time.Second}

type googleUserInfo struct {
	Sub           string `json:"sub"`            // Google user ID (stable, unique)
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"` // "true" or "false" (string, not bool)
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Aud           string `json:"aud"` // must match our Client ID
}

// verifyGoogleIDToken calls Google's tokeninfo endpoint and verifies:
//   - HTTP 200 from Google (token is valid and not expired)
//   - aud matches our Client ID (prevents token substitution attacks)
//   - email_verified == "true"
//   - email and sub are present
func verifyGoogleIDToken(idToken, clientID string) (*googleUserInfo, error) {
	req, err := http.NewRequest(http.MethodGet, googleTokenInfoURL+idToken, nil)
	if err != nil {
		return nil, utils.Internal(err)
	}

	resp, err := googleHTTPClient.Do(req)
	if err != nil {
		utils.Error("Google tokeninfo request failed", err, nil)
		return nil, utils.Internal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, utils.Unauthorized("Invalid or expired Google token")
	}

	var info googleUserInfo
	if err = json.NewDecoder(resp.Body).Decode(&info); err != nil {
		utils.Error("Failed to decode Google tokeninfo response", err, nil)
		return nil, utils.Internal(err)
	}

	// ── Security checks ──────────────────────────────────────────────

	// Audience MUST match our client ID — prevents using tokens issued for
	// a different app from being accepted here.
	if info.Aud != clientID {
		return nil, utils.Unauthorized("Google token audience mismatch")
	}

	// Only accept verified Google emails.
	if info.EmailVerified != "true" {
		return nil, utils.BadRequest("Google account email is not verified")
	}

	if info.Email == "" || info.Sub == "" {
		return nil, utils.BadRequest("Incomplete Google account data")
	}

	return &info, nil
}

// ─── Google Auth (Sign-in / Sign-up) ─────────────────────────────────────────
func (s *AccountService) GoogleAuth(
	ctx context.Context,
	idToken, ip, userAgent string,
) (*dto.AuthResponse, string, error) {

	// 1. Verify the Google ID token
	googleUser, err := verifyGoogleIDToken(idToken, s.cfg.Google.ClientID)
	if err != nil {
		return nil, "", err
	}

	email := strings.ToLower(strings.TrimSpace(googleUser.Email))
	name := strings.TrimSpace(googleUser.Name)
	if name == "" {
		name = strings.Split(email, "@")[0] // sensible fallback
	}

	// 2. Look up by email
	existing, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", utils.Internal(err)
	}

	// ── Existing user ────────────────────────────────────────────────
	if existing != nil {
		// Guard: different provider → reject to avoid accidental linking
		if existing.AuthProvider != models.AuthProviderGoogle {
			return nil, "", utils.BadRequest(
				"An account with this email already exists using " +
					string(existing.AuthProvider) +
					" sign-in. Please use that method instead",
			)
		}

		if existing.IsBanned() {
			return nil, "", utils.Forbidden("Your account has been permanently banned")
		}
		if existing.IsSuspended() {
			return nil, "", utils.Forbidden("Your account is temporarily suspended")
		}

		// Existing Google user → straight login (no payment check needed)
		return s.createAuthSession(ctx, existing, ip, userAgent)
	}

	// ── New user (Google sign-up) ────────────────────────────────────
	now := time.Now()
	googleSub := googleUser.Sub

	user := &models.User{
		ID:             security.NEWID(),
		Name:           name,
		Email:          email,
		AvatarURL:      nilIfEmpty(googleUser.Picture),
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: &googleSub,
		Status:         models.UserStatusActive, // Google users are active immediately
		IsVerified:     true,                    // Google guarantees email verification
		VerifiedAt:     &now,
		Preferences:    models.DefaultPreferences(),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err = s.userRepo.Create(ctx, user); err != nil {
		return nil, "", utils.Internal(err)
	}

	return s.createAuthSession(ctx, user, ip, userAgent)
}

// nilIfEmpty returns a pointer to s if non-empty, else nil.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

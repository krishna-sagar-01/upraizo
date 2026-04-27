package service

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	_ "image/png"
	"os"
	"time"

	"server/internal/config"
	"server/internal/dto"
	"server/internal/queue"
	"server/internal/repository"
	"server/internal/storage"
	"server/internal/utils"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

type UserService struct {
	userRepo *repository.UserRepository
	queueMgr *queue.Manager
	r2       *storage.R2Client
	cfg      *config.Config
}

func NewUserService(userRepo *repository.UserRepository, queueMgr *queue.Manager, r2 *storage.R2Client, cfg *config.Config) *UserService {
	return &UserService{
		userRepo: userRepo,
		queueMgr: queueMgr,
		r2:       r2,
		cfg:      cfg,
	}
}

func (s *UserService) Config() *config.Config {
	return s.cfg
}

// GetProfile retrieves the sanitized user data by ID.
func (s *UserService) GetProfile(ctx context.Context, userID uuid.UUID) (dto.SafeUser, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return dto.SafeUser{}, utils.Internal(err)
	}

	if user == nil {
		return dto.SafeUser{}, utils.NotFound("User not found")
	}

	return dto.ToSafeUser(user), nil
}

// UpdateProfile handles partial updates to user profile (Name & Preferences).
func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, req *dto.UpdateProfileRequest) (dto.SafeUser, error) {
	// 1. Fetch current user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return dto.SafeUser{}, utils.Internal(err)
	}
	if user == nil {
		return dto.SafeUser{}, utils.NotFound("User not found")
	}

	// 2. Map updates (Deep Partial Merge)
	updated := false
	if req.Name != nil && *req.Name != user.Name {
		user.Name = *req.Name
		updated = true
	}

	if req.Theme != nil && *req.Theme != user.Preferences.Theme {
		user.Preferences.Theme = *req.Theme
		updated = true
	}

	if req.Notifications != nil {
		user.Preferences.Notifications = *req.Notifications
		updated = true
	}

	// 3. Persist if changed
	if updated {
		// Optimization: Update name only if it changed
		if req.Name != nil {
			if err := s.userRepo.Update(ctx, user); err != nil {
				return dto.SafeUser{}, utils.Internal(err)
			}
		}

		// Optimization: Update JSONB preferences if any part of it changed
		if req.Theme != nil || req.Notifications != nil {
			if err := s.userRepo.UpdatePreferences(ctx, userID, user.Preferences); err != nil {
				return dto.SafeUser{}, utils.Internal(err)
			}
		}
	}

	return dto.ToSafeUser(user), nil
}

// QueueAvatarUpdate sends a task to the background worker for processing.
func (s *UserService) QueueAvatarUpdate(ctx context.Context, userID uuid.UUID, localPath string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	oldURL := ""
	if user.AvatarURL != nil {
		oldURL = *user.AvatarURL
	}

	task := queue.AvatarTask{
		UserID:       userID.String(),
		LocalPath:    localPath,
		OldAvatarURL: oldURL,
	}

	return s.queueMgr.Publish(s.cfg.RabbitMQ.AvatarQueue, task)
}

// UpdateAvatarSync processes the image synchronously and updates the user's avatar.
func (s *UserService) UpdateAvatarSync(ctx context.Context, userID uuid.UUID, localPath string) (string, error) {
	// Cleanup local temp file ALWAYS at the end
	defer func() {
		if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
			utils.Warn("Failed to delete temp avatar file", map[string]any{"path": localPath, "error": err.Error()})
		}
	}()

	// 1. Fetch current user to get old avatar URL
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", utils.Internal(err)
	}
	if user == nil {
		return "", utils.NotFound("User not found")
	}

	oldAvatarURL := ""
	if user.AvatarURL != nil {
		oldAvatarURL = *user.AvatarURL
	}

	// 2. Load and process image
	img, err := imaging.Open(localPath)
	if err != nil {
		return "", utils.BadRequest("Failed to open avatar image")
	}

	// 3. Resize/Process (400x400 center crop)
	processed := imaging.Fill(img, 400, 400, imaging.Center, imaging.Lanczos)

	// 4. Encode to JPEG
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, processed, &jpeg.Options{Quality: 85}); err != nil {
		return "", utils.Internal(fmt.Errorf("failed to encode image: %w", err))
	}

	// 5. Upload to R2
	fileName := fmt.Sprintf("avatars/%s/%d.jpg", userID.String(), time.Now().Unix())
	publicURL, err := s.r2.Upload(fileName, buf.Bytes(), "image/jpeg")
	if err != nil {
		return "", utils.Internal(err)
	}

	// 6. Update DB
	user.AvatarURL = &publicURL
	if err := s.userRepo.Update(ctx, user); err != nil {
		return "", utils.Internal(err)
	}

	// 7. Delete OLD avatar from R2 (Asynchronously to not block response)
	if oldAvatarURL != "" {
		go func() {
			if err := s.r2.Delete(oldAvatarURL); err != nil {
				utils.Warn("Failed to delete old avatar from R2", map[string]any{"url": oldAvatarURL, "error": err.Error()})
			}
		}()
	}

	return publicURL, nil
}

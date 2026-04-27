package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"server/internal/dto"
	service "server/internal/services/user"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// GetProfile: GET /user/profile
func (h *UserHandler) GetProfile(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return utils.Unauthorized("Authentication required")
	}

	resp, err := h.userService.GetProfile(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// UpdateProfile: PUT /user/profile
func (h *UserHandler) UpdateProfile(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return utils.Unauthorized("Authentication required")
	}

	var req dto.UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return err
	}

	resp, err := h.userService.UpdateProfile(c.Context(), userID, &req)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// UploadAvatar: POST /user/avatar
func (h *UserHandler) UploadAvatar(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return utils.Unauthorized("Authentication required")
	}

	// 1. Parse File
	file, err := c.FormFile("avatar")
	if err != nil {
		return utils.BadRequest("Avatar file is required")
	}

	// 2. Validate Size (Max 1MB)
	if file.Size > 1*1024*1024 {
		return utils.BadRequest("File size too large (Max 1MB)")
	}

	// 3. Validate Type
	ext := strings.ToLower(filepath.Ext(file.Filename))
	validExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	if !validExts[ext] {
		return utils.BadRequest("Invalid file type. Supported: JPG, PNG, WEBP")
	}

	// 4. Save locally temporarily
	tempName := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	localPath := filepath.Join(h.userService.Config().App.AvatarTempPath(), tempName)

	if err := c.SaveFile(file, localPath); err != nil {
		utils.Error("Failed to save temp avatar", err, nil)
		return utils.Internal(err)
	}

	// 5. Process Synchronously
	avatarURL, err := h.userService.UpdateAvatarSync(c.Context(), userID, localPath)
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success":    true,
		"message":    "Avatar updated successfully",
		"avatar_url": avatarURL,
	})
}

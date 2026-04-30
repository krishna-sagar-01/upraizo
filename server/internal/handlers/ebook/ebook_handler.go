package handler

import (
	"server/internal/dto"
	service "server/internal/services/ebook"
	"server/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type EbookHandler struct {
	service *service.EbookService
}

func NewEbookHandler(service *service.EbookService) *EbookHandler {
	return &EbookHandler{service: service}
}

// ── Admin Handlers ──

func (h *EbookHandler) Create(c *fiber.Ctx) error {
	var req dto.CreateEbookRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	ebook, err := h.service.CreateEbook(c.Context(), req)
	if err != nil {
		return utils.Internal(err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    ebook,
	})
}

func (h *EbookHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest("Invalid ebook ID")
	}

	var req dto.UpdateEbookRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest("Invalid request body")
	}

	ebook, err := h.service.UpdateEbook(c.Context(), id, req)
	if err != nil {
		return utils.Internal(err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    ebook,
	})
}

func (h *EbookHandler) UploadFile(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest("Invalid ebook ID")
	}

	file, err := c.FormFile("file")
	if err != nil {
		return utils.BadRequest("File is required")
	}

	fileContent, err := file.Open()
	if err != nil {
		return utils.Internal(err)
	}
	defer fileContent.Close()

	url, err := h.service.UploadEbookFile(c.Context(), id, fileContent, file.Filename, file.Header.Get("Content-Type"))
	if err != nil {
		return utils.Internal(err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"url":     url,
	})
}

func (h *EbookHandler) UploadThumbnail(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest("Invalid ebook ID")
	}

	file, err := c.FormFile("thumbnail")
	if err != nil {
		return utils.BadRequest("Thumbnail is required")
	}

	fileContent, err := file.Open()
	if err != nil {
		return utils.Internal(err)
	}
	defer fileContent.Close()

	url, err := h.service.UploadThumbnail(c.Context(), id, fileContent, file.Filename, file.Header.Get("Content-Type"))
	if err != nil {
		return utils.Internal(err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"url":     url,
	})
}

func (h *EbookHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest("Invalid ebook ID")
	}

	if err := h.service.DeleteEbook(c.Context(), id); err != nil {
		return utils.Internal(err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Ebook deleted successfully",
	})
}

// ── Public Handlers ──

func (h *EbookHandler) List(c *fiber.Ctx) error {
	onlyPublished := true
	ebooks, err := h.service.ListEbooks(c.Context(), onlyPublished)
	if err != nil {
		return utils.Internal(err)
	}

	// Filter for public view
	publicEbooks := make([]dto.PublicEbookResponse, len(ebooks))
	for i, e := range ebooks {
		publicEbooks[i] = dto.PublicEbookResponse{
			ID:                e.ID,
			Title:             e.Title,
			Slug:              e.Slug,
			Description:       e.Description,
			ThumbnailURL:      e.ThumbnailURL,
			Price:             e.Price,
			OriginalPrice:     e.OriginalPrice,
			DiscountLabel:     e.DiscountLabel,
			DiscountExpiresAt: e.DiscountExpiresAt,
			IsPublished:       e.IsPublished,
			CreatedAt:         e.CreatedAt,
			UpdatedAt:         e.UpdatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    publicEbooks,
	})
}

func (h *EbookHandler) GetMyEbooks(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uuid.UUID)
	ebooks, err := h.service.GetPurchasedEbooks(c.Context(), userID)
	if err != nil {
		return utils.Internal(err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    ebooks,
	})
}

func (h *EbookHandler) AdminList(c *fiber.Ctx) error {
	ebooks, err := h.service.ListEbooks(c.Context(), false)
	if err != nil {
		return utils.Internal(err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    ebooks,
	})
}

func (h *EbookHandler) GetBySlug(c *fiber.Ctx) error {
	slug := c.Params("slug")
	ebook, err := h.service.GetEbookBySlug(c.Context(), slug)
	if err != nil {
		return utils.Internal(err)
	}
	if ebook == nil {
		return utils.NotFound("Ebook not found")
	}

	// Check if user has purchased this ebook
	var fileURL string
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if ok {
		hasPurchased, _ := h.service.HasPurchased(c.Context(), userID, ebook.ID)
		if hasPurchased {
			fileURL = ebook.FileURL
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":                  ebook.ID,
			"title":               ebook.Title,
			"slug":                ebook.Slug,
			"description":         ebook.Description,
			"thumbnail_url":       ebook.ThumbnailURL,
			"file_url":            fileURL,
			"price":               ebook.Price,
			"original_price":      ebook.OriginalPrice,
			"discount_label":      ebook.DiscountLabel,
			"discount_expires_at": ebook.DiscountExpiresAt,
			"is_published":        ebook.IsPublished,
			"created_at":          ebook.CreatedAt,
			"updated_at":          ebook.UpdatedAt,
		},
	})
}

func (h *EbookHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest("Invalid ebook ID")
	}

	ebook, err := h.service.GetEbookByID(c.Context(), id)
	if err != nil {
		return utils.Internal(err)
	}
	if ebook == nil {
		return utils.NotFound("Ebook not found")
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    ebook,
	})
}

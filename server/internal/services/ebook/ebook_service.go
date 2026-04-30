package service

import (
	"context"
	"fmt"
	"io"
	"server/internal/dto"
	"server/internal/models"
	"server/internal/repository"
	"server/internal/storage"
	"server/internal/utils"

	"strings"

	"github.com/google/uuid"
)

type EbookService struct {
	repo         *repository.EbookRepository
	purchaseRepo *repository.PurchaseRepository
	r2Client     *storage.R2Client
}

func NewEbookService(
	repo *repository.EbookRepository, 
	purchaseRepo *repository.PurchaseRepository,
	r2Client *storage.R2Client,
) *EbookService {
	return &EbookService{
		repo:         repo,
		purchaseRepo: purchaseRepo,
		r2Client:     r2Client,
	}
}

func (s *EbookService) CreateEbook(ctx context.Context, req dto.CreateEbookRequest) (*models.Ebook, error) {
	slug := utils.Slugify(req.Title)
	
	// Check if slug exists
	existing, _ := s.repo.GetBySlug(ctx, slug)
	if existing != nil {
		slug = fmt.Sprintf("%s-%s", slug, uuid.New().String()[:8])
	}

	ebook := &models.Ebook{
		ID:                uuid.New(),
		Title:             req.Title,
		Slug:              slug,
		Description:       req.Description,
		Price:             req.Price,
		OriginalPrice:     req.OriginalPrice,
		DiscountLabel:     req.DiscountLabel,
		DiscountExpiresAt: req.DiscountExpiresAt,
		IsPublished:       req.IsPublished,
	}

	// file_url is mandatory but will be updated after upload. 
	// For now, setting a placeholder or leaving it empty if the table allows (it doesn't).
	// Usually, we upload first, then create the record. Or create with empty and update.
	ebook.FileURL = "pending_upload"

	err := s.repo.Create(ctx, ebook)
	if err != nil {
		return nil, err
	}

	return ebook, nil
}

func (s *EbookService) GetEbookByID(ctx context.Context, id uuid.UUID) (*models.Ebook, error) {
	ebook, err := s.repo.GetByID(ctx, id)
	if err != nil || ebook == nil {
		return ebook, err
	}
	s.resolveURLs(ebook)
	return ebook, nil
}

func (s *EbookService) GetEbookBySlug(ctx context.Context, slug string) (*models.Ebook, error) {
	ebook, err := s.repo.GetBySlug(ctx, slug)
	if err != nil || ebook == nil {
		return ebook, err
	}
	s.resolveURLs(ebook)
	return ebook, nil
}

func (s *EbookService) ListEbooks(ctx context.Context, onlyPublished bool) ([]*models.Ebook, error) {
	ebooks, err := s.repo.List(ctx, onlyPublished)
	if err != nil {
		return nil, err
	}
	for _, e := range ebooks {
		s.resolveURLs(e)
	}
	return ebooks, nil
}

func (s *EbookService) GetPurchasedEbooks(ctx context.Context, userID uuid.UUID) ([]*models.Ebook, error) {
	ebooks, err := s.repo.GetPurchasedEbooks(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, e := range ebooks {
		s.resolveURLs(e)
	}
	return ebooks, nil
}

func (s *EbookService) HasPurchased(ctx context.Context, userID uuid.UUID, ebookID uuid.UUID) (bool, error) {
	return s.purchaseRepo.HasPurchasedEbook(ctx, userID, ebookID)
}

func (s *EbookService) UpdateEbook(ctx context.Context, id uuid.UUID, req dto.UpdateEbookRequest) (*models.Ebook, error) {
	ebook, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ebook == nil {
		return nil, fmt.Errorf("ebook not found")
	}

	if req.Title != nil {
		ebook.Title = *req.Title
		ebook.Slug = utils.Slugify(*req.Title)
	}
	if req.Description != nil {
		ebook.Description = req.Description
	}
	if req.Price != nil {
		ebook.Price = *req.Price
	}
	if req.OriginalPrice != nil {
		ebook.OriginalPrice = req.OriginalPrice
	}
	if req.DiscountLabel != nil {
		ebook.DiscountLabel = req.DiscountLabel
	}
	if req.DiscountExpiresAt != nil {
		ebook.DiscountExpiresAt = req.DiscountExpiresAt
	}
	if req.IsPublished != nil {
		ebook.IsPublished = *req.IsPublished
	}

	err = s.repo.Update(ctx, ebook)
	if err != nil {
		return nil, err
	}

	return ebook, nil
}

func (s *EbookService) UploadEbookFile(ctx context.Context, id uuid.UUID, file io.ReadSeeker, fileName string, contentType string) (string, error) {
	key := fmt.Sprintf("ebooks/%s/%s", id.String(), fileName)
	url, err := s.r2Client.UploadStream(key, file, contentType)
	if err != nil {
		return "", err
	}

	ebook, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if ebook == nil {
		return "", fmt.Errorf("ebook not found")
	}

	ebook.FileURL = url
	err = s.repo.Update(ctx, ebook)
	if err != nil {
		return "", err
	}

	return url, nil
}

func (s *EbookService) UploadThumbnail(ctx context.Context, id uuid.UUID, file io.ReadSeeker, fileName string, contentType string) (string, error) {
	key := fmt.Sprintf("ebooks/%s/thumbnails/%s", id.String(), fileName)
	url, err := s.r2Client.UploadStream(key, file, contentType)
	if err != nil {
		return "", err
	}

	ebook, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if ebook == nil {
		return "", fmt.Errorf("ebook not found")
	}

	ebook.ThumbnailURL = &url
	err = s.repo.Update(ctx, ebook)
	if err != nil {
		return "", err
	}

	return url, nil
}

func (s *EbookService) DeleteEbook(ctx context.Context, id uuid.UUID) error {
	ebook, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ebook == nil {
		return fmt.Errorf("ebook not found")
	}

	// Delete from R2
	if ebook.FileURL != "" && ebook.FileURL != "pending_upload" {
		s.r2Client.Delete(ebook.FileURL)
	}
	if ebook.ThumbnailURL != nil {
		s.r2Client.Delete(*ebook.ThumbnailURL)
	}

	return s.repo.Delete(ctx, id)
}

func (s *EbookService) resolveURLs(e *models.Ebook) {
	if e == nil {
		return
	}

	// 1. Resolve Thumbnail (Public)
	if e.ThumbnailURL != nil && *e.ThumbnailURL != "" && !strings.HasPrefix(*e.ThumbnailURL, "http") {
		cdn := strings.TrimSuffix(s.r2Client.CDN, "/")
		resolved := cdn + "/" + strings.TrimPrefix(*e.ThumbnailURL, "/")
		e.ThumbnailURL = &resolved
	}

	// 2. Resolve FileURL (Safe Public Link)
	if e.FileURL != "" && e.FileURL != "pending_upload" && !strings.HasPrefix(e.FileURL, "http") {
		cdn := strings.TrimSuffix(s.r2Client.CDN, "/")
		// Ensure FileURL starts with a slash
		path := e.FileURL
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		e.FileURL = cdn + path
	}
}

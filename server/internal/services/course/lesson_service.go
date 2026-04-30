package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"server/internal/config"
	"server/internal/dto"
	"server/internal/models"
	"server/internal/queue"
	"server/internal/repository"
	"server/internal/storage"
	"server/internal/utils"
	"time"

	"github.com/google/uuid"
)

type LessonService struct {
	repo           *repository.LessonRepository
	attachmentRepo *repository.AttachmentRepository
	queueManager   *queue.Manager
	r2             *storage.R2Client
	cfg            *config.Config
}

func NewLessonService(
	repo *repository.LessonRepository,
	attachmentRepo *repository.AttachmentRepository,
	queueManager *queue.Manager,
	r2 *storage.R2Client,
	cfg *config.Config,
) *LessonService {
	return &LessonService{
		repo:           repo,
		attachmentRepo: attachmentRepo,
		queueManager:   queueManager,
		r2:             r2,
		cfg:            cfg,
	}
}

// ───────────────── CRUD ─────────────────

func (s *LessonService) Create(ctx context.Context, req dto.CreateLessonRequest) (dto.LessonResponse, error) {
	lesson := &models.Lesson{
		ModuleID:    req.ModuleID,
		Title:       req.Title,
		OrderIndex:  req.OrderIndex,
		VideoStatus: models.VideoStatusPending,
		IsPreview:   req.IsPreview,
	}

	if err := s.repo.Create(ctx, lesson); err != nil {
		return dto.LessonResponse{}, utils.Internal(err)
	}

	return dto.ToLessonResponse(lesson, nil, nil), nil
}

func (s *LessonService) GetByID(ctx context.Context, id uuid.UUID) (dto.LessonResponse, error) {
	l, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return dto.LessonResponse{}, utils.Internal(err)
	}
	if l == nil {
		return dto.LessonResponse{}, utils.NotFound("Lesson not found")
	}

	attachments, _ := s.attachmentRepo.GetByLessonID(ctx, id)
	
	var videoURL *string
	if l.VideoKey != nil {
		url := fmt.Sprintf("%s/%s", s.cfg.R2.PublicURL, *l.VideoKey)
		videoURL = &url
	}

	return dto.ToLessonResponse(l, videoURL, attachments), nil
}

func (s *LessonService) ListByModule(ctx context.Context, moduleID uuid.UUID) ([]dto.LessonResponse, error) {
	lessons, err := s.repo.GetByModuleID(ctx, moduleID)
	if err != nil {
		return nil, utils.Internal(err)
	}

	res := make([]dto.LessonResponse, 0, len(lessons))
	for _, l := range lessons {
		attachments, _ := s.attachmentRepo.GetByLessonID(ctx, l.ID)
		var videoURL *string
		if l.VideoKey != nil {
			url := fmt.Sprintf("%s/%s", s.cfg.R2.PublicURL, *l.VideoKey)
			videoURL = &url
		}
		res = append(res, dto.ToLessonResponse(l, videoURL, attachments))
	}
	return res, nil
}

func (s *LessonService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateLessonRequest) (dto.LessonResponse, error) {
	l, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return dto.LessonResponse{}, utils.Internal(err)
	}
	if l == nil {
		return dto.LessonResponse{}, utils.NotFound("Lesson not found")
	}

	if req.Title != nil { l.Title = *req.Title }
	if req.OrderIndex != nil { l.OrderIndex = *req.OrderIndex }
	if req.IsPreview != nil { l.IsPreview = *req.IsPreview }

	l.UpdatedAt = time.Now()

	if err := s.repo.UpdateMetadata(ctx, l); err != nil {
		return dto.LessonResponse{}, utils.Internal(err)
	}

	return s.GetByID(ctx, id)
}

func (s *LessonService) Delete(ctx context.Context, id uuid.UUID) error {
	l, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return utils.Internal(err)
	}

	// 1. Cleanup R2 Video
	if l.VideoKey != nil {
		_ = s.r2.Delete(*l.VideoKey)
	}

	// 2. Cleanup R2 Attachments
	attachments, _ := s.attachmentRepo.GetByLessonID(ctx, id)
	for _, a := range attachments {
		_ = s.r2.Delete(a.FileKey)
	}

	// 3. Delete from DB
	return s.repo.PermanentDelete(ctx, id)
}

// ───────────────── VIDEO HANDLING ─────────────────

func (s *LessonService) GetVideoUploadURL(ctx context.Context, lessonID uuid.UUID, filename string, contentType string) (string, string, error) {
	// 1. Check lesson existence
	l, err := s.repo.GetByID(ctx, lessonID)
	if err != nil {
		return "", "", utils.Internal(err)
	}
	if l == nil {
		return "", "", utils.NotFound("Lesson not found")
	}

	// 2. Generate unique key
	videoKey := fmt.Sprintf("lessons/videos/%s%s", uuid.New().String(), filepath.Ext(filename))
	
	// 3. Get Presigned URL
	url, err := s.r2.GetPresignedURL(videoKey, contentType)
	if err != nil {
		return "", "", utils.Internal(err)
	}

	return url, videoKey, nil
}

func (s *LessonService) UploadVideo(ctx context.Context, lessonID uuid.UUID, fileReader io.ReadSeeker, filename string, contentType string, duration int) error {
	// 1. Check lesson existence
	l, err := s.repo.GetByID(ctx, lessonID)
	if err != nil {
		return utils.Internal(err)
	}
	if l == nil {
		return utils.NotFound("Lesson not found")
	}

	// 2. Generate unique key
	videoKey := fmt.Sprintf("lessons/videos/%s%s", uuid.New().String(), filepath.Ext(filename))

	// 3. Upload directly to R2 from server
	_, err = s.r2.UploadStream(videoKey, fileReader, contentType)
	if err != nil {
		return utils.Internal(err)
	}

	// 4. Update DB status to ready
	if err := s.repo.UpdateVideoStatus(ctx, lessonID, &videoKey, models.VideoStatusReady, &duration); err != nil {
		return utils.Internal(err)
	}

	// 5. Cleanup OLD video from R2 if exists
	if l.VideoKey != nil && *l.VideoKey != videoKey {
		_ = s.r2.Delete(*l.VideoKey)
	}

	return nil
}

func (s *LessonService) CompleteVideoUpload(ctx context.Context, lessonID uuid.UUID, videoKey string, duration int) error {
	// 1. Check lesson existence
	l, err := s.repo.GetByID(ctx, lessonID)
	if err != nil {
		return utils.Internal(err)
	}
	if l == nil {
		return utils.NotFound("Lesson not found")
	}

	// 2. Update DB status to ready
	if err := s.repo.UpdateVideoStatus(ctx, lessonID, &videoKey, models.VideoStatusReady, &duration); err != nil {
		return utils.Internal(err)
	}

	// 3. Cleanup OLD video from R2 if exists
	if l.VideoKey != nil && *l.VideoKey != videoKey {
		_ = s.r2.Delete(*l.VideoKey)
	}

	return nil
}


// ───────────────── ATTACHMENTS ─────────────────

func (s *LessonService) AddAttachment(ctx context.Context, lessonID uuid.UUID, title string, fileReader io.Reader, filename string, contentType string, size int64) (dto.AttachmentDTO, error) {
	// 1. Upload to R2 (under attachments folder)
	fileKey := fmt.Sprintf("lessons/attachments/%s/%s", lessonID.String(), filename)
	
	// Read full body for R2 Upload (current R2Client implementation needs []byte)
	body, err := io.ReadAll(fileReader)
	if err != nil {
		return dto.AttachmentDTO{}, utils.Internal(err)
	}

	publicURL, err := s.r2.Upload(fileKey, body, contentType)
	if err != nil {
		return dto.AttachmentDTO{}, utils.Internal(err)
	}

	// 2. Save to DB
	attachment := &models.LessonAttachment{
		LessonID: lessonID,
		Title:    title,
		FileKey:  fileKey,
		FileSize: size,
		MimeType: contentType,
	}

	if err := s.attachmentRepo.Create(ctx, attachment); err != nil {
		// Cleanup R2 if DB fails
		_ = s.r2.Delete(fileKey)
		return dto.AttachmentDTO{}, utils.Internal(err)
	}

	return dto.AttachmentDTO{
		ID:        attachment.ID,
		Title:     attachment.Title,
		FileURL:   publicURL,
		FileSize:  attachment.FileSize,
		MimeType:  attachment.MimeType,
		CreatedAt: attachment.CreatedAt,
	}, nil
}

func (s *LessonService) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	a, err := s.attachmentRepo.GetByID(ctx, id)
	if err != nil {
		return utils.Internal(err)
	}
	if a == nil {
		return utils.NotFound("Attachment not found")
	}

	// Delete from R2
	_ = s.r2.Delete(a.FileKey)

	// Delete from DB
	return s.attachmentRepo.PermanentDelete(ctx, id)
}

package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"server/internal/config"
	"server/internal/dto"
	"server/internal/models"
	"server/internal/queue"
	"server/internal/redis"
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
	progressRepo   *redis.VideoProgressRepository
	cfg            *config.Config
}

func NewLessonService(
	repo *repository.LessonRepository,
	attachmentRepo *repository.AttachmentRepository,
	queueManager *queue.Manager,
	r2 *storage.R2Client,
	progressRepo *redis.VideoProgressRepository,
	cfg *config.Config,
) *LessonService {
	return &LessonService{
		repo:           repo,
		attachmentRepo: attachmentRepo,
		queueManager:   queueManager,
		r2:             r2,
		progressRepo:   progressRepo,
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

func (s *LessonService) UploadVideo(ctx context.Context, lessonID uuid.UUID, fileReader io.Reader, filename string) error {
	// 1. Check lesson existence
	l, err := s.repo.GetByID(ctx, lessonID)
	if err != nil {
		return utils.Internal(err)
	}
	if l == nil {
		return utils.NotFound("Lesson not found")
	}

	// 2. Save to local temp storage
	tempFileName := fmt.Sprintf("%s%s", uuid.New().String(), filepath.Ext(filename))
	tempFilePath := filepath.Join(s.cfg.App.VideoTempPath(), tempFileName)

	out, err := os.Create(tempFilePath)
	if err != nil {
		return utils.Internal(fmt.Errorf("failed to create temp file: %w", err))
	}
	defer out.Close()

	if _, err := io.Copy(out, fileReader); err != nil {
		return utils.Internal(fmt.Errorf("failed to save temp file: %w", err))
	}

	// 3. Update status to processing
	if err := s.repo.UpdateVideoStatus(ctx, lessonID, nil, models.VideoStatusProcessing, nil); err != nil {
		os.Remove(tempFilePath)
		return utils.Internal(err)
	}

	// 4. Publish to RabbitMQ
	originalStatus := l.VideoStatus
	oldVideoKey := l.VideoKey

	job := map[string]any{
		"lesson_id":      lessonID.String(),
		"temp_file_path": tempFilePath,
		"old_video_key":  oldVideoKey,
		"retry_count":    0,
	}

	if err := s.queueManager.Publish(s.cfg.RabbitMQ.VideoQueue, job); err != nil {
		// Rollback DB status if publish fails
		_ = s.repo.UpdateVideoStatus(ctx, lessonID, oldVideoKey, originalStatus, l.DurationSeconds)
		os.Remove(tempFilePath)
		return utils.Internal(err)
	}

	return nil
}

func (s *LessonService) GetVideoProgress(ctx context.Context, lessonID uuid.UUID) (*redis.VideoProgress, error) {
	// 1. Try fetching from Redis
	progress, err := s.progressRepo.GetProgress(ctx, lessonID)
	if err != nil {
		utils.Warn("Failed to fetch video progress from Redis", map[string]any{"lesson_id": lessonID, "error": err})
	}
	
	if progress != nil {
		return progress, nil
	}

	// 2. If not in Redis, check DB for status
	l, err := s.repo.GetByID(ctx, lessonID)
	if err != nil {
		return nil, utils.Internal(err)
	}
	if l == nil {
		return nil, utils.NotFound("Lesson not found")
	}

	// Return a progress-like object from DB status
	percentage := 0
	if l.VideoStatus == models.VideoStatusReady {
		percentage = 100
	}

	return &redis.VideoProgress{
		Percentage: percentage,
		Status:     string(l.VideoStatus),
		UpdatedAt:  l.UpdatedAt.Unix(),
	}, nil
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

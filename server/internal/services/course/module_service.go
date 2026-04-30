package service

import (
	"context"
	"server/internal/dto"
	"server/internal/models"
	"server/internal/repository"
	"server/internal/utils"

	"github.com/google/uuid"
)

type ModuleService struct {
	repo *repository.ModuleRepository
}

func NewModuleService(repo *repository.ModuleRepository) *ModuleService {
	return &ModuleService{repo: repo}
}

func (s *ModuleService) Create(ctx context.Context, req dto.CreateModuleRequest) (dto.ModuleResponse, error) {
	module := &models.Module{
		CourseID:   req.CourseID,
		Title:      req.Title,
		OrderIndex: req.OrderIndex,
	}

	if err := s.repo.Create(ctx, module); err != nil {
		return dto.ModuleResponse{}, utils.Internal(err)
	}

	return dto.ToModuleResponse(module), nil
}

func (s *ModuleService) GetByID(ctx context.Context, id uuid.UUID) (dto.ModuleResponse, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return dto.ModuleResponse{}, utils.Internal(err)
	}
	if m == nil {
		return dto.ModuleResponse{}, utils.NotFound("Module not found")
	}
	return dto.ToModuleResponse(m), nil
}

func (s *ModuleService) GetByCourseID(ctx context.Context, courseID uuid.UUID) ([]dto.ModuleResponse, error) {
	modules, err := s.repo.GetByCourseID(ctx, courseID)
	if err != nil {
		return nil, utils.Internal(err)
	}

	res := make([]dto.ModuleResponse, 0, len(modules))
	for _, m := range modules {
		res = append(res, dto.ToModuleResponse(m))
	}
	return res, nil
}

func (s *ModuleService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateModuleRequest) (dto.ModuleResponse, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return dto.ModuleResponse{}, utils.Internal(err)
	}
	if m == nil {
		return dto.ModuleResponse{}, utils.NotFound("Module not found")
	}

	if req.Title != nil {
		if err := s.repo.UpdateTitle(ctx, id, *req.Title); err != nil {
			return dto.ModuleResponse{}, utils.Internal(err)
		}
		m.Title = *req.Title
	}

	if req.OrderIndex != nil {
		if err := s.repo.UpdateOrder(ctx, id, *req.OrderIndex); err != nil {
			return dto.ModuleResponse{}, utils.Internal(err)
		}
		m.OrderIndex = *req.OrderIndex
	}

	return dto.ToModuleResponse(m), nil
}

func (s *ModuleService) Delete(ctx context.Context, id uuid.UUID, permanent bool) error {
	if permanent {
		return s.repo.PermanentDelete(ctx, id)
	}
	return s.repo.SoftDelete(ctx, id)
}

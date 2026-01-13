package application

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"aurora/internal/models"
	"aurora/internal/repository"
)

type LocationService struct {
	repo *repository.LocationRepository
}

func NewLocationService(repo *repository.LocationRepository) *LocationService {
	return &LocationService{repo: repo}
}

func (s *LocationService) Create(ctx context.Context, name, desc, tags, createdBy string) (*models.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, sql.ErrNoRows
	}
	if createdBy == "" {
		createdBy = "gm"
	}

	id, err := s.repo.Create(ctx, name, desc, tags, createdBy)
	if err != nil {
		return nil, err
	}

	if id == 0 {
		return s.repo.GetByName(ctx, name)
	}

	return &models.Location{
		ID:          id,
		Name:        name,
		Description: desc,
		Tags:        tags,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
	}, nil
}

func (s *LocationService) GetByName(ctx context.Context, name string) (*models.Location, error) {
	return s.repo.GetByName(ctx, name)
}

func (s *LocationService) GetByID(ctx context.Context, id int64) (*models.Location, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *LocationService) List(ctx context.Context, limit int) ([]models.Location, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.repo.List(ctx, limit)
}

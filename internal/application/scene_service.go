package application

import (
	"context"
	"database/sql"
	"fmt"

	"aurora/internal/models"
	"aurora/internal/repository"
)

type SceneService struct {
	repo *repository.SceneRepository
}

func NewSceneService(repo *repository.SceneRepository) *SceneService {
	return &SceneService{repo: repo}
}

func (s *SceneService) EnsureDefaultScene() error {
	return nil
}

func (s *SceneService) SetGMMode(ctx context.Context, sceneID int64, mode string) error {
	return s.repo.SetGMMode(ctx, sceneID, mode)
}

func (s *SceneService) GetOrCreateSceneForCharacter(ctx context.Context, charID int64) (models.Scene, error) {
	sc, err := s.repo.GetActiveForCharacter(ctx, charID)
	if err == nil {
		return *sc, nil
	}

	if err == sql.ErrNoRows {
		// Создаем новую
		_, createErr := s.repo.Create(ctx, charID, "Личное приключение", "Столица Авроры", "Начало пути.")
		if createErr != nil {
			return models.Scene{}, fmt.Errorf("failed to create scene: %w", createErr)
		}
		// Try again
		newSc, err := s.repo.GetActiveForCharacter(ctx, charID)
		if err != nil {
			return models.Scene{}, err
		}
		return *newSc, nil
	}
	return models.Scene{}, err
}

func (s *SceneService) AppendMessage(ctx context.Context, msg models.SceneMessage) error {
	return s.repo.AppendMessage(ctx, msg)
}

func (s *SceneService) GetLastMessagesSummary(ctx context.Context, sceneID int64, limit int) (string, error) {
	msgs, err := s.repo.GetMessages(ctx, sceneID, limit)
	if err != nil {
		return "", err
	}

	var lines []string
	for _, m := range msgs {
		prefix := "Игрок"
		if m.SenderType == "ai" {
			prefix = "Лапидарий"
		} else if m.SenderType == "system" {
			prefix = "Система"
		}
		lines = append(lines, prefix+": "+m.Content)
	}

	// Reverse and join
	result := ""
	for i := len(lines) - 1; i >= 0; i-- {
		result += lines[i] + "\n"
	}
	return result, nil
}

func (s *SceneService) UpdateSceneLocation(ctx context.Context, sceneID int64, locID sql.NullInt64, locName string) error {
	return s.repo.UpdateLocation(ctx, sceneID, locID, locName)
}

func (s *SceneService) GetMessageCount(ctx context.Context, sceneID int64) (int, error) {
	return s.repo.GetMessageCount(ctx, sceneID)
}

func (s *SceneService) UpdateSummary(ctx context.Context, sceneID int64, newSummary string) error {
	return s.repo.UpdateSummary(ctx, sceneID, newSummary)
}

func (s *SceneService) PruneMessages(ctx context.Context, sceneID int64, keep int) error {
	return s.repo.PruneMessages(ctx, sceneID, keep)
}

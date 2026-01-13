package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"aurora/internal/models"
	"aurora/internal/repository"
)

type CharacterService struct {
	repo *repository.CharacterRepository
}

func NewCharacterService(repo *repository.CharacterRepository) *CharacterService {
	return &CharacterService{repo: repo}
}

func (s *CharacterService) GetEffects(ctx context.Context, charID int64) ([]models.Effect, error) {
	return s.repo.GetEffects(ctx, charID)
}

func (s *CharacterService) GetOrCreateByVK(ctx context.Context, vkUserID int64) (*models.Character, error) {
	ch, err := s.repo.GetByVKID(ctx, vkUserID)
	if err == nil {
		effects, _ := s.repo.GetEffects(ctx, ch.ID)
		ch.Effects = effects
		return ch, nil
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	newChar := &models.Character{
		VKUserID:     vkUserID,
		Name:         "Безымянный",
		Status:       "жив",
		LocationName: "Столица Авроры",
		CombatPower:  10,
		CombatHealth: 100,
		Gold:         0,
		CreatedAt:    time.Now(),
	}

	id, err := s.repo.Create(ctx, newChar)
	if err != nil {
		return nil, err
	}
	newChar.ID = id
	return newChar, nil
}

func (s *CharacterService) UpdateCombatAndGold(ctx context.Context, ch *models.Character) error {
	return s.repo.Update(ctx, ch)
}

func (s *CharacterService) UpdateFromNormalizedForm(ctx context.Context, vkID int64, f *models.NormalizedCharacterForm) (*models.Character, error) {
	ch, err := s.GetOrCreateByVK(ctx, vkID)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(f.Name) != "" {
		ch.Name = strings.TrimSpace(f.Name)
	}
	if strings.TrimSpace(f.Gender) != "" {
		ch.Gender = strings.TrimSpace(f.Gender)
	}
	if strings.TrimSpace(f.Race) != "" {
		ch.Race = strings.TrimSpace(f.Race)
	}
	if strings.TrimSpace(f.Country) != "" {
		ch.Country = strings.TrimSpace(f.Country)
	}
	if len(f.Abilities) > 0 {
		ch.Abilities = strings.Join(f.Abilities, "; ")
	}
	if strings.TrimSpace(f.Bio) != "" {
		ch.Bio = strings.TrimSpace(f.Bio)
	}

	var traits []string
	if len(f.TraitsPos) > 0 {
		traits = append(traits, f.TraitsPos...)
	}
	if len(f.TraitsNeg) > 0 {
		traits = append(traits, f.TraitsNeg...)
	}
	if len(traits) > 0 {
		ch.Traits = strings.Join(traits, "; ")
	}

	if strings.TrimSpace(f.Motivation) != "" {
		ch.Goal = strings.TrimSpace(f.Motivation)
	}

	sheetJSON, _ := json.Marshal(f)
	ch.SheetJSON = string(sheetJSON)

	if err := s.repo.Update(ctx, ch); err != nil {
		return nil, err
	}

	return ch, nil
}

type Form struct {
	Name         string
	Race         string
	Traits       string
	Goal         string
	LocationName string
	Abilities    string
	Bio          string
}

func (s *CharacterService) UpdateFromForm(ctx context.Context, vkUserID int64, f Form) (*models.Character, error) {
	ch, err := s.GetOrCreateByVK(ctx, vkUserID)
	if err != nil {
		return nil, err
	}

	if f.Name != "" {
		ch.Name = f.Name
	}
	if f.Race != "" {
		ch.Race = f.Race
	}
	if f.Traits != "" {
		ch.Traits = f.Traits
	}
	if f.Goal != "" {
		ch.Goal = f.Goal
	}
	if f.LocationName != "" {
		ch.LocationName = f.LocationName
	}
	if f.Abilities != "" {
		ch.Abilities = f.Abilities
	}
	if f.Bio != "" {
		ch.Bio = f.Bio
	}

	if err := s.repo.Update(ctx, ch); err != nil {
		return nil, err
	}

	return ch, nil
}

func (s *CharacterService) TickTurn(ctx context.Context, charID int64) ([]string, error) {
	expired, err := s.repo.GetExpiringEffects(ctx, charID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.DecrementEffectDurations(ctx, charID); err != nil {
		return nil, err
	}
	if err := s.repo.DeleteExpiredEffects(ctx, charID); err != nil {
		return nil, err
	}

	return expired, nil
}

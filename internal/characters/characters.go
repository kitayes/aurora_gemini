package characters

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"aurora/internal/models"
)

type Form struct {
	Name         string
	Race         string
	Traits       string
	Goal         string
	LocationName string
	Abilities    string
	Bio          string
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) GetOrCreateByVK(ctx context.Context, vkUserID int64) (models.Character, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT
  id,
  vk_user_id,
  name,
  IFNULL(race, ''),
  IFNULL(class, ''),
  IFNULL(faction_id, 0),
  IFNULL(faction_name, ''),
  IFNULL(traits, ''),
  IFNULL(goal, ''),
  IFNULL(location_id, 0),
  IFNULL(location_name, ''),
  IFNULL(status, ''),
  IFNULL(abilities, ''),
  IFNULL(bio, ''),
  IFNULL(combat_power, 10),
  IFNULL(combat_health, 100),
  IFNULL(gold, 0),
  IFNULL(gender, ''),
  IFNULL(country, ''),
  IFNULL(sheet_json, ''),
  created_at
FROM characters
WHERE vk_user_id = ?
LIMIT 1
`, vkUserID)

	var ch models.Character
	err := row.Scan(
		&ch.ID,
		&ch.VKUserID,
		&ch.Name,
		&ch.Race,
		&ch.Class,
		&ch.FactionID,
		&ch.FactionName,
		&ch.Traits,
		&ch.Goal,
		&ch.LocationID,
		&ch.LocationName,
		&ch.Status,
		&ch.Abilities,
		&ch.Bio,
		&ch.CombatPower,
		&ch.CombatHealth,
		&ch.Gold,
		&ch.Gender,
		&ch.Country,
		&ch.SheetJSON,
		&ch.CreatedAt,
	)
	if err == sql.ErrNoRows {
		res, err := s.db.ExecContext(ctx, `
INSERT INTO characters (vk_user_id, name, status, location_name, combat_power, combat_health, gold)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
			vkUserID, "Безымянный", "жив", "Столица Авроры", 10, 100, 0,
		)
		if err != nil {
			return models.Character{}, err
		}
		id, _ := res.LastInsertId()
		return models.Character{
			ID:           id,
			VKUserID:     vkUserID,
			Name:         "Безымянный",
			Status:       "жив",
			LocationName: "Столица Авроры",
			CombatPower:  10,
			CombatHealth: 100,
			Gold:         0,
			CreatedAt:    time.Now(),
		}, nil
	} else if err != nil {
		return models.Character{}, err
	}

	return ch, nil
}

func (s *Service) UpdateCombatAndGold(ctx context.Context, ch models.Character) error {
	_, err := s.db.ExecContext(ctx, `UPDATE characters SET combat_health=?, gold=? WHERE id=?`,
		ch.CombatHealth, ch.Gold, ch.ID)
	return err
}

func (s *Service) UpdateFromNormalizedForm(ctx context.Context, vkID int64, f *models.NormalizedCharacterForm) (*models.Character, error) {

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

	// ⬇️ у тебя НЕТ repo, обновляем напрямую через db
	_, err = s.db.ExecContext(ctx, `
UPDATE characters
SET name = ?,
    gender = ?,
    race = ?,
    country = ?,
    traits = ?,
    goal = ?,
    abilities = ?,
    bio = ?,
    sheet_json = ?
WHERE id = ?`,
		ch.Name,
		ch.Gender,
		ch.Race,
		ch.Country,
		ch.Traits,
		ch.Goal,
		ch.Abilities,
		ch.Bio,
		ch.SheetJSON,
		ch.ID,
	)
	if err != nil {
		return nil, err
	}

	// Возвращаем pointer (как и объявлено)
	return &ch, nil
}

func (s *Service) UpdateFromForm(ctx context.Context, vkUserID int64, f Form) (models.Character, error) {
	ch, err := s.GetOrCreateByVK(ctx, vkUserID)
	if err != nil {
		return models.Character{}, err
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

	_, err = s.db.ExecContext(ctx, `
UPDATE characters
SET name = ?, race = ?, traits = ?, goal = ?, location_name = ?, abilities = ?, bio = ?
WHERE id = ?`,
		ch.Name, ch.Race, ch.Traits, ch.Goal, ch.LocationName, ch.Abilities, ch.Bio, ch.ID,
	)
	if err != nil {
		return models.Character{}, err
	}

	return ch, nil
}

package repository

import (
	"context"
	"database/sql"
	"time"

	"aurora/internal/models"
)

type CharacterRepository struct {
	db *sql.DB
}

func NewCharacterRepository(db *sql.DB) *CharacterRepository {
	return &CharacterRepository{db: db}
}

// GetEffects загружает список эффектов персонажа из БД
func (r *CharacterRepository) GetEffects(ctx context.Context, charID int64) ([]models.Effect, error) {
	query := `SELECT id, character_id, name, description, duration_turns, is_hidden 
              FROM character_effects WHERE character_id = ?`

	rows, err := r.db.QueryContext(ctx, query, charID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var effects []models.Effect
	for rows.Next() {
		var e models.Effect
		if err := rows.Scan(&e.ID, &e.CharacterID, &e.Name, &e.Description, &e.Duration, &e.IsHidden); err != nil {
			continue
		}
		effects = append(effects, e)
	}
	return effects, nil
}

func (r *CharacterRepository) GetByVKID(ctx context.Context, vkUserID int64) (*models.Character, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
  id, vk_user_id, name, IFNULL(race, ''), IFNULL(class, ''), IFNULL(faction_id, 0), IFNULL(faction_name, ''),
  IFNULL(traits, ''), IFNULL(goal, ''), IFNULL(location_id, 0), IFNULL(location_name, ''),
  IFNULL(status, ''), IFNULL(abilities, ''), IFNULL(bio, ''), IFNULL(combat_power, 10),
  IFNULL(combat_health, 100), IFNULL(gold, 0), IFNULL(gender, ''), IFNULL(country, ''), IFNULL(sheet_json, ''), created_at
FROM characters WHERE vk_user_id = ? LIMIT 1`, vkUserID)

	var ch models.Character
	err := row.Scan(
		&ch.ID, &ch.VKUserID, &ch.Name, &ch.Race, &ch.Class, &ch.FactionID, &ch.FactionName,
		&ch.Traits, &ch.Goal, &ch.LocationID, &ch.LocationName, &ch.Status, &ch.Abilities,
		&ch.Bio, &ch.CombatPower, &ch.CombatHealth, &ch.Gold, &ch.Gender, &ch.Country, &ch.SheetJSON, &ch.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (r *CharacterRepository) Create(ctx context.Context, apiChar *models.Character) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
INSERT INTO characters (vk_user_id, name, status, location_name, combat_power, combat_health, gold, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		apiChar.VKUserID, apiChar.Name, apiChar.Status, apiChar.LocationName, apiChar.CombatPower, apiChar.CombatHealth, apiChar.Gold, time.Now(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *CharacterRepository) Update(ctx context.Context, ch *models.Character) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE characters
SET name=?, gender=?, race=?, country=?, traits=?, goal=?, abilities=?, bio=?, sheet_json=?, 
    location_name=?, combat_health=?, gold=?
WHERE id=?`,
		ch.Name, ch.Gender, ch.Race, ch.Country, ch.Traits, ch.Goal, ch.Abilities, ch.Bio, ch.SheetJSON,
		ch.LocationName, ch.CombatHealth, ch.Gold, ch.ID,
	)
	return err
}

func (r *CharacterRepository) DecrementEffectDurations(ctx context.Context, charID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE character_effects
		SET duration_turns = duration_turns - 1
		WHERE character_id = ? AND duration_turns > 0
	`, charID)
	return err
}

func (r *CharacterRepository) DeleteExpiredEffects(ctx context.Context, charID int64) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM character_effects
		WHERE character_id = ? AND duration_turns = 0
	`, charID)
	return err
}

func (r *CharacterRepository) GetExpiringEffects(ctx context.Context, charID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT name FROM character_effects 
		WHERE character_id = ? AND duration_turns = 1`, charID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			names = append(names, name)
		}
	}
	return names, nil
}

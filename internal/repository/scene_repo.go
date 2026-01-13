package repository

import (
	"context"
	"database/sql"

	"aurora/internal/models"
)

type SceneRepository struct {
	db *sql.DB
}

func NewSceneRepository(db *sql.DB) *SceneRepository {
	return &SceneRepository{db: db}
}

func (r *SceneRepository) SetGMMode(ctx context.Context, sceneID int64, mode string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE scenes SET gm_mode=? WHERE id=?`, mode, sceneID)
	return err
}

func (r *SceneRepository) GetActiveForCharacter(ctx context.Context, charID int64) (*models.Scene, error) {
	query := `
		SELECT
		  id,
		  IFNULL(location_id, 0),
		  IFNULL(location_name, 'Неизвестно'),
		  IFNULL(name, 'Личное приключение'),
		  CASE WHEN gm_mode = 'ai_assist' OR gm_mode = '0' THEN 0 ELSE 1 END,
		  IFNULL(summary, ''),
		  IFNULL(is_active, 1),
		  created_at
		FROM scenes
		WHERE is_active = 1 AND character_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`
	var sc models.Scene
	err := r.db.QueryRowContext(ctx, query, charID).Scan(
		&sc.ID,
		&sc.LocationID,
		&sc.LocationName,
		&sc.Name,
		&sc.GMMode,
		&sc.Summary,
		&sc.IsActive,
		&sc.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	sc.Status = "active"
	sc.CharacterID = charID
	return &sc, nil
}

func (r *SceneRepository) Create(ctx context.Context, charID int64, name, locName, summary string) (int64, error) {
	insertQuery := `
			INSERT INTO scenes (character_id, name, location_name, gm_mode, is_active, summary) 
			VALUES (?, ?, ?, '0', 1, ?)
		`
	res, err := r.db.ExecContext(ctx, insertQuery, charID, name, locName, summary)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *SceneRepository) AppendMessage(ctx context.Context, msg models.SceneMessage) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO scene_messages (scene_id, sender_type, sender_id, content, created_at)
VALUES (?, ?, ?, ?, ?)
`, msg.SceneID, msg.SenderType, msg.SenderID, msg.Content, msg.CreatedAt)
	return err
}

func (r *SceneRepository) GetMessages(ctx context.Context, sceneID int64, limit int) ([]models.SceneMessage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sender_type, content 
		FROM scene_messages
		WHERE scene_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, sceneID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []models.SceneMessage
	for rows.Next() {
		var m models.SceneMessage
		if err := rows.Scan(&m.SenderType, &m.Content); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (r *SceneRepository) UpdateLocation(ctx context.Context, sceneID int64, locID sql.NullInt64, locName string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE scenes 
		SET location_id = ?, location_name = ?
		WHERE id = ?
	`, locID, locName, sceneID)
	return err
}

func (r *SceneRepository) GetMessageCount(ctx context.Context, sceneID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM scene_messages WHERE scene_id = ?", sceneID).Scan(&count)
	return count, err
}

func (r *SceneRepository) UpdateSummary(ctx context.Context, sceneID int64, newSummary string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE scenes SET summary = ? WHERE id = ?", newSummary, sceneID)
	return err
}

func (r *SceneRepository) PruneMessages(ctx context.Context, sceneID int64, keep int) error {
	_, err := r.db.ExecContext(ctx, `
        DELETE FROM scene_messages 
        WHERE id NOT IN (
            SELECT id FROM scene_messages 
            WHERE scene_id = ? 
            ORDER BY created_at DESC 
            LIMIT ?
        ) AND scene_id = ?`, sceneID, keep, sceneID)
	return err
}

package scenes

import (
	"context"
	"database/sql"
	"fmt"

	"aurora/internal/models"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) EnsureDefaultScene() error {
	return nil
}

func (s *Service) SetGMMode(ctx context.Context, sceneID int64, mode string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE scenes SET gm_mode=? WHERE id=?`, mode, sceneID)
	return err
}

func (s *Service) GetOrCreateSceneForCharacter(ctx context.Context, charID int64) (models.Scene, error) {
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
	err := s.db.QueryRowContext(ctx, query, charID).Scan(
		&sc.ID,
		&sc.LocationID,
		&sc.LocationName,
		&sc.Name,
		&sc.GMMode,
		&sc.Summary,
		&sc.IsActive,
		&sc.CreatedAt,
	)

	if err == sql.ErrNoRows {
		insertQuery := `
			INSERT INTO scenes (character_id, name, location_name, gm_mode, is_active, summary) 
			VALUES (?, ?, ?, '0', 1, 'Начало пути.')
		`
		_, createErr := s.db.ExecContext(ctx, insertQuery, charID, "Личное приключение", "Столица Авроры")
		if createErr != nil {
			return models.Scene{}, fmt.Errorf("failed to create scene: %w", createErr)
		}

		return s.GetOrCreateSceneForCharacter(ctx, charID)
	}

	if err != nil {
		return models.Scene{}, err
	}

	sc.Status = "active"
	sc.CharacterID = charID
	return sc, nil
}

func (s *Service) AppendMessage(ctx context.Context, msg models.SceneMessage) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO scene_messages (scene_id, sender_type, sender_id, content, created_at)
VALUES (?, ?, ?, ?, ?)
`, msg.SceneID, msg.SenderType, msg.SenderID, msg.Content, msg.CreatedAt)
	return err
}

func (s *Service) GetLastMessagesSummary(ctx context.Context, sceneID int64, limit int) (string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT sender_type, content 
		FROM scene_messages
		WHERE scene_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, sceneID, limit)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var st, ct string
		if err := rows.Scan(&st, &ct); err != nil {
			continue
		}
		prefix := "Игрок"
		if st == "ai" {
			prefix = "Лапидарий"
		} else if st == "system" {
			prefix = "Система"
		}
		lines = append(lines, prefix+": "+ct)
	}

	result := ""
	for i := len(lines) - 1; i >= 0; i-- {
		result += lines[i] + "\n"
	}

	return result, nil
}

func (s *Service) UpdateSceneLocation(ctx context.Context, sceneID int64, locID sql.NullInt64, locName string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE scenes 
		SET location_id = ?, location_name = ?
		WHERE id = ?
	`, locID, locName, sceneID)
	return err
}

func (s *Service) GetMessageCount(ctx context.Context, sceneID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM scene_messages WHERE scene_id = ?", sceneID).Scan(&count)
	return count, err
}

func (s *Service) UpdateSummary(ctx context.Context, sceneID int64, newSummary string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE scenes SET summary = ? WHERE id = ?", newSummary, sceneID)
	return err
}

func (s *Service) PruneMessages(ctx context.Context, sceneID int64, keep int) error {
	_, err := s.db.ExecContext(ctx, `
        DELETE FROM scene_messages 
        WHERE id NOT IN (
            SELECT id FROM scene_messages 
            WHERE scene_id = ? 
            ORDER BY created_at DESC 
            LIMIT ?
        ) AND scene_id = ?`, sceneID, keep, sceneID)
	return err
}

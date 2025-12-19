package scenes

import (
	"context"
	"database/sql"
	"errors"

	"aurora/internal/models"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) EnsureDefaultScene() error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM scenes WHERE is_active = 1`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = s.db.Exec(`INSERT INTO scenes (name, location_name, gm_mode, is_active, summary) 
VALUES ('Основная сцена', 'Столица Авроры', 'ai_assist', 1, 'Начало кампании.')`)
	return err
}

func (s *Service) GetActiveScene(ctx context.Context) (models.Scene, error) {
	// Исправленный запрос: IFNULL(gm_mode, 0) вместо 'ai_assist'
	row := s.db.QueryRowContext(ctx, `
SELECT
  id,
  character_id,
  IFNULL(location_id, 0),
  IFNULL(location_name, 'Неизвестно'),
  IFNULL(name, 'Сцена'),
  IFNULL(gm_mode, 0),
  IFNULL(status, 'active'),
  IFNULL(summary, ''),
  IFNULL(context, ''),
  IFNULL(is_active, 1),
  created_at
FROM scenes
WHERE is_active = 1
ORDER BY created_at DESC
LIMIT 1
`)

	var sc models.Scene
	err := row.Scan(
		&sc.ID,
		&sc.CharacterID,
		&sc.LocationID,
		&sc.LocationName,
		&sc.Name,
		&sc.GMMode,
		&sc.Status,
		&sc.Summary,
		&sc.Context,
		&sc.IsActive,
		&sc.CreatedAt,
	)
	if err != nil {
		return models.Scene{}, err
	}
	return sc, nil
}

func (s *Service) AppendMessage(ctx context.Context, msg models.SceneMessage) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO scene_messages (scene_id,sender_type,sender_id,content) 
VALUES (?,?,?,?)`, msg.SceneID, msg.SenderType, msg.SenderID, msg.Content)
	return err
}

func (s *Service) GetLastMessagesSummary(ctx context.Context, sceneID int64, limit int) (string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT sender_type,content FROM scene_messages 
WHERE scene_id=? ORDER BY id DESC LIMIT ?`, sceneID, limit)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	type line struct{ st, ct string }
	var tmp []line
	for rows.Next() {
		var t, c string
		if err := rows.Scan(&t, &c); err != nil {
			return "", err
		}
		tmp = append(tmp, line{t, c})
	}
	if len(tmp) == 0 {
		return "История сцены ещё не сформирована.", nil
	}
	out := ""
	for i := len(tmp) - 1; i >= 0; i-- {
		prefix := ""
		switch tmp[i].st {
		case "player":
			prefix = "Игрок: "
		case "ai":
			prefix = "Мир: "
		case "gm":
			prefix = "Ведущий: "
		}
		out += prefix + tmp[i].ct + "\n"
	}
	return out, nil
}

func (s *Service) SetGMMode(ctx context.Context, sceneID int64, mode string) error {
	if mode != "human" && mode != "ai_assist" && mode != "ai_full" {
		return errors.New("invalid gm mode")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE scenes SET gm_mode=? WHERE id=?`, mode, sceneID)
	return err
}

func (s *Service) SetActiveSceneLocation(ctx context.Context, locID sql.NullInt64, locName string) error {
	var v interface{}
	if locID.Valid {
		v = locID.Int64
	} else {
		v = nil
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE scenes
SET location_id = ?, location_name = ?
WHERE is_active = 1`, v, locName)
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

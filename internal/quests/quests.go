package quests

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"aurora/internal/models"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) GetActiveForCharacter(ctx context.Context, charID int64) ([]models.Quest, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,character_id,title,description,stage,status,from_source,difficulty,reward_value,created_at,updated_at 
FROM quests WHERE character_id=? AND status='active'`, charID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []models.Quest
	for rows.Next() {
		var q models.Quest
		if err := rows.Scan(
			&q.ID, &q.CharacterID, &q.Title, &q.Description, &q.Stage,
			&q.Status, &q.From, &q.Difficulty, &q.RewardValue,
			&q.CreatedAt, &q.UpdatedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, q)
	}
	return res, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (models.Quest, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,character_id,title,description,stage,status,from_source,difficulty,reward_value,created_at,updated_at 
FROM quests WHERE id=?`, id)
	var q models.Quest
	err := row.Scan(
		&q.ID, &q.CharacterID, &q.Title, &q.Description, &q.Stage,
		&q.Status, &q.From, &q.Difficulty, &q.RewardValue,
		&q.CreatedAt, &q.UpdatedAt,
	)
	return q, err
}

func (s *Service) UpdateProgress(ctx context.Context, q models.Quest) error {
	q.UpdatedAt = time.Now()
	_, err := s.db.ExecContext(ctx, `UPDATE quests SET stage=?,status=?,reward_value=?,updated_at=? WHERE id=?`,
		q.Stage, q.Status, q.RewardValue, q.UpdatedAt, q.ID)
	return err
}

func (s *Service) CreateFromAI(ctx context.Context, charID int64, raw string) (*models.Quest, error) {
	lines := strings.Split(raw, "\n")
	var title, desc, qtype, qdiff string
	var qvalue int

	for _, ln := range lines {
		l := strings.TrimSpace(ln)
		switch {
		case strings.HasPrefix(l, "[QUEST_TITLE]:"):
			title = strings.TrimSpace(strings.TrimPrefix(l, "[QUEST_TITLE]:"))
		case strings.HasPrefix(l, "[QUEST_DESCRIPTION]:"):
			desc = strings.TrimSpace(strings.TrimPrefix(l, "[QUEST_DESCRIPTION]:"))
		case strings.HasPrefix(l, "[QUEST_TYPE]:"):
			qtype = strings.TrimSpace(strings.TrimPrefix(l, "[QUEST_TYPE]:"))
		case strings.HasPrefix(l, "[QUEST_DIFFICULTY]:"):
			qdiff = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(l, "[QUEST_DIFFICULTY]:")))
		case strings.HasPrefix(l, "[QUEST_VALUE]:"):
			vStr := strings.TrimSpace(strings.TrimPrefix(l, "[QUEST_VALUE]:"))
			if v, err := strconv.Atoi(vStr); err == nil {
				qvalue = v
			}
		}
	}
	if title == "" {
		return nil, nil
	}
	if qtype != "" {
		desc += "\n(Тип: " + qtype + ")"
	}
	if qdiff == "" {
		qdiff = "normal"
	}
	if qvalue <= 0 {
		qvalue = 100
	}

	now := time.Now()
	res, err := s.db.ExecContext(ctx, `INSERT INTO quests (character_id,title,description,stage,status,from_source,difficulty,reward_value,created_at,updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?)`,
		charID, title, desc, 1, "active", "ai", qdiff, qvalue, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &models.Quest{
		ID:          id,
		CharacterID: charID,
		Title:       title,
		Description: desc,
		Stage:       1,
		Status:      "active",
		From:        "ai",
		Difficulty:  qdiff,
		RewardValue: qvalue,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (s *Service) SetLocation(ctx context.Context, questID, locID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE quests SET location_id=? WHERE id=?`, locID, questID)
	return err
}

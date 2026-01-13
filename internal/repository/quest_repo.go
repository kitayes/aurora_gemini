package repository

import (
	"context"
	"database/sql"

	"aurora/internal/models"
)

type QuestRepository struct {
	db *sql.DB
}

func NewQuestRepository(db *sql.DB) *QuestRepository {
	return &QuestRepository{db: db}
}

func (r *QuestRepository) GetActiveForCharacter(ctx context.Context, charID int64) ([]models.Quest, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,character_id,title,description,stage,status,from_source,difficulty,reward_value,created_at,updated_at 
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

func (r *QuestRepository) GetByID(ctx context.Context, id int64) (models.Quest, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,character_id,title,description,stage,status,from_source,difficulty,reward_value,created_at,updated_at 
FROM quests WHERE id=?`, id)
	var q models.Quest
	err := row.Scan(
		&q.ID, &q.CharacterID, &q.Title, &q.Description, &q.Stage,
		&q.Status, &q.From, &q.Difficulty, &q.RewardValue,
		&q.CreatedAt, &q.UpdatedAt,
	)
	return q, err
}

func (r *QuestRepository) Update(ctx context.Context, q models.Quest) error {
	_, err := r.db.ExecContext(ctx, `UPDATE quests SET stage=?,status=?,reward_value=?,updated_at=?, location_id=? WHERE id=?`,
		q.Stage, q.Status, q.RewardValue, q.UpdatedAt, q.LocationID, q.ID)
	return err
}

func (r *QuestRepository) Create(ctx context.Context, q *models.Quest) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO quests (character_id,title,description,stage,status,from_source,difficulty,reward_value,created_at,updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?)`,
		q.CharacterID, q.Title, q.Description, q.Stage, q.Status, q.From, q.Difficulty, q.RewardValue, q.CreatedAt, q.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

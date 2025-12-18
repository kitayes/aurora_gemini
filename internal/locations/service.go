package locations

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"aurora/internal/models"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) Create(ctx context.Context, name, desc, tags, createdBy string) (*models.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, sql.ErrNoRows
	}
	if createdBy == "" {
		createdBy = "gm"
	}

	res, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO locations(name, description, tags, created_by)
VALUES (?, ?, ?, ?)`,
		name, desc, tags, createdBy,
	)
	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	if id == 0 {
		return s.GetByName(ctx, name)
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

func (s *Service) GetByName(ctx context.Context, name string) (*models.Location, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, IFNULL(description,''), IFNULL(tags,''), created_by, created_at
FROM locations
WHERE name = ?
LIMIT 1`, name)

	var l models.Location
	if err := row.Scan(&l.ID, &l.Name, &l.Description, &l.Tags, &l.CreatedBy, &l.CreatedAt); err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*models.Location, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, IFNULL(description,''), IFNULL(tags,''), created_by, created_at
FROM locations
WHERE id = ?
LIMIT 1`, id)

	var l models.Location
	if err := row.Scan(&l.ID, &l.Name, &l.Description, &l.Tags, &l.CreatedBy, &l.CreatedAt); err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *Service) List(ctx context.Context, limit int) ([]models.Location, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, IFNULL(description,''), IFNULL(tags,''), created_by, created_at
FROM locations
ORDER BY id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Location
	for rows.Next() {
		var l models.Location
		if err := rows.Scan(&l.ID, &l.Name, &l.Description, &l.Tags, &l.CreatedBy, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

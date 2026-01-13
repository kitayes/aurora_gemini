package repository

import (
	"context"
	"database/sql"

	"aurora/internal/models"
)

type LocationRepository struct {
	db *sql.DB
}

func NewLocationRepository(db *sql.DB) *LocationRepository {
	return &LocationRepository{db: db}
}

func (r *LocationRepository) Create(ctx context.Context, name, desc, tags, createdBy string) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
INSERT OR IGNORE INTO locations(name, description, tags, created_by)
VALUES (?, ?, ?, ?)`,
		name, desc, tags, createdBy,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *LocationRepository) GetByName(ctx context.Context, name string) (*models.Location, error) {
	row := r.db.QueryRowContext(ctx, `
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

func (r *LocationRepository) GetByID(ctx context.Context, id int64) (*models.Location, error) {
	row := r.db.QueryRowContext(ctx, `
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

func (r *LocationRepository) List(ctx context.Context, limit int) ([]models.Location, error) {
	rows, err := r.db.QueryContext(ctx, `
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

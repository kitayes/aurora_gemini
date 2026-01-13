package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
)

type VectorDocument struct {
	ID       string
	Title    string
	Content  string
	Vector   []float32
	Zone     string
	Tags     []string
	Metadata map[string]string
}

type VectorStats struct {
	TotalDocuments  int
	DocumentsByZone map[string]int
}

type VectorRepository interface {
	IndexDocument(ctx context.Context, doc VectorDocument) error
	IndexBatch(ctx context.Context, docs []VectorDocument) error
	SearchSimilar(ctx context.Context, queryVector []float32, limit int, filters map[string]string) ([]VectorDocument, error)
	DeleteByZone(ctx context.Context, zone string) error
	GetStats(ctx context.Context) (VectorStats, error)
	DeleteAll(ctx context.Context) error
}

type sqliteVectorRepo struct {
	db *sql.DB
}

func NewVectorRepository(db *sql.DB) VectorRepository {
	return &sqliteVectorRepo{db: db}
}

func (r *sqliteVectorRepo) IndexDocument(ctx context.Context, doc VectorDocument) error {
	tagsJSON, err := json.Marshal(doc.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	vectorBlob, err := serializeVector(doc.Vector)
	if err != nil {
		return fmt.Errorf("serialize vector: %w", err)
	}

	query := `
		INSERT OR REPLACE INTO lore_vectors (id, title, content, zone, tags, vector, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(ctx, query,
		doc.ID,
		doc.Title,
		doc.Content,
		doc.Zone,
		string(tagsJSON),
		vectorBlob,
		string(metadataJSON),
	)

	if err != nil {
		return fmt.Errorf("insert document: %w", err)
	}

	return nil
}

func (r *sqliteVectorRepo) IndexBatch(ctx context.Context, docs []VectorDocument) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO lore_vectors (id, title, content, zone, tags, vector, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, doc := range docs {
		tagsJSON, err := json.Marshal(doc.Tags)
		if err != nil {
			return fmt.Errorf("marshal tags for %s: %w", doc.ID, err)
		}

		metadataJSON, err := json.Marshal(doc.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata for %s: %w", doc.ID, err)
		}

		vectorBlob, err := serializeVector(doc.Vector)
		if err != nil {
			return fmt.Errorf("serialize vector for %s: %w", doc.ID, err)
		}

		_, err = stmt.ExecContext(ctx,
			doc.ID,
			doc.Title,
			doc.Content,
			doc.Zone,
			string(tagsJSON),
			vectorBlob,
			string(metadataJSON),
		)
		if err != nil {
			return fmt.Errorf("insert document %s: %w", doc.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *sqliteVectorRepo) SearchSimilar(ctx context.Context, queryVector []float32, limit int, filters map[string]string) ([]VectorDocument, error) {
	query := `SELECT id, title, content, zone, tags, vector, metadata FROM lore_vectors`
	args := []interface{}{}

	if zone, ok := filters["zone"]; ok && zone != "" {
		query += ` WHERE zone = ?`
		args = append(args, zone)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query documents: %w", err)
	}
	defer rows.Close()

	type docWithScore struct {
		doc   VectorDocument
		score float64
	}

	var candidates []docWithScore

	for rows.Next() {
		var doc VectorDocument
		var tagsJSON, metadataJSON string
		var vectorBlob []byte

		err := rows.Scan(
			&doc.ID,
			&doc.Title,
			&doc.Content,
			&doc.Zone,
			&tagsJSON,
			&vectorBlob,
			&metadataJSON,
		)
		if err != nil {
			continue
		}

		if err := json.Unmarshal([]byte(tagsJSON), &doc.Tags); err != nil {
			continue
		}

		if err := json.Unmarshal([]byte(metadataJSON), &doc.Metadata); err != nil {
			doc.Metadata = make(map[string]string)
		}

		vector, err := deserializeVector(vectorBlob)
		if err != nil {
			continue
		}
		doc.Vector = vector

		similarity := cosineSimilarity(queryVector, vector)
		candidates = append(candidates, docWithScore{doc: doc, score: similarity})
	}

	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	if limit > len(candidates) {
		limit = len(candidates)
	}

	results := make([]VectorDocument, limit)
	for i := 0; i < limit; i++ {
		results[i] = candidates[i].doc
	}

	return results, nil
}

func (r *sqliteVectorRepo) DeleteByZone(ctx context.Context, zone string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM lore_vectors WHERE zone = ?`, zone)
	return err
}

func (r *sqliteVectorRepo) DeleteAll(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM lore_vectors`)
	return err
}

func (r *sqliteVectorRepo) GetStats(ctx context.Context) (VectorStats, error) {
	var stats VectorStats
	stats.DocumentsByZone = make(map[string]int)

	row := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM lore_vectors`)
	if err := row.Scan(&stats.TotalDocuments); err != nil {
		return stats, fmt.Errorf("count total: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `SELECT zone, COUNT(*) FROM lore_vectors GROUP BY zone`)
	if err != nil {
		return stats, fmt.Errorf("count by zone: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var zone string
		var count int
		if err := rows.Scan(&zone, &count); err != nil {
			continue
		}
		stats.DocumentsByZone[zone] = count
	}

	return stats, nil
}

func serializeVector(vec []float32) ([]byte, error) {
	return json.Marshal(vec)
}

func deserializeVector(data []byte) ([]float32, error) {
	var vec []float32
	if err := json.Unmarshal(data, &vec); err != nil {
		return nil, err
	}
	return vec, nil
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

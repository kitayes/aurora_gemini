-- Таблица для векторных представлений lore
CREATE TABLE IF NOT EXISTS lore_vectors (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    zone TEXT NOT NULL,
    tags TEXT NOT NULL,
    vector BLOB NOT NULL,
    metadata TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Индекс для быстрого поиска по зонам
CREATE INDEX IF NOT EXISTS idx_lore_vectors_zone ON lore_vectors(zone);

-- Индекс для поиска по времени создания
CREATE INDEX IF NOT EXISTS idx_lore_vectors_created ON lore_vectors(created_at);

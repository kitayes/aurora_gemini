CREATE TABLE IF NOT EXISTS locations (
                                         id INTEGER PRIMARY KEY AUTOINCREMENT,
                                         name TEXT NOT NULL,
                                         description TEXT,
                                         tags TEXT,
                                         created_by TEXT NOT NULL DEFAULT 'gm',
                                         created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_locations_name ON locations(name);

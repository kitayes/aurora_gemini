CREATE TABLE character_effects (
                                   id INTEGER PRIMARY KEY AUTOINCREMENT,
                                   character_id INTEGER NOT NULL,
                                   name TEXT NOT NULL,
                                   description TEXT,
                                   duration_turns INTEGER DEFAULT 0,
                                   is_hidden BOOLEAN DEFAULT 0,
                                   created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                                   FOREIGN KEY(character_id) REFERENCES characters(id) ON DELETE CASCADE
);

CREATE INDEX idx_character_effects_char_id ON character_effects(character_id);
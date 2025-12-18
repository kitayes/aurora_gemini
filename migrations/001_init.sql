CREATE TABLE IF NOT EXISTS characters (
                                          id INTEGER PRIMARY KEY AUTOINCREMENT,
                                          vk_user_id INTEGER NOT NULL,
                                          name TEXT NOT NULL,
                                          race TEXT,
                                          class TEXT,
                                          faction_id INTEGER,
                                          faction_name TEXT,
                                          traits TEXT,
                                          goal TEXT,
                                          location_id INTEGER,
                                          location_name TEXT,
                                          status TEXT,
                                          created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS scenes (
                                      id INTEGER PRIMARY KEY AUTOINCREMENT,
                                      name TEXT NOT NULL,
                                      location_id INTEGER,
                                      location_name TEXT,
                                      summary TEXT,
                                      gm_mode TEXT NOT NULL DEFAULT 'ai_assist',
                                      is_active BOOLEAN NOT NULL DEFAULT 1,
                                      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS scene_messages (
                                              id INTEGER PRIMARY KEY AUTOINCREMENT,
                                              scene_id INTEGER NOT NULL,
                                              sender_type TEXT NOT NULL,
                                              sender_id INTEGER,
                                              content TEXT NOT NULL,
                                              created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                              FOREIGN KEY(scene_id) REFERENCES scenes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS quests (
                                      id INTEGER PRIMARY KEY AUTOINCREMENT,
                                      character_id INTEGER NOT NULL,
                                      title TEXT NOT NULL,
                                      description TEXT,
                                      stage INTEGER NOT NULL DEFAULT 1,
                                      status TEXT NOT NULL DEFAULT 'active',
                                      from_source TEXT NOT NULL DEFAULT 'ai',
                                      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                      updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                      FOREIGN KEY(character_id) REFERENCES characters(id) ON DELETE CASCADE
);

package history

import "database/sql"

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS searches (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    query         TEXT NOT NULL,
    response      TEXT NOT NULL,
    sources       TEXT NOT NULL DEFAULT '[]',
    project_dir   TEXT NOT NULL DEFAULT '',
    project_stack TEXT NOT NULL DEFAULT '',
    llm_backend   TEXT NOT NULL DEFAULT '',
    output_format TEXT NOT NULL DEFAULT 'concise',
    search_ms     INTEGER NOT NULL DEFAULT 0,
    llm_ms        INTEGER NOT NULL DEFAULT 0,
    total_ms      INTEGER NOT NULL DEFAULT 0,
    is_followup   INTEGER NOT NULL DEFAULT 0,
    parent_id     INTEGER,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_id) REFERENCES searches(id)
);

CREATE INDEX IF NOT EXISTS idx_searches_query ON searches(query);
CREATE INDEX IF NOT EXISTS idx_searches_created_at ON searches(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_searches_project ON searches(project_dir);

CREATE VIRTUAL TABLE IF NOT EXISTS searches_fts USING fts5(
    query,
    response,
    content=searches,
    content_rowid=id
);

CREATE TRIGGER IF NOT EXISTS searches_ai AFTER INSERT ON searches BEGIN
    INSERT INTO searches_fts(rowid, query, response)
    VALUES (new.id, new.query, new.response);
END;

CREATE TRIGGER IF NOT EXISTS searches_ad AFTER DELETE ON searches BEGIN
    INSERT INTO searches_fts(searches_fts, rowid, query, response)
    VALUES ('delete', old.id, old.query, old.response);
END;

CREATE TRIGGER IF NOT EXISTS searches_au AFTER UPDATE ON searches BEGIN
    INSERT INTO searches_fts(searches_fts, rowid, query, response)
    VALUES ('delete', old.id, old.query, old.response);
    INSERT INTO searches_fts(rowid, query, response)
    VALUES (new.id, new.query, new.response);
END;
`

func ensureSchema(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}

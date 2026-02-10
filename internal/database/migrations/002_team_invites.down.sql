-- Rollback team invites - recreate teams table without new columns
PRAGMA foreign_keys=OFF;

DROP INDEX IF EXISTS idx_teams_invite_id;

CREATE TABLE teams_old (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    owner_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
);

INSERT INTO teams_old (id, name, description, owner_id, created_at, updated_at)
SELECT id, name, description, owner_id, created_at, updated_at
FROM teams;

DROP TABLE teams;

ALTER TABLE teams_old RENAME TO teams;

PRAGMA foreign_keys=ON;

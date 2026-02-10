-- SQLite doesn't support adding UNIQUE columns easily, so we recreate the table
PRAGMA foreign_keys=OFF;

-- Create new table with the correct schema
CREATE TABLE teams_new (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    owner_id TEXT NOT NULL,
    invite_id TEXT UNIQUE NOT NULL,
    invite_permission TEXT NOT NULL DEFAULT 'owner_only',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Copy data from old table to new table with generated invite codes
INSERT INTO teams_new (id, name, description, owner_id, invite_id, invite_permission, created_at, updated_at)
SELECT id, name, description, owner_id, hex(randomblob(16)), 'owner_only', created_at, updated_at
FROM teams;

-- Drop old table
DROP TABLE teams;

-- Rename new table to teams
ALTER TABLE teams_new RENAME TO teams;

-- Re-enable foreign keys
PRAGMA foreign_keys=ON;

-- Create index for fast invite code lookups
CREATE INDEX idx_teams_invite_id ON teams(invite_id);

-- Create challenge_files table for multiple files per challenge
CREATE TABLE challenge_files (
    id TEXT PRIMARY KEY,
    challenge_id TEXT NOT NULL REFERENCES challenges(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    storage_type TEXT NOT NULL, -- 'local' or 'external'
    storage_path TEXT,          -- path/URL for the file
    size_bytes INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_challenge_files_challenge ON challenge_files(challenge_id);

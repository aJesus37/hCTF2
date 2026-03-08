-- Restore unique constraint (keeps only the latest submission per user per question)
CREATE TABLE submissions_new (
    id TEXT PRIMARY KEY,
    question_id TEXT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id TEXT REFERENCES teams(id) ON DELETE SET NULL,
    submitted_flag TEXT NOT NULL,
    is_correct BOOLEAN NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(question_id, user_id)
);
INSERT OR IGNORE INTO submissions_new SELECT id, question_id, user_id, team_id, submitted_flag, is_correct, created_at FROM submissions ORDER BY created_at DESC;
DROP TABLE submissions;
ALTER TABLE submissions_new RENAME TO submissions;

CREATE TABLE score_history (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id TEXT REFERENCES teams(id) ON DELETE CASCADE,
    score INTEGER NOT NULL,
    solve_count INTEGER NOT NULL DEFAULT 0,
    recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_score_history_recorded ON score_history(recorded_at);
CREATE INDEX idx_score_history_user ON score_history(user_id, recorded_at);
CREATE INDEX idx_score_history_team ON score_history(team_id, recorded_at);

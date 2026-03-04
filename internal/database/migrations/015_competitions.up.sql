CREATE TABLE IF NOT EXISTS competitions (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    name                TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    rules_html          TEXT NOT NULL DEFAULT '',
    start_at            DATETIME,
    end_at              DATETIME,
    registration_start  DATETIME,
    registration_end    DATETIME,
    scoreboard_frozen   INTEGER NOT NULL DEFAULT 0,
    scoreboard_blackout INTEGER NOT NULL DEFAULT 0,
    status              TEXT NOT NULL DEFAULT 'draft',
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS competition_challenges (
    competition_id INTEGER NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    challenge_id   TEXT    NOT NULL REFERENCES challenges(id)   ON DELETE CASCADE,
    PRIMARY KEY (competition_id, challenge_id)
);

CREATE TABLE IF NOT EXISTS competition_teams (
    competition_id INTEGER NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    team_id        TEXT    NOT NULL REFERENCES teams(id)        ON DELETE CASCADE,
    joined_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (competition_id, team_id)
);

CREATE INDEX idx_competition_challenges_challenge ON competition_challenges(challenge_id);
CREATE INDEX idx_competition_teams_team ON competition_teams(team_id);

-- UUIDv7 Migration: Remove hex(randomblob(16)) defaults
-- IDs are now generated in Go code using UUIDv7
-- This is a clean migration (no data preservation needed)

-- Disable foreign keys during migration
PRAGMA foreign_keys = OFF;

-- Users table
CREATE TABLE users_new (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    name TEXT NOT NULL,
    avatar_url TEXT,
    team_id TEXT REFERENCES teams(id) ON DELETE SET NULL,
    is_admin BOOLEAN DEFAULT 0,
    password_reset_token TEXT,
    password_reset_expires DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO users_new SELECT id, email, password_hash, name, avatar_url, team_id, is_admin, password_reset_token, password_reset_expires, created_at, updated_at FROM users;
DROP TABLE users;
ALTER TABLE users_new RENAME TO users;
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_reset_token ON users(password_reset_token);

-- Teams table
CREATE TABLE teams_new (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    owner_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    invite_id TEXT UNIQUE NOT NULL,
    invite_permission TEXT NOT NULL DEFAULT 'owner_only',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO teams_new SELECT id, name, description, owner_id, invite_id, invite_permission, created_at, updated_at FROM teams;
DROP TABLE teams;
ALTER TABLE teams_new RENAME TO teams;
CREATE INDEX idx_teams_invite_id ON teams(invite_id);

-- Challenges table
CREATE TABLE challenges_new (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL,
    category TEXT NOT NULL,
    difficulty TEXT NOT NULL,
    tags JSON,
    visible BOOLEAN DEFAULT 1,
    sql_enabled BOOLEAN DEFAULT 0,
    sql_dataset_url TEXT,
    sql_schema_hint TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO challenges_new SELECT id, name, description, category, difficulty, tags, visible, sql_enabled, sql_dataset_url, sql_schema_hint, created_at, updated_at FROM challenges;
DROP TABLE challenges;
ALTER TABLE challenges_new RENAME TO challenges;

-- Questions table
CREATE TABLE questions_new (
    id TEXT PRIMARY KEY,
    challenge_id TEXT NOT NULL REFERENCES challenges(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    flag TEXT NOT NULL,
    flag_mask TEXT,
    case_sensitive BOOLEAN DEFAULT 0,
    points INTEGER DEFAULT 100,
    file_url TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO questions_new SELECT id, challenge_id, name, description, flag, flag_mask, case_sensitive, points, file_url, created_at, updated_at FROM questions;
DROP TABLE questions;
ALTER TABLE questions_new RENAME TO questions;

-- Hints table
CREATE TABLE hints_new (
    id TEXT PRIMARY KEY,
    question_id TEXT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    cost INTEGER DEFAULT 0,
    "order" INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO hints_new SELECT id, question_id, content, cost, "order", created_at FROM hints;
DROP TABLE hints;
ALTER TABLE hints_new RENAME TO hints;

-- Submissions table
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
INSERT INTO submissions_new SELECT id, question_id, user_id, team_id, submitted_flag, is_correct, created_at FROM submissions;
DROP TABLE submissions;
ALTER TABLE submissions_new RENAME TO submissions;

-- Hint unlocks table
CREATE TABLE hint_unlocks_new (
    id TEXT PRIMARY KEY,
    hint_id TEXT NOT NULL REFERENCES hints(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id TEXT REFERENCES teams(id) ON DELETE SET NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(hint_id, user_id)
);
INSERT INTO hint_unlocks_new SELECT id, hint_id, user_id, team_id, created_at FROM hint_unlocks;
DROP TABLE hint_unlocks;
ALTER TABLE hint_unlocks_new RENAME TO hint_unlocks;

-- Categories table
CREATE TABLE categories_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO categories_new SELECT id, name, sort_order, created_at FROM categories;
DROP TABLE categories;
ALTER TABLE categories_new RENAME TO categories;

-- Difficulties table
CREATE TABLE difficulties_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    color TEXT NOT NULL DEFAULT 'bg-gray-600 text-gray-100',
    text_color TEXT NOT NULL DEFAULT 'text-gray-400',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO difficulties_new SELECT id, name, color, text_color, sort_order, created_at FROM difficulties;
DROP TABLE difficulties;
ALTER TABLE difficulties_new RENAME TO difficulties;

-- Recreate indexes
CREATE INDEX idx_submissions_user ON submissions(user_id, created_at);
CREATE INDEX idx_submissions_team ON submissions(team_id, created_at);
CREATE INDEX idx_submissions_question ON submissions(question_id, is_correct);
CREATE INDEX idx_questions_challenge ON questions(challenge_id);
CREATE INDEX idx_challenges_category ON challenges(category, visible);
CREATE INDEX idx_hint_unlocks_team ON hint_unlocks(team_id);

-- Re-enable foreign keys
PRAGMA foreign_keys = ON;

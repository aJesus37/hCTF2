-- Add setting to control whether admins appear in scoreboard
ALTER TABLE settings ADD COLUMN admin_visible_in_scoreboard BOOLEAN DEFAULT 0;

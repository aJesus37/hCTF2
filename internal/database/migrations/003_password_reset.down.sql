-- Rollback password reset token fields
DROP INDEX IF EXISTS idx_users_reset_token;
ALTER TABLE users DROP COLUMN password_reset_expires;
ALTER TABLE users DROP COLUMN password_reset_token;

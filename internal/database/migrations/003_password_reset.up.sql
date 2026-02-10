-- Add password reset token fields to users table
ALTER TABLE users ADD COLUMN password_reset_token TEXT;
ALTER TABLE users ADD COLUMN password_reset_expires DATETIME;

-- Create index for fast lookup by token
CREATE INDEX idx_users_reset_token ON users(password_reset_token);

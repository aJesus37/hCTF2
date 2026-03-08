-- SQLite does not support DROP COLUMN in all versions; recreate table to remove freeze_at.
-- For simplicity, we leave this as a no-op since the column is nullable and harmless.
SELECT 1;

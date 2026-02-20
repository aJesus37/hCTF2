-- Rollback: This is a no-op since the data format is compatible
-- The only change was removing DEFAULT clauses, which SQLite handles fine
-- Old hex(randomblob(16)) IDs are still valid TEXT primary keys
SELECT 1;

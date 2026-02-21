-- Remove dynamic scoring columns from challenges table
ALTER TABLE challenges DROP COLUMN dynamic_scoring;
ALTER TABLE challenges DROP COLUMN initial_points;
ALTER TABLE challenges DROP COLUMN minimum_points;
ALTER TABLE challenges DROP COLUMN decay_threshold;

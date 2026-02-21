-- Add dynamic scoring columns to challenges table
ALTER TABLE challenges ADD COLUMN dynamic_scoring BOOLEAN DEFAULT FALSE;
ALTER TABLE challenges ADD COLUMN initial_points INTEGER;
ALTER TABLE challenges ADD COLUMN minimum_points INTEGER DEFAULT 100;
ALTER TABLE challenges ADD COLUMN decay_threshold INTEGER DEFAULT 100;

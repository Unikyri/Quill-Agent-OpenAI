DROP INDEX IF EXISTS idx_entities_universe_candidates;
ALTER TABLE entities DROP COLUMN IF EXISTS confidence;

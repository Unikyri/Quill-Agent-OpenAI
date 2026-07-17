-- Sprint 6: confidence-gated entity candidates.
-- Candidate rows remain in the entity inventory so evidence can be reviewed,
-- while their status keeps them out of active memory until accepted.
ALTER TABLE entities
    ADD COLUMN IF NOT EXISTS confidence DOUBLE PRECISION NOT NULL DEFAULT 1
        CHECK (confidence >= 0 AND confidence <= 1);

CREATE INDEX idx_entities_universe_candidates
    ON entities (universe_id, status, updated_at DESC)
    WHERE status = 'candidate';

CREATE TABLE entity_relevance_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    universe_id UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    relevance_score DOUBLE PRECISION NOT NULL,
    status TEXT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_entity_relevance_history_entity_recorded
    ON entity_relevance_history (entity_id, recorded_at);

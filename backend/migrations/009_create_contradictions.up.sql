CREATE TABLE contradictions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    universe_id UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    entity_id UUID REFERENCES entities(id),
    severity VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    suggestion TEXT,
    evidence_a TEXT,
    evidence_a_chapter_id UUID REFERENCES chapters(id),
    evidence_b TEXT,
    evidence_b_chapter_id UUID REFERENCES chapters(id),
    fingerprint VARCHAR(255) UNIQUE,
    status VARCHAR(50) DEFAULT 'open',
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_contradictions_universe_id ON contradictions(universe_id);
CREATE INDEX idx_contradictions_status ON contradictions(universe_id, status);

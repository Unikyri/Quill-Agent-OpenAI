CREATE TABLE entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    universe_id UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    aliases TEXT[],
    description TEXT,
    properties JSONB DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'active',
    confidence DOUBLE PRECISION NOT NULL DEFAULT 1 CHECK (confidence >= 0 AND confidence <= 1),
    relevance_score FLOAT DEFAULT 0.8,
    last_mentioned_chapter_id UUID,
    last_mentioned_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_entities_universe_id ON entities(universe_id);
CREATE INDEX idx_entities_type ON entities(universe_id, type);
CREATE UNIQUE INDEX idx_entities_name_unique ON entities(universe_id, LOWER(name));

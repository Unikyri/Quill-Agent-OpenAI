CREATE TABLE consolidated_memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    summary TEXT NOT NULL,
    key_facts TEXT[] DEFAULT '{}',
    embedding vector(1024),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(entity_id)
);
CREATE INDEX idx_consolidated_memories_embedding_hnsw
    ON consolidated_memories USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

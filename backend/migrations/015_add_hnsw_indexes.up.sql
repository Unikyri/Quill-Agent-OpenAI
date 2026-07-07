CREATE INDEX idx_entity_embeddings_hnsw ON entity_embeddings
    USING hnsw (description_embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX idx_paragraph_embeddings_hnsw ON paragraph_embeddings
    USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);

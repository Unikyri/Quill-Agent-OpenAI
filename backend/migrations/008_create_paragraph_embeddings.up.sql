CREATE TABLE paragraph_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chapter_id UUID NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    paragraph_index INTEGER NOT NULL,
    paragraph_node_id VARCHAR(255),
    content TEXT,
    embedding vector(1024),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_paragraph_embeddings_chapter_id ON paragraph_embeddings(chapter_id);

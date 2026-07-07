ALTER TABLE paragraph_embeddings
  ADD COLUMN content_tsv tsvector GENERATED ALWAYS AS (to_tsvector('english', content)) STORED;
CREATE INDEX idx_paragraph_embeddings_content_tsv ON paragraph_embeddings USING GIN (content_tsv);

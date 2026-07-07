DROP INDEX IF EXISTS idx_paragraph_embeddings_content_tsv;
ALTER TABLE paragraph_embeddings DROP COLUMN IF EXISTS content_tsv;

-- entity_embeddings.entity_id had no unique constraint, so SaveEntityEmbedding's
-- `ON CONFLICT (entity_id) DO UPDATE` has always been invalid SQL (42P10) —
-- pre-existing bug, unrelated to and discovered while fixing the pgvector codec gap.
ALTER TABLE entity_embeddings ADD CONSTRAINT entity_embeddings_entity_id_key UNIQUE (entity_id);

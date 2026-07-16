BEGIN;

-- Migration 005 enforced a universe-wide name key, which prevented a place and
-- an object from legitimately sharing a name. Replace it only after any legacy
-- duplicates of the new, type-aware key have been consolidated.
DROP INDEX IF EXISTS idx_entities_name_unique;

CREATE TEMP TABLE entity_duplicate_map ON COMMIT DROP AS
WITH ranked AS (
    SELECT
        id,
        FIRST_VALUE(id) OVER (
            PARTITION BY universe_id, LOWER(name), type
            ORDER BY LENGTH(COALESCE(description, '')) DESC, created_at ASC, id ASC
        ) AS winner_id
    FROM entities
)
SELECT id AS loser_id, winner_id
FROM ranked
WHERE id <> winner_id;

-- Keep the richest description and all aliases (including losing canonical
-- spellings) on the deterministic winner before foreign keys are repointed.
WITH group_members AS (
    SELECT winner_id, winner_id AS member_id FROM entity_duplicate_map
    UNION
    SELECT winner_id, loser_id AS member_id FROM entity_duplicate_map
), longest_descriptions AS (
    SELECT DISTINCT ON (gm.winner_id)
        gm.winner_id,
        e.description
    FROM group_members gm
    JOIN entities e ON e.id = gm.member_id
    ORDER BY gm.winner_id, LENGTH(COALESCE(e.description, '')) DESC, e.created_at ASC, e.id ASC
), merged_aliases AS (
    SELECT winner_id, ARRAY_AGG(alias ORDER BY LOWER(alias), alias) AS aliases
    FROM (
        SELECT DISTINCT ON (winner_id, LOWER(alias)) winner_id, alias
        FROM (
            SELECT gm.winner_id, e.name AS alias
            FROM group_members gm
            JOIN entities e ON e.id = gm.member_id
            WHERE e.id <> gm.winner_id

            UNION ALL

            SELECT gm.winner_id, alias
            FROM group_members gm
            JOIN entities e ON e.id = gm.member_id
            CROSS JOIN LATERAL UNNEST(COALESCE(e.aliases, ARRAY[]::TEXT[])) AS alias
        ) AS candidates
        WHERE BTRIM(alias) <> ''
        ORDER BY winner_id, LOWER(alias), alias
    ) AS distinct_aliases
    GROUP BY winner_id
)
UPDATE entities e
SET description = d.description,
    aliases = COALESCE(a.aliases, e.aliases),
    updated_at = NOW()
FROM longest_descriptions d
LEFT JOIN merged_aliases a ON a.winner_id = d.winner_id
WHERE e.id = d.winner_id;

UPDATE entity_mentions em
SET entity_id = m.winner_id
FROM entity_duplicate_map m
WHERE em.entity_id = m.loser_id;

UPDATE entity_relevance_history erh
SET entity_id = m.winner_id
FROM entity_duplicate_map m
WHERE erh.entity_id = m.loser_id;

UPDATE contradictions c
SET entity_id = m.winner_id
FROM entity_duplicate_map m
WHERE c.entity_id = m.loser_id;

UPDATE timeline_events te
SET event_entity_id = m.winner_id
FROM entity_duplicate_map m
WHERE te.event_entity_id = m.loser_id;

UPDATE timeline_events te
SET participants = (
    SELECT ARRAY_AGG(DISTINCT COALESCE(m.winner_id, participant_id) ORDER BY COALESCE(m.winner_id, participant_id))
    FROM UNNEST(te.participants) AS participant_id
    LEFT JOIN entity_duplicate_map m ON m.loser_id = participant_id
)
WHERE te.participants && ARRAY(SELECT loser_id FROM entity_duplicate_map);

UPDATE plot_holes ph
SET related_entity_ids = (
    SELECT ARRAY_AGG(DISTINCT COALESCE(m.winner_id, related_id) ORDER BY COALESCE(m.winner_id, related_id))
    FROM UNNEST(ph.related_entity_ids) AS related_id
    LEFT JOIN entity_duplicate_map m ON m.loser_id = related_id
)
WHERE ph.related_entity_ids && ARRAY(SELECT loser_id FROM entity_duplicate_map);

-- Both tables allow exactly one vector-bearing row per entity. Transfer the
-- newest duplicate row only when the winner has none, then remove superseded
-- rows; this preserves a valid embedding/memory without violating uniqueness.
WITH transferable_embeddings AS (
    SELECT DISTINCT ON (m.winner_id) ee.id, m.winner_id
    FROM entity_embeddings ee
    JOIN entity_duplicate_map m ON m.loser_id = ee.entity_id
    WHERE NOT EXISTS (
        SELECT 1 FROM entity_embeddings current_embedding
        WHERE current_embedding.entity_id = m.winner_id
    )
    ORDER BY m.winner_id, ee.updated_at DESC, ee.id ASC
)
UPDATE entity_embeddings ee
SET entity_id = t.winner_id, updated_at = NOW()
FROM transferable_embeddings t
WHERE ee.id = t.id;

DELETE FROM entity_embeddings ee
USING entity_duplicate_map m
WHERE ee.entity_id = m.loser_id;

WITH transferable_memories AS (
    SELECT DISTINCT ON (m.winner_id) cm.id, m.winner_id
    FROM consolidated_memories cm
    JOIN entity_duplicate_map m ON m.loser_id = cm.entity_id
    WHERE NOT EXISTS (
        SELECT 1 FROM consolidated_memories current_memory
        WHERE current_memory.entity_id = m.winner_id
    )
    ORDER BY m.winner_id, cm.created_at DESC, cm.id ASC
)
UPDATE consolidated_memories cm
SET entity_id = t.winner_id
FROM transferable_memories t
WHERE cm.id = t.id;

DELETE FROM consolidated_memories cm
USING entity_duplicate_map m
WHERE cm.entity_id = m.loser_id;

-- AGE relationships are not relational foreign keys. Canonicalize the node
-- property used by every graph query before deleting the losing SQL entity;
-- existing relationship topology remains intact, and relationship properties
-- that carry entity IDs are remapped as well. We deliberately do not detach
-- delete graph vertices: AGE cannot generically retarget relationship endpoints
-- without recreating dynamic edge labels/properties, which would lose data.
DO $$
DECLARE
    mapping RECORD;
    graph_name NAME;
    cypher_query TEXT;
BEGIN
    IF EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'ag_catalog')
       AND EXISTS (SELECT 1 FROM pg_proc WHERE proname = 'cypher') THEN
        EXECUTE 'LOAD ''age''';
        PERFORM set_config('search_path', 'ag_catalog, "$user", public', true);

        FOR mapping IN
            SELECT m.loser_id, m.winner_id, e.universe_id
            FROM entity_duplicate_map m
            JOIN public.entities e ON e.id = m.loser_id
        LOOP
            graph_name := format('universe_%s', mapping.universe_id)::name;
            IF EXISTS (SELECT 1 FROM ag_catalog.ag_graph WHERE name = graph_name) THEN
                cypher_query := format(
                    'MATCH (n {entity_id: ''%s''}) SET n.entity_id = ''%s'' RETURN n',
                    mapping.loser_id,
                    mapping.winner_id
                );
                EXECUTE format(
                    'SELECT * FROM ag_catalog.cypher(%L, $cypher$%s$cypher$) AS (n ag_catalog.agtype)',
                    graph_name,
                    cypher_query
                );

                FOREACH cypher_query IN ARRAY ARRAY[
                    format('MATCH ()-[r]->() WHERE r.entity_id = ''%s'' SET r.entity_id = ''%s'' RETURN r', mapping.loser_id, mapping.winner_id),
                    format('MATCH ()-[r]->() WHERE r.source_entity_id = ''%s'' SET r.source_entity_id = ''%s'' RETURN r', mapping.loser_id, mapping.winner_id),
                    format('MATCH ()-[r]->() WHERE r.target_entity_id = ''%s'' SET r.target_entity_id = ''%s'' RETURN r', mapping.loser_id, mapping.winner_id)
                ] LOOP
                    EXECUTE format(
                        'SELECT * FROM ag_catalog.cypher(%L, $cypher$%s$cypher$) AS (r ag_catalog.agtype)',
                        graph_name,
                        cypher_query
                    );
                END LOOP;
            END IF;
        END LOOP;
    END IF;
END $$;

DELETE FROM entities e
USING entity_duplicate_map m
WHERE e.id = m.loser_id;

CREATE UNIQUE INDEX entities_universe_name_type_key
    ON entities (universe_id, LOWER(name), type);

COMMIT;

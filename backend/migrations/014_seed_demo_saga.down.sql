-- Migration 014: Down — Remove seeded demo saga data
-- Deletes all data seeded in 014_seed_demo_saga.up.sql by template universe FK.
-- Template universe row and demo user are preserved (owned by migration 013).

-- AGE graph cleanup: drop the template graph entirely (not just its rows) so
-- this down migration is safe to re-run and the up migration's create_graph()
-- doesn't fail with "graph already exists" on a second apply — this was
-- previously untested since no migration set ran past 014 more than once.
-- LOAD/search_path mirrors the up migration's preamble: drop_graph() lives in
-- ag_catalog and isn't reachable unqualified without it on a fresh session.
LOAD 'age';
SET search_path = ag_catalog, "$user", public;
SELECT drop_graph('universe_00000000-0000-0000-0000-000000000002', true);

-- ponytail: reset search_path immediately after the AGE call — same shared-
-- connection leak as the up migration (see its RESET search_path comment).
RESET search_path;

-- Delete in FK-dependent order (reverse of seed)

DELETE FROM timeline_events
WHERE universe_id = '00000000-0000-0000-0000-000000000002'
  AND id >= '00000000-0000-0000-0000-000000000400'
  AND id <= '00000000-0000-0000-0000-000000000407';

DELETE FROM plot_holes
WHERE universe_id = '00000000-0000-0000-0000-000000000002'
  AND id >= '00000000-0000-0000-0000-000000000500'
  AND id <= '00000000-0000-0000-0000-000000000503';

DELETE FROM contradictions
WHERE universe_id = '00000000-0000-0000-0000-000000000002'
  AND id >= '00000000-0000-0000-0000-000000000300'
  AND id <= '00000000-0000-0000-0000-000000000305';

DELETE FROM entity_embeddings
WHERE entity_id IN (
    SELECT id FROM entities
    WHERE universe_id = '00000000-0000-0000-0000-000000000002'
      AND id >= '00000000-0000-0000-0000-000000000100'
      AND id <= '00000000-0000-0000-0000-000000000142'
);

DELETE FROM entity_mentions
WHERE id >= '00000000-0000-0000-0000-000000000200'
  AND id <= '00000000-0000-0000-0000-000000000237';

DELETE FROM entities
WHERE universe_id = '00000000-0000-0000-0000-000000000002'
  AND id >= '00000000-0000-0000-0000-000000000100'
  AND id <= '00000000-0000-0000-0000-000000000142';

DELETE FROM chapters
WHERE work_id IN (
    '00000000-0000-0000-0000-000000000010',
    '00000000-0000-0000-0000-000000000011',
    '00000000-0000-0000-0000-000000000012'
);

DELETE FROM works
WHERE universe_id = '00000000-0000-0000-0000-000000000002'
  AND id >= '00000000-0000-0000-0000-000000000010'
  AND id <= '00000000-0000-0000-0000-000000000012';

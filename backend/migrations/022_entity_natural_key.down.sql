-- Recreating the Sprint 1 index is safe only while no valid, cross-type name
-- collisions have been created under the new natural-key rule. Refuse a
-- destructive rollback instead of silently deleting one of those entities.
BEGIN;

DO $$
BEGIN
	IF to_regclass('public.entities') IS NULL THEN
		RETURN;
	END IF;
	IF to_regclass('public.entities_universe_name_type_key') IS NULL THEN
		RETURN;
	END IF;

    IF EXISTS (
        SELECT 1
        FROM entities
        GROUP BY universe_id, LOWER(name)
        HAVING COUNT(DISTINCT type) > 1
    ) THEN
        RAISE EXCEPTION 'cannot restore idx_entities_name_unique while cross-type entity names exist';
    END IF;

	DROP INDEX IF EXISTS entities_universe_name_type_key;
	IF to_regclass('public.idx_entities_name_unique') IS NULL THEN
		CREATE UNIQUE INDEX idx_entities_name_unique ON entities (universe_id, LOWER(name));
	END IF;
END $$;

COMMIT;

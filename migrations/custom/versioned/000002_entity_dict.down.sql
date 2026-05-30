-- Custom: 000002_entity_dict (down)

DO $$ BEGIN RAISE NOTICE '[Custom 000002] Dropping entity_dict table'; END $$;

DROP TABLE IF EXISTS entity_dict;

DO $$ BEGIN RAISE NOTICE '[Custom 000002] entity_dict table dropped'; END $$;

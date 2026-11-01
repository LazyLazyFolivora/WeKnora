-- Custom: 000001_graph_sync (rollback)

DO $$ BEGIN RAISE NOTICE '[Custom 000001] Dropping graph_relations and graph_entities tables'; END $$;

DROP TABLE IF EXISTS graph_relations;
DROP TABLE IF EXISTS graph_entities;

DO $$ BEGIN RAISE NOTICE '[Custom 000001] Rollback complete'; END $$;

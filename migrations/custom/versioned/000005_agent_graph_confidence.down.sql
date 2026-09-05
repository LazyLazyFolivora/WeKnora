-- Custom: 000005_agent_graph_confidence (rollback)

DO $$ BEGIN RAISE NOTICE '[Custom 000005] Dropping confidence/strength columns'; END $$;

ALTER TABLE agent_graph_nodes DROP COLUMN IF EXISTS confidence;
ALTER TABLE agent_graph_edges DROP COLUMN IF EXISTS strength;

DO $$ BEGIN RAISE NOTICE '[Custom 000005] confidence/strength dropped'; END $$;

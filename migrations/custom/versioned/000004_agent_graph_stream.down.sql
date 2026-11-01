-- Custom: 000004_agent_graph_stream (rollback)

DO $$ BEGIN RAISE NOTICE '[Custom 000004] Dropping agent_graph_* tables'; END $$;

DROP TABLE IF EXISTS agent_graph_edges;
DROP TABLE IF EXISTS agent_graph_nodes;
DROP TABLE IF EXISTS agent_graph_events;
DROP TABLE IF EXISTS agent_graph_runs;

DO $$ BEGIN RAISE NOTICE '[Custom 000004] agent_graph_* tables dropped'; END $$;

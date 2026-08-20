-- Custom: 000005_agent_graph_confidence
-- Add node confidence (EntityPlanned.confidence) and edge strength
-- (RelationFound.strength) to the agent_graph projections.
--
-- 000004 already carries these columns for fresh databases; this migration is
-- for databases that ran 000004 before the columns existed. Custom migrations
-- are not version-tracked and re-run on every startup, so each statement must
-- be idempotent (Postgres supports ADD COLUMN IF NOT EXISTS).

DO $$ BEGIN RAISE NOTICE '[Custom 000005] Adding confidence/strength to agent_graph tables'; END $$;

ALTER TABLE agent_graph_nodes ADD COLUMN IF NOT EXISTS confidence DOUBLE PRECISION;
ALTER TABLE agent_graph_edges ADD COLUMN IF NOT EXISTS strength DOUBLE PRECISION NOT NULL DEFAULT 0.5;

DO $$ BEGIN RAISE NOTICE '[Custom 000005] confidence/strength added'; END $$;

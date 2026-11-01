-- Custom: 000003_remove_graph_kb_id
-- Remove knowledge_base_id from graph_entities and graph_relations.
-- The graph is now tenant-scoped instead of KB-scoped.

DO $$ BEGIN RAISE NOTICE '[Custom 000003] Removing knowledge_base_id from graph tables'; END $$;

-- graph_entities: drop old indexes
DROP INDEX IF EXISTS idx_graph_entities_unique_source;
DROP INDEX IF EXISTS idx_graph_entities_doc;
DROP INDEX IF EXISTS idx_graph_entities_kb;

-- graph_entities: drop column
ALTER TABLE graph_entities DROP COLUMN IF EXISTS knowledge_base_id;

-- graph_entities: create new indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_graph_entities_unique_source
    ON graph_entities (tenant_id, source_entity_id);

CREATE INDEX IF NOT EXISTS idx_graph_entities_doc
    ON graph_entities (tenant_id, source_doc_uuid);

CREATE INDEX IF NOT EXISTS idx_graph_entities_tenant
    ON graph_entities (tenant_id);

-- graph_relations: drop old indexes
DROP INDEX IF EXISTS idx_graph_relations_unique_source;
DROP INDEX IF EXISTS idx_graph_relations_doc;
DROP INDEX IF EXISTS idx_graph_relations_kb;
DROP INDEX IF EXISTS idx_graph_relations_endpoints;

-- graph_relations: drop column
ALTER TABLE graph_relations DROP COLUMN IF EXISTS knowledge_base_id;

-- graph_relations: create new indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_graph_relations_unique_source
    ON graph_relations (tenant_id, source_relation_id);

CREATE INDEX IF NOT EXISTS idx_graph_relations_doc
    ON graph_relations (tenant_id, source_doc_uuid);

CREATE INDEX IF NOT EXISTS idx_graph_relations_tenant
    ON graph_relations (tenant_id);

CREATE INDEX IF NOT EXISTS idx_graph_relations_endpoints
    ON graph_relations (tenant_id, from_entity_id, to_entity_id);

DO $$ BEGIN RAISE NOTICE '[Custom 000003] knowledge_base_id removed from graph tables'; END $$;

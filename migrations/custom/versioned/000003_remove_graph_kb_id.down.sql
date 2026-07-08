-- Custom: 000003_remove_graph_kb_id (rollback)
-- Restore knowledge_base_id on graph_entities and graph_relations.

DO $$ BEGIN RAISE NOTICE '[Custom 000003] Restoring knowledge_base_id on graph tables'; END $$;

-- graph_entities: add back column
ALTER TABLE graph_entities ADD COLUMN IF NOT EXISTS knowledge_base_id VARCHAR(36) NOT NULL DEFAULT '';

-- graph_entities: drop new indexes
DROP INDEX IF EXISTS idx_graph_entities_unique_source;
DROP INDEX IF EXISTS idx_graph_entities_doc;
DROP INDEX IF EXISTS idx_graph_entities_tenant;

-- graph_entities: recreate old indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_graph_entities_unique_source
    ON graph_entities (tenant_id, knowledge_base_id, source_entity_id);

CREATE INDEX IF NOT EXISTS idx_graph_entities_doc
    ON graph_entities (tenant_id, knowledge_base_id, source_doc_uuid);

CREATE INDEX IF NOT EXISTS idx_graph_entities_kb
    ON graph_entities (tenant_id, knowledge_base_id);

-- graph_relations: add back column
ALTER TABLE graph_relations ADD COLUMN IF NOT EXISTS knowledge_base_id VARCHAR(36) NOT NULL DEFAULT '';

-- graph_relations: drop new indexes
DROP INDEX IF EXISTS idx_graph_relations_unique_source;
DROP INDEX IF EXISTS idx_graph_relations_doc;
DROP INDEX IF EXISTS idx_graph_relations_tenant;
DROP INDEX IF EXISTS idx_graph_relations_endpoints;

-- graph_relations: recreate old indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_graph_relations_unique_source
    ON graph_relations (tenant_id, knowledge_base_id, source_relation_id);

CREATE INDEX IF NOT EXISTS idx_graph_relations_doc
    ON graph_relations (tenant_id, knowledge_base_id, source_doc_uuid);

CREATE INDEX IF NOT EXISTS idx_graph_relations_kb
    ON graph_relations (tenant_id, knowledge_base_id);

CREATE INDEX IF NOT EXISTS idx_graph_relations_endpoints
    ON graph_relations (tenant_id, knowledge_base_id, from_entity_id, to_entity_id);

DO $$ BEGIN RAISE NOTICE '[Custom 000003] knowledge_base_id restored on graph tables'; END $$;

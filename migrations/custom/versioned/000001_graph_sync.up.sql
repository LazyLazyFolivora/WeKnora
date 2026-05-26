-- Custom: 000001_graph_sync
-- Add graph_entities and graph_relations tables for DB-backed knowledge graph sync.
-- The database is the source of truth; Neo4j is a derived projection.

DO $$ BEGIN RAISE NOTICE '[Custom 000001] Creating graph_entities and graph_relations tables'; END $$;

CREATE TABLE IF NOT EXISTS graph_entities (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    source_entity_id VARCHAR(255) NOT NULL,
    entity_type VARCHAR(100) NOT NULL,
    entity_name VARCHAR(500) NOT NULL,
    entity_data JSONB NOT NULL DEFAULT '{}',
    source_doc_uuid VARCHAR(255) DEFAULT '',
    source_site VARCHAR(255) DEFAULT '',
    source_text TEXT DEFAULT '',
    confidence_score DOUBLE PRECISION,
    confidence_reason TEXT DEFAULT '',
    review_status VARCHAR(50) DEFAULT 'pending',
    sync_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    neo4j_node_id VARCHAR(255) DEFAULT '',
    is_deleted BOOLEAN NOT NULL DEFAULT false,
    synced_at TIMESTAMPTZ,
    sync_error TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_graph_entities_unique_source
    ON graph_entities (tenant_id, knowledge_base_id, source_entity_id);

CREATE INDEX IF NOT EXISTS idx_graph_entities_sync
    ON graph_entities (sync_status, updated_at);

CREATE INDEX IF NOT EXISTS idx_graph_entities_doc
    ON graph_entities (tenant_id, knowledge_base_id, source_doc_uuid);

CREATE INDEX IF NOT EXISTS idx_graph_entities_kb
    ON graph_entities (tenant_id, knowledge_base_id);

CREATE TABLE IF NOT EXISTS graph_relations (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    source_relation_id VARCHAR(255) NOT NULL,
    from_entity_id VARCHAR(255) NOT NULL,
    to_entity_id VARCHAR(255) NOT NULL,
    relation_type VARCHAR(100) NOT NULL,
    relation_props JSONB NOT NULL DEFAULT '{}',
    source_doc_uuid VARCHAR(255) DEFAULT '',
    source_site VARCHAR(255) DEFAULT '',
    source_text TEXT DEFAULT '',
    confidence_score DOUBLE PRECISION,
    confidence_reason TEXT DEFAULT '',
    review_status VARCHAR(50) DEFAULT 'pending',
    sync_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    neo4j_relation_id VARCHAR(255) DEFAULT '',
    is_deleted BOOLEAN NOT NULL DEFAULT false,
    synced_at TIMESTAMPTZ,
    sync_error TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_graph_relations_unique_source
    ON graph_relations (tenant_id, knowledge_base_id, source_relation_id);

CREATE INDEX IF NOT EXISTS idx_graph_relations_sync
    ON graph_relations (sync_status, updated_at);

CREATE INDEX IF NOT EXISTS idx_graph_relations_doc
    ON graph_relations (tenant_id, knowledge_base_id, source_doc_uuid);

CREATE INDEX IF NOT EXISTS idx_graph_relations_kb
    ON graph_relations (tenant_id, knowledge_base_id);

CREATE INDEX IF NOT EXISTS idx_graph_relations_endpoints
    ON graph_relations (tenant_id, knowledge_base_id, from_entity_id, to_entity_id);

DO $$ BEGIN RAISE NOTICE '[Custom 000001] graph_entities and graph_relations created'; END $$;

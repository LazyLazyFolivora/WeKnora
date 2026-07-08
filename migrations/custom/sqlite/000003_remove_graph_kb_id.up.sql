-- Custom: 000003_remove_graph_kb_id (SQLite)
-- Remove knowledge_base_id from graph_entities and graph_relations.
-- SQLite does not support DROP COLUMN directly, so we rebuild the tables.

-- graph_entities: create new table without knowledge_base_id
CREATE TABLE graph_entities_new (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    source_entity_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_name TEXT NOT NULL,
    entity_data TEXT NOT NULL DEFAULT '{}',
    source_doc_uuid TEXT DEFAULT '',
    source_site TEXT DEFAULT '',
    source_text TEXT DEFAULT '',
    confidence_score REAL,
    confidence_reason TEXT DEFAULT '',
    review_status TEXT DEFAULT 'pending',
    sync_status TEXT NOT NULL DEFAULT 'pending',
    neo4j_node_id TEXT DEFAULT '',
    is_deleted INTEGER NOT NULL DEFAULT 0,
    synced_at DATETIME,
    sync_error TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

INSERT INTO graph_entities_new
SELECT id, tenant_id, source_entity_id, entity_type, entity_name, entity_data,
       source_doc_uuid, source_site, source_text, confidence_score, confidence_reason,
       review_status, sync_status, neo4j_node_id, is_deleted, synced_at, sync_error,
       created_at, updated_at, deleted_at
FROM graph_entities;

DROP TABLE graph_entities;
ALTER TABLE graph_entities_new RENAME TO graph_entities;

CREATE UNIQUE INDEX IF NOT EXISTS idx_graph_entities_unique_source
    ON graph_entities (tenant_id, source_entity_id);

CREATE INDEX IF NOT EXISTS idx_graph_entities_sync
    ON graph_entities (sync_status, updated_at);

CREATE INDEX IF NOT EXISTS idx_graph_entities_doc
    ON graph_entities (tenant_id, source_doc_uuid);

CREATE INDEX IF NOT EXISTS idx_graph_entities_tenant
    ON graph_entities (tenant_id);

-- graph_relations: create new table without knowledge_base_id
CREATE TABLE graph_relations_new (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    source_relation_id TEXT NOT NULL,
    from_entity_id TEXT NOT NULL,
    to_entity_id TEXT NOT NULL,
    relation_type TEXT NOT NULL,
    relation_props TEXT NOT NULL DEFAULT '{}',
    source_doc_uuid TEXT DEFAULT '',
    source_site TEXT DEFAULT '',
    source_text TEXT DEFAULT '',
    confidence_score REAL,
    confidence_reason TEXT DEFAULT '',
    review_status TEXT DEFAULT 'pending',
    sync_status TEXT NOT NULL DEFAULT 'pending',
    neo4j_relation_id TEXT DEFAULT '',
    is_deleted INTEGER NOT NULL DEFAULT 0,
    synced_at DATETIME,
    sync_error TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

INSERT INTO graph_relations_new
SELECT id, tenant_id, source_relation_id, from_entity_id, to_entity_id, relation_type,
       relation_props, source_doc_uuid, source_site, source_text, confidence_score,
       confidence_reason, review_status, sync_status, neo4j_relation_id, is_deleted,
       synced_at, sync_error, created_at, updated_at, deleted_at
FROM graph_relations;

DROP TABLE graph_relations;
ALTER TABLE graph_relations_new RENAME TO graph_relations;

CREATE UNIQUE INDEX IF NOT EXISTS idx_graph_relations_unique_source
    ON graph_relations (tenant_id, source_relation_id);

CREATE INDEX IF NOT EXISTS idx_graph_relations_sync
    ON graph_relations (sync_status, updated_at);

CREATE INDEX IF NOT EXISTS idx_graph_relations_doc
    ON graph_relations (tenant_id, source_doc_uuid);

CREATE INDEX IF NOT EXISTS idx_graph_relations_tenant
    ON graph_relations (tenant_id);

CREATE INDEX IF NOT EXISTS idx_graph_relations_endpoints
    ON graph_relations (tenant_id, from_entity_id, to_entity_id);

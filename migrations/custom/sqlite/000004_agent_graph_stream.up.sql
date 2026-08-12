-- Custom: 000004_agent_graph_stream (SQLite)
-- Per-turn incremental knowledge graph produced by streaming MCP agents.

CREATE TABLE IF NOT EXISTS agent_graph_runs (
    id               TEXT PRIMARY KEY,
    tenant_id        INTEGER  NOT NULL,
    session_id       TEXT     NOT NULL,
    message_id       TEXT     NOT NULL,
    stream_key       TEXT     NOT NULL,
    tool_call_id     TEXT     NOT NULL DEFAULT '',
    status           TEXT     NOT NULL DEFAULT 'running',
    phase            TEXT     NOT NULL DEFAULT '',
    step             INTEGER  NOT NULL DEFAULT 0,
    entity_count     INTEGER  NOT NULL DEFAULT 0,
    relation_count   INTEGER  NOT NULL DEFAULT 0,
    last_seq         INTEGER  NOT NULL DEFAULT 0,
    last_msg_seq     INTEGER  NOT NULL DEFAULT 0,
    duration_seconds REAL,
    started_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at     DATETIME,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_graph_runs_stream ON agent_graph_runs (stream_key);
CREATE INDEX IF NOT EXISTS idx_agent_graph_runs_message ON agent_graph_runs (message_id);
CREATE INDEX IF NOT EXISTS idx_agent_graph_runs_tenant_message ON agent_graph_runs (tenant_id, message_id);

CREATE TABLE IF NOT EXISTS agent_graph_events (
    id           TEXT PRIMARY KEY,
    tenant_id    INTEGER  NOT NULL,
    session_id   TEXT     NOT NULL,
    message_id   TEXT     NOT NULL,
    stream_key   TEXT     NOT NULL,
    seq          INTEGER  NOT NULL,
    msg_seq      INTEGER  NOT NULL DEFAULT 0,
    event_type   TEXT     NOT NULL,
    event_id     TEXT     NOT NULL DEFAULT '',
    payload      TEXT     NOT NULL DEFAULT '{}',
    emitted_at   DATETIME,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_graph_events_dedup ON agent_graph_events (stream_key, seq);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_graph_events_msg_seq ON agent_graph_events (message_id, msg_seq);
CREATE INDEX IF NOT EXISTS idx_agent_graph_events_cursor ON agent_graph_events (message_id, msg_seq);

CREATE TABLE IF NOT EXISTS agent_graph_nodes (
    id            TEXT PRIMARY KEY,
    tenant_id     INTEGER  NOT NULL,
    session_id    TEXT     NOT NULL,
    message_id    TEXT     NOT NULL,
    stream_key    TEXT     NOT NULL,
    entity_name   TEXT     NOT NULL,
    entity_type   TEXT     NOT NULL DEFAULT '',
    status        TEXT     NOT NULL DEFAULT 'searching',
    source_kb     TEXT     NOT NULL DEFAULT '',
    observations  TEXT     NOT NULL DEFAULT '[]',
    first_seq     INTEGER  NOT NULL,
    last_seq      INTEGER  NOT NULL,
    first_msg_seq INTEGER  NOT NULL DEFAULT 0,
    last_msg_seq  INTEGER  NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_graph_nodes_unique ON agent_graph_nodes (message_id, entity_name);
CREATE INDEX IF NOT EXISTS idx_agent_graph_nodes_cursor ON agent_graph_nodes (message_id, last_msg_seq);

CREATE TABLE IF NOT EXISTS agent_graph_edges (
    id            TEXT PRIMARY KEY,
    tenant_id     INTEGER  NOT NULL,
    session_id    TEXT     NOT NULL,
    message_id    TEXT     NOT NULL,
    stream_key    TEXT     NOT NULL,
    source_entity TEXT     NOT NULL,
    target_entity TEXT     NOT NULL,
    relation_type TEXT     NOT NULL DEFAULT '',
    first_seq     INTEGER  NOT NULL,
    last_seq      INTEGER  NOT NULL,
    first_msg_seq INTEGER  NOT NULL DEFAULT 0,
    last_msg_seq  INTEGER  NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_graph_edges_unique
    ON agent_graph_edges (message_id, source_entity, target_entity, relation_type);
CREATE INDEX IF NOT EXISTS idx_agent_graph_edges_cursor ON agent_graph_edges (message_id, last_msg_seq);

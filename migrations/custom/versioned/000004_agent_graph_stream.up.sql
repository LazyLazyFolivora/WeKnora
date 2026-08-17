-- Custom: 000004_agent_graph_stream
-- Per-turn incremental knowledge graph produced by streaming MCP agents.
-- agent_graph_events is the append-only source of truth; nodes/edges are a
-- derived projection kept in sync on write, mirroring the graph_sync design.
--
-- stream_key = "<assistant_message_id>:<tool_call_id>"; it is the correlation
-- key sent to the MCP server as session_id. tool_call_id is kept alongside for
-- troubleshooting only, since LLM-generated ids are not reliably unique.

DO $$ BEGIN RAISE NOTICE '[Custom 000004] Creating agent_graph_* tables'; END $$;

CREATE TABLE IF NOT EXISTS agent_graph_runs (
    id               VARCHAR(36) PRIMARY KEY,
    tenant_id        INTEGER      NOT NULL,
    session_id       VARCHAR(36)  NOT NULL,
    message_id       VARCHAR(36)  NOT NULL,
    stream_key       VARCHAR(200) NOT NULL,
    tool_call_id     VARCHAR(128) NOT NULL DEFAULT '',
    status           VARCHAR(32)  NOT NULL DEFAULT 'running',
    phase            VARCHAR(64)  NOT NULL DEFAULT '',
    step             INTEGER      NOT NULL DEFAULT 0,
    entity_count     INTEGER      NOT NULL DEFAULT 0,
    relation_count   INTEGER      NOT NULL DEFAULT 0,
    last_seq         BIGINT       NOT NULL DEFAULT 0,
    last_msg_seq     BIGINT       NOT NULL DEFAULT 0,
    duration_seconds DOUBLE PRECISION,
    started_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_graph_runs_stream ON agent_graph_runs (stream_key);
CREATE INDEX IF NOT EXISTS idx_agent_graph_runs_message ON agent_graph_runs (message_id);
CREATE INDEX IF NOT EXISTS idx_agent_graph_runs_tenant_message ON agent_graph_runs (tenant_id, message_id);

CREATE TABLE IF NOT EXISTS agent_graph_events (
    id           VARCHAR(36) PRIMARY KEY,
    tenant_id    INTEGER      NOT NULL,
    session_id   VARCHAR(36)  NOT NULL,
    message_id   VARCHAR(36)  NOT NULL,
    stream_key   VARCHAR(200) NOT NULL,
    seq          BIGINT       NOT NULL,
    msg_seq      BIGINT       NOT NULL DEFAULT 0,
    event_type   VARCHAR(64)  NOT NULL,
    event_id     VARCHAR(64)  NOT NULL DEFAULT '',
    payload      JSONB        NOT NULL DEFAULT '{}',
    emitted_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_graph_events_dedup ON agent_graph_events (stream_key, seq);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_graph_events_msg_seq ON agent_graph_events (message_id, msg_seq);
CREATE INDEX IF NOT EXISTS idx_agent_graph_events_cursor ON agent_graph_events (message_id, msg_seq);

CREATE TABLE IF NOT EXISTS agent_graph_nodes (
    id           VARCHAR(36) PRIMARY KEY,
    tenant_id    INTEGER      NOT NULL,
    session_id   VARCHAR(36)  NOT NULL,
    message_id   VARCHAR(36)  NOT NULL,
    stream_key   VARCHAR(200) NOT NULL,
    entity_name  VARCHAR(500) NOT NULL,
    entity_type  VARCHAR(100) NOT NULL DEFAULT '',
    status       VARCHAR(32)  NOT NULL DEFAULT 'searching',  -- planned|searching|confirmed
    source_kb    VARCHAR(100) NOT NULL DEFAULT '',
    observations JSONB        NOT NULL DEFAULT '[]',
    first_seq    BIGINT       NOT NULL,
    last_seq     BIGINT       NOT NULL,
    first_msg_seq BIGINT      NOT NULL DEFAULT 0,
    last_msg_seq  BIGINT      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_graph_nodes_unique ON agent_graph_nodes (message_id, entity_name);
CREATE INDEX IF NOT EXISTS idx_agent_graph_nodes_cursor ON agent_graph_nodes (message_id, last_msg_seq);

CREATE TABLE IF NOT EXISTS agent_graph_edges (
    id            VARCHAR(36) PRIMARY KEY,
    tenant_id     INTEGER      NOT NULL,
    session_id    VARCHAR(36)  NOT NULL,
    message_id    VARCHAR(36)  NOT NULL,
    stream_key    VARCHAR(200) NOT NULL,
    source_entity VARCHAR(500) NOT NULL,
    target_entity VARCHAR(500) NOT NULL,
    relation_type VARCHAR(200) NOT NULL DEFAULT '',
    first_seq     BIGINT       NOT NULL,
    last_seq      BIGINT       NOT NULL,
    first_msg_seq BIGINT       NOT NULL DEFAULT 0,
    last_msg_seq  BIGINT       NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_graph_edges_unique
    ON agent_graph_edges (message_id, source_entity, target_entity, relation_type);
CREATE INDEX IF NOT EXISTS idx_agent_graph_edges_cursor ON agent_graph_edges (message_id, last_msg_seq);

DO $$ BEGIN RAISE NOTICE '[Custom 000004] agent_graph_* tables created'; END $$;

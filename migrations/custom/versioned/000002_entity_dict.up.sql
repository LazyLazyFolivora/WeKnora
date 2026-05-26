-- Custom: 000002_entity_dict
-- Mirror ZK's entity_dict table for PrimeKG copy initialization.

DO $$ BEGIN RAISE NOTICE '[Custom 000002] Creating entity_dict table'; END $$;

CREATE TABLE IF NOT EXISTS entity_dict (
    id BIGINT PRIMARY KEY,
    entity_type VARCHAR(100) NOT NULL,
    external_ids JSONB NOT NULL DEFAULT '{}',
    canonical_data JSONB NOT NULL DEFAULT '{}',
    canonical_source VARCHAR(255) NOT NULL DEFAULT '',
    is_deleted BOOLEAN NOT NULL DEFAULT false,
    synced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$ BEGIN RAISE NOTICE '[Custom 000002] entity_dict table created'; END $$;

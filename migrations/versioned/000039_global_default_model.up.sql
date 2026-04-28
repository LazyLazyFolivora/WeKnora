-- Migration: 000039_global_default_model
-- Description: Add is_global_default column to models table for cross-tenant global default model feature.
-- NOTE: The existing is_default column retains its per-tenant semantics and is NOT modified.
DO $$ BEGIN RAISE NOTICE '[Migration 000039] Applying global default model schema'; END $$;

ALTER TABLE models ADD COLUMN IF NOT EXISTS is_global_default BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN models.is_global_default IS
    'Whether this model is the global default for its type, visible to all non-admin users transparently. '
    'At most one global default per type. Distinct from is_default (per-tenant default).';

-- Partial unique index: at most one global default model per type (excluding soft-deleted rows)
CREATE UNIQUE INDEX IF NOT EXISTS idx_models_is_global_default_type
    ON models (type)
    WHERE is_global_default = TRUE AND deleted_at IS NULL;

DO $$ BEGIN RAISE NOTICE '[Migration 000039] Global default model schema applied successfully'; END $$;

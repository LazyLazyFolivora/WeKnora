-- Migration: 000039_global_default_model (rollback)
DO $$ BEGIN RAISE NOTICE '[Migration 000039 DOWN] Reverting global default model schema'; END $$;

DROP INDEX IF EXISTS idx_models_is_global_default_type;
ALTER TABLE models DROP COLUMN IF EXISTS is_global_default;

DO $$ BEGIN RAISE NOTICE '[Migration 000039 DOWN] Global default model schema reverted'; END $$;

-- Migration: 000038_public_knowledge_base (rollback)
-- Description: Reverse the public knowledge base schema changes.
DO $$ BEGIN RAISE NOTICE '[Migration 000038 DOWN] Reverting public knowledge base schema'; END $$;

DROP INDEX IF EXISTS idx_knowledge_bases_is_public;

ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS is_public;

ALTER TABLE users DROP COLUMN IF EXISTS is_admin;

DO $$ BEGIN RAISE NOTICE '[Migration 000038 DOWN] Public knowledge base schema reverted successfully'; END $$;

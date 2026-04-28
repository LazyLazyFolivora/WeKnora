-- Migration: 000038_public_knowledge_base
-- Description: Add is_admin to users table and is_public to knowledge_bases table
-- for public/private knowledge base feature.
DO $$ BEGIN RAISE NOTICE '[Migration 000038] Applying public knowledge base schema'; END $$;

-- ---------------------------------------------------------------------------
-- 1) Add is_admin column to users table
-- Admin users can create public knowledge bases visible to all members.
-- ---------------------------------------------------------------------------
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN users.is_admin IS 'Whether the user has admin privileges. Admin users can create public knowledge bases.';

-- ---------------------------------------------------------------------------
-- 2) Add is_public column to knowledge_bases table
-- Public knowledge bases are visible to all tenant members (read-only for non-owners).
-- ---------------------------------------------------------------------------
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN knowledge_bases.is_public IS 'Whether this knowledge base is visible to all tenant members. Public KBs are created by admin users and are read-only for non-owners.';

-- Create index for efficient public KB queries
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_is_public
    ON knowledge_bases (is_public)
    WHERE is_public = TRUE AND is_temporary = FALSE;

DO $$ BEGIN RAISE NOTICE '[Migration 000038] Public knowledge base schema applied successfully'; END $$;

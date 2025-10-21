-- Migration: Add title column to groups table
-- Feature: 013-mygroups-command
-- Date: 2025-10-21
-- Purpose: Store group names for display in /mygroups command (FR-004)

-- Add title column to groups table
ALTER TABLE groups
ADD COLUMN title TEXT DEFAULT NULL;

-- Create index for alphabetical sorting (NULLS LAST optimization)
CREATE INDEX idx_groups_title ON groups(title) WHERE title IS NOT NULL;

-- Comments for documentation
COMMENT ON COLUMN groups.title IS 'Group display name (may be NULL if not yet captured)';

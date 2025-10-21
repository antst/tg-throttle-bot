-- Rollback: Remove title column from groups table
-- Feature: 013-mygroups-command
-- Date: 2025-10-21

-- Remove index
DROP INDEX IF EXISTS idx_groups_title;

-- Remove title column
ALTER TABLE groups
DROP COLUMN IF EXISTS title;

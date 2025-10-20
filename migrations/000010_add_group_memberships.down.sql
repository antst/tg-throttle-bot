-- Rollback: Remove group_memberships table
-- Feature: 011-proactive-member-sync
-- Date: 2025-10-20

DROP TRIGGER IF EXISTS trg_group_memberships_updated_at ON group_memberships;
DROP FUNCTION IF EXISTS update_group_memberships_updated_at();
DROP INDEX IF EXISTS idx_group_memberships_stale;
DROP INDEX IF EXISTS idx_group_memberships_chat_active;
DROP INDEX IF EXISTS idx_group_memberships_updated_at;
DROP INDEX IF EXISTS idx_group_memberships_status;
DROP INDEX IF EXISTS idx_group_memberships_user_id;
DROP INDEX IF EXISTS idx_group_memberships_chat_id;
DROP TABLE IF EXISTS group_memberships;

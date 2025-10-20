-- Rollback: Remove sync_metadata and sync_events tables
-- Feature: 011-proactive-member-sync
-- Date: 2025-10-20

DROP TRIGGER IF EXISTS trg_sync_metadata_updated_at ON sync_metadata;
DROP FUNCTION IF EXISTS update_sync_metadata_updated_at();
DROP INDEX IF EXISTS idx_sync_events_status;
DROP INDEX IF EXISTS idx_sync_events_metadata_id;
DROP INDEX IF EXISTS idx_sync_events_started_at;
DROP INDEX IF EXISTS idx_sync_metadata_failed;
DROP INDEX IF EXISTS idx_sync_metadata_next_sync;
DROP TABLE IF EXISTS sync_events;
DROP TABLE IF EXISTS sync_metadata;

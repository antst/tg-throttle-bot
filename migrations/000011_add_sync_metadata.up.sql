-- Migration: Add sync_metadata and sync_events tables
-- Feature: 011-proactive-member-sync
-- Date: 2025-10-20

-- Sync metadata table (one row per group)
CREATE TABLE IF NOT EXISTS sync_metadata (
    id BIGSERIAL PRIMARY KEY,
    
    -- Foreign key (referencing existing schema: groups.chat_id)
    chat_id BIGINT NOT NULL UNIQUE REFERENCES groups(chat_id) ON DELETE CASCADE,
    
    -- Sync status
    sync_status TEXT NOT NULL CHECK (sync_status IN ('pending', 'in_progress', 'completed', 'failed')),
    
    -- Timestamps
    last_sync_at TIMESTAMPTZ,
    next_sync_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Stats (for metrics)
    total_members INTEGER DEFAULT 0,
    failed_attempts INTEGER DEFAULT 0,
    last_error TEXT
);

-- Index for "groups needing sync" query (periodic sync job)
CREATE INDEX idx_sync_metadata_next_sync 
    ON sync_metadata(next_sync_at) 
    WHERE sync_status != 'in_progress';

-- Index for failed sync monitoring
CREATE INDEX idx_sync_metadata_failed 
    ON sync_metadata(sync_status, failed_attempts) 
    WHERE sync_status = 'failed';

-- Sync events table (audit trail for sync operations)
CREATE TABLE IF NOT EXISTS sync_events (
    id BIGSERIAL PRIMARY KEY,
    
    -- Foreign key
    metadata_id BIGINT NOT NULL REFERENCES sync_metadata(id) ON DELETE CASCADE,
    
    -- Event details
    event_type TEXT NOT NULL CHECK (event_type IN ('initial_sync', 'periodic_sync', 'notification_processed')),
    
    -- Timestamps
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    
    -- Result
    status TEXT NOT NULL CHECK (status IN ('started', 'completed', 'failed')),
    error_message TEXT,
    
    -- Stats
    members_processed INTEGER DEFAULT 0,
    members_added INTEGER DEFAULT 0,
    members_updated INTEGER DEFAULT 0,
    members_removed INTEGER DEFAULT 0
);

-- Index for recent sync events (monitoring dashboard)
CREATE INDEX idx_sync_events_started_at ON sync_events(started_at DESC);
CREATE INDEX idx_sync_events_metadata_id ON sync_events(metadata_id);
CREATE INDEX idx_sync_events_status ON sync_events(status) WHERE status = 'failed';

-- Trigger to auto-update sync_metadata.updated_at
CREATE OR REPLACE FUNCTION update_sync_metadata_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_sync_metadata_updated_at
    BEFORE UPDATE ON sync_metadata
    FOR EACH ROW
    EXECUTE FUNCTION update_sync_metadata_updated_at();

-- Comments
COMMENT ON TABLE sync_metadata IS 'Tracks sync status for each group (one row per group)';
COMMENT ON TABLE sync_events IS 'Audit trail for sync operations (multiple rows per group)';
COMMENT ON COLUMN sync_metadata.sync_status IS 'Current sync state: pending, in_progress, completed, failed';
COMMENT ON COLUMN sync_metadata.next_sync_at IS 'When the next periodic sync should run (every 24h)';
COMMENT ON COLUMN sync_events.event_type IS 'Type of sync: initial_sync, periodic_sync, notification_processed';

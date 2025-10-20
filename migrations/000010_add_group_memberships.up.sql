-- Migration: Add group_memberships table for proactive member tracking
-- Feature: 011-proactive-member-sync
-- Date: 2025-10-20

-- Main membership table (soft delete pattern)
CREATE TABLE IF NOT EXISTS group_memberships (
    id BIGSERIAL PRIMARY KEY,
    
    -- Foreign keys (referencing existing schema: groups.chat_id and users.user_id)
    chat_id BIGINT NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    
    -- Membership status
    status TEXT NOT NULL CHECK (status IN ('active', 'left', 'kicked', 'banned')),
    
    -- Timestamps
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Metadata
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    can_send_messages BOOLEAN NOT NULL DEFAULT TRUE,
    
    -- Ensure unique active membership per user per group
    CONSTRAINT unique_active_membership UNIQUE (chat_id, user_id)
);

-- Indexes for common queries
CREATE INDEX idx_group_memberships_chat_id ON group_memberships(chat_id);
CREATE INDEX idx_group_memberships_user_id ON group_memberships(user_id);
CREATE INDEX idx_group_memberships_status ON group_memberships(status);
CREATE INDEX idx_group_memberships_updated_at ON group_memberships(updated_at);

-- Composite index for "active members in group" query (most common)
CREATE INDEX idx_group_memberships_chat_active 
    ON group_memberships(chat_id, status) 
    WHERE status = 'active';

-- Index for stale record detection (updated_at > 48h ago)
CREATE INDEX idx_group_memberships_stale 
    ON group_memberships(updated_at) 
    WHERE status = 'active';

-- Trigger to auto-update updated_at on row modification
CREATE OR REPLACE FUNCTION update_group_memberships_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_group_memberships_updated_at
    BEFORE UPDATE ON group_memberships
    FOR EACH ROW
    EXECUTE FUNCTION update_group_memberships_updated_at();

-- Comments for documentation
COMMENT ON TABLE group_memberships IS 'Tracks group membership status with soft delete pattern';
COMMENT ON COLUMN group_memberships.status IS 'Membership status: active, left, kicked, banned';
COMMENT ON COLUMN group_memberships.joined_at IS 'First join timestamp (never updated on rejoin)';
COMMENT ON COLUMN group_memberships.left_at IS 'Most recent leave/kick timestamp';
COMMENT ON COLUMN group_memberships.updated_at IS 'Last status change (for idempotent updates)';
COMMENT ON COLUMN group_memberships.is_admin IS 'True if user is admin/creator in the group';
COMMENT ON COLUMN group_memberships.can_send_messages IS 'True if user can send messages (not restricted)';

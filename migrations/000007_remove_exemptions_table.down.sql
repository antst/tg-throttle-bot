-- Migration: 000007_remove_exemptions_table (DOWN)
-- Purpose: Restore exemptions table if needed
-- Note: This restores schema only - table will be empty after rollback

CREATE TABLE IF NOT EXISTS exemptions (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    user_id BIGINT,
    role VARCHAR(20),
    window_slot CHAR(1) CHECK (window_slot IN ('a', 'b', 'c') OR window_slot IS NULL),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by BIGINT NOT NULL,
    FOREIGN KEY (chat_id) REFERENCES groups(chat_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT exemption_type CHECK (
        (user_id IS NOT NULL AND role IS NULL) OR
        (user_id IS NULL AND role IS NOT NULL)
    ),
    CONSTRAINT valid_role CHECK (role IS NULL OR role IN ('admin', 'moderator'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_exemptions_user_window ON exemptions(chat_id, user_id, window_slot) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_exemptions_role_window ON exemptions(chat_id, role, window_slot) WHERE role IS NOT NULL;

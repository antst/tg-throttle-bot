-- Complete Telegram Bot Schema with Multi-Window Rate Limiting
-- Single consolidated migration for clean deployments

-- Users Table (with username support)
CREATE TABLE IF NOT EXISTS users (
    user_id BIGINT PRIMARY KEY,
    username VARCHAR(255),
    first_seen TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_lower ON users(LOWER(username)) WHERE username IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_last_seen ON users(last_seen);

-- Groups Table
CREATE TABLE IF NOT EXISTS groups (
    chat_id BIGINT PRIMARY KEY,
    paused BOOLEAN NOT NULL DEFAULT FALSE,
    resume_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_groups_paused ON groups(paused) WHERE paused = TRUE;

-- Window Slots Table
CREATE TABLE IF NOT EXISTS window_slots (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    slot_id CHAR(1) NOT NULL CHECK (slot_id IN ('a', 'b', 'c')),
    char_limit INTEGER NOT NULL CHECK (char_limit > 0 AND char_limit <= 10000000),
    duration_value INTEGER NOT NULL CHECK (duration_value > 0 AND duration_value <= 365),
    duration_unit VARCHAR(10) NOT NULL CHECK (duration_unit IN ('minute', 'hour', 'day')),
    window_duration INTEGER NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (chat_id) REFERENCES groups(chat_id) ON DELETE CASCADE,
    UNIQUE(chat_id, slot_id)
);

CREATE INDEX IF NOT EXISTS idx_window_slots_chat ON window_slots(chat_id);
CREATE INDEX IF NOT EXISTS idx_window_slots_enabled ON window_slots(chat_id, enabled) WHERE enabled = TRUE;

-- Simple Messages Table
CREATE TABLE IF NOT EXISTS simple_messages (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    slot_id CHAR(1) NOT NULL CHECK (slot_id IN ('a', 'b', 'c')),
    char_count INTEGER NOT NULL CHECK (char_count >= 0),
    sent_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (chat_id) REFERENCES groups(chat_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_simple_messages_window_lookup ON simple_messages(user_id, chat_id, slot_id, sent_at DESC);
CREATE INDEX IF NOT EXISTS idx_simple_messages_cleanup ON simple_messages(sent_at);

-- User Overrides Table
CREATE TABLE IF NOT EXISTS user_overrides (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    override_state BOOLEAN,
    reason TEXT,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (chat_id) REFERENCES groups(chat_id) ON DELETE CASCADE,
    UNIQUE(chat_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_overrides_lookup ON user_overrides(chat_id, user_id);
CREATE INDEX IF NOT EXISTS idx_user_overrides_expires ON user_overrides(expires_at) WHERE expires_at IS NOT NULL;

-- Exemptions Table
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

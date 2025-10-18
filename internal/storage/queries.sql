-- Complete SQLC queries for all Storage interface methods

-- ============================================================================
-- LEGACY Message Tracking Queries (DISABLED - table removed in migration 000003)
-- ============================================================================

-- LEGACY: GetUserCurrentUsage - table 'messages' removed
-- LEGACY: StoreMessage - table 'messages' removed
-- LEGACY: DeleteOldMessages - table 'messages' removed
-- LEGACY: GetUserMessages - table 'messages' removed
-- LEGACY: DeleteUserMessages - table 'messages' removed

-- ============================================================================
-- User Management Queries
-- ============================================================================

-- name: UpsertUser :one
-- Create or update a user record
INSERT INTO users (user_id, username, first_seen, last_seen)
VALUES ($1, $2, NOW(), NOW())
ON CONFLICT (user_id) DO UPDATE
SET username = EXCLUDED.username,
    last_seen = NOW()
RETURNING *;

-- ============================================================================
-- LEGACY Restriction Management Queries (DISABLED - table removed in migration 000003)
-- ============================================================================

-- LEGACY: GetActiveRestriction - table 'restrictions' removed
-- LEGACY: GetAllActiveRestrictions - table 'restrictions' removed
-- LEGACY: CreateRestriction - table 'restrictions' removed
-- LEGACY: UpdateRestriction - table 'restrictions' removed
-- LEGACY: GetUserRestrictionHistory - table 'restrictions' removed
-- LEGACY: GetRestrictedUsers - table 'restrictions' removed

-- ============================================================================
-- REMOVED: Exemption Queries (table removed in migration 000007)
-- ============================================================================
-- REMOVED: IsUserExempt - exemptions table removed (use user_overrides instead)
-- REMOVED: RemoveExemption - exemptions table removed
-- REMOVED: IsUserExemptFromWindow - exemptions table removed (see below)

-- ============================================================================
-- Warning State Management Queries
-- ============================================================================

-- LEGACY: GetWarningState - table 'warning_states' removed
-- LEGACY: UpsertWarningState - table 'warning_states' removed
-- LEGACY: DeleteOldWarningStates - table 'warning_states' removed
-- LEGACY: DeleteUserWarningStates - table 'warning_states' removed

-- ============================================================================
-- Group Configuration Queries
-- ============================================================================

-- name: GetGroupConfig :one
-- Get rate limit configuration for a group
SELECT * FROM groups
WHERE chat_id = $1;

-- name: GetAllGroups :many
-- Get all configured groups (simple messages design - no enabled column)
SELECT chat_id FROM groups
ORDER BY created_at DESC;

-- LEGACY: UpsertGroupConfig - columns char_limit, duration_value, duration_unit, window_duration removed from groups table

-- name: SetGroupPaused :exec
-- Pause or unpause rate limiting for a group
UPDATE groups
SET paused = $2,
    resume_at = $3,
    updated_at = NOW()
WHERE chat_id = $1;

-- name: IsGroupPaused :one
-- Check if a group's rate limiting is paused
SELECT paused, resume_at FROM groups
WHERE chat_id = $1;

-- ============================================================================
-- LEGACY Statistics Queries (DISABLED - table 'messages' removed)
-- ============================================================================

-- LEGACY: GetGroupStats - table 'messages' removed

-- ============================================================================
-- Multi-Window Configuration Queries (Feature 006)
-- ============================================================================

-- name: EnsureGroup :exec
-- Create group record if it doesn't exist (required for foreign key constraints)
INSERT INTO groups (chat_id)
VALUES ($1)
ON CONFLICT (chat_id) DO NOTHING;

-- name: EnsureUser :exec
-- Create user record if it doesn't exist (required for foreign key constraints)
-- Also updates username and last_seen on every call (for username harvesting)
INSERT INTO users (user_id, username, first_seen, last_seen)
VALUES ($1, $2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (user_id) DO UPDATE
SET username = EXCLUDED.username,
    last_seen = CURRENT_TIMESTAMP;

-- name: GetUserByUsername :one
-- Resolve @username to user_id (case-insensitive lookup)
SELECT user_id FROM users
WHERE LOWER(username) = LOWER($1) 
  AND username != '';

-- name: GetUsernameByUserID :one
-- Reverse lookup: get username from user_id (for display purposes)
SELECT username FROM users
WHERE user_id = $1;

-- name: CreateDefaultWindows :exec
-- Initialize default windows for a new group
-- Window A: 100 chars/1 minute (enabled by default)
-- Window B: 1000 chars/1 hour (disabled by default) 
-- Window C: 10000 chars/1 day (disabled by default)
INSERT INTO window_slots (chat_id, slot_id, char_limit, duration_value, duration_unit, window_duration, enabled)
VALUES 
    ($1, 'a', 100, 1, 'minute', 60, TRUE),
    ($1, 'b', 1000, 1, 'hour', 3600, FALSE),
    ($1, 'c', 10000, 1, 'day', 86400, FALSE)
ON CONFLICT (chat_id, slot_id) DO NOTHING;

-- name: GetEnabledWindows :many
-- Get all enabled windows for evaluation
SELECT id, slot_id, char_limit, duration_value, duration_unit, window_duration
FROM window_slots
WHERE chat_id = $1 AND enabled = TRUE
ORDER BY slot_id;

-- name: GetAllWindows :many
-- Get all windows for a chat (for /showwindows command)
SELECT *
FROM window_slots
WHERE chat_id = $1
ORDER BY slot_id;

-- name: GetWindowSlot :one
-- Get specific window configuration
SELECT *
FROM window_slots
WHERE chat_id = $1 AND slot_id = $2;

-- name: UpdateWindowSlot :exec
-- Update window configuration (/setwindow command)
INSERT INTO window_slots (chat_id, slot_id, char_limit, duration_value, duration_unit, window_duration, enabled, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
ON CONFLICT (chat_id, slot_id) 
DO UPDATE SET 
    char_limit = EXCLUDED.char_limit,
    duration_value = EXCLUDED.duration_value,
    duration_unit = EXCLUDED.duration_unit,
    window_duration = EXCLUDED.window_duration,
    enabled = EXCLUDED.enabled,
    updated_at = NOW();

-- name: SetWindowEnabled :exec
-- Enable or disable a window (/enablewindow, /disablewindow commands)
UPDATE window_slots
SET enabled = $3, updated_at = NOW()
WHERE chat_id = $1 AND slot_id = $2;

-- ============================================================================
-- Multi-Window State Tracking Queries
-- ============================================================================

-- GetWindowState removed - no longer needed with simple messages design
-- Current usage is calculated on-demand via GetWindowUsage

-- name: RecordMessage :exec
-- Record a message for sliding window calculation (one row per message, shared across all windows)
INSERT INTO simple_messages (user_id, chat_id, char_count, sent_at)
VALUES ($1, $2, $3, NOW());

-- name: GetWindowUsage :one
-- Calculate current usage for a window using sliding window
-- Returns sum of char_count for all messages within the time window
-- All windows share the same message log, filtered by time window
SELECT COALESCE(SUM(char_count), 0)::INTEGER as total_chars
FROM simple_messages
WHERE user_id = $1 
  AND chat_id = $2 
  AND sent_at > NOW() - ($3 || ' seconds')::INTERVAL;

-- name: CleanupOldMessages :exec
-- Delete messages older than retention period (cleanup job)
-- Should be run periodically to prevent unbounded growth
DELETE FROM simple_messages
WHERE sent_at < NOW() - ($1 || ' seconds')::INTERVAL;

-- name: ResetWindowMessages :exec
-- Reset all messages for a user in a chat (affects all windows)
DELETE FROM simple_messages
WHERE user_id = $1 AND chat_id = $2;

-- name: ResetWindowMessagesForAll :exec
-- Reset all messages for all users in a chat (affects all windows)
DELETE FROM simple_messages
WHERE chat_id = $1;

-- name: GetUserWindowStats :many
-- Get current usage statistics for all windows for a user (for /mystatus command)
-- All windows share the same message log, each window calculates usage based on its time window
SELECT 
    wsl.slot_id,
    wsl.char_limit,
    wsl.duration_value,
    wsl.duration_unit,
    wsl.window_duration,
    wsl.enabled,
    COALESCE(SUM(m.char_count), 0)::INTEGER as current_usage
FROM window_slots wsl
LEFT JOIN simple_messages m ON 
    m.chat_id = wsl.chat_id 
    AND m.user_id = $1
    AND m.sent_at > NOW() - (wsl.window_duration || ' seconds')::INTERVAL
WHERE wsl.chat_id = $2
GROUP BY wsl.slot_id, wsl.char_limit, wsl.duration_value, wsl.duration_unit, wsl.window_duration, wsl.enabled
ORDER BY wsl.slot_id;

-- Warning levels are now calculated on-demand from current usage
-- No need to store them in database

-- GetAllWindowStates removed - replaced by GetUserWindowStats which uses messages table

-- ============================================================================
-- Multi-Window Evaluation (No Restriction Table Needed!)
-- ============================================================================
-- With simple design, restriction is calculated on-demand:
-- User is "restricted" when: current_usage + new_message_length > limit
-- No need to store restriction state!

-- ============================================================================
-- Multi-Window Statistics Queries
-- ============================================================================

-- name: GetWindowStats :one
-- Get statistics for a specific window from simple_messages table
-- All windows share the same message log, filtered by the window's time range
SELECT 
    COUNT(DISTINCT m.user_id) AS active_users,
    COALESCE(AVG(m.char_count), 0)::INTEGER AS avg_usage
FROM simple_messages m
JOIN window_slots ws ON ws.chat_id = m.chat_id
WHERE m.chat_id = $1 AND ws.slot_id = $2
  AND m.sent_at > NOW() - (ws.window_duration || ' seconds')::INTERVAL;

-- GetMostViolatedWindow removed - no restriction table in simple design
-- Violations are not tracked; messages are just deleted when over limit

-- ============================================================================
-- REMOVED: Multi-Window Exemption Queries (table removed in migration 000007)
-- ============================================================================
-- REMOVED: IsUserExemptFromWindow - exemptions table removed
-- Note: Use user_overrides table with /override command instead

-- ============================================================================
-- User Overrides (3-State Manual Control)
-- ============================================================================

-- name: GetUserOverride :one
-- Get manual override state for a user (NULL=follow rate limiter, TRUE=whitelist, FALSE=blacklist)
SELECT override_state, reason, expires_at FROM user_overrides
WHERE chat_id = $1 AND user_id = $2;

-- name: SetUserOverride :exec
-- Set manual override state for a user
INSERT INTO user_overrides (chat_id, user_id, override_state, reason, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (chat_id, user_id) 
DO UPDATE SET 
    override_state = EXCLUDED.override_state,
    reason = EXCLUDED.reason,
    created_by = EXCLUDED.created_by,
    expires_at = EXCLUDED.expires_at,
    created_at = CURRENT_TIMESTAMP;

-- name: RemoveUserOverride :exec
-- Remove manual override (return to rate limiter control)
DELETE FROM user_overrides
WHERE chat_id = $1 AND user_id = $2;

-- name: GetAllOverrides :many
-- Get all manual overrides for a chat
SELECT user_id, override_state, reason, created_at, created_by, expires_at
FROM user_overrides
WHERE chat_id = $1
ORDER BY created_at DESC;

-- name: CleanupExpiredOverrides :exec
-- Remove expired overrides
DELETE FROM user_overrides
WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP;

-- ============================================================================
-- LEGACY Queries (DISABLED - columns removed from groups table in migration 000003)
-- ============================================================================

-- LEGACY: GetChatLimit - columns 'char_limit', 'window_duration' removed from groups table
-- LEGACY: SetChatLimit - columns removed from groups table

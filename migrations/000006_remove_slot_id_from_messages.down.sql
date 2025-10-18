-- Restore slot_id column to simple_messages table

-- Drop the optimized index
DROP INDEX IF EXISTS idx_simple_messages_lookup;

-- Add back the slot_id column
ALTER TABLE simple_messages ADD COLUMN slot_id VARCHAR(1) NOT NULL DEFAULT 'a';

-- Restore the original index with slot_id
CREATE INDEX idx_simple_messages_lookup ON simple_messages(user_id, chat_id, slot_id, sent_at DESC);

-- Remove slot_id from simple_messages table
-- Optimization: One message row should serve all windows, not duplicate per slot

-- Drop the index that includes slot_id
DROP INDEX IF EXISTS idx_simple_messages_lookup;

-- Drop the slot_id column
ALTER TABLE simple_messages DROP COLUMN slot_id;

-- Create new optimized index without slot_id
-- This index supports: WHERE user_id = X AND chat_id = Y AND sent_at > Z
CREATE INDEX idx_simple_messages_lookup ON simple_messages(user_id, chat_id, sent_at DESC);

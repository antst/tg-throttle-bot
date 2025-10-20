-- Add language column to users table for private chat responses (Feature 008)
-- Default: NULL (will fall back to group language or 'en')
ALTER TABLE users 
ADD COLUMN language VARCHAR(5) DEFAULT NULL;

-- Create index for language queries
CREATE INDEX idx_users_language ON users(language) WHERE language IS NOT NULL;

-- Add check constraint to ensure valid language codes
ALTER TABLE users 
ADD CONSTRAINT chk_users_language CHECK (language IS NULL OR language IN ('en', 'ru', 'nl'));

-- Add username column to groups table for @groupname parameter parsing (Feature 008)
-- Optional: not all groups have usernames
ALTER TABLE groups 
ADD COLUMN username VARCHAR(255) DEFAULT NULL;

-- Create unique index for username lookups (case-insensitive)
CREATE UNIQUE INDEX idx_groups_username_lower ON groups(LOWER(username)) WHERE username IS NOT NULL AND username != '';

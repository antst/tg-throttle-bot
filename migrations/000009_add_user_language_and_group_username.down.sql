-- Remove user language and group username columns (Feature 008 rollback)

-- Remove groups username column
DROP INDEX IF EXISTS idx_groups_username_lower;
ALTER TABLE groups 
DROP COLUMN IF EXISTS username;

-- Remove users language column
ALTER TABLE users 
DROP CONSTRAINT IF EXISTS chk_users_language;

DROP INDEX IF EXISTS idx_users_language;

ALTER TABLE users 
DROP COLUMN IF EXISTS language;

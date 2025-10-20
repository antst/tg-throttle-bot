-- Rollback language support
-- Remove language column from groups table

-- Drop check constraint
ALTER TABLE groups 
DROP CONSTRAINT IF EXISTS chk_groups_language;

-- Drop index
DROP INDEX IF EXISTS idx_groups_language;

-- Drop column
ALTER TABLE groups 
DROP COLUMN IF EXISTS language;

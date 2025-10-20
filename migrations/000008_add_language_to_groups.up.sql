-- Add language column to groups table
-- Default: 'en' (English)
-- Valid values: 'en', 'ru', 'nl'

ALTER TABLE groups 
ADD COLUMN language VARCHAR(5) NOT NULL DEFAULT 'en';

-- Create index for language queries
CREATE INDEX idx_groups_language ON groups(language);

-- Add check constraint to ensure valid language codes
ALTER TABLE groups 
ADD CONSTRAINT chk_groups_language CHECK (language IN ('en', 'ru', 'nl'));

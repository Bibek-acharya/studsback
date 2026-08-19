-- Migration: Rename institution_id to target_id and add target_type to user_follows
-- This allows following both institutions and universities

-- Add new columns
ALTER TABLE user_follows ADD COLUMN IF NOT EXISTS target_id BIGINT;
ALTER TABLE user_follows ADD COLUMN IF NOT EXISTS target_type VARCHAR(50) DEFAULT 'institution';

-- Copy data from institution_id to target_id
UPDATE user_follows SET target_id = institution_id WHERE target_id IS NULL;

-- Set target_type for existing records
UPDATE user_follows SET target_type = 'institution' WHERE target_type IS NULL OR target_type = '';

-- Drop old column and index (after data migration)
DROP INDEX IF EXISTS idx_follow_user_institution;

-- Create new unique index
CREATE UNIQUE INDEX IF NOT EXISTS idx_follow_user_target ON user_follows(user_id, target_id, target_type);

-- Make target_id NOT NULL after migration
ALTER TABLE user_follows ALTER COLUMN target_id SET NOT NULL;
ALTER TABLE user_follows ALTER COLUMN target_type SET NOT NULL;

-- Drop old column
ALTER TABLE user_follows DROP COLUMN IF EXISTS institution_id;

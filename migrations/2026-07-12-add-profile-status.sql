-- Add profile_status column and backfill existing published institutions
ALTER TABLE institution_users
ADD COLUMN IF NOT EXISTS profile_status VARCHAR(20) NOT NULL DEFAULT 'draft';

-- Backfill: set all existing institutions with public_profile=true to 'published'
UPDATE institution_users
SET profile_status = 'published'
WHERE id IN (
  SELECT institution_id
  FROM institution_settings
  WHERE public_profile = true
);

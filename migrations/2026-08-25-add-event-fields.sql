-- Add new fields to events table (education events)
ALTER TABLE events ADD COLUMN IF NOT EXISTS end_date TIMESTAMP;
ALTER TABLE events ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'upcoming';
ALTER TABLE events ADD COLUMN IF NOT EXISTS application_link TEXT;
ALTER TABLE events ADD COLUMN IF NOT EXISTS registration_deadline TIMESTAMP;

-- Add new fields to provider_events table
ALTER TABLE provider_events ADD COLUMN IF NOT EXISTS application_link TEXT;
ALTER TABLE provider_events ADD COLUMN IF NOT EXISTS registration_deadline TIMESTAMP;

-- Add new fields to institution_events table
ALTER TABLE institution_events ADD COLUMN IF NOT EXISTS application_link TEXT;
ALTER TABLE institution_events ADD COLUMN IF NOT EXISTS registration_deadline TIMESTAMP;

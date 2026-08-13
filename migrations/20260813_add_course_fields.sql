-- Migration: Add new fields to courses table and update institution_programs

-- 1. Create affiliations table (must exist before FK references)
CREATE TABLE IF NOT EXISTS affiliations (
    id SERIAL PRIMARY KEY,
    name VARCHAR NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 2. Add new fields to courses table
ALTER TABLE courses ADD COLUMN IF NOT EXISTS who_should_choose JSONB DEFAULT '[]';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS features JSONB DEFAULT '[]';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS eligibility_rows JSONB DEFAULT '[]';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS admission_steps JSONB DEFAULT '[]';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS subject_groups JSONB DEFAULT '[]';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS fee_items JSONB DEFAULT '[]';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS scholarship_desc TEXT DEFAULT '';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS scholarship_notes TEXT DEFAULT '';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS scholarships JSONB DEFAULT '[]';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS full_time_courses JSONB DEFAULT '[]';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS faqs JSONB DEFAULT '[]';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS banner_url VARCHAR DEFAULT '';

-- 3. Add affiliation_id FK
ALTER TABLE courses ADD COLUMN IF NOT EXISTS affiliation_id INTEGER REFERENCES affiliations(id);
-- Add non-university affiliation for secondary courses
ALTER TABLE courses ADD COLUMN IF NOT EXISTS non_university_affiliation VARCHAR DEFAULT '';

-- 4. Update institution_programs table
ALTER TABLE institution_programs ADD COLUMN IF NOT EXISTS who_should_choose JSONB DEFAULT '[]';
ALTER TABLE institution_programs ADD COLUMN IF NOT EXISTS features JSONB DEFAULT '[]';
ALTER TABLE institution_programs ADD COLUMN IF NOT EXISTS full_time_courses JSONB DEFAULT '[]';
ALTER TABLE institution_programs ADD COLUMN IF NOT EXISTS fee_items JSONB DEFAULT '[]';
ALTER TABLE institution_programs ADD COLUMN IF NOT EXISTS overrides JSONB DEFAULT '{}';
ALTER TABLE institution_programs ADD COLUMN IF NOT EXISTS nullified_fields JSONB DEFAULT '[]';

-- 5. Create course_approval_requests table
CREATE TABLE IF NOT EXISTS course_approval_requests (
    id SERIAL PRIMARY KEY,
    institution_id INTEGER NOT NULL REFERENCES institution_users(id) ON DELETE CASCADE,
    title VARCHAR NOT NULL,
    description TEXT DEFAULT '',
    duration VARCHAR DEFAULT '',
    level VARCHAR DEFAULT '',
    affiliation_id INTEGER REFERENCES affiliations(id),
    non_university_affiliation VARCHAR DEFAULT '',
    banner_url VARCHAR DEFAULT '',
    careers JSONB DEFAULT '[]',
    faqs JSONB DEFAULT '[]',
    eligibility_rows JSONB DEFAULT '[]',
    admission_steps JSONB DEFAULT '[]',
    subject_groups JSONB DEFAULT '[]',
    scholarship_desc TEXT DEFAULT '',
    scholarship_notes TEXT DEFAULT '',
    scholarships JSONB DEFAULT '[]',
    fee VARCHAR DEFAULT '',
    eligibility TEXT DEFAULT '',
    capacity INTEGER DEFAULT 0,
    who_should_choose JSONB DEFAULT '[]',
    features JSONB DEFAULT '[]',
    full_time_courses JSONB DEFAULT '[]',
    fee_items JSONB DEFAULT '[]',
    status VARCHAR(20) DEFAULT 'pending',
    reviewed_by INTEGER,
    reviewed_at TIMESTAMP,
    rejection_reason TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_course_approval_requests_institution ON course_approval_requests(institution_id);
CREATE INDEX IF NOT EXISTS idx_course_approval_requests_status ON course_approval_requests(status);

-- 6. Make global_course_id required (after data migration)
-- ALTER TABLE institution_programs ALTER COLUMN global_course_id SET NOT NULL;

CREATE TABLE IF NOT EXISTS landing_course_fields (
    id SERIAL PRIMARY KEY,
    field_of_study VARCHAR(255) NOT NULL UNIQUE,
    display_order INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS landing_course_institutions (
    id SERIAL PRIMARY KEY,
    field_id INTEGER NOT NULL REFERENCES landing_course_fields(id) ON DELETE CASCADE,
    institution_id INTEGER NOT NULL,
    institution_type VARCHAR(20) NOT NULL DEFAULT 'institution',
    institution_name VARCHAR(255) NOT NULL DEFAULT '',
    institution_logo TEXT DEFAULT '',
    slug VARCHAR(255) DEFAULT '',
    order_index INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(field_id, institution_id)
);

CREATE INDEX idx_landing_course_fields_order ON landing_course_fields(display_order);
CREATE INDEX idx_landing_course_institutions_field ON landing_course_institutions(field_id);

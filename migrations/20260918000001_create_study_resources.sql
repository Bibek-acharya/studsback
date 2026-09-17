-- Study resources (past questions, study notes, model questions, syllabus)
-- GORM AutoMigrate owns the actual schema; this file documents the table shape.

BEGIN;

CREATE TABLE IF NOT EXISTS study_resources (
    id              BIGSERIAL PRIMARY KEY,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    title           VARCHAR(255) NOT NULL,
    description     TEXT DEFAULT '',
    resource_type   VARCHAR(100) NOT NULL DEFAULT '',
    course          VARCHAR(255) DEFAULT '',
    year            VARCHAR(20) DEFAULT '',
    file_name       VARCHAR(255) NOT NULL,
    file_path       VARCHAR(512) NOT NULL,
    file_url        VARCHAR(512) NOT NULL,
    file_size       BIGINT NOT NULL DEFAULT 0,
    mime_type       VARCHAR(255) DEFAULT '',
    downloads       INT NOT NULL DEFAULT 0,
    uploaded_by     BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_study_resources_resource_type ON study_resources(resource_type);
CREATE INDEX IF NOT EXISTS idx_study_resources_course ON study_resources(course);
CREATE INDEX IF NOT EXISTS idx_study_resources_year ON study_resources(year);
CREATE INDEX IF NOT EXISTS idx_study_resources_deleted_at ON study_resources(deleted_at);

COMMIT;

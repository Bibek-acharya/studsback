package migrations

import (
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

// AddStudyResourceVideoColumns adds the video-lecture columns to
// study_resources and (re)creates the list-filter indexes. GORM AutoMigrate
// already owns this table via studyresources.StudyResource; this migration
// documents the shape for the SQL-migration path and guarantees the
// is_published backfill on databases that were created before the column
// existed.
//
// Everything here is additive and idempotent: no column is dropped, no
// resource row is rewritten except the NULL -> TRUE backfill, and unknown or
// legacy resource_type values are never touched.
func AddStudyResourceVideoColumns(db *gorm.DB) error {
	statements := []string{
		`ALTER TABLE study_resources ADD COLUMN IF NOT EXISTS is_published BOOLEAN NOT NULL DEFAULT TRUE`,
		`ALTER TABLE study_resources ADD COLUMN IF NOT EXISTS duration_seconds INT NOT NULL DEFAULT 0`,
		`ALTER TABLE study_resources ADD COLUMN IF NOT EXISTS views INT NOT NULL DEFAULT 0`,
		// Safety net for a column that somehow exists but is nullable: every
		// pre-existing resource must stay publicly visible.
		`UPDATE study_resources SET is_published = TRUE WHERE is_published IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_study_resources_published ON study_resources(is_published)`,
		`CREATE INDEX IF NOT EXISTS idx_study_resources_resource_type ON study_resources(resource_type)`,
		`CREATE INDEX IF NOT EXISTS idx_study_resources_course ON study_resources(course)`,
		`CREATE INDEX IF NOT EXISTS idx_study_resources_year ON study_resources(year)`,
		`CREATE INDEX IF NOT EXISTS idx_study_resources_deleted_at ON study_resources(deleted_at)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	logger.Info("Study resource video/publish columns migration completed")
	return nil
}

// CreateMockTestTables creates the mock-test graph tables. AutoMigrate owns
// them too; this mirrors the documented-shape pattern of the other table
// migrations so a fresh SQL-only bootstrap works.
func CreateMockTestTables(db *gorm.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS mock_tests (
			id                BIGSERIAL PRIMARY KEY,
			title             VARCHAR(255) NOT NULL,
			description       TEXT DEFAULT '',
			course            VARCHAR(255) DEFAULT '',
			year              VARCHAR(100) DEFAULT '',
			duration_minutes  INT NOT NULL DEFAULT 0,
			is_published      BOOLEAN NOT NULL DEFAULT TRUE,
			views             INT NOT NULL DEFAULT 0,
			attempts          INT NOT NULL DEFAULT 0,
			created_by        BIGINT NOT NULL DEFAULT 0,
			created_at        TIMESTAMPTZ,
			updated_at        TIMESTAMPTZ,
			deleted_at        TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS mock_questions (
			id                BIGSERIAL PRIMARY KEY,
			mock_test_id      BIGINT NOT NULL,
			question_text     TEXT NOT NULL,
			explanation       TEXT DEFAULT '',
			"order"           INT NOT NULL DEFAULT 0,
			correct_option_id BIGINT NOT NULL DEFAULT 0,
			created_at        TIMESTAMPTZ,
			updated_at        TIMESTAMPTZ,
			CONSTRAINT fk_mock_questions_test FOREIGN KEY (mock_test_id) REFERENCES mock_tests(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS mock_options (
			id               BIGSERIAL PRIMARY KEY,
			mock_question_id BIGINT NOT NULL,
			option_text      TEXT NOT NULL,
			"order"          INT NOT NULL DEFAULT 0,
			created_at       TIMESTAMPTZ,
			updated_at       TIMESTAMPTZ,
			CONSTRAINT fk_mock_options_question FOREIGN KEY (mock_question_id) REFERENCES mock_questions(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS mock_attempts (
			id                 BIGSERIAL PRIMARY KEY,
			mock_test_id       BIGINT NOT NULL,
			user_id            BIGINT NOT NULL,
			score              INT NOT NULL DEFAULT 0,
			total_questions    INT NOT NULL DEFAULT 0,
			answered_questions INT NOT NULL DEFAULT 0,
			result_json        TEXT DEFAULT '',
			created_at         TIMESTAMPTZ,
			updated_at         TIMESTAMPTZ,
			CONSTRAINT fk_mock_attempts_test FOREIGN KEY (mock_test_id) REFERENCES mock_tests(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mock_tests_published ON mock_tests(is_published)`,
		`CREATE INDEX IF NOT EXISTS idx_mock_tests_course ON mock_tests(course)`,
		`CREATE INDEX IF NOT EXISTS idx_mock_tests_year ON mock_tests(year)`,
		`CREATE INDEX IF NOT EXISTS idx_mock_tests_created_by ON mock_tests(created_by)`,
		`CREATE INDEX IF NOT EXISTS idx_mock_questions_test_id ON mock_questions(mock_test_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mock_questions_correct_option ON mock_questions(correct_option_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mock_options_question_id ON mock_options(mock_question_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mock_attempts_test ON mock_attempts(mock_test_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mock_attempts_user ON mock_attempts(user_id)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	logger.Info("Mock test tables migration completed")
	return nil
}

package migrations

import (
	"studsphere/backend/internal/shared/logger"

	"gorm.io/gorm"
)

var landingCourseFields = []string{
	"Management & Business",
	"Accounting & Finance",
	"Computer Science & Information Technology",
	"Engineering",
	"Science & Mathematics",
	"Medicine & Health Sciences",
	"Nursing",
	"Pharmacy",
	"Dentistry",
	"Ayurveda & Alternative Medicine",
	"Agriculture",
	"Veterinary & Animal Science",
	"Forestry & Environmental Studies",
	"Education & Teaching",
	"Humanities",
	"Social Sciences",
	"Law & Legal Studies",
	"Economics",
	"Hospitality & Hotel Management",
	"Travel & Tourism",
	"Architecture, Design & Planning",
	"Media & Communication",
	"Arts & Fine Arts",
	"Fashion & Textile",
	"Aviation",
	"Sports & Physical Education",
	"Library & Information Science",
	"Languages & Literature",
	"Public Administration & Governance",
	"Development Studies",
	"Disaster & Risk Management",
	"Maritime / Marine Studies",
	"Food & Nutrition",
	"Religious & Cultural Studies",
	"Security & Defence Studies",
	"Technical & Vocational",
	"Professional Studies",
	"Language & Test Preparation",
	"Skill & Short-Term Courses",
	"Other / Interdisciplinary",
}

func SeedLandingCourseFields(db *gorm.DB) error {
	for i, name := range landingCourseFields {
		var count int64
		if err := db.Table("landing_course_fields").Where("field_of_study = ?", name).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.Exec(
			`INSERT INTO landing_course_fields (field_of_study, display_order, is_active, created_at, updated_at) VALUES (?, ?, true, NOW(), NOW())`,
			name, i+1,
		).Error; err != nil {
			logger.Warn("Failed to seed landing course field", "field", name, "error", err)
		}
	}
	return nil
}

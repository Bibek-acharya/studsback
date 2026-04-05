package seeder

import "gorm.io/gorm"

// Seed runs all database seeders in the correct order.
func Seed(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if err := SeedSuperAdmin(db); err != nil {
		return err
	}
	if err := SeedUniversities(db); err != nil {
		return err
	}
	if err := SeedCourses(db); err != nil {
		return err
	}
	if err := SeedColleges(db); err != nil {
		return err
	}
	if err := SeedCollegeUniversityCourseMappings(db); err != nil {
		return err
	}
	if err := SeedExams(db); err != nil {
		return err
	}
	if err := SeedNews(db); err != nil {
		return err
	}
	if err := SeedEvents(db); err != nil {
		return err
	}
	if err := SeedForum(); err != nil {
		return err
	}
	return SeedScholarships(db)
}

package seeder

import (
	"time"

	"gorm.io/gorm"
)

func ClearAcademicData(db *gorm.DB) error {
	return db.Exec("DELETE FROM college_university_courses").Error
}

func SeedCollegeUniversityCourseMappings(db *gorm.DB) error {
	type row struct {
		CollegeID    uint
		UniversityID uint
		CourseID     uint
	}

	var rows []row
	err := db.Table("institution_programs ip").
		Select("iu.college_id, COALESCE(iu.university_id, 0) as university_id, ip.global_course_id as course_id").
		Joins("JOIN institution_users iu ON iu.id = ip.institution_id AND iu.deleted_at IS NULL").
		Where("ip.status = ? AND ip.deleted_at IS NULL AND iu.college_id > 0 AND ip.global_course_id > 0", "active").
		Group("iu.college_id, iu.university_id, ip.global_course_id").
		Scan(&rows).Error
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		return nil
	}

	now := time.Now()
	type mapping struct {
		CollegeID    uint
		UniversityID uint
		CourseID     uint
		CreatedAt    time.Time
		UpdatedAt    time.Time
	}

	batch := make([]mapping, len(rows))
	for i, r := range rows {
		batch[i] = mapping{
			CollegeID:    r.CollegeID,
			UniversityID: r.UniversityID,
			CourseID:     r.CourseID,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
	}

	return db.CreateInBatches(batch, 500).Error
}

package models

import (
	"time"

	"gorm.io/gorm"
)

type CollegeUniversityCourse struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	CollegeID    uint           `gorm:"not null;index;uniqueIndex:idx_college_uni_course" json:"college_id"`
	UniversityID uint           `gorm:"not null;index;uniqueIndex:idx_college_uni_course" json:"university_id"`
	CourseID     uint           `gorm:"not null;index;uniqueIndex:idx_college_uni_course" json:"course_id"`
	Status       string         `json:"status"`
	College      College        `gorm:"foreignKey:CollegeID" json:"-"`
	University   University     `gorm:"foreignKey:UniversityID" json:"-"`
	Course       Course         `gorm:"foreignKey:CourseID" json:"-"`
}

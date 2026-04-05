package counselling

import (
	"time"

	"gorm.io/gorm"
)

type CounsellingBooking struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	UserID           uint           `gorm:"index;not null" json:"user_id"`
	College          string         `gorm:"not null" json:"college"`
	ProgramLevel     string         `gorm:"not null" json:"program_level"`
	InterestedCourse string         `gorm:"not null" json:"interested_course"`
	SessionMode      string         `gorm:"type:varchar(20);not null;default:'in_person'" json:"session_mode"`
	SessionDate      string         `gorm:"not null" json:"session_date"`
	SessionTime      string         `gorm:"not null" json:"session_time"`
	StudentName      string         `gorm:"not null" json:"student_name"`
	StudentPhone     string         `gorm:"not null" json:"student_phone"`
	StudentEmail     string         `gorm:"not null" json:"student_email"`
	StudentNotes     string         `gorm:"type:text" json:"student_notes"`
	Status           string         `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
}

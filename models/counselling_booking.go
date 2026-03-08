package models

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
	User             User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
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

type CreateCounsellingBookingRequest struct {
	College          string `json:"college" binding:"required"`
	ProgramLevel     string `json:"program_level" binding:"required"`
	InterestedCourse string `json:"interested_course" binding:"required"`
	SessionMode      string `json:"session_mode" binding:"required"`
	SessionDate      string `json:"session_date" binding:"required"`
	SessionTime      string `json:"session_time" binding:"required"`
	StudentName      string `json:"student_name" binding:"required"`
	StudentPhone     string `json:"student_phone" binding:"required"`
	StudentEmail     string `json:"student_email" binding:"required,email"`
	StudentNotes     string `json:"student_notes"`
}

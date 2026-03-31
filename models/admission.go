package models

import (
	"time"

	"gorm.io/gorm"
)

type Admission struct {
	ID                uint           `gorm:"primarykey" json:"id"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
	UserID            *uint          `gorm:"index" json:"user_id,omitempty"`
	User              *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	CollegeID         uint           `gorm:"index;not null" json:"college_id"`
	College           College        `gorm:"foreignKey:CollegeID" json:"college,omitempty"`
	ProgramName       string         `gorm:"not null" json:"program_name"`
	ProgramLevel      string         `gorm:"not null" json:"program_level"`
	StudentName       string         `gorm:"not null" json:"student_name"`
	StudentEmail      string         `gorm:"not null" json:"student_email"`
	StudentPhone      string         `gorm:"not null" json:"student_phone"`
	DateOfBirth       *time.Time     `json:"date_of_birth,omitempty"`
	Gender            string         `json:"gender,omitempty"`
	Address           string         `json:"address,omitempty"`
	City              string         `json:"city,omitempty"`
	LastQualification string         `json:"last_qualification,omitempty"`
	Institution       string         `json:"institution,omitempty"`
	GPA               string         `json:"gpa,omitempty"`
	EntranceScore     string         `json:"entrance_score,omitempty"`
	Documents         []byte         `gorm:"type:jsonb" json:"documents,omitempty"`
	Statement         string         `gorm:"type:text" json:"statement,omitempty"`
	Status            string         `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	Notes             string         `gorm:"type:text" json:"notes,omitempty"`
	ReviewedBy        *uint          `json:"reviewed_by,omitempty"`
	ReviewedAt        *time.Time     `json:"reviewed_at,omitempty"`
}

type CreateAdmissionRequest struct {
	CollegeID         uint   `json:"college_id" binding:"required"`
	ProgramName       string `json:"program_name" binding:"required"`
	ProgramLevel      string `json:"program_level" binding:"required"`
	StudentName       string `json:"student_name" binding:"required"`
	StudentEmail      string `json:"student_email" binding:"required,email"`
	StudentPhone      string `json:"student_phone" binding:"required"`
	DateOfBirth       string `json:"date_of_birth"`
	Gender            string `json:"gender"`
	Address           string `json:"address"`
	City              string `json:"city"`
	LastQualification string `json:"last_qualification"`
	Institution       string `json:"institution"`
	GPA               string `json:"gpa"`
	EntranceScore     string `json:"entrance_score"`
	Statement         string `json:"statement"`
}

type UpdateAdmissionRequest struct {
	ProgramName       *string `json:"program_name"`
	ProgramLevel      *string `json:"program_level"`
	StudentName       *string `json:"student_name"`
	StudentEmail      *string `json:"student_email"`
	StudentPhone      *string `json:"student_phone"`
	DateOfBirth       *string `json:"date_of_birth"`
	Gender            *string `json:"gender"`
	Address           *string `json:"address"`
	City              *string `json:"city"`
	LastQualification *string `json:"last_qualification"`
	Institution       *string `json:"institution"`
	GPA               *string `json:"gpa"`
	EntranceScore     *string `json:"entrance_score"`
	Statement         *string `json:"statement"`
}

type UpdateAdmissionStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=pending under_review approved rejected waitlisted"`
	Notes  string `json:"notes"`
}

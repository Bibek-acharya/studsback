package jobs

import (
	"time"

	"gorm.io/gorm"
)

type Job struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	Title              string         `gorm:"type:varchar(255);not null" json:"title"`
	Department         string         `gorm:"type:varchar(100);not null" json:"department"`
	Description        string         `gorm:"type:text;not null" json:"description"`
	Requirements       string         `gorm:"type:text" json:"requirements"`
	Location           string         `gorm:"type:varchar(255)" json:"location"`
	JobType            string         `gorm:"type:varchar(50);not null" json:"job_type"`
	PositionsOpen      int            `gorm:"not null;default:1" json:"positions_open"`
	SalaryRange        string         `gorm:"type:varchar(100)" json:"salary_range"`
	ApplicationDeadline *time.Time    `json:"application_deadline,omitempty"`
	Status             string         `gorm:"type:varchar(20);not null;default:'draft'" json:"status"`
}

type JobApplication struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	JobID           uint           `gorm:"index;not null" json:"job_id"`
	FullName        string         `gorm:"type:varchar(255);not null" json:"full_name"`
	Email           string         `gorm:"type:varchar(255);not null" json:"email"`
	Phone           string         `gorm:"type:varchar(50);not null" json:"phone"`
	ResumeURL       string         `gorm:"type:varchar(500);not null" json:"resume_url"`
	CoverLetterURL  string         `gorm:"type:varchar(500)" json:"cover_letter_url"`
	Status          string         `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	Notes           string         `gorm:"type:text" json:"notes"`

	Job Job `gorm:"foreignKey:JobID" json:"job,omitempty"`
}

func (Job) TableName() string {
	return "jobs"
}

func (JobApplication) TableName() string {
	return "job_applications"
}

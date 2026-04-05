package admission

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
	CollegeID         uint           `gorm:"index;not null" json:"college_id"`
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

	College College `gorm:"foreignKey:CollegeID" json:"college,omitempty"`
	User    *User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

type College struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type User struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

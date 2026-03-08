package models

import (
	"time"

	"gorm.io/gorm"
)

type Exam struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	Slug         string         `gorm:"uniqueIndex" json:"slug"`
	Title        string         `gorm:"not null" json:"title" binding:"required"`
	Board        string         `json:"board"`
	Badges       []byte         `gorm:"type:jsonb" json:"badges"` // Array of strings
	Level        string         `json:"level"`
	Type         string         `json:"type"`
	ExamDate     string         `json:"examDate"`   // Display date
	ExamDateAD   time.Time      `json:"examDateAD"` // Actual date
	FormDeadline string         `json:"formDeadline"`
	Fee          string         `json:"fee"`
	Highlights   []byte         `gorm:"type:jsonb" json:"highlights"` // Array of strings
	Description  string         `gorm:"type:text" json:"description"`
	Status       string         `json:"status"` // ongoing, closed, upcoming
	ImageUrl     string         `json:"imageUrl"`
	University   string         `json:"university"`
	Faculty      string         `json:"faculty"`
	NepaliDate   string         `json:"nepaliDate"`
	Overview     string         `gorm:"type:text" json:"overview"`
	Weightage    []byte         `gorm:"type:jsonb" json:"weightage"` // JSON data
	Timeline     []byte         `gorm:"type:jsonb" json:"timeline"`  // JSON data
	Notices      []byte         `gorm:"type:jsonb" json:"notices"`   // JSON data
	Faqs         []byte         `gorm:"type:jsonb" json:"faqs"`      // JSON data
}

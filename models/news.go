package models

import (
	"time"

	"gorm.io/gorm"
)

type News struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Category  string         `json:"category"` // Academic, Tech, Jobs, Policy
	Title     string         `gorm:"not null" json:"title" binding:"required"`
	Excerpt   string         `gorm:"type:text" json:"excerpt"`
	Content   string         `gorm:"type:text" json:"content"`
	Image     string         `json:"image"`
	Author    string         `json:"author"`
	Date      string         `json:"date"`
	ReadTime  string         `json:"readTime"`
	Source    string         `json:"source"`
	Tags      []byte         `gorm:"type:jsonb" json:"tags"` // Array of strings
}

package models

import (
	"time"

	"gorm.io/gorm"
)

type Blog struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Title     string         `gorm:"not null" json:"title" binding:"required"`
	Slug      string         `gorm:"uniqueIndex" json:"slug"`
	Excerpt   string         `gorm:"type:text" json:"excerpt"`
	Content   string         `gorm:"type:text" json:"content"`
	Image     string         `json:"image"`
	Author    string         `json:"author"`
	Category  string         `json:"category"`
	Tags      []byte         `gorm:"type:jsonb" json:"tags"`
	ReadTime  string         `json:"read_time"`
	Featured  bool           `gorm:"default:false" json:"featured"`
	Published bool           `gorm:"default:true" json:"published"`
	Views     int            `gorm:"default:0" json:"views"`
}

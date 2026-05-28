package chat

import (
	"time"

	"gorm.io/gorm"
)

type SitePage struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Slug      string         `gorm:"uniqueIndex;not null" json:"slug"`
	Title     string         `gorm:"not null" json:"title"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Embedding []float32      `gorm:"-" json:"-"`
}

func (SitePage) TableName() string {
	return "site_pages"
}

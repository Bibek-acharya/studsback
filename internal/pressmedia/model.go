package pressmedia

import (
	"time"

	"gorm.io/gorm"
)

type PressMediaItem struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	Title       string         `gorm:"not null" json:"title"`
	Slug        string         `gorm:"uniqueIndex;not null" json:"slug"`
	Category    string         `gorm:"not null;default:''" json:"category"`
	Summary     string         `gorm:"default:''" json:"summary"`
	Content     string         `gorm:"default:''" json:"content"`
	ImageURL    string         `gorm:"default:''" json:"image_url"`
	FileURL     string         `gorm:"default:''" json:"file_url"`
	ExternalURL string         `gorm:"default:''" json:"external_url"`
	PublishedAt *time.Time     `json:"published_at"`
	IsPublished bool           `gorm:"not null;default:false" json:"is_published"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

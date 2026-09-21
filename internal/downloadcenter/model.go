package downloadcenter

import (
	"time"

	"gorm.io/gorm"
)

type DownloadItem struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	Title         string         `gorm:"not null" json:"title"`
	Description   string         `gorm:"default:''" json:"description"`
	Category      string         `gorm:"not null;default:''" json:"category"`
	FileURL       string         `gorm:"not null;default:''" json:"file_url"`
	FilePath      string         `gorm:"not null;default:''" json:"file_path"`
	FileName      string         `gorm:"not null;default:''" json:"file_name"`
	FileSize      int64          `gorm:"not null;default:0" json:"file_size"`
	MimeType      string         `gorm:"default:''" json:"mime_type"`
	DownloadCount int64          `gorm:"not null;default:0" json:"download_count"`
	PublishedAt   *time.Time     `json:"published_at"`
	IsPublished   bool           `gorm:"not null;default:false" json:"is_published"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

package studyresources

import (
	"time"

	"gorm.io/gorm"
)

type StudyResource struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	Title        string         `gorm:"not null" json:"title"`
	Description  string         `gorm:"default:''" json:"description"`
	ResourceType string         `gorm:"not null;default:''" json:"resource_type"`
	Course       string         `gorm:"default:''" json:"course"`
	Year         string         `gorm:"default:''" json:"year"`
	FileName     string         `gorm:"not null" json:"file_name"`
	FilePath     string         `gorm:"not null" json:"file_path"`
	FileURL      string         `gorm:"not null" json:"file_url"`
	FileSize     int64          `gorm:"not null;default:0" json:"file_size"`
	MimeType     string         `gorm:"default:''" json:"mime_type"`
	Downloads    int            `gorm:"not null;default:0" json:"downloads"`
	UploadedBy   uint           `gorm:"not null;default:0" json:"uploaded_by"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

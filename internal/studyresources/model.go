package studyresources

import (
	"time"

	"gorm.io/gorm"
)

type StudyResource struct {
	ID           uint   `gorm:"primarykey" json:"id"`
	Title        string `gorm:"not null" json:"title"`
	Description  string `gorm:"default:''" json:"description"`
	ResourceType string `gorm:"not null;default:'';index:idx_study_resources_resource_type" json:"resource_type"`
	Course       string `gorm:"default:'';index:idx_study_resources_course" json:"course"`
	Year         string `gorm:"default:'';index:idx_study_resources_year" json:"year"`
	FileName     string `gorm:"not null" json:"file_name"`
	FilePath     string `gorm:"not null" json:"file_path"`
	FileURL      string `gorm:"not null" json:"file_url"`
	FileSize     int64  `gorm:"not null;default:0" json:"file_size"`
	MimeType     string `gorm:"default:''" json:"mime_type"`
	Downloads    int    `gorm:"not null;default:0" json:"downloads"`
	// Views counts video-lecture stream requests. Documents keep using
	// Downloads; the counter is incremented by the stream endpoint only.
	Views int `gorm:"not null;default:0" json:"views"`
	// IsPublished defaults to TRUE so every pre-existing row stays publicly
	// visible after the column is added. Public list filters published rows
	// only; the admin list sees everything.
	IsPublished bool `gorm:"not null;default:true;index:idx_study_resources_published" json:"is_published"`
	// DurationSeconds is the optional video length used by the player
	// before metadata is loaded. Zero means "unknown".
	DurationSeconds int            `gorm:"not null;default:0" json:"duration_seconds"`
	UploadedBy      uint           `gorm:"not null;default:0" json:"uploaded_by"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

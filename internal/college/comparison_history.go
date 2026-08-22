package college

import (
	"time"

	"gorm.io/gorm"
)

type ComparisonHistory struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	College1ID      uint           `gorm:"not null;index" json:"college1_id"`
	College2ID      uint           `gorm:"not null;index" json:"college2_id"`
	College1Name    string         `gorm:"not null" json:"college1_name"`
	College2Name    string         `gorm:"not null" json:"college2_name"`
	ComparisonCount int            `gorm:"default:1;index" json:"comparison_count"`
}

func (ComparisonHistory) TableName() string {
	return "comparison_history"
}

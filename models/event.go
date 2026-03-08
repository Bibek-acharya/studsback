package models

import (
	"time"

	"gorm.io/gorm"
)

type Event struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	Title      string         `gorm:"not null" json:"title" binding:"required"`
	Date       string         `json:"date"`
	Location   string         `json:"location"`
	Image      string         `json:"image"`
	Interested int            `json:"interested"`
	Trending   bool           `json:"trending"`
}

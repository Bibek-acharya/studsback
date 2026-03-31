package models

import (
	"time"

	"gorm.io/gorm"
)

type ContactInquiry struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Name      string         `gorm:"not null" json:"name"`
	Email     string         `gorm:"not null" json:"email"`
	Phone     string         `json:"phone"`
	Subject   string         `json:"subject"`
	Message   string         `gorm:"type:text" json:"message"`
	Type      string         `gorm:"default:'general'" json:"type"`
	Status    string         `gorm:"default:'new'" json:"status"`
}

type Ad struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Title       string         `gorm:"not null" json:"title"`
	ImageURL    string         `json:"image_url"`
	LinkURL     string         `json:"link_url"`
	Page        string         `gorm:"index" json:"page"`
	Position    string         `json:"position"`
	StartDate   time.Time      `json:"start_date"`
	EndDate     time.Time      `json:"end_date"`
	Active      bool           `gorm:"default:true;index" json:"active"`
	Clicks      int            `gorm:"default:0" json:"clicks"`
	Impressions int            `gorm:"default:0" json:"impressions"`
	Priority    int            `gorm:"default:0" json:"priority"`
}

type CarouselSlide struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Page        string         `gorm:"index;default:'landing'" json:"page"`
	Title       string         `json:"title"`
	Subtitle    string         `json:"subtitle"`
	Description string         `gorm:"type:text" json:"description"`
	ImageURL    string         `json:"image_url"`
	LinkURL     string         `json:"link_url"`
	ButtonText  string         `json:"button_text"`
	Order       int            `gorm:"default:0;index" json:"order"`
	Active      bool           `gorm:"default:true;index" json:"active"`
}

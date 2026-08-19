package faq

import (
	"time"

	"gorm.io/gorm"
)

type FAQCategory struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Name        string         `gorm:"not null" json:"name"`
	Description string         `gorm:"default:''" json:"description"`
	Order       int            `gorm:"default:0" json:"order"`
	Items       []FAQItem      `gorm:"foreignKey:CategoryID" json:"items"`
}

type FAQItem struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	CategoryID uint           `gorm:"index;not null" json:"category_id"`
	Question   string         `gorm:"not null" json:"question"`
	Answer     string         `gorm:"type:text;not null" json:"answer"`
	Order      int            `gorm:"default:0" json:"order"`
}

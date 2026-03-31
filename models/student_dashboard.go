package models

import (
	"time"

	"gorm.io/gorm"
)

type Message struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	SenderID   uint           `gorm:"index" json:"sender_id"`
	ReceiverID uint           `gorm:"index" json:"receiver_id"`
	Subject    string         `json:"subject"`
	Content    string         `gorm:"type:text" json:"content"`
	Read       bool           `gorm:"default:false" json:"read"`
	Direction  string         `json:"direction"`
}

type CalendarEvent struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	UserID      uint           `gorm:"index" json:"user_id"`
	Title       string         `gorm:"not null" json:"title"`
	Description string         `gorm:"type:text" json:"description"`
	StartDate   time.Time      `json:"start_date"`
	EndDate     time.Time      `json:"end_date"`
	Location    string         `json:"location"`
	Link        string         `json:"link"`
	Color       string         `json:"color"`
	Reminder    bool           `gorm:"default:true" json:"reminder"`
}

type SphereInvite struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	UserID        uint           `gorm:"index" json:"user_id"`
	InstitutionID uint           `gorm:"index" json:"institution_id"`
	Title         string         `json:"title"`
	Message       string         `gorm:"type:text" json:"message"`
	Status        string         `gorm:"default:'pending'" json:"status"`
	Type          string         `json:"type"`
}

type Bookmark struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UserID    uint      `gorm:"index" json:"user_id"`
	ItemID    uint      `json:"item_id"`
	ItemType  string    `json:"type"`
}

type Notification struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"index" json:"user_id"`
	Title     string         `json:"title"`
	Message   string         `gorm:"type:text" json:"message"`
	Type      string         `json:"type"`
	Read      bool           `gorm:"default:false" json:"read"`
	Link      string         `json:"link"`
}
